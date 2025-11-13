package xslm2

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
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

// ToJSON json string
func ToJSON(v any) string {
	j, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(j)
}

func WGrid(grid *int64Grid) string {
	if grid == nil {
		return ""
	}
	b := strings.Builder{}
	b.WriteString("\n")
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			sym := grid[r][c]
			b.WriteString(fmt.Sprintf("%3d", sym))
			if c < _colCount-1 {
				b.WriteString("| ")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// gridToString 网格转字符串（通用函数）
func gridToString(grid *int64Grid) string {
	if grid == nil {
		return ""
	}

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
	if grid == nil {
		return false
	}
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
算分：女性百搭（10，11，12）可替换为基础符号（1，2，3，4，5，6，7，8，9），但连线上必须要有基础符号

消除：
	基础模式：消除中奖的女性符号（7，8，9）及百搭，如果盘面有夺宝则百搭不消除
	免费模式：
		1> 全屏情况：当触发111时（ABC都大于10）， 女性百搭参与的中奖符号（1-12）全部消除，（普通百搭 13 保留、夺宝 14 保留）
		2> 非全屏情况：中奖的女性符号会消除（7，8，9，10，11，12），夺宝符号和百搭不消除
*/

//
// v2
/*
算分：女性百搭（10，11，12）可替换为基础符号（1，2，3，4，5，6，7，8，9），但连线上必须要有基础符号

消除：
	基础模式：消除中奖的女性符号（7，8，9）及百搭，如果盘面有夺宝则百搭不消除
	免费模式：
		1> 全屏情况：每个中奖Way找女性百搭，找到则改way除百搭13之外的符号都全部消除
		2> 非全屏情况：每个中奖way找女性，找到该way女性及女性百搭都消除
*/
