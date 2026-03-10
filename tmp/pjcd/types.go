package pjcd

// int64Grid 3行5列符号网格
type int64Grid [_rowCount][_colCount]int64

// WildStateGrid 3行5列百搭状态网格
type WildStateGrid [_rowCount][_colCount]int8

// WinInfo 中奖信息
type WinInfo struct {
	Symbol      int64     // 中奖符号ID
	SymbolCount int64     // 连续符号数量
	LineIndex   int64     // 中奖线索引
	Odds        int64     // 赔付倍数
	Multiplier  int64     // 实际倍数（赔率 × 轮次倍数）
	WinGrid     int64Grid // 中奖网格
}

// SymbolRoller 符号滚轮
type SymbolRoller struct {
	BoardSymbol []int64 `json:"board"` // 滚轮上的符号（从下到上）
	Position    int     `json:"pos"`   // 当前位置
	Length      int     `json:"len"`   // 滚轮长度
}

// rtpDebugData RTP测试调试数据
type rtpDebugData struct {
	open   bool
	symbol int64
	count  int64
}
