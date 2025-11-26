package xslm2

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

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
	s.betAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_baseMultiplier))

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
	if s.stepMultiplier <= 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	s.bonusAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(s.stepMultiplier))
}

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
func isBlockedCell(r, c int64) bool { return r == 0 && (c == 0 || c == _colCount-1) }

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

func GridToString(grid *int64Grid, winGrid *int64Grid) string {
	if grid == nil {
		return "(空)\n"
	}
	var buf strings.Builder
	rGrid := reverseGridRows(grid)
	rWinGrid := reverseGridRows(winGrid)
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbol := rGrid[r][c]
			isWin := rWinGrid[r][c] != _blank && rWinGrid[r][c] != _blocked
			if symbol == _blank {
				if isWin {
					buf.WriteString("   *|")
				} else {
					buf.WriteString("    |")
				}
			} else {
				if isWin {
					fmt.Fprintf(&buf, " %2d*|", symbol)
				} else {
					fmt.Fprintf(&buf, " %2d |", symbol)
				}
			}
			if c < _colCount-1 {
				buf.WriteString(" ")
			}
		}
		buf.WriteString("\n")
	}
	return buf.String()
}

// reverseGridRows 网格行序反转
func reverseGridRows(grid *int64Grid) int64Grid {
	if grid == nil {
		return int64Grid{}
	}
	var reversed int64Grid
	for i := int64(0); i < _rowCount; i++ {
		reversed[i] = grid[_rowCount-1-i]
	}
	return reversed
}
