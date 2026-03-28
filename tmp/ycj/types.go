package ycj

// 1x3 符号网格 (1行3列)
type int64Grid [_rowCount][_colCount]int64

// 判奖结果
type WinResult struct {
	Win           bool    `json:"win"`           // 是否中奖
	Multiplier    float64 `json:"multiplier"`    // 返奖倍数
	TriggerExtend bool    `json:"triggerExtend"` // 触发推展模式
	TriggerRespin bool    `json:"triggerRespin"` // 触发重转模式
	TriggerFree   bool    `json:"triggerFree"`   // 触发夺宝模式
	FreeSpinNum   int64   `json:"freeSpinNum"`   // 免费次数
}

// rtpDebugData RTP调试数据
type rtpDebugData struct {
	open       bool // 是否开启调试模式
	extendUsed bool // 本步是否进入推展模式
	respinUsed bool // 本步是否进入重转模式
}
