package ycpd

import "github.com/shopspring/decimal"

func int64GridToArray(grid int64Grid) []int64 {
	elements := make([]int64, _rowCount*_colCount)
	for row := 0; row < _rowCount; row++ {
		for col := 0; col < _colCount; col++ {
			elements[row*_colCount+col] = grid[row][col]
		}
	}
	return elements
}

func colMultipliers(gameMul [_colCount]int64) []int64 {
	result := make([]int64, _colCount)
	for col := 0; col < _colCount; col++ {
		result[col] = gameMul[col]
	}
	return result
}

func (s *betOrderService) getScatterCount() int64 {
	var treasure int64
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.symbolGrid[r][c] == _treasure {
				treasure++
			}
		}
	}
	return treasure
}

func (s *betOrderService) checkSymbolGridWin() []WinInfo {
	var winInfos []WinInfo
	s.winGrid = int64Grid{}

	for symbol := _blank + 1; symbol < _treasure; symbol++ {
		if info, ok := s.findSymbolWinInfo(symbol); ok {
			winInfos = append(winInfos, info)
		}
	}
	for r := 0; r < _rowCount; r++ {
		for c := 1; c < _colCount-1; c++ {
			if s.winGrid[r][c] > 0 {
				s.curGameMultiple[c] += 1
				s.scene.RemoveMultiple[c] += 1
			}
		}
	}
	gameMultiple := int64(0)
	s.gameMultiple = s.scene.GameMultiple
	for c := int64(1); c < _colCount-1; c++ {
		if s.scene.RemoveMultiple[c] > 0 {
			gameMultiple += s.scene.RemoveMultiple[c]
		}
	}
	s.scene.GameMultiple = gameMultiple
	if s.gameMultiple <= 0 {
		s.gameMultiple = 1
	}
	return winInfos
}

func (s *betOrderService) findSymbolWinInfo(symbol int64) (WinInfo, bool) {
	lineCount := int64(1)
	var winGrid int64Grid

	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := 0; r < _lineNumber[c]; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || currSymbol == _wild {
				count++
				winGrid[r][c] = currSymbol
			}
		}
		if count == 0 {
			if c >= _minMatchCount {
				info := s.setWinInfo(symbol, c, lineCount, winGrid)
				return info, true
			}
			break
		}
		lineCount *= count
		if c == _colCount-1 {
			info := s.setWinInfo(symbol, _colCount, lineCount, winGrid)
			return info, true
		}
	}
	return WinInfo{}, false
}

func (s *betOrderService) setWinInfo(symbol, symbolCount, lineCount int64, winGrid int64Grid) WinInfo {
	symbolMul := s.getSymbolBaseMultiplier(symbol, int(symbolCount))
	s.lineMultiplier += lineCount * symbolMul
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if winGrid[r][c] > 0 {
				s.winGrid[r][c] = winGrid[r][c]
			}
		}
	}
	return WinInfo{
		Symbol: symbol, SymbolCount: symbolCount, LineCount: lineCount,
		Odds: symbolMul, Multiplier: lineCount * symbolMul,
	}
}

func (s *betOrderService) handleSymbolGrid() {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.symbolGrid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
}

func (s *betOrderService) updateSpinBonusAmount(bonusAmount decimal.Decimal) {
	bonAmount := bonusAmount.Round(2).InexactFloat64()
	s.scene.TotalWin = decimal.NewFromFloat(s.scene.TotalWin).Add(
		decimal.NewFromFloat(bonAmount)).Round(2).InexactFloat64()
	s.scene.RoundWin = decimal.NewFromFloat(s.scene.RoundWin).Add(
		decimal.NewFromFloat(bonAmount)).Round(2).InexactFloat64()

	if s.isFreeRound {
		s.scene.FreeWin = decimal.NewFromFloat(s.scene.FreeWin).Add(
			decimal.NewFromFloat(bonAmount)).Round(2).InexactFloat64()
	}
}

func (s *betOrderService) moveSymbols() int64Grid {
	moveSymbolGrid := s.symbolGrid

	for c := 0; c < _colCount; c++ {
		for r := 0; r < _lineNumber[c]; r++ {
			if s.winGrid[r][c] > 0 {
				moveSymbolGrid[r][c] = 0
			}
		}
	}

	for col := 0; col < _colCount; col++ {
		for row := _lineNumber[col] - 1; row >= 0; row-- {
			if moveSymbolGrid[row][col] == 0 {
				for above := row - 1; above >= 0; above-- {
					if moveSymbolGrid[above][col] != 0 {
						moveSymbolGrid[row][col] = moveSymbolGrid[above][col]
						moveSymbolGrid[above][col] = 0
						break
					}
				}
			}
		}
	}

	return moveSymbolGrid
}

func (s *betOrderService) fallingWinSymbols(moveSymbolGrid int64Grid) {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.scene.SymbolRoller[c].BoardSymbol[r] = moveSymbolGrid[r][c]
		}
	}

	for i, r := range s.scene.SymbolRoller {
		r.ringSymbol(s.gameConfig, i)
		s.scene.SymbolRoller[i] = r
	}
}
