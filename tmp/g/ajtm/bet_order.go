package ajtm

import (
	"errors"
	"fmt"

	"egame-grpc/game/ajtm/pb"
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
	isRoundOver    bool                 // 当前 round 是否结束
	isFreeRound    bool                 // 当前是否处于免费模式
	limit          bool                 // 是否触发最大可赢封顶
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
	c, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("betOrder", zap.Error(errors.New("user not exists")))
		return nil, "", fmt.Errorf("client not exist")
	}
	s.client = c
	c.BetLock.Lock()
	defer c.BetLock.Unlock()
	if err := s.getRequestContext(); err != nil {
		return nil, "", InternalServerError
	}

	lastOrder, _, err := c.GetLastOrder()
	if err != nil {
		return nil, "", InternalServerError
	}
	s.lastOrder = lastOrder
	if s.lastOrder == nil {
		s.cleanScene()
	}

	if err = s.reloadScene(); err != nil {
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
	result, err := gameOrderToResponse(s.gameOrder)
	if err != nil {
		return nil, "", err
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

func gameOrderToResponse(gameOrder *game.GameOrder) (*pb.Ajtm_BetOrderResponse, error) {
	winDetail := WinDetails{}
	if err := json.CJSON.UnmarshalFromString(gameOrder.WinDetails, &winDetail); err != nil {
		return nil, err
	}
	symbolGrid := int64Grid{}
	if err := json.CJSON.UnmarshalFromString(gameOrder.BetRawDetail, &symbolGrid); err != nil {
		return nil, err
	}
	winGrid := int64Grid{}
	if err := json.CJSON.UnmarshalFromString(gameOrder.BonusRawDetail, &winGrid); err != nil {
		return nil, err
	}
	bet := gameOrder.BaseAmount * float64(gameOrder.BaseMultiple*gameOrder.Multiple)
	stepMul := gameOrder.LineMultiple
	return &pb.Ajtm_BetOrderResponse{
		Sn:           &gameOrder.OrderSn,
		Balance:      &gameOrder.CurBalance,
		BetAmount:    &bet,
		CurWin:       &gameOrder.BonusAmount,
		FreeWin:      &winDetail.FreeWin,
		RoundWin:     &winDetail.RoundWin,
		IsRoundOver:  &winDetail.IsRoundOver,
		IsFree:       proto.Bool(gameOrder.IsFree == 1),
		State:        &winDetail.State,
		FreeNum:      &gameOrder.FreeNum,
		FreeTime:     &gameOrder.FreeTimes,
		NewFreeTimes: &winDetail.NewFreeTimes,
		SymGrid:      int64GridToArray(symbolGrid),
		WinGrid:      int64GridToArray(winGrid),
		WinArr:       winDetail.WinArr,
		WinMys:       winDetail.WinMys,
		StepMul:      &stepMul,
		MysMul:       &winDetail.MysMul,
		Limit:        &winDetail.Limit,
		ScatterCount: &gameOrder.HuNum,
	}, nil
}
