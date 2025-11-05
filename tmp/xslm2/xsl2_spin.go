package xslm2

// spin Spin数据结构（管理单个step的符号网格和中奖计算）
type spin struct {
	femaleCountsForFree     [_femaleC - _femaleA + 1]int64 // 女性符号计数（当前step）
	nextFemaleCountsForFree [_femaleC - _femaleA + 1]int64 // 女性符号计数（下一step，用于收集）

	enableFullElimination bool         // 全屏消除标志（女性>=10触发）
	symbolGrid            *int64Grid   // 符号网格（4×5）
	winGrid               *int64Grid   // 中奖网格（标记中奖位置）
	winInfos              []*winInfo   // 中奖信息（原始数据）
	winResults            []*winResult // 中奖结果（计算后）
	stepMultiplier        int64        // Step总倍数（所有中奖倍数之和）
	isRoundOver           bool         // 回合结束标志（true=需要下一回合）
	hasFemaleWin          bool         // 有女性中奖标志（控制连消逻辑）
	treasureCount         int64        // 夺宝符号数量（触发免费）
	newFreeRoundCount     int64        // 新增免费次数
}

// baseSpin 核心spin逻辑（动态生成符号网格）
func (s *spin) baseSpin(isFreeRound bool) {
	// 1. 动态生成符号网格
	symbolGrid := _cnf.initSpinSymbol(isFreeRound)
	s.symbolGrid = &symbolGrid

	// 2. 免费模式：检查是否触发全屏消除
	if isFreeRound {
		s.enableFullElimination = checkAllFemaleCountsFull(s.femaleCountsForFree)
	}

	// 3. 查找中奖
	s.findWinInfos()

	// 4. 处理步骤
	if !isFreeRound {
		s.processStepForBase()
	} else {
		s.processStepForFree()
	}
}

// findWinInfos 查找所有中奖组合
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

	// 查找女性百搭符号中奖（10-12）
	for symbol := _wildFemaleA; symbol < _wild; symbol++ {
		if info, ok := s.findWildSymbolWinInfo(symbol); ok {
			s.hasFemaleWin = true
			winInfos = append(winInfos, info)
		}
	}

	s.winInfos = winInfos
}

// findNormalSymbolWinInfo 查找普通符号中奖（Ways玩法）
// 符号范围: 1-9（普通符号和女性符号）
// Wild符号(10-13)可以替代
func (s *spin) findNormalSymbolWinInfo(symbol int64) (*winInfo, bool) {
	exist, lineCount := false, int64(1)
	var winGrid int64Grid

	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			curr := s.symbolGrid[r][c]
			// Wild相关符号可以替代普通符号
			if curr == symbol || isWildSymbol(curr) {
				exist = exist || (curr == symbol)
				count++
				winGrid[r][c] = curr
			}
		}

		// 断线检查
		if count == 0 {
			if c >= _minMatchCount && exist {
				return &winInfo{Symbol: symbol, SymbolCount: c, LineCount: lineCount, WinGrid: winGrid}, true
			}
			break
		}

		lineCount *= count

		// 到达最后一列
		if c == _colCount-1 && exist {
			return &winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, WinGrid: winGrid}, true
		}
	}
	return nil, false
}

// findWildSymbolWinInfo 查找Wild女性符号中奖 [10,11,12]
func (s *spin) findWildSymbolWinInfo(symbol int64) (*winInfo, bool) {
	lineCount, winGrid := int64(1), int64Grid{}
	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			if curr := s.symbolGrid[r][c]; curr == symbol || curr == _wild {
				count++
				winGrid[r][c] = curr
			}
		}
		if count == 0 {
			if c >= _minMatchCount {
				return &winInfo{Symbol: symbol, SymbolCount: c, LineCount: lineCount, WinGrid: winGrid}, true
			}
			return nil, false
		}
		lineCount *= count
	}
	return &winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, WinGrid: winGrid}, true
}

// processStepForBase 处理基础模式step
// 基础模式：有女性中奖+有Wild时才继续下一step，否则回合结束
func (s *spin) processStepForBase() {
	if s.hasFemaleWin && hasWildSymbol(s.symbolGrid) {
		s.updateStepResults(true)
		return
	}
	s.updateStepResults(false)
	s.isRoundOver = true
	s.treasureCount = getTreasureCount(s.symbolGrid)
	s.newFreeRoundCount = _cnf.getFreeRoundCount(s.treasureCount)
}

// processStepForFree 处理免费模式step
// 免费模式：全屏消除/有女性中奖时继续，否则回合结束
func (s *spin) processStepForFree() {
	if s.enableFullElimination {
		s.updateStepResults(false)
		s.collectFemaleSymbols()
		s.isRoundOver = len(s.winResults) == 0
	} else if s.hasFemaleWin {
		s.updateStepResults(true)
		s.collectFemaleSymbols()
	} else {
		s.updateStepResults(false)
		s.isRoundOver = true
	}
	if s.isRoundOver {
		s.newFreeRoundCount = getTreasureCount(s.symbolGrid)
	}
}

// collectFemaleSymbols 收集中奖的女性符号
func (s *spin) collectFemaleSymbols() {
	for _, row := range s.winGrid {
		for _, symbol := range row {
			if isFemaleSymbol(symbol) {
				s.updateFemaleCountForFree(symbol)
			}
		}
	}
}

// updateFemaleCountForFree 更新女性符号收集计数（达到阈值触发全屏消除）
func (s *spin) updateFemaleCountForFree(symbol int64) {
	if !isFemaleSymbol(symbol) {
		return
	}
	idx := symbol - _femaleA
	if s.nextFemaleCountsForFree[idx] > _femaleSymbolCountForFullElimination {
		return
	}
	s.nextFemaleCountsForFree[idx]++
}

// updateStepResults 计算中奖结果（partialElimination=true时只计算女性符号）
func (s *spin) updateStepResults(partialElimination bool) {
	var winResults []*winResult
	var winGrid int64Grid
	var winGrids []int64Grid
	totalMultiplier := int64(0)

	for _, info := range s.winInfos {
		// 部分消除模式：跳过普通符号（只计算女性符号）
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

	// 合并所有中奖网格
	mergeWinGrids(&winGrid, winGrids)

	s.stepMultiplier = totalMultiplier
	s.winResults, s.winGrid = winResults, &winGrid
}
