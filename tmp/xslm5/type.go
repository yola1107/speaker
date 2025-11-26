package xslm2

type int64Grid = [_rowCount][_colCount]int64

type winInfo struct {
	Symbol      int64
	SymbolCount int64
	LineCount   int64
	WinGrid     int64Grid
}

type winResult struct {
	Symbol             int64     `json:"symbol"`
	SymbolCount        int64     `json:"symbolCount"`
	LineCount          int64     `json:"lineCount"`
	BaseLineMultiplier int64     `json:"baseLineMultiplier"`
	TotalMultiplier    int64     `json:"totalMultiplier"`
	WinGrid            int64Grid `json:"winGrid"`
}

type rtpDebugData struct {
	open bool // 是否开启调试模式
}
