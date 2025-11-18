package xslm

import (
	cryptorand "crypto/rand"
	"egame-grpc/global"
	"encoding/binary"
	"go.uber.org/zap"
	"math/rand"
	"sync"
	"time"
)

func getSeed() int64 {
	var seed int64
	if err := binary.Read(cryptorand.Reader, binary.BigEndian, &seed); err != nil {
		global.GVA_LOG.Error("getSeed", zap.Error(err))
		return time.Now().UnixNano()
	}
	return seed
}

var randPool = sync.Pool{
	New: func() interface{} {
		return rand.New(rand.NewSource(getSeed()))
	},
}

var _freeRounds = []int64{7, 10, 15}

var _symbolMultiplierGroups = [][]int64{
	{2, 3, 5},
	{2, 3, 5},
	{2, 3, 5},
	{2, 3, 5},
	{5, 8, 12},
	{6, 10, 15},
	{10, 15, 25},
	{10, 15, 25},
	{10, 15, 25},
	{15, 25, 40},
	{15, 25, 40},
	{15, 25, 40},
}
