package ajtm

const GameID = 18985 // 埃及探秘

const _baseMultiplier = 20 // 固定下注倍率

const (
	_rowCount = 6 // 行数
	_colCount = 5 // 列数
)

// 符号ID
const (
	_blank    int64 = 0  // 空位
	_         int64 = 1  // 9
	_         int64 = 2  // 10
	_         int64 = 3  // J
	_         int64 = 4  // Q
	_         int64 = 5  // K
	_         int64 = 6  // A
	_         int64 = 7  // 荷鲁斯之眼
	_         int64 = 8  // 阿努比斯
	_         int64 = 9  // 圣甲虫贝斯特
	_         int64 = 10 // 荷鲁斯权杖
	_         int64 = 11 // 王后雕像
	_         int64 = 12 // 黄金面具
	_wild     int64 = 13 // wild
	_treasure int64 = 14 // 夺宝
)

// Ways 至少连续命中 3 列才算中奖。
const _minMatchCount = 3

// 运行阶段。
const (
	_spinTypeBase    = 1  // 普通 spin
	_spinTypeBaseEli = 11 // 普通消除 step
	_spinTypeFree    = 21 // 免费 spin
	_spinTypeFreeEli = 22 // 免费消除 step
)

// 长符号相关常量。
const (
	_maxLongBlocks = 9    // 最多 3 列 * 每列 3 个长符号
	_longSymbol    = 1000 // 尾部标记偏移
)

/*
设计意图：
原盘面: [A, B, C, D, E, F]  (行0-5)
长符号布局: 行2-3 (startRow=2, endRow=3)

处理后:
┌─────────────────────────┐
│ 行0: A                  │ ← 保持
│ 行1: B                  │ ← 保持
│ 行2: C (头部)           │ ← 保留原符号
│ 行3: 1000+C (长标记尾)  │ ← 写入 _longSymbol + C
│ 行4: D                  │ ← 原行3的值下移
│ 行5: E                  │ ← 原行4的值下移
│   F                     │ ← 溢出，被丢弃
└─────────────────────────┘

*/
