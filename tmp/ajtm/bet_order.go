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
	gameConfig     *gameConfigJson      // 游戏配置
	gameOrder      *game.GameOrder      // 当前订单
	bonusAmount    decimal.Decimal      // 本步奖金
	betAmount      decimal.Decimal      // 本局下注金额
	amount         decimal.Decimal      // 本步扣款金额
	orderSn        *common.OrderSN      // 订单号
	isRoundOver    bool                 // 当前 round 是否结束
	isFreeRound    bool                 // 当前是否处于免费模式
	scatterCount   int64                // 当前盘面夺宝数量
	addFreeTime    int64                // 本步新增免费次数
	lineMultiplier int64                // 本步基础中奖倍数
	stepMultiplier int64                // 本步最终结算倍数
	winInfos       []WinInfo            // 本步中奖明细
	symbolGrid     int64Grid            // 当前盘面
	winGrid        int64Grid            // 中奖展示网格
	eliGrid        int64Grid            // 实际消除网格
	nextSymbolGrid int64Grid            // 消除并下落后的盘面
	winMys         []Block              // 中奖的长符号(神秘符号)（transform 后补齐 NewSymbol）
	mysMul         int64                // 长符号倍数
	extMul         int64                // 触发免费模式额外奖励倍数; 3 个夺宝是3倍投注金额的奖金，每多一个夺宝符号将额外获得 2 倍的投注金额
	debug          rtpDebugData
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
		Sn:                 proto.String(s.orderSn.OrderSN),
		Balance:            proto.Float64(s.gameOrder.CurBalance),
		BetAmount:          proto.Float64(s.betAmount.Round(2).InexactFloat64()),
		CurWin:             proto.Float64(s.bonusAmount.Round(2).InexactFloat64()),
		FreeTotalWin:       proto.Float64(s.client.ClientOfFreeGame.GetFreeTotalMoney()),
		TotalWin:           proto.Float64(s.client.ClientOfFreeGame.GetGeneralWinTotal()),
		IsFree:             proto.Bool(s.isFreeRound),
		Review:             proto.Int64(s.req.Review),
		WinInfo:            s.buildWinInfo(),
		Cards:              s.int64GridToArray(s.symbolGrid),
		ScatterCount:       proto.Int64(s.scatterCount),
		IsRoundOver:        proto.Bool(s.isRoundOver),
		State:              proto.Int64(int64(s.scene.Stage)),
		RemainingFreeTimes: proto.Int64(int64(s.client.ClientOfFreeGame.GetFreeNum())),
		TotalFreeTimes:     proto.Int64(int64(s.client.ClientOfFreeGame.GetFreeTimes())),
		StepMul:            proto.Int64(s.stepMultiplier),
		WinGrid:            s.int64GridToArray(s.winGrid),
		IsGameOver:         proto.Bool(s.isFreeRound && s.isRoundOver && s.scene.FreeNum <= 0),
		RoundWin:           proto.Float64(s.calcRoundWin()),
		MysMul:             proto.Int64(s.mysMul),
		WinMys:             s.buildWinMys(),
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
			RoadNum: proto.Int64(elem.LineCount),
			Odds:    proto.Int64(elem.Odds),
		}
	}
	return &pb.Ajtm_WinInfo{
		WinArr:     winArr,
		AddFreeNum: proto.Int64(s.addFreeTime),
	}
}

func (s *betOrderService) buildWinMys() []*pb.Ajtm_WinMys {
	events := make([]*pb.Ajtm_WinMys, len(s.winMys))
	for i, event := range s.winMys {
		events[i] = &pb.Ajtm_WinMys{
			Col:       proto.Int64(event.Col),
			HeadRow:   proto.Int64(event.HeadRow),
			TailRow:   proto.Int64(event.TailRow),
			OldSymbol: proto.Int64(event.OldSymbol),
			NewSymbol: proto.Int64(event.NewSymbol),
		}
	}
	return events
}

func (s *betOrderService) calcRoundWin() float64 {
	if s.scene.RoundMultiplier == 0 {
		return 0
	}
	return decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(s.scene.RoundMultiplier)).
		Round(2).InexactFloat64()
}

//func (s *betOrderService) calcRoundWin() float64 {
//	if s.scene.RoundMultiplier == 0 {
//		return 0
//	}
//	return s.betAmount.
//		Mul(decimal.NewFromInt(s.scene.RoundMultiplier)).
//		Div(decimal.NewFromInt(_baseMultiplier)).
//		Round(2).
//		InexactFloat64()
//}
