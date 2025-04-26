package encoding

import (
	"encoding/json"

	"github.com/yola1107/speaker/contrib/log/zap"
)

func ToJson(data interface{}) string {
	d, _ := json.Marshal(data)
	return string(d)
}

func Print(data interface{}) {
	zap.Log(data)
}
