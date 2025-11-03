package xslm2

// int64Grid 符号网格类型（4行×5列）
type int64Grid = [_rowCount][_colCount]int64

// winInfo Ways中奖信息（内部计算用）
type winInfo struct {
	Symbol      int64     // 符号ID
	SymbolCount int64     // 连续列数
	LineCount   int64     // Ways路数
	WinGrid     int64Grid // 中奖位置网格
}

// stepMap 预设数据的单个step结构
type stepMap struct {
	ID                  int64                        `json:"id"` // step编号
	FemaleCountsForFree []int64                      `json:"fc"` // 女性符号收集计数 [A, B, C]
	Map                 [_rowCount * _colCount]int64 `json:"mp"` // 符号网格（一维数组）
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
