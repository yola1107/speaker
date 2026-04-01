package ajtm

// int64Grid 表示 6x5 盘面网格。
type int64Grid [_rowCount][_colCount]int64

// WinInfo 表示一个符号在当前 step 的 Ways 中奖结果。
type WinInfo struct {
	Symbol        int64     `json:"val"`     // 中奖符号 ID
	LineCount     int64     `json:"roadNum"` // 路数
	SymbolCount   int64     `json:"starNum"` // 连续命中列数
	Odds          int64     `json:"odds"`    // 基础赔率
	Multiplier    int64     `json:"mul"`     // 结算倍数 = Odds * LineCount
	MysMultiplier int64     `json:"mysMul"`  // 神秘符号累计倍数
	WinGrid       int64Grid `json:"loc"`     // 该符号的中奖网格
}

// Block 统一描述长符号块。
// 布局阶段只关心 Col/HeadRow/TailRow，
// 中奖转变阶段会补充 OldSymbol/NewSymbol。
type Block struct {
	Col       int64 `json:"col"`       // 列号
	HeadRow   int64 `json:"headRow"`   // 长符号头部行号
	TailRow   int64 `json:"tailRow"`   // 长符号尾部行号，固定等于 HeadRow+1
	OldSymbol int64 `json:"oldSymbol"` // 转变前符号
	NewSymbol int64 `json:"newSymbol"` // 转变后符号
}

// rtpDebugData 保存 RTP 调试状态。
type rtpDebugData struct {
	open             bool      // 是否开启 RTP 调试
	mark             int32     // 调试日志对齐标记
	originSymbolGrid int64Grid //
}
