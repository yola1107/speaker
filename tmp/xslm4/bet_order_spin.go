package xslm

import (
	"egame-grpc/global"
	"egame-grpc/model/slot"
	"fmt"
	"go.uber.org/zap"
)

type spin struct {
	preset                  *slot.XSLM
	stepMap                 *stepMap
	femaleCountsForFree     [_femaleC - _femaleA + 1]int64
	enableFullElimination   bool
	isRoundOver             bool
	symbolGrid              *int64Grid
	winInfos                []*winInfo
	winResults              []*winResult
	winGrid                 *int64Grid
	hasFemaleWin            bool
	lineMultiplier          int64
	stepMultiplier          int64
	nextFemaleCountsForFree [_femaleC - _femaleA + 1]int64
	treasureCount           int64
	newFreeRoundCount       int64
}

func (s *spin) updateStepResult(isFreeRound bool) bool {
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

func (s *spin) processStepForBase() {
	switch {
	case s.hasFemaleWin && s.hasWildSymbol():
		s.updateStepResults(true)
	default:
		s.updateStepResults(false)
		s.isRoundOver = true
		s.treasureCount = s.getTreasureCount()
		if s.treasureCount >= _triggerTreasureCount {
			s.newFreeRoundCount = _freeRounds[s.treasureCount-_triggerTreasureCount]
		}
	}
}

// Free spin operations

func (s *spin) processStepForFree() {
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

func (s *spin) updateFemaleCountForFree(symbol int64) {
	switch symbol {
	case _femaleA:
		if s.nextFemaleCountsForFree[_femaleA-_femaleA] > _femaleSymbolCountForFullElimination {
			return
		}
		s.nextFemaleCountsForFree[_femaleA-_femaleA]++
	case _femaleB:
		if s.nextFemaleCountsForFree[_femaleB-_femaleA] > _femaleSymbolCountForFullElimination {
			return
		}
		s.nextFemaleCountsForFree[_femaleB-_femaleA]++
	case _femaleC:
		if s.nextFemaleCountsForFree[_femaleC-_femaleA] > _femaleSymbolCountForFullElimination {
			return
		}
		s.nextFemaleCountsForFree[_femaleC-_femaleA]++
	}
}

// Spin helper operations

func (s *spin) loadStepData(isFreeRound bool) bool {
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

func (s *spin) updateStepData() {
	if len(s.stepMap.FemaleCountsForFree) == 0 {
		return
	}
	for _, c := range s.stepMap.FemaleCountsForFree {
		if c < _femaleSymbolCountForFullElimination {
			return
		}
	}
	s.enableFullElimination = true
}

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

func (s *spin) findWinInfos() bool {
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
	return len(winInfos) > 0
}

func (s *spin) findNormalSymbolWinInfo(symbol int64) (*winInfo, bool) {
	exist := false
	lineCount := int64(1)
	var winGrid int64Grid
	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || (currSymbol >= _wildFemaleA && currSymbol <= _wild) {
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

func (s *spin) findWildSymbolWinInfo(symbol int64) (*winInfo, bool) {
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

func (s *spin) updateStepResults(partialElimination bool) {
	var winResults []*winResult
	var winGrid int64Grid
	lineMultiplier := int64(0)
	for _, info := range s.winInfos {
		if partialElimination && info.Symbol < _femaleA {
			continue
		}
		baseLineMultiplier := _symbolMultiplierGroups[info.Symbol-1][info.SymbolCount-_minMatchCount]
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
