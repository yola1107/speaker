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
	var winInfos []WinInfo
	var totalWinGrid int64Grid        // 完整4行格式（内部使用，保留完整信息）
	var totalWinGridReward int64GridW // 奖励3行格式（返回客户端）

	//var wildForm int64Grid // 记录参与中奖的百搭形态

	for i, line := range s.gameConfig.Lines {
		for symbol := _blank + 1; symbol < _wild; symbol++ {
			var count int64
			var winGrid int64Grid
			//var wildForm int64 // 记录参与中奖的百搭形态

			for _, p := range line {
				r := p / _colCount
				c := p % _colCount
				if r >= _rowCountReward {
					break
				}
				currSymbol := s.symbolGrid[r][c]
				//if currSymbol == symbol || currSymbol == _wild {
				if currSymbol == symbol || isWild(currSymbol) {
					winGrid[r][c] = currSymbol
					count++
					//// 记录百搭形态（如果有）
					//if currSymbol == _wild && s.scene.WildStateGrid[r][c] > 0 {
					//	wildForm = s.scene.WildStateGrid[r][c]
					//}
				} else {
					break
				}
			}

			if count >= _minMatchCount {
				odds := s.getSymbolBaseMultiplier(symbol, int(count))
				if odds > 0 {
					// 直接创建最终格式，避免后续转换
					winInfos = append(winInfos, WinInfo{
						Symbol:      symbol,
						SymbolCount: count,
						LineCount:   int64(i),
						Odds:        odds,
						WinGrid:     winGrid, // 保留完整4行信息
						//WildForm:    wildForm, // 记录百搭形态

						//Multiplier: odds,
					})
					// 同时更新完整4行和奖励3行两种格式
					for r := int64(0); r < _rowCount; r++ {
						for c := int64(0); c < _colCount; c++ {
							if winGrid[r][c] > 0 {
								totalWinGrid[r][c] = 1 // 完整4行
								if r < _rowCountReward {
									totalWinGridReward[r][c] = 1 // 前3行
								}
							}
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

func (s *betOrderService) moveSymbols() int64Grid {
	nextSymbolGrid := s.symbolGrid
	//var nextWildGrid int64Grid

	for r := 0; r < _rowCountReward; r++ {
		for c := 0; c < _colCount; c++ {
			//// SCATTER 不参与消除
			//if s.symbolGrid[r][c] == _treasure {
			//	continue
			//}
			//// 保留百搭状态（非蝴蝶）
			//if s.scene.WildStateGrid[r][c] > 0 && s.scene.WildStateGrid[r][c] != _wildFormButterfly {
			//	continue
			//}
			// 中奖位置消除
			if sym := s.winGrid[r][c]; sym > 0 {
				if !isWild(sym) {
					nextSymbolGrid[r][c] = 0 // 消除
					continue
				}
				// 蝴蝶参与中奖：累加蝴蝶百搭个数，然后消除
				if isEmiWild(sym) {
					s.scene.ButterflyCount++
					nextSymbolGrid[r][c] = 0 // 消除
					continue
				}
				// 保留百搭状态（非蝴蝶），同时状态递增
				nextSymbolGrid[r][c] += _wildMask
				continue
			}
		}
	}

	// 保留百搭状态
	//s.preserveWildStates(&nextSymbolGrid, &nextWildGrid)
	s.dropSymbols(&nextSymbolGrid)
	//s.scene.WildStateGrid = nextWildGrid

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

/*
	func GridToString(grid *int64Grid, winGrid *int64Grid) string {
		if grid == nil {
			return "(空)\n"
		}
		var buf strings.Builder
		buf.Grow(512)
		writeGridToBuilder(&buf, grid, winGrid)
		return buf.String()
	}
*/

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
	return symbol%_wildMask == _wild
}

// isEmiWild 是否是可消除百搭 > 蝴蝶百搭 (毛虫→蝶茧→蝴蝶)
func isEmiWild(symbol int64) bool {
	return symbol/_wildMask >= 3
}
