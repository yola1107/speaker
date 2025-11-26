package xslm2

import (
	"fmt"

	"egame-grpc/global"

	"go.uber.org/zap"
)

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
		if _debugLogOpen {
			global.GVA_LOG.Debug("新回合开始",
				zap.Int8("Stage", s.scene.Stage),
				zap.Int64("FreeNum", s.scene.FreeNum),
				zap.Any("scene", s.scene),
			)
		}
	}

	// 处理符号网格、查找中奖、更新结果
	s.handleSymbolGrid()
	s.findWinInfos()
	s.updateStepResults(false)

	// 处理消除和结果
	hasElimination := s.processElimination()
	if s.isFreeRound {
		s.eliminateResultForFree(hasElimination)
	} else {
		s.eliminateResultForBase(hasElimination)
	}
	s.updateCurrentBalance()
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

// findWinInfos 查找中奖信息
func (s *betOrderService) findWinInfos() bool {
	// 重置标志，避免上一轮的值影响当前轮
	s.hasFemaleWin = false
	s.hasFemaleWildWin = false

	var winInfos []*winInfo
	for symbol := _blank + 1; symbol < _wildFemaleA; symbol++ {
		if info, ok := s.findNormalSymbolWinInfo(symbol); ok {
			if symbol >= _femaleA {
				s.hasFemaleWin = true
			}
			if !s.hasFemaleWildWin && infoHasFemaleWild(info.WinGrid) {
				s.hasFemaleWildWin = true // 女性百搭参与中奖标记
			}
			winInfos = append(winInfos, info)
		}
	}
	for symbol := _wildFemaleA; symbol < _wild; symbol++ {
		if info, ok := s.findWildSymbolWinInfo(symbol); ok {
			//s.hasFemaleWin = true
			s.hasFemaleWildWin = true
			winInfos = append(winInfos, info)
		}
	}
	s.winInfos = winInfos
	return len(winInfos) > 0
}

func isMatchingFemaleWild(symbol, currSymbol int64) bool {
	return (currSymbol >= _wildFemaleA && currSymbol <= _wildFemaleC) && (symbol >= (_blank+1) && symbol <= _femaleC)
}

// findNormalSymbolWinInfo 查找普通符号中奖
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

// findWildSymbolWinInfo 查找女性百搭符号中奖
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

// updateStepResults 更新步骤结果
func (s *betOrderService) updateStepResults(partialElimination bool) {
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

// processElimination 处理消除逻辑
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

	if _debugLogOpen {
		fmt.Printf("Step%d 消除前:\n%s", s.scene.Steps, GridToString(s.symbolGrid, s.winGrid))
	}

	s.collectFemaleSymbol()
	s.dropSymbols(&nextGrid)
	s.fallingWinSymbols(nextGrid)
	s.nextSymbolGrid = &nextGrid

	if _debugLogOpen {
		fmt.Printf("Step%d 消除后:\n%s", s.scene.Steps, GridToString(s.nextSymbolGrid, nil))
	}
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
		if !infoHasBaseWild(w.WinGrid) {
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
		if w == nil || !infoHasFemaleWild(w.WinGrid) {
			continue
		}
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if w.WinGrid[r][c] == _blank || isBlockedCell(r, c) {
					continue
				}
				sym := s.symbolGrid[r][c]
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
			s.scene.TreasureNum = 0
		} else {
			s.scene.NextStage = _spinTypeBase
			s.scene.TreasureNum = 0
		}
	}

	if s.stepMultiplier > 0 {
		s.updateBonusAmount()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncRoundBonus(s.bonusAmount.Round(2).InexactFloat64())
	}

	if _debugLogOpen {
		str := " 回合消除"
		if s.isRoundOver {
			str = "回合结束 "
		}
		global.GVA_LOG.Debug(str,
			zap.Int8("Stage", s.scene.Stage),
			zap.Int64("FreeNum", s.scene.FreeNum),
			zap.Any("scene", s.scene),
		)
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
		s.newFreeRoundCount = 0
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
			s.scene.TreasureNum = 0
		}
	}

	if s.stepMultiplier > 0 {
		s.updateBonusAmount()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncrFreeTotalMoney(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncRoundBonus(s.bonusAmount.Round(2).InexactFloat64())
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
	for i := range s.scene.SymbolRoller {
		s.scene.SymbolRoller[i].ringSymbol(s.gameConfig)
	}

	// 免费模式下转换女性符号为百搭
	if s.isFreeRound {
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
}

// hasWildSymbol 检测是否有百搭符号
func (s *betOrderService) hasWildSymbol() bool {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.symbolGrid[r][c] == _wild {
				return true
			}
		}
	}
	return false
}

// getTreasureCount 统计夺宝符号数量
func (s *betOrderService) getTreasureCount() int64 {
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
