package zap

import (
	"fmt"

	"github.com/yola1107/speaker/encoding"
)

func Log(data interface{}) {
	fmt.Println(data)
}

func Log2(data interface{}) {
	encoding.Print("b")
}
