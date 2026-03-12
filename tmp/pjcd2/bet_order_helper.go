package pjcd

import (
	"fmt"
	"strconv"
	"strings"

	"egame-grpc/game/common"
	"egame-grpc/gamelogic"
	"egame-grpc/global"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

func (s *betOrderService) getRequestContext() bool {
	mer, mem, ga, ok := common.GetRequestContext(s.req)
	if !ok {
		global.GVA_LOG.Error("getRequestContext error.")
		return false
	}
	s.merchant, s.member, s.game = mer, mem, ga
	return true
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

func (s *betOrderService) symbolGridToString(symbolGrid int64Grid) string {
	var b strings.Builder
	b.Grow(512)
	cellIndex := 0
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			b.WriteString(strconv.Itoa(cellIndex + 1))
			b.WriteString(":")
			b.WriteString(strconv.FormatInt(symbolGrid[r][c], 10))
			b.WriteString("; ")
			cellIndex++
		}
	}
	return b.String()
}

func (s *betOrderService) winGridToString(winGridW int64GridW) string {
	var b strings.Builder
	b.Grow(512)
	cellIndex := 0
	for r := 0; r < _rowCountReward; r++ {
		for c := 0; c < _colCount; c++ {
			b.WriteString(strconv.Itoa(cellIndex + 1))
			b.WriteString(":")
			b.WriteString(strconv.FormatInt(winGridW[r][c], 10))
			b.WriteString("; ")
			cellIndex++
		}
	}
	return b.String()
}

func (s *betOrderService) updateBonusAmount(stepMultiplier int64) {
	// RTP测试模式或无倍数时直接返回
	if s.debug.open || stepMultiplier == 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	bonusAmount := s.betAmount.
		Mul(decimal.NewFromInt(stepMultiplier)).
		Div(decimal.NewFromInt(_baseMultiplier))
	s.bonusAmount = bonusAmount

	if s.bonusAmount.GreaterThan(decimal.Zero) {
		rounded := bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(rounded)
		s.client.ClientOfFreeGame.IncRoundBonus(rounded)
		if s.isFreeRound {
			s.client.ClientOfFreeGame.IncrFreeTotalMoney(rounded)
		}
	}
}

func (s *betOrderService) getScatterCount() int64 {
	var count int64
	for r := 0; r < _rowCountReward; r++ {
		for c := 0; c < _colCount; c++ {
			if s.symbolGrid[r][c] == _treasure {
				count++
			}
		}
	}
	return count
}

func (s *betOrderService) handleSymbolGrid() {
	var symbolGrid int64Grid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			symbolGrid[_rowCount-1-r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
	s.symbolGrid = symbolGrid
}

// checkSymbolGridWin 检查符号网格中奖情况
func (s *betOrderService) checkSymbolGridWin() {
	symbolKinds := int(_wild - _blank - 1)
	winInfos := make([]WinInfo, 0, len(s.gameConfig.Lines)*symbolKinds)
	var totalWinGrid int64Grid
	var totalWinGridReward int64GridW

	for i, line := range s.gameConfig.Lines {
		for symbol := _blank + 1; symbol < _wild; symbol++ {
			var matchedCount int64
			var winGrid int64Grid
			var matchedPositions [_rowCountReward * _colCount]int64

			for _, p := range line {
				r := p / _colCount
				c := p % _colCount
				if r >= _rowCountReward {
					break
				}
				currSymbol := s.symbolGrid[r][c]
				if currSymbol == symbol || isWild(currSymbol) {
					winGrid[r][c] = currSymbol
					matchedPositions[matchedCount] = p
					matchedCount++
				} else {
					break
				}
			}

			if matchedCount >= _minMatchCount {
				odds := s.getSymbolBaseMultiplier(symbol, int(matchedCount))
				if odds > 0 {
					winInfos = append(winInfos, WinInfo{
						Symbol:      symbol,
						SymbolCount: matchedCount,
						LineCount:   int64(i),
						Odds:        odds,
						WinGrid:     winGrid,
					})
					for j := int64(0); j < matchedCount; j++ {
						p := matchedPositions[j]
						r := p / _colCount
						c := p % _colCount
						totalWinGrid[r][c] = 1
						// 只有奖励行才设置 reward 网格
						if r < _rowCountReward {
							totalWinGridReward[r][c] = 1
						}
					}
				}
			}
		}
	}

	s.winInfos = winInfos
	s.winGrid = totalWinGrid
	s.winGridReward = totalWinGridReward
}

func (s *betOrderService) calcWildForm() int64Grid {
	s.addWildEliCount = 0
	s.wildMultiplier = 0
	var wildForm int64Grid

	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.winGrid[r][c] > 0 && isWild(s.symbolGrid[r][c]) {
				wildForm[r][c] = s.symbolGrid[r][c] + _mask
				if isEmiWild(wildForm[r][c]) {
					s.addWildEliCount++
				}
			}
		}
	}
	s.scene.TotalWildEliCount += s.addWildEliCount
	return wildForm
}

func (s *betOrderService) moveSymbols(wildForm int64Grid) int64Grid {
	nextSymbolGrid := s.symbolGrid
	for r := 0; r < _rowCountReward; r++ {
		for c := 0; c < _colCount; c++ {
			if s.winGrid[r][c] > 0 {
				// 只有当存在wildForm且不是中奖的蝴蝶百搭时才保留，否则置0
				if wildForm[r][c] > 0 && !isEmiWild(wildForm[r][c]) {
					nextSymbolGrid[r][c] = wildForm[r][c]
				} else {
					nextSymbolGrid[r][c] = 0
				}
			}
		}
	}

	s.dropSymbols(&nextSymbolGrid)
	return nextSymbolGrid
}

// dropSymbols 符号下落
func (s *betOrderService) dropSymbols(grid *int64Grid) {
	for c := 0; c < _colCount; c++ {
		writePos := 0
		for r := 0; r < _rowCount; r++ {
			if val := (*grid)[r][c]; val != 0 {
				if r != writePos {
					(*grid)[writePos][c] = val
					(*grid)[r][c] = 0
				}
				writePos++
			}
		}
	}
}

func (s *betOrderService) fallingWinSymbols(nextSymbolGrid int64Grid) {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.scene.SymbolRoller[c].BoardSymbol[r] = nextSymbolGrid[_rowCount-1-r][c]
		}
	}
	for i := range s.scene.SymbolRoller {
		s.scene.SymbolRoller[i].ringSymbol(s.gameConfig)
	}
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
			isWin := rWinGrid[r][c] != 0
			if symbol == 0 {
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

// isWild 检查符号是否为wild符号（可替代、不可消除、下落时占位）
func isWild(symbol int64) bool {
	return symbol%_mask == _wild
}

// isEmiWild 是否是可消除百搭 > 蝴蝶百搭 (毛虫→蝶茧→蝴蝶)
func isEmiWild(symbol int64) bool {
	return symbol/_mask >= 3
}
