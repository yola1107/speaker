package ajtm

import (
	"errors"
	"fmt"

	"egame-grpc/game/ajtm/pb"
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
	"google.golang.org/protobuf/proto"
)

type betOrderService struct {
	req            *request.BetOrderReq // 用户请求
	merchant       *merchant.Merchant   // 商户信息
	member         *member.Member       // 用户信息
	game           *game.Game           // 游戏信息
	client         *client.Client       // 用户上下文
	lastOrder      *game.GameOrder      // 用户上一个订单
	scene          *SpinSceneData       // 场景中间态数据
	gameConfig     *gameConfigJson      // 配置数据
	gameOrder      *game.GameOrder      // 订单
	bonusAmount    decimal.Decimal      // 奖金金额
	betAmount      decimal.Decimal      // spin 下注金额
	amount         decimal.Decimal      // step 扣费金额
	orderSn        *common.OrderSN      // 订单号
	isRoundOver    bool                 // 回合是否结束
	isFreeRound    bool                 // 是否为免费回合
	scatterCount   int64                // 夺宝符个数
	addFreeTime    int64                // 增加的免费次数
	lineMultiplier int64                // 线赔率合计
	stepMultiplier int64                // Step倍数
	winInfos       []WinInfo            // 中奖信息
	symbolGrid     int64Grid            // 符号网格
	winGrid        int64Grid            // 中奖网格
	nextSymbolGrid int64Grid            // 消除+下落后的下一盘面
	debug          rtpDebugData         // RTP测试流程

	long longMatrix // 长符号矩阵
}

func newBetOrderService() *betOrderService {
	s := &betOrderService{}
	s.initGameConfigs()
	return s
}

func (s *betOrderService) betOrder(req *request.BetOrderReq) ([]byte, string, error) {
	s.req = req
	if err := s.getRequestContext(); err != nil {
		return nil, "", InternalServerError
	}
	c, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("betOrder", zap.Error(errors.New("user not exists")))
		return nil, "", fmt.Errorf("client not exist")
	}
	s.client = c
	c.BetLock.Lock()
	defer c.BetLock.Unlock()

	lastOrder, _, err := c.GetLastOrder()
	if err != nil {
		return nil, "", InternalServerError
	}
	s.lastOrder = lastOrder

	if s.lastOrder == nil {
		s.cleanScene()
	}

	if err = s.reloadScene(); err != nil {
		global.GVA_LOG.Error("betOrder: reloadScene failed", zap.Error(err))
		return nil, "", err
	}
	if err = s.baseSpin(); err != nil {
		return nil, "", err
	}
	if err = s.updateGameOrder(); err != nil {
		return nil, "", err
	}
	if err = s.settleStep(); err != nil {
		return nil, "", err
	}
	if err = s.saveScene(); err != nil {
		return nil, "", err
	}

	return s.getBetResultMap()
}

func (s *betOrderService) getBetResultMap() ([]byte, string, error) {
	result := &pb.Ajtm_BetOrderResponse{
		OrderSN:      s.orderSn.OrderSN,
		Balance:      s.gameOrder.CurBalance,
		BetAmount:    s.betAmount.Round(2).InexactFloat64(),
		CurrentWin:   s.bonusAmount.Round(2).InexactFloat64(),
		FreeWin:      s.client.ClientOfFreeGame.GetFreeTotalMoney(),
		TotalWin:     s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		Free:         s.isFreeRound,
		Review:       s.req.Review,
		WinInfo:      s.buildWinInfo(),
		Cards:        s.int64GridToArray(s.symbolGrid),
		ScatterCount: s.scatterCount,
		IsRoundOver:  s.isRoundOver,
		State:        int64(s.scene.Stage),
		FreeNum:      int64(s.client.ClientOfFreeGame.GetFreeNum()),
		FreeTime:     int64(s.client.ClientOfFreeGame.GetFreeTimes()),
		WinGrid:      s.int64GridToArray(s.winGrid),
		IsGameOver:   s.isFreeRound && s.isRoundOver && s.scene.FreeNum <= 0,
		RoundWin:     s.calcRoundWin(),
	}
	pbData, err := proto.Marshal(result)
	if err != nil {
		return nil, "", err
	}
	jsonData, err := json.CJSON.MarshalToString(result)
	if err != nil {
		return nil, "", err
	}
	return pbData, jsonData, nil
}

func (s *betOrderService) buildWinInfo() *pb.Ajtm_WinInfo {
	winArr := make([]*pb.Ajtm_WinArr, len(s.winInfos))
	for i, elem := range s.winInfos {
		winArr[i] = &pb.Ajtm_WinArr{
			RoadNum: elem.LineCount,
			Odds:    elem.Odds,
		}
	}
	return &pb.Ajtm_WinInfo{
		WinArr:     winArr,
		AddFreeNum: s.addFreeTime,
	}
}

func (s *betOrderService) calcRoundWin() float64 {
	if s.stepMultiplier == 0 {
		return 0
	}
	return s.bonusAmount.Round(2).InexactFloat64()
}
