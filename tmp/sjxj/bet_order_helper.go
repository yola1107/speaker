package sjxj

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

func (s *betOrderService) getRequestContext() error {
	mer, mem, ga, err := common.GetRequestContext(s.req)
	if err != nil {
		global.GVA_LOG.Error("getRequestContext error.")
		return err
	}
	s.merchant, s.member, s.game = mer, mem, ga
	return nil
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
	startRow := _rowCountReward //_rowCount - _rowCountReward
	if s.isFreeRound {
		startRow = _rowCount - s.scene.UnlockedRows
	}
	for r := startRow; r < _rowCount; r++ {
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
			symbolGrid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
	s.symbolGrid = symbolGrid
}

// checkSymbolGridWin 检查符号网格中奖情况
func (s *betOrderService) checkSymbolGridWin() {
	var winInfos []WinInfo
	var totalWinGrid int64Grid

	for i, line := range s.gameConfig.Lines {
		// 优化：基础符号只由该条线的最左格（第1列）决定。
		// 若最左格不是 Wild，则只有该符号能在左到右匹配中成立；
		// 若最左格是 Wild，为保持与原实现一致，需要保留1..9全枚举。
		firstP := line[0]
		firstR := firstP / _colCount
		firstC := firstP % _colCount
		firstSymbol := s.symbolGrid[firstR][firstC]

		var (
			symbolCandidates [9]int64
			candCount        int
		)

		if firstSymbol == _wild {
			for symbol := _blank + 1; symbol < _wild; symbol++ {
				symbolCandidates[candCount] = symbol
				candCount++
			}
		} else if firstSymbol >= _blank+1 && firstSymbol < _wild {
			symbolCandidates[0] = firstSymbol
			candCount = 1
		} else {
			// firstSymbol 为空/Scatter 等不可能触发线奖，直接跳过本线
			continue
		}

		for idx := 0; idx < candCount; idx++ {
			symbol := symbolCandidates[idx]
			var count int64
			var winGrid int64Grid

			for _, p := range line {
				r := p / _colCount
				c := p % _colCount
				currSymbol := s.symbolGrid[r][c]
				if currSymbol == symbol || currSymbol == _wild {
					winGrid[r][c] = currSymbol
					count++
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
					})
					for r := 0; r < _rowCount; r++ {
						for c := 0; c < _colCount; c++ {
							if winGrid[r][c] > 0 {
								totalWinGrid[r][c] = 1
							}
						}
					}
				}
			}
		}
	}

	s.winInfos = winInfos
	s.winGrid = totalWinGrid
}

func writeGridToBuilder(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
	if grid == nil {
		buf.WriteString("(空)\n")
		return
	}
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			symbol := (*grid)[r][c]
			isWin := winGrid != nil && (*winGrid)[r][c] != 0
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
		if r == _rowCountReward-1 {
			buf.WriteString("--------------------------------\n")
		}
	}
}

func btoi(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
