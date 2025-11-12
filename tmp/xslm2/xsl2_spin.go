package xslm2

import "egame-grpc/global"

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
	treasureCount         int64
	newFreeRoundCount     int64

	// 免费局夺宝累计（按步增量累加，避免净差值漏计）
	treasureGainedThisRound int64
	lastTreasureCount       int64

	// 本回合开始时的女性收集，用于调试输出
	roundStartFemaleCounts [3]int64
	roundStartTreasure     int64
}

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
		s.roundStartTreasure = getTreasureCount(s.symbolGrid)
		s.nextFemaleCountsForFree = s.femaleCountsForFree
	} else if nextGrid == nil || rollers == nil {
		global.GVA_LOG.Sugar().Errorf("免费模式下，空nextGrid/rollers：%s", s.desc())
		s.initSpin(isFree)
		s.roundStartTreasure = getTreasureCount(s.symbolGrid)
		s.nextFemaleCountsForFree = s.femaleCountsForFree
		s.roundStartFemaleCounts = s.femaleCountsForFree
	} else {
		s.symbolGrid = nextGrid
		s.rollers = *rollers
	}

	if isFree {
		convertFemaleToWild(s.symbolGrid, s.femaleCountsForFree)
		currTreasure := getTreasureCount(s.symbolGrid)
		if isFirst {
			s.treasureGainedThisRound = currTreasure
		} else if diff := currTreasure - s.lastTreasureCount; diff > 0 {
			s.treasureGainedThisRound += diff
		}
		s.lastTreasureCount = currTreasure
	} else {
		s.treasureGainedThisRound, s.lastTreasureCount = 0, 0
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
			s.hasFemaleWin, s.hasFemaleWildWin = true, true
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

	eliminate, collect := s.shouldCascade(isFree)
	if !eliminate {
		s.finishRound(isFree)
		return
	}
	if collect {
		s.collectFemaleSymbols()
	}

	elimGrid := s.winGrid
	/*
		// TODO 可能网格不一样，做补充
			当前 elimGrid 一直等于 winGrid。
			原先的设计是：免费且 enableFullElimination && hasFemaleWildWin 时，只消含女性百搭的那些位置；这一段现在缺失，业务行为会变成“总是按普通网格消除”。
			如果这是暂时的，请补上或确保不会遗漏预期玩法。
	*/
	nextGrid := s.executeCascade(elimGrid, isFree)
	if nextGrid == nil {
		s.finishRound(isFree)
		return
	}
	s.nextSymbolGrid = nextGrid
	s.femaleCountsForFree = s.nextFemaleCountsForFree
	s.isRoundOver = false
}

func (s *spin) updateStepResults() {
	var winResults []*winResult
	var winGrid int64Grid
	var lineMultiplier int64

	for _, info := range s.winInfos {
		base := _cnf.getSymbolMultiplier(info.Symbol, info.SymbolCount)
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

func (s *spin) shouldCascade(isFree bool) (bool, bool) {
	if isFree {
		return s.hasFemaleWin, s.hasFemaleWin
	}
	return s.hasFemaleWin && hasWildSymbol(s.symbolGrid), false
}

func (s *spin) collectFemaleSymbols() {
	if s.winGrid == nil {
		return
	}
	for _, row := range s.winGrid {
		for _, symbol := range row {
			if symbol < _femaleA || symbol > _femaleC {
				continue
			}
			if idx := symbol - _femaleA; idx >= 0 && s.nextFemaleCountsForFree[idx] < _femaleSymbolCountForFullElimination {
				s.nextFemaleCountsForFree[idx]++
			}
		}
	}
}

func (s *spin) finalizeRound(isFree bool) {
	if !s.isRoundOver {
		s.treasureCount = 0
		s.newFreeRoundCount = 0
		return
	}

	s.treasureCount = getTreasureCount(s.symbolGrid)
	if isFree {
		if s.treasureGainedThisRound < 0 {
			s.treasureGainedThisRound = 0
		}
		s.newFreeRoundCount = s.treasureGainedThisRound
		s.treasureGainedThisRound = 0
		s.lastTreasureCount = 0
	} else {
		s.newFreeRoundCount = _cnf.getFreeRoundCount(s.treasureCount)
	}
}

func (s *spin) executeCascade(elimGrid *int64Grid, isFree bool) *int64Grid {
	grid := *s.symbolGrid
	clearBlockedCells(&grid)
	hasTreasure := getTreasureCount(&grid) > 0
	rule := s.selectEliminationRule(isFree, hasTreasure)

	eliminated := 0
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if isBlockedCell(r, c) || elimGrid[r][c] == _blank {
				continue
			}
			symbol := grid[r][c]
			if symbol != _treasure && rule(symbol, hasTreasure) {
				grid[r][c] = _eliminated
				eliminated++
			}
		}
	}
	if eliminated == 0 {
		global.GVA_LOG.Error("no symbols eliminated, forcing round end")
		return nil
	}

	s.dropSymbols(&grid)
	s.fillBlanks(&grid)
	clearBlockedCells(&grid)
	if isFree {
		convertFemaleToWild(&grid, s.femaleCountsForFree)
	}
	return &grid
}

type eliminationRule func(symbol int64, hasTreasure bool) bool

func (s *spin) selectEliminationRule(isFree, hasTreasure bool) eliminationRule {
	if isFree {
		return func(sym int64, _ bool) bool {
			return sym != _wild && ((sym >= _femaleA && sym <= _femaleC) || (sym >= _wildFemaleA && sym <= _wildFemaleC))
		}
	}
	return func(sym int64, hasTreasure bool) bool {
		if sym >= _femaleA && sym <= _femaleC {
			return true
		}
		return sym == _wild && !hasTreasure
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

func (s *spin) fillBlanks(grid *int64Grid) {
	for c := int64(0); c < _colCount; c++ {
		for r := int64(0); r < _rowCount; r++ {
			if !isBlockedCell(r, c) && (*grid)[r][c] == _blank {
				(*grid)[r][c] = s.rollers[c].getFallSymbol()
			}
		}
	}
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

func isMatchingFemaleWild(target, cand int64) bool {
	return (cand >= _wildFemaleA && cand <= _wildFemaleC) &&
		(target >= _femaleA && target <= _femaleC) &&
		(cand-_wildFemaleA) == (target-_femaleA)
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
