package clzw

func int64GridToArray(grid int64Grid) []int64 {
	elements := make([]int64, _rowCount*_colCount)
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			elements[r*_colCount+c] = grid[r][c]
		}
	}
	return elements
}

func symbolCount(grid int64Grid, symbol int64) int {
	var count int
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if grid[r][c] == symbol {
				count++
			}
		}
	}
	return count
}

func (s *betOrderService) getScatterCount() int64 {
	return int64(symbolCount(s.symbolGrid, _treasure))
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

// findWinInfos 查找中奖信息（Ways玩法：从左到右连续匹配）
func (s *betOrderService) findWinInfos() {
	winInfos := make([]WinInfo, 0, _wild-_blank-1)
	var totalWinGrid int64Grid

	seen := make(map[int64]struct{})
	checkSymbols := make([]int64, 0, _rowCount)
	for row := 0; row < _rowCount; row++ {
		if _, ok := seen[s.symbolGrid[row][0]]; !ok && s.symbolGrid[row][0] < _wild {
			seen[s.symbolGrid[row][0]] = struct{}{}
			checkSymbols = append(checkSymbols, s.symbolGrid[row][0])
		}
	}

	for _, symbol := range checkSymbols {
		info, ok := s.findSymbolWinInfo(symbol, false)
		if !ok {
			continue
		}
		winInfos = append(winInfos, *info)
		// 合并中奖位置到总网格（用于消除）
		for r := 0; r < _rowCount; r++ {
			for c := int64(0); c < info.SymbolCount; c++ {
				if info.WinGrid[r][c] != 0 {
					totalWinGrid[r][c] = info.WinGrid[r][c]
				}
			}
		}
	}

	s.winInfos = winInfos
	s.winGrid = totalWinGrid
}

// findSymbolWinInfo 查找符号中奖（Ways玩法：从左到右连续，至少3列，Wild可替代）
func (s *betOrderService) findSymbolWinInfo(symbol int64, _ bool) (*WinInfo, bool) {
	hasRealSymbol := false
	lineCount := int64(1)
	var winGrid int64Grid

	// 逐列扫描，统计匹配的符号
	for c := 0; c < _colCount; c++ {
		matchCount := int64(0)
		for r := 0; r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || currSymbol == _wild {
				if currSymbol == symbol {
					hasRealSymbol = true
				}
				matchCount++
				winGrid[r][c] = currSymbol // 存储实际符号值
			}
		}

		// 当前列没有匹配
		if matchCount == 0 {
			if c >= _minMatchCount && hasRealSymbol {
				if odds := s.gameConfig.getSymbolBaseMultiplier(symbol, c); odds > 0 {
					return &WinInfo{Symbol: symbol, SymbolCount: int64(c), LineCount: lineCount, Odds: odds, Multiplier: odds * lineCount, WinGrid: winGrid}, true
				}
			}
			return nil, false
		}

		// 计算路数：每列匹配数相乘
		lineCount *= matchCount

		// 如果到了最后一列且有真实符号，返回中奖信息
		if c == _colCount-1 && hasRealSymbol {
			if odds := s.gameConfig.getSymbolBaseMultiplier(symbol, _colCount); odds > 0 {
				return &WinInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, Odds: odds, Multiplier: odds * lineCount, WinGrid: winGrid}, true
			}
		}
	}

	return nil, false
}

func (s *betOrderService) getStepMultiplier() int64 {
	multiArr := s.gameConfig.BaseMulti
	if s.isFreeRound {
		multiArr = s.gameConfig.FreeMulti
	}
	idx := int(s.scene.Steps)
	if idx >= len(multiArr) {
		idx = len(multiArr) - 1
	}
	return multiArr[idx]
}

// getLineMultiByTier 获取狮子王倍数 3个及以上
// 最高赔付图标，独立计算，不和wild结合，全盘任何地方超过3个即可直接赔付对应的奖金，奖金独立计算，不参与中奖时的倍数翻倍
func (s *betOrderService) applyLionMultiplier() {
	if !s.isRoundOver {
		return
	}
	count := symbolCount(s.symbolGrid, _lion)
	if count < 3 {
		return
	}
	idx := count - 3
	if idx < 0 || idx >= len(s.gameConfig.LionMulti) {
		return
	}
	s.stepMultiplier += s.gameConfig.LionMulti[idx]
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
	s.scene.PurchaseAmount = 0
	s.addFreeTime = 0
	s.scene.Steps = 0
	s.isRoundOver = true
	s.scene.NextStage = _spinTypeBase
}
