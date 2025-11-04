package xslm2

// spin Spin数据结构（管理单个step的符号网格和中奖计算）
type spin struct {
	femaleCountsForFree     [_femaleC - _femaleA + 1]int64 // 女性符号计数（当前step）
	nextFemaleCountsForFree [_femaleC - _femaleA + 1]int64 // 女性符号计数（下一step，用于收集）
	enableFullElimination   bool                           // 全屏消除标志（女性>=10触发）
	symbolGrid              *int64Grid                     // 符号网格（4×5）
	winGrid                 *int64Grid                     // 中奖网格（标记中奖位置）
	winInfos                []*winInfo                     // 中奖信息（原始数据）
	winResults              []*winResult                   // 中奖结果（计算后）
	lineMultiplier          int64                          // 线倍数（Ways玩法计算）
	stepMultiplier          int64                          // Step总倍数（所有中奖倍数之和）
	isRoundOver             bool                           // 回合结束标志（true=需要下一回合）
	hasFemaleWin            bool                           // 有女性中奖标志（控制连消逻辑）
	treasureCount           int64                          // 夺宝符号数量（触发免费）
	newFreeRoundCount       int64                          // 新增免费次数
}

// baseSpin 核心spin逻辑（动态生成符号网格）
func (s *spin) baseSpin(isFreeRound bool) {
	// 1. 动态生成符号网格
	symbolArray := _cnf.initSpinSymbol(isFreeRound)
	var symbolGrid int64Grid
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			symbolGrid[row][col] = symbolArray[row*_colCount+col]
		}
	}
	s.symbolGrid = &symbolGrid

	// 2. 免费模式：检查是否触发全屏消除
	if isFreeRound {
		allFull := true
		for _, c := range s.femaleCountsForFree {
			if c < _cnf.FemaleSymbolCountForFullElimination {
				allFull = false
				break
			}
		}
		s.enableFullElimination = allFull
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

// processStepForBase 处理基础模式step
// 基础模式：有女性中奖+有Wild时才继续下一step，否则回合结束
func (s *spin) processStepForBase() {
	if s.hasFemaleWin && s.hasWildSymbol() {
		s.updateStepResults(true)
		return
	}
	s.updateStepResults(false)
	s.isRoundOver = true
	s.treasureCount = s.getTreasureCount()
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
		s.newFreeRoundCount = s.getTreasureCount()
	}
}

// collectFemaleSymbols 收集中奖的女性符号
func (s *spin) collectFemaleSymbols() {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbol := s.winGrid[r][c]
			if symbol >= _femaleA && symbol <= _femaleC {
				s.updateFemaleCountForFree(symbol)
			}
		}
	}
}

// updateFemaleCountForFree 更新女性符号收集计数（达到10个触发全屏消除）
func (s *spin) updateFemaleCountForFree(symbol int64) {
	if symbol < _femaleA || symbol > _femaleC {
		return
	}
	idx := symbol - _femaleA
	if s.nextFemaleCountsForFree[idx] >= _cnf.FemaleSymbolCountForFullElimination {
		return
	}
	s.nextFemaleCountsForFree[idx]++
}

// hasWildSymbol 判断是否有Wild符号
func (s *spin) hasWildSymbol() bool {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.symbolGrid[r][c] == _wild {
				return true
			}
		}
	}
	return false
}

// getTreasureCount 获取夺宝符号数量
func (s *spin) getTreasureCount() int64 {
	count := int64(0)
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.symbolGrid[r][c] == _treasure {
				count++
			}
		}
	}
	return count
}

// findWinInfos 查找所有中奖组合
func (s *spin) findWinInfos() {
	var winInfos []*winInfo
	for symbol := _blank + 1; symbol < _wildFemaleA; symbol++ {
		if info, ok := s.findNormalSymbolWinInfo(symbol); ok {
			if symbol >= _femaleA {
				s.hasFemaleWin = true
			}
			winInfos = append(winInfos, info)
		}
	}
	for symbol := _wildFemaleA; symbol < _wild; symbol++ {
		if info, ok := s.findWildSymbolWinInfo(symbol); ok {
			s.hasFemaleWin = true
			winInfos = append(winInfos, info)
		}
	}
	s.winInfos = winInfos
}

// findNormalSymbolWinInfo 查找普通符号中奖（Ways玩法）
func (s *spin) findNormalSymbolWinInfo(symbol int64) (*winInfo, bool) {
	exist, lineCount := false, int64(1)
	var winGrid int64Grid
	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			curr := s.symbolGrid[r][c]
			if curr == symbol || (curr >= _wildFemaleA && curr <= _wild) {
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

// findWildSymbolWinInfo 查找Wild女性符号中奖
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

// updateStepResults 计算中奖结果（partialElimination=true时只计算女性符号）
func (s *spin) updateStepResults(partialElimination bool) {
	var winResults []*winResult
	var winGrid int64Grid
	totalMultiplier := int64(0)

	for _, info := range s.winInfos {
		if partialElimination && info.Symbol < _femaleA {
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
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if info.WinGrid[r][c] != _blank {
					winGrid[r][c] = info.WinGrid[r][c]
				}
			}
		}
		totalMultiplier += winMul
	}

	s.stepMultiplier, s.lineMultiplier = totalMultiplier, totalMultiplier
	s.winResults, s.winGrid = winResults, &winGrid
}
