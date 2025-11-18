package xslm

import (
	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"errors"
	"fmt"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"strconv"
)

func (s *betOrderService) getRequestContext() bool {
	switch {
	case !s.mdbGetMerchant():
		return false
	case !s.mdbGetMember():
		return false
	case !s.mdbGetGame():
		return false
	default:
		return true
	}
}

func (s *betOrderService) selectGameRedis() {
	index := _gameID % int64(len(global.GVA_GAME_REDIS))
	s.gameRedis = global.GVA_GAME_REDIS[index]
}

func (s *betOrderService) updateBetAmount() bool {
	betAmount := decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_baseMultiplier))
	s.betAmount = betAmount
	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
		return false
	}
	return true
}

func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

func (s *betOrderService) symbolGridToString() string {
	symbolStr := ""
	symbolSN := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			symbolStr += strconv.Itoa(symbolSN)
			symbolStr += ":"
			symbolStr += strconv.FormatInt(s.spin.symbolGrid[row][col], 10)
			symbolStr += "; "
			symbolSN++
		}
	}
	return symbolStr
}

func (s *betOrderService) winGridToString() string {
	if s.spin.winGrid == nil {
		return ""
	}
	winningStr := ""
	winningSN := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			winningStr += strconv.Itoa(winningSN)
			winningStr += ":"
			winningStr += strconv.FormatInt(s.spin.winGrid[row][col], 10)
			winningStr += "; "
			winningSN++
		}
	}
	return winningStr
}

func (s *betOrderService) updateBonusAmount() {
	bonusAmount := decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(s.spin.stepMultiplier))
	s.bonusAmount = bonusAmount
}

// Log operations

func (s *betOrderService) showPostUpdateErrorLog() {
	global.GVA_LOG.Error(
		"showPostUpdateErrorLog",
		zap.Error(errors.New("step state mismatch")),
		zap.Int64("id", s.member.ID),
		zap.Bool("isFreeRound", s.isFreeRound),
		zap.Uint64("lastWinID", s.client.ClientOfFreeGame.GetLastWinId()),
		zap.Uint64("lastMapID", s.client.ClientOfFreeGame.GetLastMapId()),
		zap.Uint64("freeNum", s.client.ClientOfFreeGame.GetFreeNum()),
		zap.Uint64("freeTimes", s.client.ClientOfFreeGame.GetFreeTimes()),
		zap.String("orderSn", s.orderSN),
		zap.String("parentOrderSN", s.parentOrderSN),
		zap.String("freeOrderSN", s.freeOrderSN),
	)
}
