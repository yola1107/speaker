package xxg2

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	mathRand "math/rand"
	"strconv"
	"strings"
	"sync"
)

// randPool 随机数生成器对象池
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

// ToJSONPretty converts any value to a pretty-printed JSON string.
// If encoding fails, it returns the error string.
func ToJSONPretty(v any) string {
	j, err := json.MarshalIndent(v, "", "  ") // 使用两个空格缩进
	if err != nil {
		return err.Error()
	}
	return string(j)
}

// gridToString 网格转字符串
func gridToString(grid *int64Grid) string {
	if grid == nil {
		return ""
	}

	var b strings.Builder
	b.Grow(int(_rowCount * _colCount * gridStringCapacity))

	sn := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			b.WriteString(strconv.Itoa(sn))
			b.WriteByte(':')
			b.WriteString(strconv.FormatInt(grid[row][col], 10))
			b.WriteString("; ")
			sn++
		}
	}
	return b.String()
}

// reverseBats 交换bat的X/Y坐标(服务器行列→客户端列行)
func reverseBats(bats []*Bat) []*Bat {
	if len(bats) == 0 {
		return []*Bat{}
	}
	reversed := make([]*Bat, len(bats))
	for i, bat := range bats {
		reversed[i] = &Bat{
			X:      bat.Y,
			Y:      bat.X,
			TransX: bat.TransY,
			TransY: bat.TransX,
			Syb:    bat.Syb,
			Sybn:   bat.Sybn,
		}
	}
	return reversed
}

// isHumanSymbol 判断是否为人符号(7/8/9)
func isHumanSymbol(symbol int64) bool {
	return symbol == _child || symbol == _woman || symbol == _oldMan
}

// newBat 创建蝙蝠移动记录
func newBat(from, to *position, oldSym, newSym int64) *Bat {
	return &Bat{
		X:      from.Row,
		Y:      from.Col,
		TransX: to.Row,
		TransY: to.Col,
		Syb:    oldSym,
		Sybn:   newSym,
	}
}
