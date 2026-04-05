package yhwy

// int64Grid 表示 4x5 盘面，按 [行][列] 存储。
type int64Grid [_rowCount][_colCount]int64

// WinInfo 描述单条中奖线的结算结果。
type WinInfo struct {
	Symbol      int64     `json:"symbol"` // 命中的主符号 ID
	SymbolCount int64     `json:"count"`  // 从左到右连续命中的个数
	LineCount   int64     `json:"line"`   // 中奖线编号，从 0 开始
	Odds        int64     `json:"odds"`   // 该中奖线对应的赔率倍数
	WinGrid     int64Grid `json:"grid"`   // 该中奖线命中的位置掩码
}

// rtpDebugData 控制本地 RTP / 冒烟测试时的调试行为。
type rtpDebugData struct {
	open bool // true 表示启用测试模式，不走线上缓存/扣费逻辑

	origin    int64Grid // 原始符号
	sakuraCol int       // 樱吹雪替换到的最远列（3/4/5） 默认-1
	mysSymbol int64     // 百变樱花本次统一揭示出的目标符号
}
