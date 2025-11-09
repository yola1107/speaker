package xslm2

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
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
	display := convertGridForDisplay(grid)
	if display == nil {
		return ""
	}
	var b strings.Builder
	for i, row := range display {
		b.WriteString("[")
		for j, val := range row {
			b.WriteString(strconv.FormatInt(val, 10))
			if j < len(row)-1 {
				b.WriteString(",")
			}
		}
		b.WriteString("]")
		if i < len(display)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// hasWildSymbol 判断符号网格中是否有Wild符号
func hasWildSymbol(grid *int64Grid) bool {
	if grid == nil {
		return false
	}
	for _, row := range grid {
		for _, symbol := range row {
			// 跳过墙格标记
			if symbol == _blocked {
				continue
			}
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
			// 跳过墙格标记
			if symbol == _blocked {
				continue
			}
			if symbol == targetSymbol {
				count++
			}
		}
	}
	return count
}

/*
// mergeWinGrids 合并多个中奖网格到目标网格
func mergeWinGrids(target *int64Grid, sources []int64Grid) {
	for _, source := range sources {
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				// 跳过墙格标记
				if source[r][c] == _blocked {
					continue
				}
				if source[r][c] != _blank {
					target[r][c] = source[r][c]
				}
			}
		}
	}
}

// isFemaleSymbol 判断是否为女性符号（7-9）
func isFemaleSymbol(symbol int64) bool {
	return symbol >= _femaleA && symbol <= _femaleC
}

// isFemaleWinSymbol 判断是否为触发女性连消的符号（含女性百搭）
func isFemaleWinSymbol(symbol int64) bool {
	return (symbol >= _femaleA && symbol <= _femaleC) || (symbol >= _wildFemaleA && symbol <= _wildFemaleC)
}

// isWildSymbol 判断是否为Wild相关符号（10-13）
func isWildSymbol(symbol int64) bool {
	return symbol >= _wildFemaleA && symbol <= _wild
}

// getFemaleCountsKey 生成配置key（"000"~"111"）
func getFemaleCountsKey(counts [3]int64) string {
	var result [3]byte
	for i := 0; i < 3; i++ {
		if counts[i] >= _femaleSymbolCountForFullElimination {
			result[i] = '1'
		} else {
			result[i] = '0'
		}
	}
	return string(result[:])
}
*/

// ToJSON json string
func ToJSON(v any) string {
	j, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(j)
}

/*
总结对比表
模式	                 消除条件	            消除范围	                         保护规则
基础模式  	  hasFemaleWin && hasWild	   只消除女性符号(7,8,9)和wild(13)	    treasure不消除；wild有treasure时不消除
免费模式-全屏	  enableFullElimination=true   消除所有中奖符号	                treasure和wild不消除
免费模式-部分	  hasFemaleWin=true	           只消除女性符号(7,8,9)	            treasure和wild不消除
免费模式-其他	  都不满足	                     不消除	                         -
*/

/*
每次 betOrder 只做一次消除，消除后立即下落和填充，保存处理后的网格供下次使用。即使还有消除，也要等下次调用。
betOrder->initSpinSymbol->findWinInfos-> 消除（根据winResult单次消除）->  有符号中奖消除时，滚轮上方的符号掉落补位-> 保存后续网格（即便还有消除）-> 下次betOrder
*/
