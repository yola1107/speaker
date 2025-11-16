package xslm2

import (
	"fmt"

	"egame-grpc/global"
)

type spin struct {
	// 持久化
	femaleCountsForFree     [3]int64
	nextFemaleCountsForFree [3]int64
	rollerKey               string
	rollers                 [_colCount]SymbolRoller
	nextSymbolGrid          *int64Grid

	// 本 step
	symbolGrid            *int64Grid
	winGrid               *int64Grid
	winInfos              []*winInfo
	winResults            []*winResult
	stepMultiplier        int64
	hasFemaleWin          bool
	hasFemaleWildWin      bool
	enableFullElimination bool
	isRoundOver           bool
	newFreeRoundCount     int64

	// 免费局夺宝累计（按步增量累加，避免净差值漏计）
	freeTreasuresGained int64
	lastTreasureCount   int64
}

type cascadeMode int

const (
	cascadeModeNone cascadeMode = iota
	cascadeModeBase
	cascadeModeFreePartial
	cascadeModeFreeFull
)

func (s *spin) desc() string { return ToJSON(s) }

func (s *spin) baseSpin(isFree, isFirst bool, nextGrid *int64Grid, rollers *[_colCount]SymbolRoller) {
	s.loadStepData(isFree, isFirst, nextGrid, rollers)
	s.findWinInfos()
	s.processStep(isFree)
	s.finalizeRound(isFree)
}

func (s *spin) loadStepData(isFree, isFirst bool, nextGrid *int64Grid, rollers *[_colCount]SymbolRoller) {
	if isFirst {
		s.initSpin(isFree)
		s.nextFemaleCountsForFree = s.femaleCountsForFree
	} else if nextGrid == nil || rollers == nil {
		global.GVA_LOG.Sugar().Errorf("免费模式下，空nextGrid/rollers：%s", s.desc())
		s.initSpin(isFree)
		s.nextFemaleCountsForFree = s.femaleCountsForFree
	} else {
		// 优先使用 nextGrid（向后兼容）
		s.symbolGrid = nextGrid
		s.rollers = *rollers
		// 验证从 BoardSymbol 恢复的网格与 nextGrid 是否一致
		if !verifyGridConsistencyWithLog(*rollers, nextGrid) {
			panic("BoardSymbol 恢复的网格与 nextGrid 不一致")
			//global.GVA_LOG.Sugar().Warnf("网格不一致：从 BoardSymbol 恢复的网格与 nextGrid 不匹配，已更新 BoardSymbol")
			//// 如果 nextGrid 存在但不一致，使用 nextGrid（向后兼容）
			//// 但应该更新 BoardSymbol 以保持一致
			//fallingWinSymbols(&s.rollers, *nextGrid)
		}
	}

	var currTreasure int64
	if isFree {
		convertFemaleToWild(s.symbolGrid, s.femaleCountsForFree)
		currTreasure = getTreasureCount(s.symbolGrid)
		if isFirst {
			s.freeTreasuresGained = 0
		} else if diff := currTreasure - s.lastTreasureCount; diff > 0 {
			s.freeTreasuresGained += diff
		}
		s.lastTreasureCount = currTreasure
	} else {
		currTreasure = getTreasureCount(s.symbolGrid)
		s.freeTreasuresGained, s.lastTreasureCount = 0, 0
	}

	s.enableFullElimination = isFree &&
		s.femaleCountsForFree[0] >= _femaleSymbolCountForFullElimination &&
		s.femaleCountsForFree[1] >= _femaleSymbolCountForFullElimination &&
		s.femaleCountsForFree[2] >= _femaleSymbolCountForFullElimination
}

func (s *spin) initSpin(isFree bool) {
	grid, rls, key := _cnf.initSpinSymbol(isFree, s.femaleCountsForFree)
	clearBlockedCells(&grid)
	s.symbolGrid, s.rollers, s.rollerKey = &grid, rls, key
}

func (s *spin) findWinInfos() bool {
	s.hasFemaleWin = false
	s.hasFemaleWildWin = false
	var wins []*winInfo
	for sym := _blank + 1; sym < _wildFemaleA; sym++ {
		if info, ok := s.findNormalWin(sym); ok {
			if sym >= _femaleA {
				s.hasFemaleWin = true
			}
			if infoHasFemaleWild(info.WinGrid) {
				s.hasFemaleWildWin = true
			}
			wins = append(wins, info)
		}
	}
	for sym := _wildFemaleA; sym < _wild; sym++ {
		if info, ok := s.findWildWin(sym); ok {
			//s.hasFemaleWin= true
			s.hasFemaleWildWin = true
			wins = append(wins, info)
		}
	}
	s.winInfos = wins
	return len(wins) > 0
}

func (s *spin) findNormalWin(sym int64) (*winInfo, bool) {
	lineCnt := int64(1)
	var wg int64Grid
	exist := false
	for c := int64(0); c < _colCount; c++ {
		cnt := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			curr := s.symbolGrid[r][c]
			if curr == sym || curr == _wild || isMatchingFemaleWild(sym, curr) {
				if curr == sym {
					exist = true
				}
				cnt++
				wg[r][c] = curr
			}
		}
		if cnt == 0 {
			if c >= _minMatchCount && exist {
				return &winInfo{Symbol: sym, SymbolCount: c, LineCount: lineCnt, WinGrid: wg}, true
			}
			break
		}
		lineCnt *= cnt
		if c == _colCount-1 && exist {
			return &winInfo{Symbol: sym, SymbolCount: _colCount, LineCount: lineCnt, WinGrid: wg}, true
		}
	}
	return nil, false
}

func (s *spin) findWildWin(sym int64) (*winInfo, bool) {
	lineCnt := int64(1)
	var wg int64Grid
	for c := int64(0); c < _colCount; c++ {
		cnt := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			curr := s.symbolGrid[r][c]
			if curr == sym || curr == _wild {
				cnt++
				wg[r][c] = curr
			}
		}
		if cnt == 0 {
			if c >= _minMatchCount {
				return &winInfo{Symbol: sym, SymbolCount: c, LineCount: lineCnt, WinGrid: wg}, true
			}
			break
		}
		lineCnt *= cnt
		if c == _colCount-1 {
			return &winInfo{Symbol: sym, SymbolCount: _colCount, LineCount: lineCnt, WinGrid: wg}, true
		}
	}
	return nil, false
}

func (s *spin) processStep(isFree bool) {
	s.updateStepResults()
	if len(s.winResults) == 0 || s.stepMultiplier == 0 || s.winGrid == nil {
		s.finishRound(isFree)
		return
	}

	mode, nextGrid := s.execEliminateGrid(isFree)
	if mode == cascadeModeNone || nextGrid == nil {
		s.finishRound(isFree)
		return
	}
	s.nextSymbolGrid = nextGrid
	s.femaleCountsForFree = s.nextFemaleCountsForFree
	s.isRoundOver = false
	// 将处理后的网格写回 SymbolRoller（实际状态恢复依赖此）
	fallingWinSymbols(&s.rollers, *nextGrid)
}

func (s *spin) updateStepResults() {
	var winResults []*winResult
	var winGrid int64Grid
	var lineMultiplier int64

	for _, info := range s.winInfos {
		base := _cnf.getSymbolMultiplier(info.Symbol, info.SymbolCount)
		if base == 0 {
			continue
		}
		total := base * info.LineCount
		winResults = append(winResults, &winResult{
			Symbol:             info.Symbol,
			SymbolCount:        info.SymbolCount,
			LineCount:          info.LineCount,
			BaseLineMultiplier: base,
			TotalMultiplier:    total,
			WinGrid:            info.WinGrid,
		})
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if info.WinGrid[r][c] != _blank {
					winGrid[r][c] = info.WinGrid[r][c]
				}
			}
		}
		lineMultiplier += total
	}
	s.stepMultiplier, s.winResults = lineMultiplier, winResults
	if lineMultiplier > 0 {
		s.winGrid = &winGrid
	} else {
		s.winGrid = nil
	}
}

func (s *spin) finishRound(isFree bool) {
	s.isRoundOver = true
	s.nextSymbolGrid = nil
	s.femaleCountsForFree = s.nextFemaleCountsForFree
}

func (s *spin) finalizeRound(isFree bool) {
	if !s.isRoundOver {
		s.newFreeRoundCount = 0
		return
	}

	treasureCount := int64(0)
	if s.symbolGrid != nil {
		treasureCount = getTreasureCount(s.symbolGrid)
	}
	if isFree {
		if s.freeTreasuresGained < 0 {
			s.freeTreasuresGained = 0
		}
		s.newFreeRoundCount = s.freeTreasuresGained
		s.freeTreasuresGained = 0
		s.lastTreasureCount = 0
	} else {
		s.newFreeRoundCount = _cnf.getFreeRoundCount(treasureCount)
	}
}

func (s *spin) dropSymbols(grid *int64Grid) {
	for c := int64(0); c < _colCount; c++ {
		writePos := int64(0)
		if c == 0 || c == _colCount-1 {
			writePos = 1
		}

		for r := int64(0); r < _rowCount; r++ {
			if isBlockedCell(r, c) {
				continue
			}
			switch val := (*grid)[r][c]; val {
			case _eliminated:
				(*grid)[r][c] = _blank
			case _blank:
				continue
			default:
				if r != writePos {
					(*grid)[writePos][c] = val
					(*grid)[r][c] = _blank
				}
				writePos++
			}
		}
	}
}

func (s *spin) fillBlanks(grid *int64Grid) int {
	filled := 0
	for c := int64(0); c < _colCount; c++ {
		for r := int64(0); r < _rowCount; r++ {
			if !isBlockedCell(r, c) && (*grid)[r][c] == _blank {
				(*grid)[r][c] = s.rollers[c].getFallSymbol()
				filled++
			}
		}
	}
	return filled
}

func convertFemaleToWild(grid *int64Grid, counts [3]int64) {
	if grid == nil {
		return
	}
	for idx := int64(0); idx < 3; idx++ {
		if counts[idx] < _femaleSymbolCountForFullElimination {
			continue
		}
		normal, wild := _femaleA+idx, _wildFemaleA+idx
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if !isBlockedCell(r, c) && (*grid)[r][c] == normal {
					(*grid)[r][c] = wild
				}
			}
		}
	}
}

func clearBlockedCells(grid *int64Grid) {
	if grid != nil {
		(*grid)[0][0], (*grid)[0][_colCount-1] = _blocked, _blocked
	}
}

func isBlockedCell(r, c int64) bool { return r == 0 && (c == 0 || c == _colCount-1) }

func isMatchingFemaleWild(target, curr int64) bool {
	if curr < _wildFemaleA || curr > _wildFemaleC {
		return false
	}
	return target >= (_blank+1) && target <= _femaleC
}

func infoHasFemaleWild(grid int64Grid) bool {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if grid[r][c] >= _wildFemaleA && grid[r][c] <= _wildFemaleC {
				return true
			}
		}
	}
	return false
}

func infoHasFemale(grid int64Grid) bool {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if grid[r][c] >= _femaleA && grid[r][c] <= _femaleC {
				return true
			}
		}
	}
	return false
}

func infoHasBaseWild(grid int64Grid) bool {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if grid[r][c] == _wild {
				return true
			}
		}
	}
	return false
}

// ----------------------------------------

//
// v2
/*
算分：女性百搭（10，11，12）可替换为基础符号（1，2，3，4，5，6，7，8，9），但连线上必须要有基础符号

消除：
	基础模式：消除中奖的女性符号（7，8，9）及百搭，如果盘面有夺宝则百搭不消除
	免费模式：
		1> 全屏情况：每个中奖Way找女性百搭，找到则改way除百搭13之外的符号都全部消除
		2> 非全屏情况：每个中奖way找女性，找到该way女性及女性百搭都消除
*/

// execEliminateGrid 计算并执行消除网格
func (s *spin) execEliminateGrid(isFree bool) (cascadeMode, *int64Grid) {
	if s.symbolGrid == nil || s.winGrid == nil {
		return cascadeModeNone, nil
	}

	nextGrid := *s.symbolGrid
	clearBlockedCells(&nextGrid)

	var cnt int
	var mode cascadeMode

	switch {
	case !isFree && s.hasFemaleWin:
		mode, cnt = cascadeModeBase, s.fillElimBase(&nextGrid)
	case isFree && s.enableFullElimination && s.hasFemaleWildWin:
		mode, cnt = cascadeModeFreeFull, s.fillElimFreeFull(&nextGrid)
	case isFree && (!s.enableFullElimination) && s.hasFemaleWin:
		mode, cnt = cascadeModeFreePartial, s.fillElimFreePartial(&nextGrid)
	}

	if cnt <= 0 {
		return cascadeModeNone, nil
	}

	s.dropSymbols(&nextGrid)
	filled := s.fillBlanks(&nextGrid)
	if filled != cnt { // 填充数量与消除数量应该相等
		panic(fmt.Sprintf("cascade fill mismatch: eliminated=%d filled=%d", cnt, filled))
	}
	clearBlockedCells(&nextGrid)
	if isFree && (mode == cascadeModeFreePartial || mode == cascadeModeFreeFull) {
		convertFemaleToWild(&nextGrid, s.femaleCountsForFree)
	}

	return mode, &nextGrid
}

func (s *spin) fillElimBase(grid *int64Grid) int {
	count := 0
	hasTreasure := getTreasureCount(s.symbolGrid) > 0
	for _, w := range s.winInfos {
		if w == nil || w.Symbol < _femaleA || w.Symbol > _femaleC {
			continue
		}
		if !infoHasBaseWild(w.WinGrid) {
			continue
		}
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if w.WinGrid[r][c] == _blank || isBlockedCell(r, c) {
					continue
				}
				sym := s.symbolGrid[r][c]
				if sym >= _femaleA && sym <= _femaleC {
					markEliminationCell(grid, r, c, &count)
					continue
				}
				if sym == _wild && !hasTreasure {
					markEliminationCell(grid, r, c, &count)
				}
			}
		}
	}

	return count
}

func (s *spin) fillElimFreeFull(grid *int64Grid) int {
	count := 0
	for _, w := range s.winInfos {
		if w == nil || !infoHasFemaleWild(w.WinGrid) {
			continue
		}
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if w.WinGrid[r][c] == _blank || isBlockedCell(r, c) {
					continue
				}
				sym := s.symbolGrid[r][c]
				if sym >= (_blank+1) && sym <= _wildFemaleC {
					markEliminationCell(grid, r, c, &count)
				}
			}
		}
	}
	return count
}

func (s *spin) fillElimFreePartial(grid *int64Grid) int {
	count := 0
	for _, w := range s.winInfos {
		if w == nil || w.Symbol < _femaleA || w.Symbol > _femaleC {
			continue
		}
		if !infoHasFemale(w.WinGrid) {
			continue
		}
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if w.WinGrid[r][c] == _blank || isBlockedCell(r, c) {
					continue
				}
				sym := s.symbolGrid[r][c]
				if sym >= _femaleA && sym <= _wildFemaleC {
					s.tryCollectFemaleSymbol(sym)
					markEliminationCell(grid, r, c, &count)
				}
			}
		}
	}
	return count
}

func (s *spin) tryCollectFemaleSymbol(sym int64) {
	if sym < _femaleA || sym > _femaleC {
		return
	}
	if idx := sym - _femaleA; idx >= 0 && idx < int64(len(s.nextFemaleCountsForFree)) {
		i := int(idx)
		if s.nextFemaleCountsForFree[i] < _femaleSymbolCountForFullElimination {
			s.nextFemaleCountsForFree[i]++
		}
	}
}

func markEliminationCell(grid *int64Grid, r, c int64, count *int) {
	if grid == nil || isBlockedCell(r, c) {
		return
	}
	curr := (*grid)[r][c]
	if curr == _eliminated || curr == _treasure {
		return
	}
	(*grid)[r][c] = _eliminated
	(*count)++
}
