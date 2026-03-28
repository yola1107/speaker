package hcsqy

// 3x3 符号网格
type int64Grid [_rowCount][_colCount]int64

// 中奖信息
type WinInfo struct {
	Symbol      int64     `json:"val"`     // 符号值
	SymbolCount int64     `json:"starNum"` // 连续相同符号的个数
	LineCount   int64     `json:"roadNum"` // 支付线编号
	Odds        int64     `json:"odds"`    // 赔率
	WinGrid     int64Grid `json:"loc"`     // 中奖位置
}

// rtpDebugData RTP调试数据
type rtpDebugData struct {
	open bool         // 是否开启调试模式（用于RTP测试时的详细日志输出）
	mode spinExecMode //
}
