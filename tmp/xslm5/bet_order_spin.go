package xslm3

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
	// 重置标志，避免上一轮的值影响当前轮
	s.hasFemaleWin = false
	s.hasFemaleWildWin = false

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

// isMatchingFemaleWild 检查女性百搭符号是否可以匹配目标符号
// 规则：女性百搭符号（10-12）可以替代除了夺宝、百搭外的所有符号（即基础符号1-9和女性符号7-9）
// 注意：女性百搭之间不可以相互替换，但可以通过此函数匹配基础符号
func isMatchingFemaleWild(target, curr int64) bool {
	// 检查 curr 是否是女性百搭符号（10-12）
	if curr < _wildFemaleA || curr > _wildFemaleC {
		return false
	}
	// 女性百搭可以匹配基础符号（1-9），包括普通符号（1-6）和女性符号（7-9）
	return target >= (_blank+1) && target <= _femaleC
}

// 遍历符号【1，9】查找中奖符号及中奖网格（线）
func (s *betOrderService) findNormalSymbolWinInfo(symbol int64) (*winInfo, bool) {
	exist := false
	lineCount := int64(1)
	var winGrid int64Grid
	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			// 符号 1-9
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
	// 检测女性百搭符号（10, 11, 12）的中奖
	// 规则：
	// - 女性百搭符号可以单独算奖，根据 payTable 里有对应的赔率（索引 9-11）
	// - 女性百搭之间不可以相互替换：只有相同的女性百搭符号本身可以匹配
	// - 百搭符号（13）可以替换女性百搭符号
	// - 例如：10 10 13 可算奖（女性百搭A + 女性百搭A + 百搭）
	exist := false
	lineCount := int64(1)
	var winGrid int64Grid
	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			// 只匹配相同的女性百搭符号本身或百搭（13），不匹配其他女性百搭符号
			if currSymbol == symbol || currSymbol == _wild {
				// 至少需要有一个目标女性百搭符号本身，百搭（13）可以作为替换
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
