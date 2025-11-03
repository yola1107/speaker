package xxg2

const GameID = 18891       // 吸血鬼
const _gameID = 18891      // 吸血鬼
const _baseMultiplier = 20 // 虚拟中奖线倍数

const (
	_rowCount int64 = 4 // 行数
	_colCount int64 = 5 // 列数
)

const (
	_blank    int64 = 0
	_         int64 = 1  // J
	_         int64 = 2  // Q
	_         int64 = 3  // K
	_         int64 = 4  // A
	_         int64 = 5  // 十字架
	_         int64 = 6  // 酒杯
	_child    int64 = 7  // 小孩（Wind符号）
	_woman    int64 = 8  // 青年女人（Wind符号）
	_oldMan   int64 = 9  // 老头（Wind符号）
	_wild     int64 = 10 // 百搭
	_treasure int64 = 11 // 夺宝
)

const _minMatchCount = 3        // 最小中奖数量
const _triggerTreasureCount = 3 // 触发免费的夺宝符号最低数量
const _maxBatPositions = 5      // 免费游戏中蝙蝠总数上限（每次spin所有蝙蝠都移动）

// 游戏阶段常量
const (
	_spinTypeBase = 1 // 基础游戏
	_spinTypeFree = 2 // 免费游戏
)

var checkWinSymbols = []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
