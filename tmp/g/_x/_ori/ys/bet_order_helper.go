package ys

func int64GridToArray(grid int64Grid) []int64 {
	elements := make([]int64, _rowCount*_colCount)
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			elements[r*_colCount+c] = grid[r][c]
		}
	}
	return elements
}

func (s *betOrderService) getScatterCount() int64 {
	var count int64
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.symbolGrid[r][c] == _treasure {
				count++
			}
		}
	}
	return count
}

func (s *betOrderService) getStepMultiplier() int64 {
	multiArr := s.gameConfig.BaseMulti
	if s.isFreeRound {
		multiArr = s.gameConfig.getFreeCfgByType(s.scene.BonusNum).Multi
	}
	idx := int(s.scene.Steps)
	if idx >= len(multiArr) {
		idx = len(multiArr) - 1
	}
	return multiArr[idx]
}

func (s *betOrderService) handleSymbolGrid() {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.symbolGrid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
}

// moveSymbols 清除中奖格并下落
func (s *betOrderService) moveSymbols() int64Grid {
	nextSymbolGrid := s.symbolGrid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.winGrid[r][c] > 0 {
				nextSymbolGrid[r][c] = 0
			}
		}
	}
	s.dropSymbols(&nextSymbolGrid)
	return nextSymbolGrid
}

// dropSymbols 符号下落：0 视为空位，把非 0 符号压到底部
func (s *betOrderService) dropSymbols(grid *int64Grid) {
	for c := 0; c < _colCount; c++ {
		writePos := _rowCount - 1
		for r := _rowCount - 1; r >= 0; r-- {
			if val := (*grid)[r][c]; val != 0 {
				if r != writePos {
					(*grid)[writePos][c] = val
					(*grid)[r][c] = 0
				}
				writePos--
			}
		}
	}
}

func (s *betOrderService) fallingWinSymbols(nextSymbolGrid int64Grid) {
	for col := range s.scene.SymbolRoller {
		roller := &s.scene.SymbolRoller[col]
		data := s.gameConfig.RealData[roller.Real][col]
		for r := _rowCount - 1; r >= 0; r-- {
			sym := nextSymbolGrid[r][col]
			if sym == 0 {
				roller.Start--
				if roller.Start < 0 {
					roller.Start = len(data) - 1
				}
				sym = data[roller.Start]
			}
			roller.BoardSymbol[r] = sym
		}
	}
}

// findWinInfos 查找中奖信息（Line玩法）
func (s *betOrderService) findWinInfos() {
	var (
		winInfos []WinInfo
		winGrid  int64Grid
	)
	for lineNo, line := range s.gameConfig.Lines {
		info, ok := s.calcLineMatch(lineNo, line)
		if !ok {
			continue
		}
		winInfos = append(winInfos, info)
		for r := 0; r < _rowCount; r++ {
			for c := 0; c < _colCount; c++ {
				if info.WinGrid[r][c] > 0 {
					winGrid[r][c] = s.symbolGrid[r][c]
				}
			}
		}
	}
	s.winInfos = winInfos
	s.winGrid = winGrid
}

func (s *betOrderService) calcLineMatch(lineNo int, line []int) (WinInfo, bool) {
	var (
		symbol  int64
		count   int64
		winGrid int64Grid
	)
	for _, p := range line {
		r := p / _colCount
		c := p % _colCount
		curr := s.symbolGrid[r][c]
		if symbol == _blank {
			symbol = curr
			count++
			winGrid[r][c] = curr
			continue
		}
		if symbol == _wild || symbol == curr || curr == _wild {
			if symbol == _wild {
				symbol = curr
			}
			if symbol == _treasure {
				break
			}
			count++
			winGrid[r][c] = curr
			continue
		}
		break
	}
	if count < _minMatchCount {
		return WinInfo{}, false
	}
	odds := s.gameConfig.getSymbolBaseMultiplier(symbol, int(count))
	if odds <= 0 {
		return WinInfo{}, false
	}
	return WinInfo{
		Symbol:      symbol,
		LineCount:   int64(lineNo + 1),
		SymbolCount: count,
		Odds:        odds,
		WinGrid:     winGrid,
	}, true
}

// applyMaxWinLimit 处理“最大可赢”封顶：
// - 基础模式：当前局累计返奖不可超过 betAmount * MaxWinMultiplier，超出部分不发并直接结束当前局；
// - 免费模式：触发免费前的基础中奖 + 免费累计中奖，同样不可超过该上限，触发即直接结束免费。
func (s *betOrderService) applyMaxWinLimit() {
	if s.stepMultiplier <= 0 {
		return
	}
	oneX := s.req.BaseMoney * float64(s.req.Multiple) // 1 倍线注现金（与 updateBonusAmount 倍数分母一致）
	if oneX <= 0 {
		return
	}
	old := s.stepMultiplier
	capF := s.betAmount.InexactFloat64() * float64(s.gameConfig.MaxWinMuli)
	alreadyF := s.scene.TotalWin
	winThisF := oneX * float64(old)
	if alreadyF+winThisF < capF {
		return
	}
	// 剩余可拿（现金）不足本步理论赢时，按 oneX 的整数倍步长截断
	if leftF := capF - alreadyF; leftF < winThisF {
		if c := int64(leftF / oneX); c <= 0 {
			s.stepMultiplier = 0
		} else if c < old {
			s.stepMultiplier = c
		}
	}

	// 封顶时终止当前局，避免继续产生超额赢金。
	s.limit = true
	s.scene.FreeNum = 0
	s.addFreeTime = 0
	s.scene.Steps = 0
	s.isRoundOver = true
	s.scene.NextStage = _spinTypeBase
}
