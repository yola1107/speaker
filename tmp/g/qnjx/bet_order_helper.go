package qnjx

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

func (s *betOrderService) handleSymbolGrid() {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.symbolGrid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
}

// moveSymbols 按 winGrid 清除并下落。
func (s *betOrderService) moveSymbols() int64Grid {
	next := s.symbolGrid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.winGrid[r][c] > 0 {
				next[r][c] = 0
			}
		}
	}
	s.dropSymbols(&next)
	return next
}

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

func (s *betOrderService) fallingWinSymbols(next int64Grid) {
	for col := range s.scene.SymbolRoller {
		roller := &s.scene.SymbolRoller[col]
		for r := 0; r < _rowCount; r++ {
			roller.BoardSymbol[r] = next[r][col]
		}
		roller.fillByFallPatternOrRing(s.gameConfig, s.isFreeRound)
	}
}

func (s *betOrderService) findWinInfos() {
	winInfos := s.winInfos[:0]
	if cap(winInfos) < int(_wild-1) {
		winInfos = make([]WinInfo, 0, _wild-1)
	}
	var winGrid int64Grid

	for symbol := _blank + 1; symbol < _wild; symbol++ {
		info, ok := s.findSymbolWinInfo(symbol)
		if !ok {
			continue
		}

		winInfos = append(winInfos, *info)
		s.mergeWinGrids(info.WinGrid, &winGrid)
	}

	s.winInfos = winInfos
	s.winGrid = winGrid
}

// findSymbolWinInfo 按 Ways 从左到右判定，至少命中 3 列。
func (s *betOrderService) findSymbolWinInfo(symbol int64) (*WinInfo, bool) {
	lineCount := int64(1)
	var winGrid int64Grid
	hasRealSymbol := false

	sum := int64(0)

	for c := 0; c < _colCount; c++ {
		matchCount := 0

		for r := 0; r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || currSymbol == _wild {
				if currSymbol == symbol {
					hasRealSymbol = true
				}
				matchCount++
				winGrid[r][c] = currSymbol
			}
		}

		sum += int64(matchCount)
		if matchCount == 0 {
			if c >= _minMatchCount && hasRealSymbol {
				if odds := s.getSymbolBaseMultiplier(symbol, c); odds > 0 {
					return &WinInfo{
						Symbol:      symbol,
						SymbolCount: int64(c),
						LineCount:   lineCount,
						Odds:        odds,
						Multiplier:  odds * lineCount,
						WinGrid:     winGrid,
						Num:         sum,
					}, true
				}
			}
			return nil, false
		}

		lineCount *= int64(matchCount)
		if c == _colCount-1 && hasRealSymbol {
			if odds := s.getSymbolBaseMultiplier(symbol, _colCount); odds > 0 {
				return &WinInfo{
					Symbol:      symbol,
					SymbolCount: int64(c),
					LineCount:   lineCount,
					Odds:        odds,
					Multiplier:  odds * lineCount,
					WinGrid:     winGrid,
					Num:         sum,
				}, true
			}
		}
	}
	return nil, false
}

func (s *betOrderService) mergeWinGrids(src int64Grid, winGrid *int64Grid) {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			cell := src[r][c]
			if cell <= 0 {
				continue
			}

			head := s.symbolGrid[r][c]
			longLen := s.getLongLenAt(r, c)
			if longLen >= 2 {
				(*winGrid)[r][c] = head
				for offset := 1; offset < longLen && r+offset < _rowCount; offset++ {
					(*winGrid)[r+offset][c] = s.symbolGrid[r+offset][c]
				}
				continue
			}
			(*winGrid)[r][c] = cell
		}
	}
}
func (s *betOrderService) getLongLenAt(r, c int) int {
	if r < 0 || r >= _rowCount || c < 0 || c >= _colCount {
		return 0
	}
	head := s.symbolGrid[r][c]
	if head <= 0 || head >= _longSymbol {
		return 0
	}

	longLen := 1
	for row := r + 1; row < _rowCount; row++ {
		if s.symbolGrid[row][c] != _longSymbol+head {
			break
		}
		longLen++
	}
	if longLen < 2 {
		return 0
	}
	return longLen
}

// applyMaxWinMultiplierLimit 处理“最大可赢”封顶：
// - 基础模式：当前局累计返奖不可超过 betAmount * MaxWinMultiplier，超出部分不发并直接结束当前局；
// - 免费模式：触发免费前的基础中奖 + 免费累计中奖，同样不可超过该上限，触发即直接结束免费。
func (s *betOrderService) applyMaxWinMultiplierLimit() {
	if s.stepMultiplier <= 0 {
		return
	}
	oneX := s.req.BaseMoney * float64(s.req.Multiple) // 1 倍线注现金（与 updateBonusAmount 倍数分母一致）
	if oneX <= 0 {
		return
	}
	old := s.stepMultiplier
	capF := s.betAmount.InexactFloat64() * float64(s.gameConfig.MaxWinMultiplier)
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
	s.mysMul = 0
}

func (s *betOrderService) collectWinningSymbols() {
	added := [3]int64{0, 0, 0}
	for _, info := range s.winInfos {
		color := colorIndex(info.Symbol)
		added[color] += info.Num
	}
	for c, n := range added {
		s.scene.ColorCount[c] += n
		if s.scene.ColorCount[c] >= 5 {
			m := s.scene.ColorCount[c] / 5
			s.scene.ColorMul[c] += m
			s.scene.ColorCount[c] -= m * 5
		}
	}
	if s.debug.open {
		s.debug.added = added
	}
}

// 绿：(1,4,7) 蓝: (2,5,8) 黄：(3,6,9)
func colorIndex(symbol int64) int {
	switch symbol {
	case 1, 4, 7:
		return 0
	case 2, 5, 8:
		return 1
	case 3, 6, 9:
		return 2
	default:
		return -1
	}
}
