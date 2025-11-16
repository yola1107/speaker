package xslm2

import "egame-grpc/global"

// handleSymbolGrid 从 SymbolRoller.BoardSymbol 恢复网格
// BoardSymbol 从下往上存储，需要转换为标准的 symbolGrid 坐标系统
func handleSymbolGrid(rollers [_colCount]SymbolRoller) int64Grid {
	var symbolGrid int64Grid
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			// BoardSymbol 从下往上存储，所以需要反转索引
			// symbolGrid[0][col] 对应 BoardSymbol[3]，symbolGrid[3][col] 对应 BoardSymbol[0]
			symbolGrid[_rowCount-1-r][c] = rollers[c].BoardSymbol[r]
		}
	}
	return symbolGrid
}

// fallingWinSymbols 将处理后的网格写回 SymbolRoller.BoardSymbol
// 注意：fillBlanks 已经填充了所有空白，所以这里只需要将网格写入 BoardSymbol
func fallingWinSymbols(rollers *[_colCount]SymbolRoller, nextSymbolGrid int64Grid) {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			// BoardSymbol 从下往上存储，所以需要反转索引
			rollers[c].BoardSymbol[r] = nextSymbolGrid[_rowCount-1-r][c]
		}
	}
	// ringSymbol 用于填充 BoardSymbol 中的 0（空白），但 fillBlanks 已经填充了所有空白
	// 所以这里调用 ringSymbol 是为了确保 BoardSymbol 中没有 0（防御性编程）
	for i := range rollers {
		rollers[i].ringSymbol()
	}
}

// verifyGridConsistencyWithLog 验证并记录不一致的详细信息
func verifyGridConsistencyWithLog(rollers [_colCount]SymbolRoller, nextSymbolGrid *int64Grid) bool {
	if nextSymbolGrid == nil {
		return false
	}
	restoredGrid := handleSymbolGrid(rollers)
	allMatch := true
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if restoredGrid[r][c] != (*nextSymbolGrid)[r][c] {
				allMatch = false
				global.GVA_LOG.Sugar().Warnf("网格不一致: row=%d col=%d, BoardSymbol恢复=%d, nextGrid=%d",
					r, c, restoredGrid[r][c], (*nextSymbolGrid)[r][c])
			}
		}
	}
	return allMatch
}

// verifyGridConsistency 验证从 BoardSymbol 恢复的网格与 nextSymbolGrid 是否一致
func verifyGridConsistency(rollers [_colCount]SymbolRoller, nextSymbolGrid *int64Grid) bool {
	if nextSymbolGrid == nil {
		return false
	}
	restoredGrid := handleSymbolGrid(rollers)
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if restoredGrid[r][c] != (*nextSymbolGrid)[r][c] {
				return false
			}
		}
	}
	return true
}
