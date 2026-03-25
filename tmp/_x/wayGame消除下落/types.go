package hcsqy

type int64Grid [_rowCount][_colCount]int64

type WinInfo struct {
	Symbol      int64     `json:"val"`     // 符号值
	LineCount   int64     `json:"roadNum"` // 路数（支付线编号，从0开始）
	SymbolCount int64     `json:"starNum"` // 连续相同符号的个数
	Odds        int64     `json:"odds"`    // 基础赔率
	Multiplier  int64     `json:"mul"`     // 倍数
	WinGrid     int64Grid `json:"loc"`     // 中奖位置网格 // 位置，取值0，1，2；1表示该值占有 2为百搭
}

// rtpDebugData RTP调试数据
type rtpDebugData struct {
	open bool  // 是否开启调试模式（用于RTP测试时的详细日志输出）
	mark int32 // 与 hbtr2 日志对齐；本游戏未维护时可恒为 0
}
