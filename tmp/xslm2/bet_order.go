package xslm2

import (
	"errors"
	"fmt"

	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"
	"egame-grpc/strategy"

	"github.com/go-redis/redis/v8"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type betOrderService struct {
	req           *request.BetOrderReq
	merchant      *merchant.Merchant
	member        *member.Member
	game          *game.Game
	client        *client.Client
	lastOrder     *game.GameOrder
	gameRedis     *redis.Client
	isFirst       bool
	betAmount     decimal.Decimal
	amount        decimal.Decimal
	strategy      *strategy.Strategy
	gameType      int64
	orderSN       string
	parentOrderSN string
	freeOrderSN   string
	isFreeRound   bool
	gameOrder     *game.GameOrder
	bonusAmount   decimal.Decimal
	currBalance   decimal.Decimal
	gameConfig    *gameConfigJson
	scene         *SpinSceneData

	// 原 spin 结构体的字段
	// 持久化
	femaleCountsForFree     [_femaleC - _femaleA + 1]int64 // 按step 累计 step：理解为消除每一步
	nextFemaleCountsForFree [_femaleC - _femaleA + 1]int64 // 按step 累计
	nextSymbolGrid          *int64Grid

	// 本 step
	enableFullElimination bool
	isRoundOver           bool
	symbolGrid            *int64Grid
	winGrid               *int64Grid
	winInfos              []*winInfo
	winResults            []*winResult
	lineMultiplier        int64
	stepMultiplier        int64
	hasFemaleWin          bool
	treasureCount         int64
	newFreeRoundCount     int64
	hasFemaleWildWin      bool
	debug                 rtpDebugData // RTP压测调试
}

func newBetOrderService() *betOrderService {
	s := &betOrderService{}
	s.initGameConfigs()
	return s
}

func (s *betOrderService) betOrder(req *request.BetOrderReq) (map[string]any, error) {
	s.req = req
	if !s.getRequestContext() {
		return nil, InternalServerError
	}
	c, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("betOrder", zap.Error(errors.New("user not exists")))
		return nil, fmt.Errorf("client not exist")
	}
	s.client = c
	c.BetLock.Lock()
	defer c.BetLock.Unlock()

	lastOrder, _, err := c.GetLastOrder()
	if err != nil {
		return nil, InternalServerError
	}
	s.lastOrder = lastOrder

	if s.lastOrder == nil {
		s.cleanScene()
	}

	if err = s.reloadScene(); err != nil {
		return nil, err
	}
	return s.doBetOrder()
}

func (s *betOrderService) doBetOrder() (map[string]any, error) {
	// 重构逻辑
	if err := s.baseSpin(); err != nil {
		global.GVA_LOG.Error("betOrder", zap.Error(err))
		return nil, InternalServerError
	}

	// 更新订单
	if !s.updateGameOrder() {
		return nil, InternalServerError
	}

	// 结算
	if !s.settleStep() {
		return nil, InternalServerError
	}

	// 保存场景数据
	if err := s.saveScene(); err != nil {
		global.GVA_LOG.Error("doBetOrder.saveScene", zap.Error(err))
		return nil, InternalServerError

	}

	return s.getBetResultMap(), nil
}

func (s *betOrderService) getBetResultMap() map[string]any {
	return map[string]any{
		"orderSN":                 s.gameOrder.OrderSn,
		"isFreeRound":             s.isFreeRound,
		"femaleCountsForFree":     s.femaleCountsForFree,
		"enableFullElimination":   s.enableFullElimination,
		"symbolGrid":              s.symbolGrid,
		"winGrid":                 s.winGrid,
		"winResults":              s.winResults,
		"baseBet":                 s.req.BaseMoney,
		"multiplier":              s.req.Multiple,
		"betAmount":               s.betAmount.Round(2).InexactFloat64(),
		"bonusAmount":             s.bonusAmount.Round(2).InexactFloat64(),
		"spinBonusAmount":         s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		"freeBonusAmount":         s.client.ClientOfFreeGame.GetFreeTotalMoney(),
		"roundBonus":              s.client.ClientOfFreeGame.RoundBonus,
		"currentBalance":          s.gameOrder.CurBalance,
		"isRoundOver":             s.isRoundOver,
		"hasFemaleWin":            s.hasFemaleWin,
		"nextFemaleCountsForFree": s.nextFemaleCountsForFree,
		"newFreeRoundCount":       s.newFreeRoundCount,
		"totalFreeRoundCount":     s.client.GetLastMaxFreeNum(),
		"remainingFreeRoundCount": s.client.ClientOfFreeGame.GetFreeNum(),
		"lineMultiplier":          s.lineMultiplier,
		"stepMultiplier":          s.stepMultiplier,
	}
}
