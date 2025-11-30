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
	var b strings.Builder
	b.Grow(512)
	symbolSN := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			b.WriteString(strconv.Itoa(symbolSN))
			b.WriteByte(':')
			b.WriteString(strconv.FormatInt(s.symbolGrid[row][col], 10))
			b.WriteString("; ")
			symbolSN++
		}
	}
	return b.String()
}

func (s *betOrderService) winGridToString() string {
	if s.winGrid == nil {
		return ""
	}
	var b strings.Builder
	b.Grow(512)
	winningSN := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			b.WriteString(strconv.Itoa(winningSN))
			b.WriteByte(':')
			b.WriteString(strconv.FormatInt(s.winGrid[row][col], 10))
			b.WriteString("; ")
			winningSN++
		}
	}
	return b.String()
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

func (s *betOrderService) hasWildSymbol() bool {
	if s.symbolGrid == nil {
		return false
	}
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.symbolGrid[r][c] == _wild {
				return true
			}
		}
	}
	return false
}

func (s *betOrderService) getTreasureCount() int64 {
	if s.symbolGrid == nil {
		return 0
	}
	count := int64(0)
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.symbolGrid[r][c] == _treasure {
				count++
			}
		}
	}
	return count
}

func isBlockedCell(r, c int64) bool {
	return r == 0 && (c == 0 || c == _colCount-1)
}

func containsFemaleWild(grid int64Grid) bool {
	return containsSymbol(grid, func(symbol int64) bool {
		return symbol >= _wildFemaleA && symbol <= _wildFemaleC
	})
}

func containsFemale(grid int64Grid) bool {
	return containsSymbol(grid, func(symbol int64) bool {
		return symbol >= _femaleA && symbol <= _femaleC
	})
}

func containsBaseWild(grid int64Grid) bool {
	return containsSymbol(grid, func(symbol int64) bool {
		return symbol == _wild
	})
}

func containsSymbol(grid int64Grid, match func(symbol int64) bool) bool {
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
	buf.Grow(int(_rowCount * _colCount * 10))
	writeGridToBuilder(&buf, grid, winGrid)
	return buf.String()
}

func writeGridToBuilder(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
	if grid == nil {
		buf.WriteString("(空)\n")
		return
	}
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
					_, _ = fmt.Fprintf(buf, " %2d*|", symbol)
				} else {
					_, _ = fmt.Fprintf(buf, " %2d |", symbol)
				}
			}

			if c < _colCount-1 {
				buf.WriteString(" ")
			}
		}
		buf.WriteString("\n")
	}
}

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
