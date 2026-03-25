package jwjzy

import (
	"errors"
	"fmt"

	"egame-grpc/game/common"
	"egame-grpc/game/jwjzy/pb"
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
	req               *request.BetOrderReq // 用户请求
	merchant          *merchant.Merchant   // 商户信息
	member            *member.Member       // 用户信息
	game              *game.Game           // 游戏信息
	client            *client.Client       // 用户上下文
	lastOrder         *game.GameOrder      // 用户上一个订单
	scene             *SpinSceneData       // 场景中间态数据
	gameConfig        *gameConfigJson      // 配置数据
	gameOrder         *game.GameOrder      // 订单
	bonusAmount       decimal.Decimal      // 奖金金额
	betAmount         decimal.Decimal      // spin 下注金额
	amount            decimal.Decimal      // step 扣费金额
	orderSn           *common.OrderSN      // 订单号
	isRoundOver       bool                 // 回合是否结束
	isFreeRound       bool                 // 是否为免费回合
	scatterCount      int64                // 夺宝符个数
	addFreeTime       int64                // 增加的免费次数
	gameMultiple      int64                // 连续消除倍数，初始1倍（从 scene.ContinueNum）
	gameMultipleIndex int64                // 当前连击的倍数列表的索引 0开始
	lineMultiplier    int64                // 线倍数
	stepMultiplier    int64                // Step倍数
	winInfos          []WinInfo            // 中奖信息
	nextSymbolGrid    int64Grid            // 下一把 step 符号网格
	symbolGrid        int64Grid            // 符号网格（4行5列）
	goldFrameGrid     boolGrid             // 金色背景属性（仅用于 2~4 格长符号）
	longGrid          boolGrid             // 2~4 格长符号占位属性（中间四列使用）
	winGrid           int64Grid            // 中奖网格（4行5列，只包含参与中奖的行）
	winGridReward     int64Grid            // 奖励网格（历史字段：用于序列化/回包口径，后续可按策划规则调整或移除）
	debug             rtpDebugData         // 是否为RTP测试流程

	addWildEliCount int64 // 消除的百搭个数
	wildMultiplier  int64 // 消除的百搭贡献的倍数
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
	result := &pb.Jwjzy_BetOrderResponse{
		OrderSN:      s.orderSn.OrderSN,
		Balance:      s.gameOrder.CurBalance,
		BetAmount:    s.betAmount.Round(2).InexactFloat64(),
		CurrentWin:   s.bonusAmount.Round(2).InexactFloat64(),
		FreeWin:      s.client.ClientOfFreeGame.GetFreeTotalMoney(),
		TotalWin:     s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		Free:         s.isFreeRound,
		Review:       s.req.Review,
		WinInfo:      s.buildWinInfo(),
		Cards:        s.int64GridToPbBoard(s.symbolGrid),
		ScatterCount: s.scatterCount,
		IsRoundOver:  s.isRoundOver,
		Multi:        s.stepMultiplier,
		State: func() int64 {
			if s.isFreeRound {
				return 1
			}
			return 0
		}(),
		FreeNum:    s.scene.FreeNum,
		FreeTime:   int64(s.client.ClientOfFreeGame.GetFreeTimes()),
		WinGrid:    s.int64GridToPbBoard(s.winGrid),
		IsGameOver: s.isFreeRound && s.isRoundOver && s.scene.FreeNum <= 0,
		RoundWin:   s.calcRoundWin(),
		MulIndex:   s.gameMultipleIndex,
		BaseMultipliers: func() []int64 {
			return s.gameConfig.BaseRoundMultipliers
		}(),
		FreeMultipliers: func() []int64 {
			return s.gameConfig.FreeRoundMultipliers
		}(),
		WildEliCount:      s.addWildEliCount,
		TotalWildEliCount: s.scene.TotalWildEliCount,
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

// buildWinInfo 构建 WinInfo
func (s *betOrderService) buildWinInfo() *pb.Jwjzy_WinInfo {
	winArr := make([]*pb.Jwjzy_WinArr, len(s.winInfos))
	for i, elem := range s.winInfos {
		winArr[i] = &pb.Jwjzy_WinArr{
			RoadNum: elem.LineCount,
			Odds:    elem.Odds,
		}
	}
	return &pb.Jwjzy_WinInfo{
		WinArr:          winArr,
		AddFreeNum:      s.addFreeTime,
		WildEliMultiple: s.wildMultiplier,
	}
}

// int64GridToPbBoard 将 int64Grid 转为 common.Board，行优先 3行×5列
func (s *betOrderService) int64GridToPbBoard(grid int64Grid) []int64 {
	elements := make([]int64, _rowCount*_colCount)
	for row := 0; row < _rowCount; row++ {
		for col := 0; col < _colCount; col++ {
			elements[row*_colCount+col] = grid[row][col]
		}
	}
	return elements
}

func (s *betOrderService) calcRoundWin() float64 {
	if s.scene.RoundMultiplier == 0 {
		return 0
	}
	return s.betAmount.
		Mul(decimal.NewFromInt(s.scene.RoundMultiplier)).
		Div(decimal.NewFromInt(_baseMultiplier)).
		Round(2).InexactFloat64()
}
