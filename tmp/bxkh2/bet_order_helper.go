// game/bxkh2/bet_order_helper.go
package bxkh2

import (
	"egame-grpc/game/common/rand"
)

// getScatterCount 获取 Scatter 数量
func (s *betOrderService) getScatterCount(grid int64Grid) int64 {
	var count int64
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if grid[r][c] == _treasure {
				count++
			}
		}
	}
	return count
}

// checkSymbolGridWin 检查中奖
func (s *betOrderService) checkSymbolGridWin() []*winInfo {
	var winInfos []*winInfo
	s.winGrid = int64Grid{}
	s.longWinGrid = int64Grid{}

	for symbol := int64(1); symbol < _treasure; symbol++ {
		if info, ok := s.findSymbolWinInfo(symbol); ok {
			winInfos = append(winInfos, info)
		}
	}
	s.winInfos = winInfos
	return winInfos
}

// findSymbolWinInfo 查找符号中奖信息
func (s *betOrderService) findSymbolWinInfo(symbol int64) (*winInfo, bool) {
	lineCount := int64(1)
	var winGrid int64Grid
	var longWinGrid int64Grid

	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || currSymbol == _wild {
				count++
				winGrid[r][c] = currSymbol
				for row := r + 1; row < _rowCount && s.symbolGrid[row][c] > _longSymbol; row++ {
					winGrid[row][c] = s.symbolGrid[row][c]
				}
			} else if currSymbol > _silverSymbol && currSymbol < _longSymbol {
				if (currSymbol%_silverSymbol) == symbol || (currSymbol%_silverSymbol) == _wild {
					count++
					winGrid[r][c] = currSymbol
					for row := r + 1; row < _rowCount && s.symbolGrid[row][c] > _longSymbol; row++ {
						winGrid[row][c] = s.symbolGrid[row][c]
						longWinGrid[row][c] = s.symbolGrid[row][c]
					}
					longWinGrid[r][c] = currSymbol
				}
			}
		}

		if count == 0 {
			if c >= _minMatchCount {
				odds := s.getSymbolBaseMultiplier(symbol, int(c))
				info := &winInfo{
					Symbol:      symbol,
					SymbolCount: c,
					LineCount:   lineCount,
					Odds:        odds,
					Multiplier:  lineCount * odds,
					WinGrid:     winGrid,
				}
				s.mergeWinGrid(winGrid, longWinGrid)
				return info, true
			}
			break
		}
		lineCount *= count

		if c == _colCount-1 {
			odds := s.getSymbolBaseMultiplier(symbol, int(_colCount))
			info := &winInfo{
				Symbol:      symbol,
				SymbolCount: _colCount,
				LineCount:   lineCount,
				Odds:        odds,
				Multiplier:  lineCount * odds,
				WinGrid:     winGrid,
			}
			s.mergeWinGrid(winGrid, longWinGrid)
			return info, true
		}
	}
	return nil, false
}

func (s *betOrderService) mergeWinGrid(winGrid, longWinGrid int64Grid) {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if winGrid[r][c] > 0 {
				s.winGrid[r][c] = winGrid[r][c]
			}
			if longWinGrid[r][c] > 0 {
				s.longWinGrid[r][c] = longWinGrid[r][c]
			}
		}
	}
}

// handleWinInfosMultiplier 处理中奖倍数
func (s *betOrderService) handleWinInfosMultiplier(infos []*winInfo) int64 {
	var total int64
	for _, info := range infos {
		total += info.Multiplier
	}
	return total
}

// moveSymbols 移动消除符号
func (s *betOrderService) moveSymbols(stage int8) int64Grid {
	nextGrid := s.symbolGrid

	// 消除中奖位置
	for c := int64(0); c < _colCount; c++ {
		for r := int64(0); r < _rowCount; r++ {
			if s.winGrid[r][c] > 0 {
				nextGrid[r][c] = 0
			}
		}
	}

	// 银符号和金符号转化
	for c := int64(1); c < _colCount-1; c++ {
		for r := int64(0); r < _rowCount; r++ {
			if s.longWinGrid[r][c] == 0 {
				continue
			}
			goldenSymbol := s.longWinGrid[r][c]

			// 银符号变金符号
			if goldenSymbol > _silverSymbol && goldenSymbol < _goldenSymbol {
				newSymbol := s.randSymbol(goldenSymbol % _silverSymbol)
				nextGrid[r][c] = _goldenSymbol + newSymbol
				s.originalSymbolGrid[r][c] = _goldenSymbol + newSymbol

				rowTmp := int64(0)
				for i := r + 1; i < _rowCount; i++ {
					if s.longWinGrid[i][c] > _longSymbol {
						nextGrid[i][c] = _longSymbol + _goldenSymbol + newSymbol
						s.originalSymbolGrid[i][c] = _longSymbol + _goldenSymbol + newSymbol
						rowTmp++
					} else {
						break
					}
				}
				r += rowTmp
			} else if goldenSymbol > _goldenSymbol && goldenSymbol < _longSymbol {
				// 金符号变 Wild
				nextGrid[r][c] = _wild
				rowTmp := int64(0)
				for i := r + 1; i < _rowCount; i++ {
					if s.longWinGrid[i][c] > _longSymbol {
						nextGrid[i][c] = _wild
						rowTmp++
					} else {
						break
					}
				}
				r += rowTmp
			}
		}
	}

	// 符号下沉
	for col := 0; col < _colCount; col++ {
		for row := _rowCount - 1; row >= 0; row-- {
			if nextGrid[row][col] == 0 {
				for above := row - 1; above >= 0; above-- {
					if nextGrid[above][col] != 0 {
						nextGrid[row][col] = nextGrid[above][col]
						nextGrid[above][col] = 0
						break
					}
				}
			}
		}
	}

	return nextGrid
}

// randSymbol 随机一个不同于当前符号的符号
func (s *betOrderService) randSymbol(symbol int64) int64 {
	var available []int64
	for i := int64(1); i < _treasure; i++ {
		if i != symbol {
			available = append(available, i)
		}
	}
	return available[rand.IntN(len(available))]
}

// fallingSymbols 符号掉落补充
func (s *betOrderService) fallingSymbols() {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.scene.SymbolRoller[c].BoardSymbol[r] = s.moveSymbolGrid[r][c]
		}
	}
	for i, r := range s.scene.SymbolRoller {
		r.ringSymbol(s.gameConfig)
		s.scene.SymbolRoller[i] = r
	}
}
