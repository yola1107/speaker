package ycj

type int64Grid [_rowCount][_colCount]int64

type WinResult struct {
	Win           bool    `json:"win"`           // 是否中奖
	Multiplier    float64 `json:"multiplier"`    // 返奖倍数
	TriggerExtend bool    `json:"triggerExtend"` // 触发推展模式
	TriggerRespin bool    `json:"triggerRespin"` // 触发重转模式
	TriggerFree   bool    `json:"triggerFree"`   // 触发夺宝模式
	FreeSpinNum   int64   `json:"freeSpinNum"`   // 免费次数
}

type WinDetails struct {
	FreeWin      float64 `json:"freeWin"`      // 免费游戏中累计赢取
	TotalWin     float64 `json:"totalWin"`     // 本回合总赢取
	IsRoundOver  bool    `json:"isRoundOver"`  // 是否结束
	NewFreeTimes int64   `json:"newFreeTimes"` // 本步新增的免费旋转次数
	Next         bool    `json:"next"`         // 是否进入下一状态/下一局 (推展/重转补判未完成）
	StepMul      float64 `json:"stepMul"`      // 返奖倍数
}

type rtpDebugData struct {
	open bool // 是否开启调试模式
}
