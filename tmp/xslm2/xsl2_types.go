package xslm2

// int64Grid 符号网格类型（4行×5列）
type int64Grid = [_rowCount][_colCount]int64

// rtpDebugData RTP压测调试数据
type rtpDebugData struct {
	open bool // 是否开启调试模式
}

// winInfo Ways中奖信息（内部计算用）
type winInfo struct {
	Symbol      int64     // 符号ID
	SymbolCount int64     // 连续列数
	LineCount   int64     // Ways路数
	WinGrid     int64Grid // 中奖位置网格
}

// winResult 中奖结果（返回给客户端）
type winResult struct {
	Symbol             int64     `json:"symbol"`             // 符号ID
	SymbolCount        int64     `json:"symbolCount"`        // 连续列数
	LineCount          int64     `json:"lineCount"`          // Ways路数
	BaseLineMultiplier int64     `json:"baseLineMultiplier"` // 基础倍率
	TotalMultiplier    int64     `json:"totalMultiplier"`    // 总倍率
	WinGrid            int64Grid `json:"winGrid"`            // 中奖位置网格
}
