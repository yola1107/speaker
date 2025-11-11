package xslm2

type spin struct {
	// 持久化
	femaleCountsForFree     [3]int64
	nextFemaleCountsForFree [3]int64
	rollers                 [_colCount]SymbolRoller
	rollerKey               string
	symbolGrid              *int64Grid
	nextSymbolGrid          *int64Grid
	treasureGainedThisRound int64
	lastTreasureCount       int64

	// 本 step
	winGrid               *int64Grid
	winInfos              []*winInfo
	winResults            []*winResult
	stepMultiplier        int64
	hasFemaleWin          bool
	hasFemaleWildWin      bool
	enableFullElimination bool
	isRoundOver           bool
	cascadeMode           int
	treasureCount         int64
	newFreeRoundCount     int64

	// 场景同步/调试
	roundStartFemaleCounts [3]int64
	roundStartTreasure     int64
}

const (
	cascadeNone = iota
	cascadeBase
	cascadeFreePartial
	cascadeFreeFull
)

func (s *spin) baseSpin(isFree, first bool, nextGrid *int64Grid, rollers *[_colCount]SymbolRoller) {
	s.cascadeMode = cascadeNone
	s.loadStepData(isFree, first, nextGrid, rollers)
	s.findWinInfos()
	s.processStep(isFree)
	if s.cascadeMode != cascadeNone {
		s.handleCascade(isFree)
	}
	s.finalizeRound(isFree)
}

func (s *spin) loadStepData(isFree, first bool, nextGrid *int64Grid, rollers *[_colCount]SymbolRoller) {
	if first || nextGrid == nil || rollers == nil {
		s.initSpin(isFree)
		s.lastTreasureCount = 0
		s.nextFemaleCountsForFree = s.femaleCountsForFree
		s.treasureGainedThisRound = 0
		s.roundStartFemaleCounts = s.femaleCountsForFree
		s.roundStartTreasure = getTreasureCount(s.symbolGrid)
	} else {
		s.symbolGrid = nextGrid
		s.rollers = *rollers
	}
	if isFree {
		currTreasure := getTreasureCount(s.symbolGrid)
		if !first {
			s.treasureGainedThisRound += currTreasure - s.lastTreasureCount
		} else {
			s.treasureGainedThisRound += currTreasure
		}
		s.lastTreasureCount = currTreasure
		promoteFemaleToWild(s.symbolGrid, s.nextFemaleCountsForFree)
	}
	s.enableFullElimination = isFree &&
		s.nextFemaleCountsForFree[0] >= _femaleSymbolCountForFullElimination &&
		s.nextFemaleCountsForFree[1] >= _femaleSymbolCountForFullElimination &&
		s.nextFemaleCountsForFree[2] >= _femaleSymbolCountForFullElimination
}

func (s *spin) initSpin(isFree bool) {
	grid, rls, key := _cnf.initSpinSymbol(isFree, s.femaleCountsForFree)
	clearBlockedCells(&grid)
	s.symbolGrid = &grid
	s.rollers = rls
	s.rollerKey = key
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
			s.hasFemaleWin = true
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
	mode, collect := s.classifyCascade(isFree)
	total, results, winGrid := s.buildStepResults(mode)

	s.stepMultiplier = total
	s.winResults = results
	s.winGrid = winGrid

	if total == 0 || mode == cascadeNone || winGrid == nil {
		s.finishRound(isFree)
		return
	}

	if collect {
		s.collectFemaleSymbols()
	}
	s.isRoundOver = false
	s.cascadeMode = mode
}

func (s *spin) classifyCascade(isFree bool) (int, bool) {
	if !isFree {
		if s.hasFemaleWin && hasWildSymbol(s.symbolGrid) {
			return cascadeBase, false
		}
		return cascadeNone, false
	}

	if s.enableFullElimination && s.hasFemaleWildWin {
		return cascadeFreeFull, true
	}

	if s.hasFemaleWin {
		return cascadeFreePartial, true
	}

	return cascadeNone, false
}

func (s *spin) buildStepResults(mode int) (int64, []*winResult, *int64Grid) {
	includeFemaleOnly := mode == cascadeFreePartial
	markFemaleWins := mode == cascadeBase || mode == cascadeFreePartial
	markFemaleWildWins := mode == cascadeFreeFull

	var (
		total   int64
		results []*winResult
		winGrid int64Grid
		marked  bool
	)

	for _, info := range s.winInfos {
		if includeFemaleOnly && (info.Symbol < _femaleA || info.Symbol > _wildFemaleC) {
			continue
		}
		base := _cnf.getSymbolMultiplier(info.Symbol, info.SymbolCount)
		if base == 0 {
			continue
		}
		lineTotal := base * info.LineCount
		total += lineTotal
		results = append(results, &winResult{
			Symbol:             info.Symbol,
			SymbolCount:        info.SymbolCount,
			LineCount:          info.LineCount,
			BaseLineMultiplier: base,
			TotalMultiplier:    lineTotal,
			WinGrid:            info.WinGrid,
		})

		shouldMark := false
		if markFemaleWins && info.Symbol >= _femaleA && info.Symbol <= _wildFemaleC {
			shouldMark = true
		}
		if markFemaleWildWins && infoHasFemaleWild(info.WinGrid) {
			shouldMark = true
		}
		if shouldMark {
			for r := int64(0); r < _rowCount; r++ {
				for c := int64(0); c < _colCount; c++ {
					if info.WinGrid[r][c] != _blank {
						winGrid[r][c] = info.WinGrid[r][c]
					}
				}
			}
			marked = true
		}
	}

	if !marked {
		return total, results, nil
	}
	return total, results, &winGrid
}

func (s *spin) collectFemaleSymbols() {
	if s.winGrid == nil {
		return
	}
	updated := false
	for _, row := range *s.winGrid {
		for _, sym := range row {
			if sym >= _femaleA && sym <= _femaleC {
				idx := sym - _femaleA
				if s.nextFemaleCountsForFree[idx] < _femaleSymbolCountForFullElimination {
					s.nextFemaleCountsForFree[idx]++
					updated = true
				}
			}
		}
	}
	if updated {
		promoteFemaleToWild(s.symbolGrid, s.nextFemaleCountsForFree)
		promoteFemaleToWild(s.winGrid, s.nextFemaleCountsForFree)
	}
}

func (s *spin) finishRound(isFree bool) {
	s.isRoundOver = true
	s.nextSymbolGrid = nil
	s.femaleCountsForFree = s.nextFemaleCountsForFree
	s.cascadeMode = cascadeNone
	if isFree {
		s.lastTreasureCount = 0
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
		return
	}
	s.newFreeRoundCount = _cnf.getFreeRoundCount(s.treasureCount)
}

func (s *spin) handleCascade(isFree bool) {
	if s.isRoundOver || s.winGrid == nil || s.cascadeMode == cascadeNone {
		s.finishRound(isFree)
		return
	}

	newGrid := *s.symbolGrid
	clearBlockedCells(&newGrid)
	clearBlockedCells(s.winGrid)

	hasTreasure := getTreasureCount(&newGrid) > 0

	elimination := selectEliminationRule(s.cascadeMode)
	if elimination == nil {
		s.finishRound(isFree)
		return
	}

	if s.applyElim(&newGrid, elimination, hasTreasure) == 0 {
		s.finishRound(isFree)
		return
	}

	for c := int64(0); c < _colCount; c++ {
		write := int64(0)
		if c == 0 || c == _colCount-1 {
			write = 1
		}
		for r := int64(0); r < _rowCount; r++ {
			if isBlockedCell(r, c) {
				continue
			}
			switch newGrid[r][c] {
			case _eliminated:
				newGrid[r][c] = _blank
				continue
			case _blank:
				continue
			}
			if r != write {
				newGrid[write][c] = newGrid[r][c]
				newGrid[r][c] = _blank
			}
			write++
		}
	}

	for c := int64(0); c < _colCount; c++ {
		for r := int64(0); r < _rowCount; r++ {
			if isBlockedCell(r, c) {
				continue
			}
			if newGrid[r][c] == _blank {
				newGrid[r][c] = s.rollers[c].getFallSymbol()
			}
		}
	}

	clearBlockedCells(&newGrid)
	if isFree {
		promoteFemaleToWild(&newGrid, s.nextFemaleCountsForFree)
		s.lastTreasureCount = getTreasureCount(&newGrid)
	}

	s.nextSymbolGrid = &newGrid
	s.femaleCountsForFree = s.nextFemaleCountsForFree
	s.cascadeMode = cascadeNone
}

func (s *spin) applyElim(grid *int64Grid, rule func(sym, win int64, hasTreasure bool) bool, hasTreasure bool) int {
	cnt := 0
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if isBlockedCell(r, c) {
				continue
			}
			if s.winGrid[r][c] == _blank {
				continue
			}
			sym := (*grid)[r][c]
			if sym == _treasure {
				continue
			}
			if rule(sym, s.winGrid[r][c], hasTreasure) {
				(*grid)[r][c] = _eliminated
				cnt++
			}
		}
	}
	return cnt
}

func selectEliminationRule(mode int) func(symbol, winSymbol int64, hasTreasure bool) bool {
	switch mode {
	case cascadeBase:
		return func(symbol, winSymbol int64, hasTreasure bool) bool {
			if symbol >= _femaleA && symbol <= _wildFemaleC {
				return true
			}
			if symbol == _wild || winSymbol == _wild {
				return !hasTreasure
			}
			return false
		}
	case cascadeFreePartial:
		return func(symbol, winSymbol int64, _ bool) bool {
			if symbol == _wild || winSymbol == _wild {
				return false
			}
			return symbol >= _femaleA && symbol <= _wildFemaleC
		}
	case cascadeFreeFull:
		return func(symbol, winSymbol int64, _ bool) bool {
			if symbol == _wild || winSymbol == _wild {
				return false
			}
			return true
		}
	default:
		return nil
	}
}

func promoteFemaleToWild(grid *int64Grid, counts [3]int64) {
	if grid == nil {
		return
	}
	for idx := int64(0); idx < 3; idx++ {
		if counts[idx] < _femaleSymbolCountForFullElimination {
			continue
		}
		normal := _femaleA + idx
		wild := _wildFemaleA + idx
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if isBlockedCell(r, c) {
					continue
				}
				if (*grid)[r][c] == normal {
					(*grid)[r][c] = wild
				}
			}
		}
	}
}

func clearBlockedCells(grid *int64Grid) {
	if grid == nil {
		return
	}
	(*grid)[0][0] = _blocked
	(*grid)[0][_colCount-1] = _blocked
}
func isBlockedCell(r, c int64) bool { return r == 0 && (c == 0 || c == _colCount-1) }

// isMatchingFemaleWild 判断女性百搭是否匹配目标女性符号
// 规则：女性百搭（10/11/12）只能匹配对应的女性符号（7/8/9），不能相互替换
func isMatchingFemaleWild(target, candidate int64) bool {
	if candidate < _wildFemaleA || candidate > _wildFemaleC {
		return false
	}
	if target < _femaleA || target > _femaleC {
		return false
	}
	return (candidate - _wildFemaleA) == (target - _femaleA)
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
