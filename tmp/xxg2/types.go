package xxg2

type int64Grid = [_rowCount][_colCount]int64

type position struct {
	Row int64 `json:"r"`
	Col int64 `json:"c"`
}

// 中奖信息
type winInfo struct {
	Symbol      int64
	SymbolCount int64
	LineCount   int64
	Positions   []*position
}

// step 预设数据
type stepMap struct {
	ID         int64                        `json:"id"`  // id
	FreeNum    int64                        `json:"fr"`  //免费次数
	IsFree     int64                        `json:"isf"` //是不是免费
	New        int64                        `json:"ne"`  //新增次数
	TreatCount int64                        `json:"tr"`  //本step夺宝符
	Bat        []*Bat                       `json:"bat"`
	Map        [_rowCount * _colCount]int64 `json:"mp"` // 预设数据
}

// 中奖结果
type winResult struct {
	Symbol             int64                       `json:"symbol"`             //  符号
	SymbolCount        int64                       `json:"symbolCount"`        //  数量
	LineCount          int64                       `json:"lineCount"`          //  中奖线数量
	BaseLineMultiplier int64                       `json:"baseLineMultiplier"` //  中奖线倍数
	TotalMultiplier    int64                       `json:"totalMultiplier"`    //  总倍数
	WinPositions       [_rowCount][_colCount]int64 `json:"winPositions"`
}

type Bat struct {
	X      int64 `json:"x"`
	Y      int64 `json:"y"`
	TransX int64 `json:"nx"`
	TransY int64 `json:"ny"`
	Syb    int64 `json:"syb"`
	Sybn   int64 `json:"sybn"`
}

// BaseSpinResult baseSpin 返回结果
type BaseSpinResult struct {
	lineMultiplier    int64        // 线倍数
	stepMultiplier    int64        // 总倍数
	treasureCount     int64        // 夺宝符个数
	symbolGrid        *int64Grid   // 符号网格
	winGrid           *int64Grid   // 中奖网格
	winResults        []*winResult // 中奖结果
	SpinOver          bool         // 一局游戏是否结束（参考 mahjong）
	InitialBatCount   int64        // 初始蝙蝠数量（用于RTP测试统计）
	AccumulatedNewBat int64        // 累计新增蝙蝠数量（用于RTP测试统计）
	IsFreeGameEnding  bool         // 免费游戏是否在本次spin后结束（用于RTP测试统计）
}
