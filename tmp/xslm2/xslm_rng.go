package xslm2

import (
	"crypto/rand"
	"encoding/binary"
	mathRand "math/rand"
	"sync"
)

// ========== 随机数生成器 ==========

var randPool = &sync.Pool{
	New: func() any {
		var seed int64
		binary.Read(rand.Reader, binary.LittleEndian, &seed)
		return mathRand.New(mathRand.NewSource(seed))
	},
}
