package jwjzy

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

	// 购买免费：按“当前投注额 * buy_free_game_multiple”扣费
	s.amount = s.betAmount
	if s.req.Purchase > 0 {
		s.amount = s.betAmount.Mul(decimal.NewFromInt(_buyFreeGameMultiple))
	}

	if s.betAmount.LessThanOrEqual(decimal.Zero) || s.amount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v,%v]", s.req.BaseMoney, s.req.Multiple, s.req.Purchase)))
		return false
	}
	return true
}

func (s *betOrderService) checkPurchase() bool {
	// 没购买则直接通过
	if s.req.Purchase <= 0 {
		return true
	}
	// 校验扣费金额是否等于 betAmount * buy_free_game_multiple
	expected := s.betAmount.Mul(decimal.NewFromInt(_buyFreeGameMultiple)).Round(0).IntPart()
	return expected == s.req.Purchase
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

func (s *betOrderService) winGridToString(winGridW int64Grid) string {
	var b strings.Builder
	b.Grow(512)
	cellIndex := 0
	for r := 0; r < _rowCount; r++ {
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
	for r := 0; r < _rowCount; r++ {
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
	var goldGrid boolGrid
	var longGrid boolGrid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			symbolGrid[_rowCount-1-r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
			goldGrid[_rowCount-1-r][c] = s.scene.SymbolRoller[c].BoardGold[r]
			longGrid[_rowCount-1-r][c] = s.scene.SymbolRoller[c].BoardLong[r]
		}
	}
	s.symbolGrid = symbolGrid
	s.goldFrameGrid = goldGrid
	s.longGrid = longGrid
}

// checkSymbolGridWin 检查符号网格中奖情况（WayGame + 左到右连续匹配）
// 这里先实现“判奖”与“回包 routeNum/odds”的基础口径；消除连锁与金色框->百搭的连锁逻辑后续会继续接上。
func (s *betOrderService) checkSymbolGridWin() {
	// candidates: 本手可能参与判奖的基础符号集合（不包含 wild/treasure）
	hasWildFirstCol := false
	for r := 0; r < _rowCount; r++ {
		if s.symbolGrid[r][0] == _wild {
			hasWildFirstCol = true
			break
		}
	}

	candidates := make([]int64, 0, 12)
	if hasWildFirstCol {
		seen := make(map[int64]struct{}, 12)
		for r := 0; r < _rowCount; r++ {
			for c := 0; c < _colCount; c++ {
				sym := s.symbolGrid[r][c]
				if sym == 0 || sym == _wild || sym == _treasure {
					continue
				}
				if _, ok := seen[sym]; !ok {
					seen[sym] = struct{}{}
					candidates = append(candidates, sym)
				}
			}
		}
	} else {
		// 文档符号集合：1~10 + 12（不含 11=雪茄非必要，也不含 14=夺宝、13=百搭）
		candidates = []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12}
	}

	winInfos := make([]WinInfo, 0, 8)
	var totalWinGrid int64Grid

	for _, symbol := range candidates {
		info, ok := s.findWaySymbolWinInfo(symbol)
		if !ok {
			continue
		}
		winInfos = append(winInfos, *info)
		// 合并本步消除需要的中奖位置
		for r := 0; r < _rowCount; r++ {
			for c := 0; c < _colCount; c++ {
				if info.WinGrid[r][c] != 0 {
					// winGrid 只做“命中位置标记”（1/0），具体符号由 symbolGrid 决定。
					totalWinGrid[r][c] = 1
				}
			}
		}
	}

	s.winInfos = winInfos
	s.winGrid = totalWinGrid
}

// findWaySymbolWinInfo 查找符号在 WayGame 下的中奖信息（从左到右连续匹配）
func (s *betOrderService) findWaySymbolWinInfo(symbol int64) (*WinInfo, bool) {
	hasRealSymbol := false
	lineCount := int64(1)
	var winGrid int64Grid

	// 如果首列已经有 wild，则允许“全 wild”口径；与 hbtr2 保持一致
	// 但仍需 matchCount != 0 且满足最小连续列数。
	hasWildFirstCol := false
	for r := 0; r < _rowCount; r++ {
		if s.symbolGrid[r][0] == _wild {
			hasWildFirstCol = true
			break
		}
	}
	if hasWildFirstCol {
		hasRealSymbol = true
	}

	for c := 0; c < _colCount; c++ {
		matchCount := int64(0)
		for r := 0; r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || isWild(currSymbol) {
				if currSymbol == symbol {
					hasRealSymbol = true
				}
				matchCount++
				winGrid[r][c] = currSymbol
			}
		}

		// 当前列没有匹配符号：如果之前连续列数>=3，则形成中奖
		if matchCount == 0 {
			if c >= _minMatchCount && hasRealSymbol {
				starN := int64(c) // 连续匹配列数
				odds := s.getSymbolBaseMultiplier(symbol, int(starN))
				if odds > 0 {
					return &WinInfo{
						Symbol:      symbol,
						SymbolCount: starN,
						LineCount:   lineCount, // routeNum
						Odds:        odds,
						WinGrid:     winGrid,
					}, true
				}
			}
			return nil, false
		}

		lineCount *= matchCount

		// 最后一列仍匹配：连续匹配列数=6
		if c == _colCount-1 && hasRealSymbol {
			odds := s.getSymbolBaseMultiplier(symbol, _colCount)
			if odds > 0 {
				return &WinInfo{
					Symbol:      symbol,
					SymbolCount: _colCount,
					LineCount:   lineCount,
					Odds:        odds,
					WinGrid:     winGrid,
				}, true
			}
		}
	}

	return nil, false
}

func (s *betOrderService) calcWildForm() int64Grid {
	s.addWildEliCount = 0
	s.wildMultiplier = 0
	var wildForm int64Grid

	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
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
	for r := 0; r < _rowCount; r++ {
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

	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
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
	for i := 0; i < _rowCount; i++ {
		reversed[i] = grid[_rowCount-1-i]
	}
	return reversed
}

// isWild 检查符号是否为wild符号（可替代、不可消除、下落时占位）
func isWild(symbol int64) bool {
	return symbol == _wild
}

// isEmiWild 是否是可消除百搭 > 蝴蝶百搭 (毛虫→蝶茧→蝴蝶)
func isEmiWild(symbol int64) bool {
	// 当前实现以策划“金色框消除后变百搭”为主，原 pjcd 的形态进化逻辑暂不适用。
	// 后续连锁实现重写后可移除本函数。
	return false
}
