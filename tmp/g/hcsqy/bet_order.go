package hcsqy

import (
	"errors"
	"fmt"

	"egame-grpc/game/hcsqy/pb"
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
	isRoundOver    bool                 // 回合是否结束
	isFreeRound    bool                 // 是否为免费回合
	next           bool                 // 是否需要继续请求（必赢重转）
	respinWildCol  int32                // 重转至赢模式长条百搭列(0-2)，每次重转重新随机
	wildExpandCol  int32                // 长条百搭变大列(0-2)
	scatterCount   int64                // 夺宝符个数
	addFreeTime    int64                // 增加的免费次数
	lineMultiplier int64                // 线赔率合计（未乘长条百搭），回包给前端
	wildMultiplier int64                // 长条百搭倍数
	stepMultiplier int64                // Step倍数
	winInfos       []WinInfo            // 中奖信息
	symbolGrid     int64Grid            // 符号网格（3行3列）
	winGrid        int64Grid            // 中奖网格（3行3列）
	debug          rtpDebugData         // 是否为RTP测试流程

	stepIsPurchase   bool // 当前步是否处于购买态（用于回包/日志口径）
	stepIsRespinMode bool // 当前步是否处于Respin态（用于回包/日志口径）
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

func gameOrderToResponse(gameOrder *game.GameOrder) (*pb.Hcsqy_BetOrderResponse, error) {
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
	return &pb.Hcsqy_BetOrderResponse{
		Sn:               &gameOrder.OrderSn,
		Balance:          &gameOrder.CurBalance,
		BaseBet:          &gameOrder.BaseAmount,
		Multiplier:       &gameOrder.Multiple,
		BetAmount:        &bet,
		CurWin:           &gameOrder.BonusAmount,
		FreeWin:          &winDetail.FreeWin,
		TotalWin:         &winDetail.TotalWin,
		IsRoundOver:      &winDetail.IsRoundOver,
		IsFree:           proto.Bool(gameOrder.IsFree == 1),
		FreeNum:          &gameOrder.FreeNum,
		FreeTime:         &gameOrder.FreeTimes,
		NewFreeTimes:     &winDetail.NewFreeTimes,
		IsPurchase:       &winDetail.IsPurchase,
		SymGrid:          int64GridToArray(symbolGrid),
		WinGrid:          int64GridToArray(winGrid),
		Next:             &winDetail.Next,
		IsRespinUntilWin: &winDetail.IsRespinUntilWin,
		RespinWildCol:    &winDetail.RespinWildCol,
		WildMultiplier:   &winDetail.WildMultiplier,
		LineMultiplier:   &winDetail.LineMultiplier,
		ScatterCount:     &winDetail.ScatterCount,
		WinArr:           winDetail.WinArr,
	}, nil
}
