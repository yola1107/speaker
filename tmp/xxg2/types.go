package xxg2

type int64Grid = [_rowCount][_colCount]int64

// position 位置坐标
type position struct {
	Row int64 `json:"r"` // 行（0-3）
	Col int64 `json:"c"` // 列（0-4）
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
	FreeNum    int64                        `json:"fr"`  // 剩余免费次数
	IsFree     int64                        `json:"isf"` // 是否免费模式（0=base, 1=free）
	New        int64                        `json:"ne"`  // 本轮新增免费次数
	TreatCount int64                        `json:"tr"`  // 本轮treasure数量
	TreatPos   []*position                  `json:"tp"`  // 本轮treasure位置（统一数据源）
	Bat        []*Bat                       `json:"bat"` // 蝙蝠移动/转换信息（前端动画用）
	Map        [_rowCount * _colCount]int64 `json:"mp"`  // 原始符号数组（4x5=20）
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

// Bat 蝙蝠移动记录（用于前端播放飞行动画）
type Bat struct {
	X      int64 `json:"x"`    // 起始行
	Y      int64 `json:"y"`    // 起始列
	TransX int64 `json:"nx"`   // 目标行
	TransY int64 `json:"ny"`   // 目标列
	Syb    int64 `json:"syb"`  // 原符号
	Sybn   int64 `json:"sybn"` // 新符号（转换后）
}

// BaseSpinResult baseSpin 返回结果
type BaseSpinResult struct {
	lineMultiplier int64        // 线倍数
	stepMultiplier int64        // 总倍数
	treasureCount  int64        // 夺宝符个数
	symbolGrid     *int64Grid   // 符号网格
	winGrid        *int64Grid   // 中奖网格
	winResults     []*winResult // 中奖结果
	SpinOver       bool         // 一局游戏是否结束
}
