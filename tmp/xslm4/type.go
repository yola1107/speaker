package xslm

type int64Grid = [_rowCount][_colCount]int64

type winInfo struct {
	Symbol      int64
	SymbolCount int64
	LineCount   int64
	WinGrid     int64Grid
}

type stepMap struct {
	ID                  int64                        `json:"id"`
	FemaleCountsForFree []int64                      `json:"fc"`
	Map                 [_rowCount * _colCount]int64 `json:"mp"`
}

type winResult struct {
	Symbol             int64     `json:"symbol"`
	SymbolCount        int64     `json:"symbolCount"`
	LineCount          int64     `json:"lineCount"`
	BaseLineMultiplier int64     `json:"baseLineMultiplier"`
	TotalMultiplier    int64     `json:"totalMultiplier"`
	WinGrid            int64Grid `json:"winGrid"`
}
