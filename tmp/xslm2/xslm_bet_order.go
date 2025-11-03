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
	req                *request.BetOrderReq
	merchant           *merchant.Merchant
	member             *member.Member
	game               *game.Game
	client             *client.Client
	lastOrder          *game.GameOrder
	gameRedis          *redis.Client
	isFirst            bool
	betAmount          decimal.Decimal
	amount             decimal.Decimal
	strategy           *strategy.Strategy
	gameType           int64
	orderSN            string
	parentOrderSN      string
	freeOrderSN        string
	isFreeRound        bool
	presetID           int64
	probMap            map[int64]game.GameDynamicProb
	probMultipliers    []int64
	probWeightSum      int64
	presetKind         int64
	expectedMultiplier int64
	presetMultiplier   int64
	scene              scene
	spin               spin
	gameOrder          *game.GameOrder
	bonusAmount        decimal.Decimal
	currBalance        decimal.Decimal
	debug              struct {
		open        bool
		delayMillis int64
	}
}

func newBetOrderService() *betOrderService {
	return &betOrderService{}
}

// betOrder 主下注逻辑
// 流程：验证 → 加载预设数据 → spin → 结算 → 返回结果
func (s *betOrderService) betOrder(req *request.BetOrderReq) (map[string]any, error) {
	s.req = req

	// 1. 获取请求上下文（商户、用户、游戏信息）
	if !s.getRequestContext() {
		return nil, InternalServerError
	}

	// 2. 获取客户端上下文
	c, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("betOrder", zap.Error(errors.New("user not exists")))
		return nil, fmt.Errorf("client not exist")
	}
	s.client = c

	// 3. 加锁保证幂等性
	c.BetLock.Lock()
	defer c.BetLock.Unlock()

	// 4. 获取上一单
	lastOrder, _, err := c.GetLastOrder()
	switch {
	case err != nil:
		global.GVA_LOG.Error("betOrder", zap.Error(err))
		return nil, InternalServerError
	case lastOrder == nil:
		s.saveScene(0, 0) // 首次下注，清空场景
	}
	s.lastOrder = lastOrder

	// 5. 选择游戏Redis
	s.selectGameRedis()

	// 6. 判断是否首次下注
	switch {
	case s.lastOrder == nil:
		s.isFirst = true
	case s.client.ClientOfFreeGame.GetLastMapId() == 0:
		s.isFirst = true
	}
	return s.doBetOrder()
}

func (s *betOrderService) doBetOrder() (map[string]any, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}
	switch {
	case !s.initPreset():
		return nil, InternalServerError
	case !s.backupScene():
		return nil, InternalServerError
	case !s.initStepMap():
		return nil, InternalServerError
	case !s.updateStepResult():
		s.restoreScene()
		return nil, InternalServerError
	case !s.updateGameOrder():
		s.restoreScene()
		return nil, InternalServerError
	case !s.settleStep():
		s.restoreScene()
		return nil, InternalServerError
	}
	s.finalize()
	return s.getBetResultMap(), nil
}

func (s *betOrderService) getBetResultMap() map[string]any {
	return map[string]any{
		"orderSN":                 s.gameOrder.OrderSn,
		"isFreeRound":             s.isFreeRound,
		"femaleCountsForFree":     s.spin.femaleCountsForFree,
		"enableFullElimination":   s.spin.enableFullElimination,
		"symbolGrid":              s.spin.symbolGrid,
		"winGrid":                 s.spin.winGrid,
		"winResults":              s.spin.winResults,
		"baseBet":                 s.req.BaseMoney,
		"multiplier":              s.req.Multiple,
		"betAmount":               s.betAmount.Round(2).InexactFloat64(),
		"bonusAmount":             s.bonusAmount.Round(2).InexactFloat64(),
		"spinBonusAmount":         s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		"freeBonusAmount":         s.client.ClientOfFreeGame.GetFreeTotalMoney(),
		"roundBonus":              s.client.ClientOfFreeGame.RoundBonus,
		"currentBalance":          s.gameOrder.CurBalance,
		"isRoundOver":             s.spin.isRoundOver,
		"hasFemaleWin":            s.spin.hasFemaleWin,
		"nextFemaleCountsForFree": s.spin.nextFemaleCountsForFree,
		"newFreeRoundCount":       s.spin.newFreeRoundCount,
		"totalFreeRoundCount":     s.client.GetLastMaxFreeNum(),
		"remainingFreeRoundCount": s.client.ClientOfFreeGame.GetFreeNum(),
		"lineMultiplier":          s.spin.lineMultiplier,
		"stepMultiplier":          s.spin.stepMultiplier,
	}
}
