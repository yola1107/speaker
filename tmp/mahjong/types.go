package mahjong

// 以'行'为组
type int64Grid [_rowCount][_colCount]int64

// 奖励专用少一行
type int64GridW [_rowCountWin][_colCount]int64

type winInfo struct {
	Symbol      int64     // 符号
	SymbolCount int64     // 符号数量
	LineCount   int64     // 路数
	Odds        int64     // 赔率
	Multiplier  int64     // 倍数
	WinGrid     int64Grid // 中奖网格
}

type winResult struct {
	Symbol             int64 `json:"symbol"`
	SymbolCount        int64 `json:"symbolCount"`
	LineCount          int64 `json:"lineCount"`
	BaseLineMultiplier int64 `json:"baseLineMultiplier"`
	TotalMultiplier    int64 `json:"totalMultiplier"`
}

type CardType struct {
	Type     int // 牌型
	Way      int // 路数 0~24
	Multiple int // 倍数
	Route    int // 几路 中了记录，0，3，4，5
}

type WinElem struct {
	Val     int64     `json:"val"`
	RoadNum int64     `json:"roadNum"`
	StarNum int64     `json:"starNum"`
	Odds    int64     `json:"odds"`
	Mul     int64     `json:"mul"`
	Loc     int64Grid `json:"loc"`
}

type BaseSpinResult struct {
	lineMultiplier    int64      // 线倍数
	stepMultiplier    int64      // 总倍数
	scatterCount      int64      // 夺宝符个数
	addFreeTime       int64      // 增加免费次数
	freeTime          int64      // 次数
	gameMultiple      int64      // 倍数，初始1倍
	bonusHeadMultiple int64      // 实际倍数
	bonusTimes        int64      // 总消除次数
	SpinOver          bool       // 是否结束
	winGrid           int64GridW // 中奖网格
	cards             int64Grid  // 符号网格
	nextSymbolGrid    int64Grid  // 下一轮符号网格
	winInfo           WinInfo    // 中奖信息
	winResult         []CardType // 中奖结果
}

type WinInfo struct {
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
	FreeSpinCount  int64      `json:"freeSpinCount"`  // 夺宝中的免费符
	WinGrid        int64GridW `json:"winGrid"`        // 中奖位置
	NextSymbolGrid int64Grid  `json:"nextSymbolGrid"` // 移动位置
	WinArr         []WinElem  `json:"winArr"`         // 中奖数组
}

type SpinResultC struct {
	Balance    float64   `json:"balance"`    // 余额
	BetAmount  float64   `json:"betAmount"`  // 下注额
	CurrentWin float64   `json:"currentWin"` // 当前Step赢
	AccWin     float64   `json:"accWin"`     // 当前round赢
	TotalWin   float64   `json:"totalWin"`   // 总赢
	Free       int       `json:"free"`       // 是否在免费
	Review     int       `json:"review"`     // 回顾
	Sn         string    `json:"sn"`         // 注单号
	LastWinId  uint64    `json:"lastWinId"`  // 上次中奖ID
	MapId      uint64    `json:"mapId"`      // 地图ID
	WinInfo    WinInfo   `json:"winInfo"`    // 中奖信息
	Cards      int64Grid `json:"cards"`      // 这把符号
	RoundBonus float64   `json:"roundBonus"` // 回合奖金
}
