package bxkh2

// int64Grid 以'行'为组的符号网格
type int64Grid [_rowCount][_colCount]int64

// winInfo 中奖信息
type winInfo struct {
	Symbol      int64
	SymbolCount int64
	LineCount   int64
	Odds        int64
	Multiplier  int64
	WinGrid     int64Grid
}

// WinElem 中奖元素
type WinElem struct {
	Val     int64 `json:"val"`
	RoadNum int64 `json:"roadNum"`
	StarNum int64 `json:"starNum"`
	Odds    int64 `json:"odds"`
	Mul     int64 `json:"mul"`
}

// BaseSpinResult 基础旋转结果
type BaseSpinResult struct {
	lineMultiplier     int64 // 线倍数
	stepMultiplier     int64 // 总倍数
	scatterCount       int64 // 夺宝符个数
	addFreeTime        int64
	freeMultiple       int64 // 免费倍数，初始1倍
	removeNum          int64 // 免费游戏中奖消除次数
	SpinOver           bool
	winGrid            int64Grid
	cards              int64Grid
	moveSymbolGrid     int64Grid
	longSymbolTail     [12][6]int64 // 长符号每列最多2个最多12个，最后一位放长度
	silverSymbolTail   [12][6]int64 // 金符号每列最多2个最多12个，最后一位放长度
	originalSymbolGrid int64Grid    // 原始符号网格
	winInfo            WinInfo
	winResult          []winResult
}

// winResult 中奖结果
type winResult struct {
	Symbol   int64 `json:"sym"`
	SymCnt   int64 `json:"scnt"`
	LineCnt  int64 `json:"lCnt"`
	BaseMult int64 `json:"baseM"`
	TotalMul int64 `json:"totalM"`
}

// WinInfo 中奖信息（用于返回前端）
type WinInfo struct {
	Next          bool      `json:"next"`
	Over          bool      `json:"over"`
	Multi         int64     `json:"multi"`
	State         int8      `json:"state"`
	FreeNum       uint64    `json:"freeNum"`
	FreeTime      uint64    `json:"freeTime"`
	TotalFreeTime uint64    `json:"totalFreeTime"`
	IsRoundOver   bool      `json:"isRoundOver"`  // 回合是否结束
	AddFreeTime   int64     `json:"addFreeTime"`  // 增加免费次数
	ScatterCount  int64     `json:"scatterCount"` // 夺宝符
	WinGrid       int64Grid `json:"winGrid"`      // 中奖位置
	WinArr        []WinElem `json:"winArr"`
}
