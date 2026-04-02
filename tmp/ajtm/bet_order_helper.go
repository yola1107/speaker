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

	s.debug.originSymbolGrid = s.symbolGrid
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
	var winGrid int64Grid
	var eliGrid int64Grid
	var seenLongHead [_rowCount][_colCount]bool

	s.winMys = s.winMys[:0]

	for symbol := _blank + 1; symbol < _wild; symbol++ {
		info, ok := s.findSymbolWinInfo(symbol)
		if !ok {
			continue
		}

		winInfos = append(winInfos, *info)
		s.mergeWinGrids(info.WinGrid, &winGrid, &eliGrid, &seenLongHead)
	}

	s.winInfos = winInfos
	s.winGrid = winGrid
	s.eliGrid = eliGrid

	// 每个命中的长符号（神秘符号）累计 +2，连消结束后在 syncGameStage 中清理。
	if n := int64(len(s.winMys)); n > 0 {
		s.scene.MysMulTotal += n * _perSymMultiple
	}
}

func (s *betOrderService) mergeWinGrids(src int64Grid, winGrid, eliGrid *int64Grid, seenLongHead *[_rowCount][_colCount]bool) {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			cell := src[r][c]
			if cell <= 0 {
				continue
			}

			head := s.symbolGrid[r][c]
			isLongHead := r < _rowCount-1 && head > 0 && head < _longSymbol && s.symbolGrid[r+1][c] == _longSymbol+head
			if isLongHead {
				if !seenLongHead[r][c] {
					seenLongHead[r][c] = true
					s.winMys = append(s.winMys, Block{
						Col:       int64(c),
						HeadRow:   int64(r),
						TailRow:   int64(r + 1),
						OldSymbol: head,
					})
				}

				// 长符号需要展示中奖，但不进入实际消除网格。
				(*winGrid)[r][c] = head
				if r+1 < _rowCount {
					(*winGrid)[r+1][c] = s.symbolGrid[r+1][c]
				}
				continue
			}
			(*winGrid)[r][c] = cell
			(*eliGrid)[r][c] = cell
		}
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
	if len(s.winMys) == 0 {
		return
	}

	writeIdx := 0
	for i := 0; i < len(s.winMys); i++ {
		block := s.winMys[i]
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

		block.OldSymbol = oldSymbol
		block.NewSymbol = newSymbol
		s.winMys[writeIdx] = block
		writeIdx++
	}
	s.winMys = s.winMys[:writeIdx]
}

func randomLongTransformSymbol(oldSymbol int64) int64 {
	return 1 // TODO delete

	for {
		// 仅在 1~12 中随机，排除自身和夺宝。
		n := int64(rand.IntN(12) + 1)
		if n != oldSymbol && n != _treasure {
			return n
		}
	}
}

//func (s *betOrderService) refreshLongCountFromRoller() {
//	// 免费模式未中奖时，按当前滚轴盘面重算 MysCount，供下一局继承。
//	for c := 0; c < _colCount; c++ {
//		s.scene.MysCount[c] = 0
//	}
//	for c := 1; c < _colCount-1; c++ {
//		for r := 0; r < _rowCount-1; r++ {
//			head := s.scene.SymbolRoller[c].BoardSymbol[r]
//			if head > 0 && head < _longSymbol && s.scene.SymbolRoller[c].BoardSymbol[r+1] == _longSymbol+head {
//				s.scene.MysCount[c]++
//				r++
//			}
//		}
//	}
//}
