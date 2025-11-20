package xslm3

import (
	"fmt"

	"egame-grpc/global"

	"go.uber.org/zap"
)

func (s *betOrderService) updateStepResultInternal(isFreeRound bool) bool {
	if !s.loadStepData(isFreeRound) {
		return false
	}
	s.updateStepData()
	s.findWinInfos()
	switch {
	case !isFreeRound:
		s.processStepForBase()
	default:
		s.processStepForFree()
	}
	return true
}

// Base spin operations

func (s *betOrderService) processStepForBase() {
	switch {
	case s.hasFemaleWin && s.hasWildSymbol():
		s.updateStepResults(true)
	default:
		s.updateStepResults(false)
		s.isRoundOver = true
		s.treasureCount = s.getTreasureCount()
		if s.treasureCount >= _triggerTreasureCount {
			//s.newFreeRoundCount = _freeRounds[s.treasureCount-_triggerTreasureCount]
			idx := int(s.treasureCount - 1)
			if idx >= len(s.gameConfig.FreeSpinCount) {
				idx = len(s.gameConfig.FreeSpinCount) - 1
			}
			s.newFreeRoundCount = s.gameConfig.FreeSpinCount[idx]
		}
	}
}

// Free spin operations

func (s *betOrderService) processStepForFree() {
	switch {
	case s.enableFullElimination:
		s.updateStepResults(false)
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				symbol := s.winGrid[r][c]
				if symbol >= _femaleA && symbol <= _femaleC {
					s.updateFemaleCountForFree(symbol)
				}
			}
		}
		s.isRoundOver = len(s.winResults) == 0
	case s.hasFemaleWin:
		s.updateStepResults(true)
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				symbol := s.winGrid[r][c]
				if symbol >= _femaleA && symbol <= _femaleC {
					s.updateFemaleCountForFree(symbol)
				}
			}
		}
	default:
		s.updateStepResults(false)
		s.isRoundOver = true
	}
	if s.isRoundOver {
		s.newFreeRoundCount = s.getTreasureCount()
	}
}

func (s *betOrderService) updateFemaleCountForFree(symbol int64) {
	switch symbol {
	case _femaleA:
		if s.nextFemaleCountsForFree[_femaleA-_femaleA] > _femaleFullCount {
			return
		}
		s.nextFemaleCountsForFree[_femaleA-_femaleA]++
	case _femaleB:
		if s.nextFemaleCountsForFree[_femaleB-_femaleA] > _femaleFullCount {
			return
		}
		s.nextFemaleCountsForFree[_femaleB-_femaleA]++
	case _femaleC:
		if s.nextFemaleCountsForFree[_femaleC-_femaleA] > _femaleFullCount {
			return
		}
		s.nextFemaleCountsForFree[_femaleC-_femaleA]++
	}
}

// Spin helper operations

func (s *betOrderService) loadStepData(isFreeRound bool) bool {
	var symbolGrid int64Grid
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			symbolGrid[row][col] = s.stepMap.Map[row*_colCount+col]
		}
	}
	s.symbolGrid = &symbolGrid
	if !isFreeRound {
		return true
	}
	if int64(len(s.stepMap.FemaleCountsForFree)) != _femaleC-_femaleA+1 {
		global.GVA_LOG.Error(
			"loadStepData",
			zap.Error(fmt.Errorf("unexpected femaleCountsForFree len: %v", len(s.stepMap.FemaleCountsForFree))),
			zap.Int64("presetID", s.preset.ID),
			zap.Int64("stepID", s.stepMap.ID),
			zap.Int64s("femaleCountsForFree", s.stepMap.FemaleCountsForFree),
		)
		return false
	}
	for i, c := range s.stepMap.FemaleCountsForFree {
		s.femaleCountsForFree[i] = c
		s.nextFemaleCountsForFree[i] = c
	}
	return true
}

func (s *betOrderService) updateStepData() {
	if len(s.stepMap.FemaleCountsForFree) == 0 {
		return
	}
	for _, c := range s.stepMap.FemaleCountsForFree {
		if c < _femaleFullCount {
			return
		}
	}
	s.enableFullElimination = true
}

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

func (s *betOrderService) findWinInfos() bool {
	var winInfos []*winInfo
	for symbol := _blank + 1; symbol < _wildFemaleA; symbol++ {
		if info, ok := s.findNormalSymbolWinInfo(symbol); ok {
			if symbol >= _femaleA {
				s.hasFemaleWin = true
			}
			if infoHasFemaleWild(info.WinGrid) {
				s.hasFemaleWildWin = true
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

// findAllWinInfosForFullElimination 全屏消除时，查找所有可能中奖的符号（包括只有女性百搭的way）
// 因为全屏消除会消除除百搭13之外的所有符号，需要先算分
// 修复：在全屏情况下，即使某个way只有女性百搭，没有基础符号，也应该算分
func (s *betOrderService) findAllWinInfosForFullElimination() {
	// 保存已有的中奖信息（按符号索引）
	existingWinInfos := make(map[int64]bool)
	for _, info := range s.winInfos {
		existingWinInfos[info.Symbol] = true
	}

	// 查找所有基础符号（1-9）的中奖，即使没有基础符号，只要有女性百搭也算
	for symbol := _blank + 1; symbol < _wildFemaleA; symbol++ {
		// 如果已经找到（有基础符号的way），跳过
		if existingWinInfos[symbol] {
			continue
		}

		// 查找只有女性百搭的way（没有基础符号，但可能有百搭13）
		lineCount := int64(1)
		var winGrid int64Grid
		hasFemaleWild := false
		hasBaseSymbol := false

		for c := int64(0); c < _colCount; c++ {
			count := int64(0)
			for r := int64(0); r < _rowCount; r++ {
				currSymbol := s.symbolGrid[r][c]
				// 匹配基础符号、百搭或对应的女性百搭
				if currSymbol == symbol || currSymbol == _wild || isMatchingFemaleWild(symbol, currSymbol) {
					if currSymbol == symbol {
						hasBaseSymbol = true
					}
					if isMatchingFemaleWild(symbol, currSymbol) {
						hasFemaleWild = true
					}
					count++
					winGrid[r][c] = currSymbol
				}
			}
			if count == 0 {
				// 如果已经有基础符号的way，不需要再查找只有女性百搭的way
				if c >= _minMatchCount && hasFemaleWild && !hasBaseSymbol {
					// 只有女性百搭的way也算分（全屏情况下）
					info := winInfo{Symbol: symbol, SymbolCount: c, LineCount: lineCount, WinGrid: winGrid}
					s.winInfos = append(s.winInfos, &info)
					s.hasFemaleWildWin = true
				}
				break
			}
			lineCount *= count
			if c == _colCount-1 && hasFemaleWild && !hasBaseSymbol {
				// 只有女性百搭的way也算分（全屏情况下）
				info := winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, WinGrid: winGrid}
				s.winInfos = append(s.winInfos, &info)
				s.hasFemaleWildWin = true
			}
		}
	}
}

func (s *betOrderService) findNormalSymbolWinInfo(symbol int64) (*winInfo, bool) {
	exist := false
	lineCount := int64(1)
	var winGrid int64Grid
	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			//if currSymbol == symbol || (currSymbol >= _wildFemaleA && currSymbol <= _wild) {
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
				info := winInfo{Symbol: symbol, SymbolCount: c, LineCount: lineCount, WinGrid: winGrid}
				return &info, true
			}
			break
		}
		lineCount *= count
		if c == _colCount-1 && exist {
			info := winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, WinGrid: winGrid}
			return &info, true
		}
	}
	return nil, false
}

func (s *betOrderService) findWildSymbolWinInfo(symbol int64) (*winInfo, bool) {
	lineCount := int64(1)
	var winGrid int64Grid
	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || currSymbol == _wild {
				count++
				winGrid[r][c] = currSymbol
			}
		}
		if count == 0 {
			if c >= _minMatchCount {
				info := winInfo{Symbol: symbol, SymbolCount: c, LineCount: lineCount, WinGrid: winGrid}
				return &info, true
			}
			break
		}
		lineCount *= count
		if c == _colCount-1 {
			info := winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, WinGrid: winGrid}
			return &info, true
		}
	}
	return nil, false
}

func (s *betOrderService) updateStepResults(partialElimination bool) {
	var winResults []*winResult
	var winGrid int64Grid
	lineMultiplier := int64(0)
	for _, info := range s.winInfos {
		if partialElimination && info.Symbol < _femaleA {
			continue
		}
		// 边界检查：确保 Symbol 和 SymbolCount 在有效范围内
		if info.Symbol < 1 || info.Symbol > 12 {
			continue
		}
		if info.SymbolCount < _minMatchCount || info.SymbolCount > _colCount {
			continue
		}
		//baseLineMultiplier := _symbolMultiplierGroups[info.Symbol-1][info.SymbolCount-_minMatchCount]
		symbolIdx := info.Symbol - 1
		countIdx := info.SymbolCount - 1
		if symbolIdx >= int64(len(s.gameConfig.PayTable)) || countIdx >= int64(len(s.gameConfig.PayTable[symbolIdx])) {
			continue
		}
		baseLineMultiplier := s.gameConfig.PayTable[symbolIdx][countIdx]
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
	s.winResults = winResults
	s.winGrid = &winGrid
}
