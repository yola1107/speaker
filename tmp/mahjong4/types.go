package mahjong4

// 以'行'为组
type int64Grid [_rowCount][_colCount]int64

// 奖励专用少一行
type int64GridW [_rowCountReward][_colCount]int64

// WinInfo 中奖元素（统一内部和外部使用，避免冗余转换）
type WinInfo struct {
	Symbol      int64     `json:"val"`     // 符号值
	SymbolCount int64     `json:"starNum"` // 连续相同符号的个数
	LineCount   int64     `json:"roadNum"` // 路数（支付线编号，从0开始）
	Odds        int64     `json:"odds"`    // 基础赔率
	Multiplier  int64     `json:"mul"`     // 倍数
	WinGrid     int64Grid `json:"loc"`     // 中奖位置网格
}

// CardType 牌型信息
type CardType struct {
	Type     int // 牌型
	Way      int // 路数 0~24
	Multiple int // 倍数
	Route    int // 几路 中了记录，0，3，4，5
}

// rtpDebugData RTP调试数据
type rtpDebugData struct {
	open bool // 是否开启调试模式（用于RTP测试时的详细日志输出）
}
