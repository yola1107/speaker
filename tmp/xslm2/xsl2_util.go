package xslm2

import (
	"crypto/rand"
	"encoding/binary"
	mathRand "math/rand"
	"strconv"
	"strings"
	"sync"
)

// 随机数生成器
var randPool = &sync.Pool{
	New: func() any {
		var seed int64
		_ = binary.Read(rand.Reader, binary.LittleEndian, &seed)
		return mathRand.New(mathRand.NewSource(seed))
	},
}

// contains 检查值是否在切片中
func contains[T comparable](slice []T, val T) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// gridToString 网格转字符串（通用函数）
func gridToString(grid *int64Grid) string {
	var b strings.Builder
	for r := int64(0); r < _rowCount; r++ {
		b.WriteString("[")
		for c := int64(0); c < _colCount; c++ {
			b.WriteString(strconv.FormatInt(grid[r][c], 10))
			if c < _colCount-1 {
				b.WriteString(",")
			}
		}
		b.WriteString("]")
		if r < _rowCount-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// hasWildSymbol 判断符号网格中是否有Wild符号
func hasWildSymbol(grid *int64Grid) bool {
	for _, row := range grid {
		for _, symbol := range row {
			if symbol == _wild {
				return true
			}
		}
	}
	return false
}

// getTreasureCount 获取符号网格中的夺宝符号数量
func getTreasureCount(grid *int64Grid) int64 {
	return countSymbol(grid, _treasure)
}

// countSymbol 统计符号网格中指定符号的数量（通用函数）
func countSymbol(grid *int64Grid, targetSymbol int64) int64 {
	count := int64(0)
	for _, row := range grid {
		for _, symbol := range row {
			if symbol == targetSymbol {
				count++
			}
		}
	}
	return count
}

// mergeWinGrids 合并多个中奖网格到目标网格
func mergeWinGrids(target *int64Grid, sources []int64Grid) {
	for _, source := range sources {
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if source[r][c] != _blank {
					target[r][c] = source[r][c]
				}
			}
		}
	}
}

// checkAllFemaleCountsFull 检查所有女性符号计数是否都达到阈值
func checkAllFemaleCountsFull(counts [_femaleC - _femaleA + 1]int64) bool {
	for _, c := range counts {
		if c < _femaleSymbolCountForFullElimination {
			return false
		}
	}
	return true
}

// isFemaleSymbol 判断是否为女性符号（7-9）
func isFemaleSymbol(symbol int64) bool {
	return symbol >= _femaleA && symbol <= _femaleC
}

// isWildSymbol 判断是否为Wild相关符号（10-13）
func isWildSymbol(symbol int64) bool {
	return symbol >= _wildFemaleA && symbol <= _wild
}
