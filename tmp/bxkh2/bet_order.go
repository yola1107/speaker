// game/bxkh2/bet_order.go
package bxkh2

import (
	"errors"
	"fmt"

	"egame-grpc/game/bxkh2/pb"
	"egame-grpc/game/common"
	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type betOrderService struct {
	req            *request.BetOrderReq
	merchant       *merchant.Merchant
	member         *member.Member
	game           *game.Game
	client         *client.Client
	lastOrder      *game.GameOrder
	scene          *SpinSceneData
	gameConfig     *gameConfigJson
	gameOrder      *game.GameOrder
	bonusAmount    decimal.Decimal
	betAmount      decimal.Decimal
	amount         decimal.Decimal
	orderSn        *common.OrderSN
	symbolGrid     int64Grid
	winGrid        int64Grid
	winInfos       []*winInfo
	isRoundFirst   bool
	stepMultiplier int64
	scatterCount   int64
	addFreeTime    int64
	freeMultiple   int64
	removeNum      int64
	debug          rtpDebugData
	// 长符号相关
	originalSymbolGrid int64Grid
	longWinGrid        int64Grid
	moveSymbolGrid     int64Grid
}

type rtpDebugData struct {
	open bool
}

func newBetOrderService() *betOrderService {
	s := &betOrderService{}
	s.initGameConfigs()
	return s
}

func (s *betOrderService) betOrder(req *request.BetOrderReq) ([]byte, string, error) {
	s.req = req

	// 获取上下文
	if err := s.getRequestContext(); err != nil {
		return nil, "", err
	}

	// 获取客户端
	c, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("betOrder", zap.Error(errors.New("user not exists")))
		return nil, "", fmt.Errorf("client not exist")
	}
	s.client = c
	c.BetLock.Lock()
	defer c.BetLock.Unlock()

	// 获取上一次订单
	lastOrder, _, err := c.GetLastOrder()
	if err != nil {
		return nil, "", InternalServerError
	}
	s.lastOrder = lastOrder

	// 首次下注清理场景
	if s.lastOrder == nil {
		s.isRoundFirst = true
		s.cleanScene()
	}
	// 找不到下注额，清除缓存
	if s.client.ClientOfFreeGame.GetBetAmount() <= 1e-9 {
		s.cleanScene()
	}

	// 加载场景数据
	if err = s.reloadScene(); err != nil {
		return nil, "", err
	}

	// 执行旋转
	if err = s.baseSpin(); err != nil {
		return nil, "", err
	}

	// 更新订单
	if err = s.updateGameOrder(); err != nil {
		return nil, "", err
	}

	// 结算
	if err = s.settleStep(); err != nil {
		return nil, "", err
	}

	// 保存场景
	if err = s.saveScene(); err != nil {
		return nil, "", err
	}

	return s.getBetResultMap()
}

func (s *betOrderService) getRequestContext() error {
	mer, mem, ga, err := common.GetRequestContext(s.req)
	if err != nil {
		global.GVA_LOG.Error("failed to get request context", zap.Error(err))
		return err
	}
	s.merchant, s.member, s.game = mer, mem, ga
	return nil
}

func (s *betOrderService) getBetResultMap() ([]byte, string, error) {
	orderSn := ""
	if s.orderSn != nil {
		orderSn = s.orderSn.OrderSN
	}
	result := &pb.Bxkh2_BetOrderResponse{
		OrderSn:       orderSn,
		Balance:       s.gameOrder.CurBalance,
		BetAmount:     s.betAmount.Round(2).InexactFloat64(),
		CurrentWin:    s.bonusAmount.Round(2).InexactFloat64(),
		RoundWin:      s.client.ClientOfFreeGame.RoundBonus,
		TotalWin:      s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		FreeTotalWin:  s.client.ClientOfFreeGame.GetFreeTotalMoney(),
		IsFree:        s.scene.IsFreeRound,
		Review:        s.req.Review,
		WinInfo:       s.buildWinInfo(),
		Cards:         s.int64GridToArray(s.symbolGrid),
		WinGrid:       s.int64GridToArray(s.winGrid),
		FreeMultiple:  s.freeMultiple,
		OriginalCards: s.int64GridToArray(s.originalSymbolGrid),
	}

	pbData, err := json.CJSON.Marshal(result)
	if err != nil {
		return nil, "", err
	}
	jsonData, err := json.CJSON.MarshalToString(result)
	if err != nil {
		return nil, "", err
	}
	return pbData, jsonData, nil
}

func (s *betOrderService) buildWinInfo() *pb.Bxkh2_WinInfo {
	winArr := make([]*pb.Bxkh2_WinArr, len(s.winInfos))
	for i, elem := range s.winInfos {
		winArr[i] = &pb.Bxkh2_WinArr{
			Val:     elem.Symbol,
			RoadNum: elem.LineCount,
			StarNum: elem.SymbolCount,
			Odds:    elem.Odds,
			Mul:     elem.Multiplier,
		}
	}
	return &pb.Bxkh2_WinInfo{
		Next:          !s.scene.RoundOver,
		Over:          s.scene.RoundOver,
		Multi:         s.scene.RoundMultiplier,
		State:         s.scene.Stage,
		FreeNum:       s.client.ClientOfFreeGame.GetFreeNum(),
		FreeTime:      s.client.ClientOfFreeGame.GetFreeTimes(),
		TotalFreeTime: s.client.ClientOfFreeGame.GetFreeNum() + s.client.ClientOfFreeGame.GetFreeTimes(),
		IsRoundOver:   s.scene.RoundOver,
		AddFreeTime:   s.addFreeTime,
		ScatterCount:  s.scatterCount,
		WinArr:        winArr,
	}
}

func (s *betOrderService) int64GridToArray(grid int64Grid) []int64 {
	elements := make([]int64, _rowCount*_colCount)
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			elements[r*_colCount+c] = grid[r][c]
		}
	}
	return elements
}
