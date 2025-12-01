package xslm2

// baseSpin 主旋转函数
func (s *betOrderService) baseSpin() error {
	if err := s.initialize(); err != nil {
		return err
	}

	s.handleStageTransition()
	s.loadSceneFemaleCount()

	// 新回合开始时初始化符号网格
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.scene.SymbolRoller = s.getSceneSymbol()
	}

	// 处理符号网格、查找中奖、更新结果
	s.handleSymbolGrid()
	s.findWinInfos()
	s.processWinInfos(false)
	s.updateBonusAmount()
	s.updateCurrentBalance()

	// 处理消除和结果
	hasElimination := s.processElimination()
	if s.isFreeRound {
		s.eliminateResultForFree(hasElimination)
	} else {
		s.eliminateResultForBase(hasElimination)
	}
	return nil
}

// loadSceneFemaleCount 加载女性符号计数
func (s *betOrderService) loadSceneFemaleCount() {
	if !s.isFreeRound {
		s.femaleCountsForFree = [3]int64{}
		s.nextFemaleCountsForFree = [3]int64{}
		s.enableFullElimination = false
		return
	}

	for i, c := range s.scene.FemaleCountsForFree {
		s.femaleCountsForFree[i] = c
		s.nextFemaleCountsForFree[i] = c
	}
	s.enableFullElimination =
		s.scene.RoundFemaleCountsForFree[0] >= _femaleFullCount &&
			s.scene.RoundFemaleCountsForFree[1] >= _femaleFullCount &&
			s.scene.RoundFemaleCountsForFree[2] >= _femaleFullCount
}

// handleSymbolGrid 处理符号网格
func (s *betOrderService) handleSymbolGrid() {
	var symbolGrid int64Grid
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbolGrid[_rowCount-1-r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
	s.symbolGrid = &symbolGrid
}

func (s *betOrderService) findWinInfos() bool {
	hasFemaleWin := false
	hasFemaleWildWin := false

	var winInfos []*winInfo
	for symbol := _blank + 1; symbol < _wildFemaleA; symbol++ {
		if info, ok := s.findNormalSymbolWinInfo(symbol); ok {
			if symbol >= _femaleA {
				hasFemaleWin = true
			}
			if !hasFemaleWildWin && containsFemaleWild(info.WinGrid) {
				hasFemaleWildWin = true // 女性百搭参与中奖标记
			}
			winInfos = append(winInfos, info)
		}
	}
	for symbol := _wildFemaleA; symbol < _wild; symbol++ {
		if info, ok := s.findWildSymbolWinInfo(symbol); ok {
			hasFemaleWildWin = true
			winInfos = append(winInfos, info)
		}
	}
	s.winInfos = winInfos
	s.hasFemaleWin = hasFemaleWin
	s.hasFemaleWildWin = hasFemaleWildWin
	return len(winInfos) > 0
}

func isMatchingFemaleWild(symbol, currSymbol int64) bool {
	return (currSymbol >= _wildFemaleA && currSymbol <= _wildFemaleC) && (symbol >= (_blank+1) && symbol <= _femaleC)
}

func (s *betOrderService) findNormalSymbolWinInfo(symbol int64) (*winInfo, bool) {
	exist := false
	lineCount := int64(1)
	var winGrid int64Grid

	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || currSymbol == _wild || isMatchingFemaleWild(symbol, currSymbol) {
				if currSymbol == symbol {
					exist = true
				}
				count++
				winGrid[r][c] = currSymbol
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

func (s *betOrderService) findWildSymbolWinInfo(symbol int64) (*winInfo, bool) {
	exist := false
	lineCount := int64(1)
	var winGrid int64Grid

	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || currSymbol == _wild {
				if currSymbol == symbol {
					exist = true
				}
				count++
				winGrid[r][c] = currSymbol
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

// processWinInfos 更新步骤结果
func (s *betOrderService) processWinInfos(partialElimination bool) {
	var winResults []*winResult
	var winGrid int64Grid
	lineMultiplier := int64(0)
	for _, info := range s.winInfos {
		if partialElimination && info.Symbol < _femaleA {
			continue
		}
		//baseLineMultiplier := _symbolMultiplierGroups[info.Symbol-1][info.SymbolCount-_minMatchCount]
		baseLineMultiplier := s.gameConfig.PayTable[info.Symbol-1][info.SymbolCount-1]
		totalMultiplier := baseLineMultiplier * info.LineCount
		result := winResult{
			Symbol:             info.Symbol,
			SymbolCount:        info.SymbolCount,
			LineCount:          info.LineCount,
			BaseLineMultiplier: baseLineMultiplier,
			TotalMultiplier:    totalMultiplier,
			WinGrid:            info.WinGrid,
		}
		winResults = append(winResults, &result)
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if info.WinGrid[r][c] != _blank {
					winGrid[r][c] = info.WinGrid[r][c]
				}
			}
		}
		lineMultiplier += totalMultiplier
	}
	s.stepMultiplier = lineMultiplier
	s.lineMultiplier = lineMultiplier
	s.winResults = winResults
	s.winGrid = &winGrid
}

/*
	算分：女性百搭（10，11，12）可替换为基础符号（1，2，3，4，5，6，7，8，9），但连线上必须要有基础符号

	消除：
		基础模式：消除中奖的女性符号（7，8，9）及百搭，如果盘面有夺宝则百搭不消除
		免费模式：
			1> 全屏情况：每个中奖Way找女性百搭，找到则改way除百搭13之外的符号都全部消除
			2> 非全屏情况：每个中奖way找女性，找到该way女性及女性百搭都消除
*/
// processElimination 计算并执行消除网格
func (s *betOrderService) processElimination() bool {
	if len(s.winInfos) == 0 || s.stepMultiplier == 0 || s.winGrid == nil {
		return false
	}

	isFree := s.isFreeRound
	nextGrid := *s.symbolGrid
	var cnt int

	switch {
	case !isFree && s.hasFemaleWin && s.hasWildSymbol():
		cnt = s.fillElimBase(&nextGrid)
	case isFree && s.enableFullElimination && s.hasFemaleWildWin:
		cnt = s.fillElimFreeFull(&nextGrid)
	case isFree && (!s.enableFullElimination) && s.hasFemaleWin:
		cnt = s.fillElimFreePartial(&nextGrid)
	}

	if cnt == 0 {
		return false
	}

	s.collectFemaleSymbol()
	s.dropSymbols(&nextGrid)
	s.fallingWinSymbols(nextGrid)
	s.nextSymbolGrid = &nextGrid

	return true
}

// fillElimBase 基础模式消除填充
func (s *betOrderService) fillElimBase(grid *int64Grid) int {
	count := 0
	hasTreasure := s.getTreasureCount() > 0
	for _, w := range s.winInfos {
		if w == nil || w.Symbol < _femaleA || w.Symbol > _femaleC {
			continue
		}
		if !containsBaseWild(w.WinGrid) {
			continue
		}
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if w.WinGrid[r][c] == _blank || isBlockedCell(r, c) {
					continue
				}
				sym := s.symbolGrid[r][c]
				if (sym >= _femaleA && sym <= _femaleC) || (sym == _wild && !hasTreasure) {
					grid[r][c] = _eliminated
					count++
				}
			}
		}
	}
	return count
}

// fillElimFreeFull 免费模式全屏消除
func (s *betOrderService) fillElimFreeFull(grid *int64Grid) int {
	count := 0
	for _, w := range s.winInfos {
		if w == nil || !containsFemaleWild(w.WinGrid) {
			continue // 需要包含【10，12】的女性百搭符号
		}
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if w.WinGrid[r][c] == _blank || isBlockedCell(r, c) {
					continue
				}
				sym := s.symbolGrid[r][c]
				// 全屏情况：除百搭13之外的符号都全部消除（女性百搭符号会消失，但百搭符号不消失）
				if sym >= (_blank+1) && sym <= _wildFemaleC && sym != _wild {
					grid[r][c] = _eliminated
					count++
				}
			}
		}
	}
	return count
}

// fillElimFreePartial 免费模式部分消除
func (s *betOrderService) fillElimFreePartial(grid *int64Grid) int {
	count := 0
	for _, w := range s.winInfos {
		if w == nil || w.Symbol < _femaleA || w.Symbol > _femaleC {
			continue
		}
		if !containsFemale(w.WinGrid) {
			continue
		}
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if w.WinGrid[r][c] == _blank || isBlockedCell(r, c) {
					continue
				}
				sym := s.symbolGrid[r][c]
				if sym >= _femaleA && sym <= _wildFemaleC {
					grid[r][c] = _eliminated
					count++
				}
			}
		}
	}
	return count
}

// eliminateResultForBase 基础模式消除结果处理
func (s *betOrderService) eliminateResultForBase(hasElimination bool) {
	if hasElimination {
		// 有消除，继续消除状态
		s.isRoundOver = false
		s.client.IsRoundOver = false
		s.scene.Steps++
		s.scene.NextStage = _spinTypeBaseEli
		s.scene.FemaleCountsForFree = [3]int64{}
		s.scene.RoundFemaleCountsForFree = [3]int64{}
		s.newFreeRoundCount = 0
	} else {
		// 没有消除，结束当前回合
		s.isRoundOver = true
		s.client.IsRoundOver = true
		s.scene.Steps = 0
		s.scene.FemaleCountsForFree = [3]int64{}
		s.scene.RoundFemaleCountsForFree = [3]int64{}
		s.nextSymbolGrid = nil
		s.treasureCount = s.getTreasureCount()
		s.newFreeRoundCount = s.getFreeRoundCountFromTreasure()

		if s.newFreeRoundCount > 0 {
			// 触发免费模式
			s.scene.FreeNum = s.newFreeRoundCount
			s.client.ClientOfFreeGame.SetFreeNum(uint64(s.newFreeRoundCount))
			s.client.SetLastMaxFreeNum(uint64(s.newFreeRoundCount))
			s.scene.NextStage = _spinTypeFree

		} else {
			// 不触发免费模式，继续基础模式
			s.scene.NextStage = _spinTypeBase
		}
	}
}

// eliminateResultForFree 免费模式消除结果处理
func (s *betOrderService) eliminateResultForFree(hasElimination bool) {
	if hasElimination {
		// 有消除，继续消除状态
		s.isRoundOver = false
		s.client.IsRoundOver = false
		s.scene.Steps++
		s.scene.NextStage = _spinTypeFreeEli
		s.scene.FemaleCountsForFree = s.nextFemaleCountsForFree
		s.newFreeRoundCount = 0 // 有消除时，不统计夺宝数量
	} else {
		// 没有消除，结束当前回合 （当前局）
		s.isRoundOver = true
		s.client.IsRoundOver = true
		s.scene.Steps = 0
		s.nextSymbolGrid = nil

		s.newFreeRoundCount = s.getTreasureCount()
		if s.newFreeRoundCount > 0 {
			s.client.ClientOfFreeGame.Incr(uint64(s.newFreeRoundCount))
			s.client.IncLastMaxFreeNum(uint64(s.newFreeRoundCount))
			s.scene.FreeNum += s.newFreeRoundCount
		}

		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
		if s.scene.FreeNum < 0 {
			s.scene.FreeNum = 0
		}

		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
			s.scene.FemaleCountsForFree = s.nextFemaleCountsForFree
			s.scene.RoundFemaleCountsForFree = s.nextFemaleCountsForFree
		} else {
			s.scene.NextStage = _spinTypeBase
			s.scene.FemaleCountsForFree = [3]int64{}
			s.scene.RoundFemaleCountsForFree = [3]int64{}
		}
	}
}

// dropSymbols 符号下落逻辑
func (s *betOrderService) dropSymbols(grid *int64Grid) {
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

// fallingWinSymbols 符号掉落处理
func (s *betOrderService) fallingWinSymbols(nextSymbolGrid int64Grid) {
	// 更新滚轮符号
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			s.scene.SymbolRoller[c].BoardSymbol[r] = nextSymbolGrid[_rowCount-1-r][c]
		}
	}

	// 补充新符号
	for i, _ := range s.scene.SymbolRoller {
		s.scene.SymbolRoller[i].ringSymbol(s.gameConfig)
	}

	// 免费模式下，填充后需要根据 ABC 计数转换女性百搭符号
	if s.isFreeRound {
		s.convertBoardSymbols()
	}
}

// collectFemaleSymbol 收集女性符号
func (s *betOrderService) collectFemaleSymbol() {
	if !s.isFreeRound {
		return
	}
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbol := s.winGrid[r][c]
			if symbol >= _femaleA && symbol <= _femaleC {
				idx := symbol - _femaleA
				if s.nextFemaleCountsForFree[idx] < _femaleFullCount {
					s.nextFemaleCountsForFree[idx]++
				}
			}
		}
	}
}

// convertBoardSymbols 转换棋盘上的女性符号为女性百搭符号
func (s *betOrderService) convertBoardSymbols() {
	for col := 0; col < int(_colCount); col++ {
		for row := 0; row < int(_rowCount); row++ {
			symbol := s.scene.SymbolRoller[col].BoardSymbol[row]
			if symbol >= _femaleA && symbol <= _femaleC {
				idx := symbol - _femaleA
				if idx >= 0 && idx < 3 && s.scene.RoundFemaleCountsForFree[idx] >= _femaleFullCount {
					s.scene.SymbolRoller[col].BoardSymbol[row] = _wildFemaleA + idx
				}
			}
		}
	}
}
