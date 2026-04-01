package ajtm

import (
	"fmt"
	"math/rand/v2"
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
	s.amount = s.betAmount

	if s.betAmount.LessThanOrEqual(decimal.Zero) || s.amount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v,%v]", s.req.BaseMoney, s.req.Multiple, s.req.Purchase)))
		return false
	}
	return true
}

func (s *betOrderService) checkBalance() bool {
	f, _ := s.amount.Float64()
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

func (s *betOrderService) int64GridToArray(grid int64Grid) []int64 {
	elements := make([]int64, _rowCount*_colCount)
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			elements[r*_colCount+c] = grid[r][c]
		}
	}
	return elements
}

func (s *betOrderService) updateBonusAmount(stepMultiplier int64) {
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
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.symbolGrid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
}

// moveSymbols 仅按 eliGrid 清除并下落。
// winGrid 只负责展示，不参与真实消除。
func (s *betOrderService) moveSymbols() int64Grid {
	next := s.symbolGrid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.eliGrid[r][c] > 0 {
				next[r][c] = 0
			}
		}
	}
	s.dropSymbols(&next)
	return next
}

func (s *betOrderService) dropSymbols(grid *int64Grid) {
	for c := 0; c < _colCount; c++ {
		isEdgeCol := c == 0 || c == _colCount-1
		writePos := _rowCount - 1
		if isEdgeCol {
			writePos = _rowCount - 2
		}
		for r := _rowCount - 1; r >= 0; r-- {
			if isEdgeCol && (r == 0 || r == _rowCount-1) {
				continue
			}
			if val := (*grid)[r][c]; val != 0 {
				if r != writePos {
					(*grid)[writePos][c] = val
					(*grid)[r][c] = 0
				}
				writePos--
			}
		}
	}
}

func (s *betOrderService) fallingWinSymbols(next int64Grid) {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.scene.SymbolRoller[c].BoardSymbol[r] = next[r][c]
		}
	}
	for i := range s.scene.SymbolRoller {
		s.scene.SymbolRoller[i].ringSymbol(s.gameConfig)
	}
}

func (s *betOrderService) findWinInfos() {
	winInfos := make([]WinInfo, 0, _wild-_blank-1)
	var displayGrid int64Grid
	var eliminateGrid int64Grid

	s.winLongBlocks = s.winLongBlocks[:0]
	seenLongHead := make(map[[2]int]struct{}, 8)

	for symbol := _blank + 1; symbol < _wild; symbol++ {
		info, ok := s.findSymbolWinInfo(symbol)
		if !ok {
			continue
		}

		winInfos = append(winInfos, *info)
		s.mergeWinGrids(info.WinGrid, &displayGrid, &eliminateGrid, seenLongHead)
	}

	if s.scene.MysMultiplierTotal > 0 {
		for i := range winInfos {
			winInfos[i].MysMultiplier = s.scene.MysMultiplierTotal
		}
	}

	s.winInfos = winInfos
	s.winGrid = displayGrid
	s.eliGrid = eliminateGrid
}

func (s *betOrderService) mergeWinGrids(src int64Grid, displayGrid, eliminateGrid *int64Grid, seenLongHead map[[2]int]struct{}) {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if src[r][c] <= 0 {
				continue
			}
			if s.isLongHeadAt(r, c) {
				s.recordWinningLongBlock(r, c, displayGrid, seenLongHead)
				continue
			}
			(*displayGrid)[r][c] = src[r][c]
			(*eliminateGrid)[r][c] = src[r][c]
		}
	}
}

func (s *betOrderService) recordWinningLongBlock(r, c int, displayGrid *int64Grid, seenLongHead map[[2]int]struct{}) {
	key := [2]int{r, c}
	if _, exists := seenLongHead[key]; !exists {
		seenLongHead[key] = struct{}{}
		s.winLongBlocks = append(s.winLongBlocks, Block{
			Col:       int64(c),
			HeadRow:   int64(r),
			TailRow:   int64(r + 1),
			OldSymbol: s.symbolGrid[r][c],
		})
	}

	// 长符号需要展示中奖，但不进入实际消除网格。
	(*displayGrid)[r][c] = s.symbolGrid[r][c]
	if r+1 < _rowCount {
		(*displayGrid)[r+1][c] = s.symbolGrid[r+1][c]
	}
}

// findSymbolWinInfo 按 Ways 从左到右判定，至少命中 3 列。
func (s *betOrderService) findSymbolWinInfo(symbol int64) (*WinInfo, bool) {
	lineCount := int64(1)
	var winGrid int64Grid
	hasRealSymbol := false

	for c := 0; c < _colCount; c++ {
		matchCount := 0
		for r := 0; r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || currSymbol == _wild {
				if currSymbol == symbol {
					hasRealSymbol = true
				}
				matchCount++
				winGrid[r][c] = currSymbol
			}
		}

		if matchCount == 0 {
			if c >= _minMatchCount && hasRealSymbol {
				if odds := s.getSymbolBaseMultiplier(symbol, c); odds > 0 {
					return &WinInfo{
						Symbol:      symbol,
						SymbolCount: int64(c),
						LineCount:   lineCount,
						Odds:        odds,
						Multiplier:  odds * lineCount,
						WinGrid:     winGrid,
					}, true
				}
			}
			return nil, false
		}

		lineCount *= int64(matchCount)
		if c == _colCount-1 && hasRealSymbol {
			if odds := s.getSymbolBaseMultiplier(symbol, _colCount); odds > 0 {
				return &WinInfo{
					Symbol:      symbol,
					SymbolCount: _colCount,
					LineCount:   lineCount,
					Odds:        odds,
					Multiplier:  odds * lineCount,
					WinGrid:     winGrid,
				}, true
			}
		}
	}
	return nil, false
}

func (s *betOrderService) isLongHeadAt(r, c int) bool {
	if r < 0 || r >= _rowCount-1 || c < 0 || c >= _colCount {
		return false
	}
	head := s.symbolGrid[r][c]
	if head <= 0 || head >= _longSymbol {
		return false
	}
	return s.symbolGrid[r+1][c] == _longSymbol+head
}

func (s *betOrderService) transformWinningLongSymbols() {
	s.longEvents = s.longEvents[:0]
	if len(s.winLongBlocks) == 0 {
		return
	}

	for _, block := range s.winLongBlocks {
		r, c := int(block.HeadRow), int(block.Col)
		if !s.isLongHeadAt(r, c) {
			continue
		}

		oldSymbol := s.symbolGrid[r][c]
		newSymbol := randomLongTransformSymbol(oldSymbol)

		// 长符号中奖后先转变，再参与后续盘面流转。
		s.symbolGrid[r][c] = newSymbol
		s.symbolGrid[r+1][c] = _longSymbol + newSymbol
		s.scene.SymbolRoller[c].BoardSymbol[r] = newSymbol
		s.scene.SymbolRoller[c].BoardSymbol[r+1] = _longSymbol + newSymbol

		s.longEvents = append(s.longEvents, Block{
			Col:       int64(c),
			HeadRow:   int64(r),
			TailRow:   int64(r + 1),
			OldSymbol: oldSymbol,
			NewSymbol: newSymbol,
		})
	}
}

func randomLongTransformSymbol(oldSymbol int64) int64 {
	for {
		// 仅在 1~12 中随机，排除自身和夺宝。
		n := int64(rand.IntN(12) + 1)
		if n != oldSymbol && n != _treasure {
			return n
		}
	}
}

func (s *betOrderService) refreshLongCountFromRoller() {
	// 免费模式未中奖时，按当前滚轴盘面重算 LongCount，供下一局继承。
	for c := 0; c < _colCount; c++ {
		s.scene.LongCount[c] = 0
	}
	for c := 1; c < _colCount-1; c++ {
		for r := 0; r < _rowCount-1; r++ {
			head := s.scene.SymbolRoller[c].BoardSymbol[r]
			if head > 0 && head < _longSymbol && s.scene.SymbolRoller[c].BoardSymbol[r+1] == _longSymbol+head {
				s.scene.LongCount[c]++
				r++
			}
		}
	}
}
