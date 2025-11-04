package xslm2

import (
	"crypto/rand"
	"encoding/binary"
	mathRand "math/rand"
	"sync"
)

// 随机数生成器
var randPool = &sync.Pool{
	New: func() any {
		var seed int64
		binary.Read(rand.Reader, binary.LittleEndian, &seed)
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
