package xslm

import (
	"errors"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"
	"egame-grpc/model/slot"
	"egame-grpc/strategy"
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
	// Spin fields (extracted from spin struct)
	preset                  *slot.XSLM
	stepMap                 *stepMap
	femaleCountsForFree     [_femaleC - _femaleA + 1]int64
	nextFemaleCountsForFree [_femaleC - _femaleA + 1]int64
	enableFullElimination   bool
	isRoundOver             bool
	symbolGrid              *int64Grid
	winInfos                []*winInfo
	winResults              []*winResult
	winGrid                 *int64Grid
	hasFemaleWin            bool
	lineMultiplier          int64
	stepMultiplier          int64
	treasureCount           int64
	newFreeRoundCount       int64
	gameOrder               *game.GameOrder
	bonusAmount             decimal.Decimal
	currBalance             decimal.Decimal
}

func newBetOrderService() *betOrderService {
	return &betOrderService{}
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
	switch {
	case err != nil:
		global.GVA_LOG.Error("betOrder", zap.Error(err))
		return nil, InternalServerError
	case lastOrder == nil:
		s.saveScene(0, 0)
	}
	s.lastOrder = lastOrder
	s.selectGameRedis()
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
