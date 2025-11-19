package xslm

import (
	"errors"
	"fmt"
	"strconv"

	"egame-grpc/gamelogic"
	"egame-grpc/global"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
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
			symbolStr += strconv.FormatInt(s.symbolGrid[row][col], 10)
			symbolStr += "; "
			symbolSN++
		}
	}
	return symbolStr
}

func (s *betOrderService) winGridToString() string {
	if s.winGrid == nil {
		return ""
	}
	winningStr := ""
	winningSN := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			winningStr += strconv.Itoa(winningSN)
			winningStr += ":"
			winningStr += strconv.FormatInt(s.winGrid[row][col], 10)
			winningStr += "; "
			winningSN++
		}
	}
	return winningStr
}

func (s *betOrderService) updateBonusAmount() {
	bonusAmount := decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(s.stepMultiplier))
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

// ___________________________________________________________________________
// getTreasureCount 获取符号网格中的夺宝符号数量
func getTreasureCount(grid *int64Grid) int64 {
	if grid == nil {
		return 0
	}
	count := int64(0)
	for _, row := range grid {
		for _, symbol := range row {
			if symbol == _treasure {
				count++
			}
		}
	}
	return count
}

func isBlockedCell(r, c int64) bool { return r == 0 && (c == 0 || c == _colCount-1) }

func isMatchingFemaleWild(target, curr int64) bool {
	if curr < _wildFemaleA || curr > _wildFemaleC {
		return false
	}
	return target >= (_blank+1) && target <= _femaleC
}

func infoHasFemaleWild(grid int64Grid) bool {
	return infoHas(grid, func(symbol int64) bool { return symbol >= _wildFemaleA && symbol <= _wildFemaleC })
}

func infoHasFemale(grid int64Grid) bool {
	return infoHas(grid, func(symbol int64) bool { return symbol >= _femaleA && symbol <= _femaleC })
}

func infoHasBaseWild(grid int64Grid) bool {
	return infoHas(grid, func(symbol int64) bool { return symbol == _wild })
}

func infoHas(grid int64Grid, match func(symbol int64) bool) bool {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if match(grid[r][c]) {
				return true
			}
		}
	}
	return false
}
