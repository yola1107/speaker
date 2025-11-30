package mahjong

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

type winResult struct {
	Symbol             int64 `json:"symbol"`             // 符号ID
	SymbolCount        int64 `json:"symbolCount"`        // 符号数量
	LineCount          int64 `json:"lineCount"`          // 路数
	BaseLineMultiplier int64 `json:"baseLineMultiplier"` // 基础线倍数
	TotalMultiplier    int64 `json:"totalMultiplier"`    // 总倍数
}

// BaseSpinResult 基础旋转结果（单次spin的返回值）
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
	stepWin           float64    // 当前Step真实奖金（来自服务器计算，不要重新计算）
	winGrid           int64GridW // 中奖网格
	cards             int64Grid  // 符号网格
	nextSymbolGrid    int64Grid  // 下一轮符号网格
	winInfo           WinInfo    // 中奖信息
	winResult         []CardType // 中奖结果
}

// CardType 牌型信息
type CardType struct {
	Type     int // 牌型
	Way      int // 路数 0~24
	Multiple int // 倍数
	Route    int // 几路 中了记录，0，3，4，5
}

// WinInfo 中奖信息（客户端返回用）
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

// WinElem 中奖元素（客户端返回用）
type WinElem struct {
	Val     int64     `json:"val"`     // 符号值（符号ID）
	RoadNum int64     `json:"roadNum"` // 路数（支付线编号，从0开始）
	StarNum int64     `json:"starNum"` // 星星数（连续相同符号的个数）
	Odds    int64     `json:"odds"`    // 赔率（基础赔率）
	Mul     int64     `json:"mul"`     // 倍数（线倍数）
	Loc     int64Grid `json:"loc"`     // 位置（中奖位置网格）
}

// SpinResultC 旋转结果（客户端返回用）
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
	BonusState int       `json:"bonusState"` // 是否需要选择免费游戏类型：0-否，1-是
}

// rtpDebugData RTP调试数据
type rtpDebugData struct {
	open bool // 是否开启调试模式（用于RTP测试时的详细日志输出）
}

/*

### 网格布局（4行 × 5列）

```
         Col 0    Col 1    Col 2    Col 3    Col 4
Row 0:  [0][0]   [0][1]   [0][2]   [0][3]   [0][4]            (0, 1, 2, 3, 4)
Row 1:  [1][0]   [1][1]   [1][2]   [1][3]   [1][4]            (5, 6, 7, 8, 9)
Row 2:  [2][0]   [2][1]   [2][2]   [2][3]   [2][4]            (10,11,12,13,14)
Row 3:  [3][0]   [3][1]   [3][2]   [3][3]   [3][4]            (15,16,17,18,19)
```
```
         Col 0    Col 1    Col 2    Col 3    Col 4
Row 0:  [0]      [1]      [2]      [3]      [4]      ← ✅ 检测行
Row 1:  [5]      [6]      [7]      [8]      [9]      ← ✅ 检测行
Row 2:  [10]     [11]     [12]     [13]     [14]     ← ✅ 检测行
Row 3:  [15]     [16]     [17]     [18]     [19]     ← ❌ 不检测（缓冲区）
        ↑
        一维索引值（在 WinLines 配置中使用）
```

************************************************************************
#### 水平线（第1条）：`[5,6,7,8,9]`

```
         Col 0    Col 1    Col 2    Col 3    Col 4
Row 0:    -        -        -        -        -
Row 1:   [5] →   [6] →   [7] →   [8] →   [9]  ← 第1条线
Row 2:    -        -        -        -        -
Row 3:    -        -        -        -        -
```

#### 对角线（第4条）：`[10,6,2,8,14]`

```
         Col 0    Col 1    Col 2    Col 3    Col 4
Row 0:    -        -      [2]       -        -
Row 1:    -      [6]       -      [8]       -
Row 2:  [10]       -        -        -     [14]  ← 第4条线
Row 3:    -        -        -        -        -
```

#### 交替线（第14条）：`[5,11,7,13,9]`

```
         Col 0    Col 1    Col 2    Col 3    Col 4
Row 0:    -        -        -        -        -
Row 1:  [5]        -      [7]        -      [9]  ← 第14条线
Row 2:    -      [11]       -      [13]       -
Row 3:    -        -        -        -        -
```


************************************************************************

// bet_order_helper.go - checkSymbolGridWin
for _, p := range line {
    r := p / _colCount  // r = p / 5
    c := p % _colCount  // c = p % 5
    if r >= _rowCountReward {  // r >= 3
        break  // 停止检测（Row 3 不检测）
    }
    currSymbol := symbolGrid[r][c]
    // ...
}


*/
