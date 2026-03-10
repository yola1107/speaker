package pjcd

import (
	"errors"
	"fmt"

	"egame-grpc/game/common"
	"egame-grpc/game/common/pb"
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

// betOrderService 下注服务
type betOrderService struct {
	req             *request.BetOrderReq // 用户请求
	merchant        *merchant.Merchant   // 商户信息
	member          *member.Member       // 用户信息
	game            *game.Game           // 游戏信息
	client          *client.Client       // 用户上下文
	lastOrder       *game.GameOrder      // 用户上一个订单
	scene           *SpinSceneData       // 场景中间态数据
	gameConfig      *gameConfigJson      // 配置数据
	gameOrder       *game.GameOrder      // 订单
	bonusAmount     decimal.Decimal      // 奖金金额
	betAmount       decimal.Decimal      // spin下注金额
	amount          decimal.Decimal      // step扣费金额
	orderSn         *common.OrderSN      // 订单号
	parentOrderSN   string               // 父订单号
	freeOrderSN     string               // 触发免费的回合的父订单号
	isRoundOver     bool                 // 回合是否结束
	isFreeRound     bool                 // 是否为免费回合
	scatterCount    int64                // 夺宝符号个数
	addFreeTime     int64                // 增加的免费次数
	roundMultiplier int64                // 轮次倍数
	stepMultiplier  int64                // Step倍数
	lineMultiplier  int64                // 线倍数
	winInfos        []WinInfo            // 中奖信息
	symbolGrid      int64Grid            // 符号网格
	winGrid         int64Grid            // 中奖网格
	nextSymbolGrid  int64Grid            // 下一盘面（消除后）
	wildStates      WildStateGrid        // 百搭状态网格
	butterflyBonus  int64                // 蝴蝶百搭累加倍数
	debug           rtpDebugData         // RTP测试调试数据
}

func newBetOrderService() *betOrderService {
	s := &betOrderService{}
	s.initGameConfigs()
	return s
}

func (s *betOrderService) betOrder(req *request.BetOrderReq) ([]byte, string, error) {
	s.req = req
	if !s.getRequestContext() {
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
	result := &pb.Pjcd_BetOrderResponse{
		OrderSN:      s.orderSn.OrderSN,
		Balance:      s.gameOrder.CurBalance,
		BetAmount:    s.betAmount.Round(2).InexactFloat64(),
		CurrentWin:   s.bonusAmount.Round(2).InexactFloat64(),
		FreeWin:      s.client.ClientOfFreeGame.GetFreeTotalMoney(),
		TotalWin:     s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		Free:         s.isFreeRound,
		Review:       s.req.Review,
		WinInfo:      s.buildWinInfo(),
		Cards:        PjcdBoard(s.symbolGrid),
		ScatterCount: s.scatterCount,
	}
	return s.marshalData(result)
}

// buildWinInfo 构建中奖详情
func (s *betOrderService) buildWinInfo() *pb.Pjcd_WinInfo {
	state := int64(0)
	if s.isFreeRound {
		state = 1
	}

	winArr := make([]*pb.Pjcd_WinArr, len(s.winInfos))
	for i, w := range s.winInfos {
		winArr[i] = &pb.Pjcd_WinArr{
			Val:     w.Symbol,
			RoadNum: w.LineIndex,
			StarNum: w.SymbolCount,
			Odds:    w.Odds,
			Mul:     w.Multiplier,
			Grid:    PjcdBoard(w.WinGrid),
		}
	}

	roundMultipliers := s.gameConfig.BaseRoundMultipliers
	if s.isFreeRound {
		roundMultipliers = s.gameConfig.FreeRoundMultipliers
	}

	// 使用场景中的RoundIndex，不需要再计算
	roundIndex := s.scene.MultipleIndex

	return &pb.Pjcd_WinInfo{
		IsRoundOver:      s.isRoundOver,
		Multi:            s.stepMultiplier,
		State:            state,
		FreeNum:          int64(s.client.ClientOfFreeGame.GetFreeNum()),
		FreeTime:         int64(s.client.ClientOfFreeGame.GetFreeTimes()),
		WinArr:           winArr,
		WinGrid:          PjcdBoard(s.winGrid),
		IsGameOver:       s.isFreeRound && s.isRoundOver && s.scene.FreeNum <= 0,
		AddFreeNum:       s.addFreeTime,
		RoundIndex:       roundIndex,
		RoundMultipliers: roundMultipliers,
		ButterflyBonus:   s.butterflyBonus,
		WildStateGrid:    PjcdWildStateBoard(s.wildStates),
	}
}

func (s *betOrderService) marshalData(data proto.Message) ([]byte, string, error) {
	pbData, err := proto.Marshal(data)
	if err != nil {
		return nil, "", err
	}
	jsonData, err := json.CJSON.MarshalToString(data)
	if err != nil {
		return nil, "", err
	}
	return pbData, jsonData, nil
}
