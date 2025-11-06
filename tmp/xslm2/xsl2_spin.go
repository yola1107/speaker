package xslm2

import (
	"egame-grpc/global"
)

type spin struct {
	// 女性符号收集（免费模式）
	femaleCountsForFree     [3]int64 // 当前step的收集进度 [A,B,C]
	nextFemaleCountsForFree [3]int64 // 下一step的收集进度（用于累加）
	enableFullElimination   bool     // 全屏消除标志（三种女性>=10触发）

	// 滚轴和符号
	rollers        [_colCount]SymbolRoller // 滚轴状态（Start递减）
	symbolGrid     *int64Grid              // 当前符号网格（4x5）
	nextSymbolGrid *int64Grid              // 下一step的网格（消除下落填充后）

	// 中奖相关
	winInfos       []*winInfo   // 中奖信息（原始）
	winResults     []*winResult // 中奖结果（计算后）
	winGrid        *int64Grid   // 中奖标记网格
	stepMultiplier int64        // step总倍数
	hasFemaleWin   bool         // 女性符号中奖标志（触发连消）

	// 回合状态
	isRoundOver       bool  // 回合结束标志（false=继续连消）
	treasureCount     int64 // 夺宝符号数量
	newFreeRoundCount int64 // 新增免费次数
}

// baseSpin 核心逻辑：生成网格 → 查找中奖 → 判断连消 → 消除下落填充
func (s *spin) baseSpin(isFreeRound bool, isFirst bool, nextGrid *int64Grid, rollers *[_colCount]SymbolRoller) {
	// 加载网格和滚轴
	s.loadStepData(isFreeRound, isFirst, nextGrid, rollers)

	// 检查全屏消除（仅免费模式）
	s.checkFullElimination(isFreeRound)

	// 查找中奖
	s.findWinInfos()

	// 判断连消
	if isFreeRound {
		s.processStepForFree()
	} else {
		s.processStepForBase()
	}

	// 如果回合未结束，消除下落填充
	if s.isRoundOver {
		s.nextSymbolGrid = nil
		return
	}

	newGrid, eliminatedCount := s.applyEliminationAndDrop(s.symbolGrid, s.winGrid, isFreeRound)

	// 防止死循环：0个符号被消除则强制结束
	if eliminatedCount == 0 {
		global.GVA_LOG.Warn("no symbols eliminated, forcing round end")
		s.forceRoundEnd()
		return
	}

	s.nextSymbolGrid = &newGrid
}

func (s *spin) loadStepData(isFreeRound bool, isFirst bool, nextGrid *int64Grid, rollers *[_colCount]SymbolRoller) {
	if isFirst {
		symbolGrid, newRollers := _cnf.initSpinSymbol(isFreeRound, s.femaleCountsForFree)
		s.symbolGrid = &symbolGrid
		s.rollers = newRollers
		return
	}

	if nextGrid == nil || rollers == nil {
		global.GVA_LOG.Error("loadStepData: nil data in non-first step")
		symbolGrid, newRollers := _cnf.initSpinSymbol(isFreeRound, s.femaleCountsForFree)
		s.symbolGrid = &symbolGrid
		s.rollers = newRollers
		return
	}

	s.symbolGrid = nextGrid
	s.rollers = *rollers
}

// findWinInfos 查找所有中奖组合（Ways玩法）
func (s *spin) findWinInfos() {
	var winInfos []*winInfo

	// 查找普通符号中奖（1-9）
	for symbol := _blank + 1; symbol < _wildFemaleA; symbol++ {
		if info, ok := s.findNormalSymbolWinInfo(symbol); ok {
			if isFemaleSymbol(symbol) {
				s.hasFemaleWin = true
			}
			winInfos = append(winInfos, info)
		}
	}

	// 查找女性百搭中奖（10-12）
	for symbol := _wildFemaleA; symbol < _wild; symbol++ {
		if info, ok := s.findWildSymbolWinInfo(symbol); ok {
			s.hasFemaleWin = true
			winInfos = append(winInfos, info)
		}
	}

	s.winInfos = winInfos
}

// findNormalSymbolWinInfo 查找普通符号中奖（Ways玩法，支持百搭替换）
func (s *spin) findNormalSymbolWinInfo(symbol int64) (*winInfo, bool) {
	exist, lineCount := false, int64(1)
	var winGrid int64Grid

	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			curr := s.symbolGrid[r][c]

			match := curr == symbol || curr == _wild
			if !match && isFemaleSymbol(symbol) {
				correspondingWild := symbol - _femaleA + _wildFemaleA
				match = (curr == correspondingWild)
			}

			if match {
				exist = exist || (curr == symbol)
				count++
				winGrid[r][c] = curr
			}
		}

		if count == 0 {
			if c >= _minMatchCount && exist {
				return &winInfo{Symbol: symbol, SymbolCount: c, LineCount: lineCount, WinGrid: winGrid}, true
			}
			break
		}

		lineCount *= count

		if c == _colCount-1 && exist {
			return &winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, WinGrid: winGrid}, true
		}
	}
	return nil, false
}

// findWildSymbolWinInfo 查找女性百搭中奖（可以不在第一列）
func (s *spin) findWildSymbolWinInfo(symbol int64) (*winInfo, bool) {
	exist, lineCount := false, int64(1)
	var winGrid int64Grid

	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			curr := s.symbolGrid[r][c]
			if curr == symbol || curr == _wild {
				exist = exist || (curr == symbol)
				count++
				winGrid[r][c] = curr
			}
		}

		if count == 0 {
			if c >= _minMatchCount && exist {
				return &winInfo{Symbol: symbol, SymbolCount: c, LineCount: lineCount, WinGrid: winGrid}, true
			}
			break
		}

		lineCount *= count

		if c == _colCount-1 && exist {
			return &winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, WinGrid: winGrid}, true
		}
	}
	return nil, false
}

// processStepForBase 基础模式：女性中奖+有Wild → 连消
func (s *spin) processStepForBase() {
	if s.hasFemaleWin && hasWildSymbol(s.symbolGrid) {
		s.updateStepResults(true)
		s.isRoundOver = false
		return
	}
	s.updateStepResults(false)
	s.forceRoundEnd()
}

// processStepForFree 免费模式：全屏消除/女性中奖 → 连消
func (s *spin) processStepForFree() {
	switch {
	case s.enableFullElimination:
		s.updateStepResults(false)
		s.collectFemaleSymbols()
		s.isRoundOver = len(s.winResults) == 0
	case s.hasFemaleWin:
		s.updateStepResults(true)
		s.collectFemaleSymbols()
		s.isRoundOver = false
		return
	default:
		s.updateStepResults(false)
		s.forceRoundEnd()
	}

	if s.isRoundOver {
		s.treasureCount = getTreasureCount(s.symbolGrid)
		s.newFreeRoundCount = _cnf.getFreeRoundCount(s.treasureCount)
	}
}

// forceRoundEnd 强制结束回合
func (s *spin) forceRoundEnd() {
	s.isRoundOver = true
	s.nextSymbolGrid = nil
	s.treasureCount = getTreasureCount(s.symbolGrid)
	s.newFreeRoundCount = _cnf.getFreeRoundCount(s.treasureCount)
}

// applyEliminationAndDrop 消除 → 下落 → 填充
func (s *spin) applyEliminationAndDrop(lastGrid *int64Grid, lastWinGrid *int64Grid, isFreeRound bool) (int64Grid, int) {
	newGrid := *lastGrid
	eliminatedCount := s.eliminateSymbols(&newGrid, lastWinGrid, isFreeRound)
	s.dropSymbols(&newGrid)
	s.fillNewSymbols(&newGrid)
	return newGrid, eliminatedCount
}

// eliminateSymbols 消除中奖符号（保护夺宝和百搭）
func (s *spin) eliminateSymbols(grid *int64Grid, winGrid *int64Grid, isFreeRound bool) int {
	if grid == nil || winGrid == nil {
		return 0
	}

	hasTreasure := getTreasureCount(grid) > 0
	count := 0

	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if winGrid[r][c] == _blank {
				continue
			}

			symbol := grid[r][c]
			if symbol == _treasure || (symbol == _wild && (isFreeRound || hasTreasure)) {
				continue
			}

			grid[r][c] = _blank
			count++
		}
	}

	return count
}

// dropSymbols 符号下落（非空符号下移到底部）
func (s *spin) dropSymbols(grid *int64Grid) {
	if grid == nil {
		return
	}

	for c := int64(0); c < _colCount; c++ {
		writePos := _rowCount - 1
		for r := _rowCount - 1; r >= 0; r-- {
			if grid[r][c] != _blank {
				if r != writePos {
					grid[writePos][c] = grid[r][c]
					grid[r][c] = _blank
				}
				writePos--
			}
		}
	}
}

// fillNewSymbols 填充新符号（倒序填充，保持符号顺序正确）
func (s *spin) fillNewSymbols(grid *int64Grid) {
	if grid == nil {
		return
	}

	for c := int64(0); c < _colCount; c++ {
		for r := int64(_rowCount - 1); r >= 0; r-- {
			if grid[r][c] == _blank {
				grid[r][c] = s.rollers[c].getFallSymbol()
			}
		}
	}
}

// checkFullElimination 检查并设置全屏消除标志
func (s *spin) checkFullElimination(isFreeRound bool) {
	if !isFreeRound {
		return
	}
	s.enableFullElimination = s.femaleCountsForFree[0] >= _femaleSymbolCountForFullElimination &&
		s.femaleCountsForFree[1] >= _femaleSymbolCountForFullElimination &&
		s.femaleCountsForFree[2] >= _femaleSymbolCountForFullElimination
}

// collectFemaleSymbols 收集中奖的女性符号
func (s *spin) collectFemaleSymbols() {
	if s.winGrid == nil {
		return
	}
	for _, row := range s.winGrid {
		for _, symbol := range row {
			if !isFemaleSymbol(symbol) {
				continue
			}
			idx := symbol - _femaleA
			if s.nextFemaleCountsForFree[idx] < _femaleSymbolCountForFullElimination {
				s.nextFemaleCountsForFree[idx]++
			}
		}
	}
}

// updateStepResults 计算中奖结果（partialElimination=true时只计算女性符号）
func (s *spin) updateStepResults(partialElimination bool) {
	var winResults []*winResult
	var winGrid int64Grid
	var winGrids []int64Grid
	totalMultiplier := int64(0)

	for _, info := range s.winInfos {
		if partialElimination && !isFemaleSymbol(info.Symbol) {
			continue
		}

		baseMul := _cnf.getSymbolMultiplier(info.Symbol, info.SymbolCount)
		winMul := baseMul * info.LineCount

		winResults = append(winResults, &winResult{
			Symbol:             info.Symbol,
			SymbolCount:        info.SymbolCount,
			LineCount:          info.LineCount,
			BaseLineMultiplier: baseMul,
			TotalMultiplier:    winMul,
			WinGrid:            info.WinGrid,
		})

		winGrids = append(winGrids, info.WinGrid)
		totalMultiplier += winMul
	}

	mergeWinGrids(&winGrid, winGrids)
	s.stepMultiplier = totalMultiplier
	s.winResults, s.winGrid = winResults, &winGrid
}
