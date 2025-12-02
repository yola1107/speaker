package mahjong4

// 以'行'为组
type int64Grid [_rowCount][_colCount]int64

// 奖励专用少一行
type int64GridW [_rowCountReward][_colCount]int64

type winInfo struct {
	Symbol      int64     // 符号
	SymbolCount int64     // 符号数量
	LineCount   int64     // 路数
	Odds        int64     // 赔率
	Multiplier  int64     // 倍数
	WinGrid     int64Grid // 中奖网格
}

// winData 中奖信息（客户端返回用）
type winData struct {
	Next           bool       `json:"next"`           // 是否继续
	Over           bool       `json:"over"`           // 是否结束
	Multi          int64      `json:"multi"`          // 倍数
	State          int8       `json:"state"`          // 普通0免费1
	FreeNum        uint64     `json:"freeNum"`        // 剩余免费次数
	FreeTime       uint64     `json:"freeTime"`       // 免费次数
	TotalFreeTime  uint64     `json:"totalFreeTime"`  // 免费总次数
	FreeMultiple   int64      `json:"gameMultiple"`   // 免费倍数，初始1倍
	IsRoundOver    bool       `json:"isRoundOver"`    // 回合是否结束
	AddFreeTime    int64      `json:"addFreeTime"`    // 增加免费次数
	ScatterCount   int64      `json:"scatterCount"`   // 夺宝符
	WinGrid        int64GridW `json:"winGrid"`        // 中奖位置
	NextSymbolGrid int64Grid  `json:"nextSymbolGrid"` // 移动位置
	WinArr         []WinElem  `json:"winArr"`         // 中奖数组
}

// WinElem 中奖元素（客户端返回用）
type WinElem struct {
	Val     int64     `json:"val"`     // 符号值（符号ID）
	RoadNum int64     `json:"roadNum"` // 路数（支付线编号，从0开始）
	StarNum int64     `json:"starNum"` // 星星数（连续相同符号的个数）
	Odds    int64     `json:"odds"`    // 赔率（基础赔率）
	Mul     int64     `json:"mul"`     // 倍数（线倍数）
	Loc     int64Grid `json:"loc"`     // 位置（中奖位置网格）
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
