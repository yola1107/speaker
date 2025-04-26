package main

import (
	"github.com/yola1107/speaker/contrib/log/zap"
	"github.com/yola1107/speaker/encoding"
)

func main() {
	zap.Log("abc")
	encoding.Print(encoding.ToJson("123def"))
}
