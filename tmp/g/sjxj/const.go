package sjxj

const GameID = 18969 // 世界小姐
const _baseMultiplier = 50

const (
	_rowCount       = 8 // 行数
	_colCount       = 5 // 列数
	_rowCountReward = 4 // 奖励行数（用于中奖检测）
)

const (
	_blank    int64 = 0  // 空白
	_         int64 = 1  // 10
	_         int64 = 2  // J
	_         int64 = 3  // Q
	_         int64 = 4  // K
	_         int64 = 5  // A
	_         int64 = 6  // 高跟鞋
	_         int64 = 7  // 绷带
	_         int64 = 8  // 权杖
	_         int64 = 9  // 头冠
	_wild     int64 = 10 // Wild（百搭符号）
	_treasure int64 = 11 // Scatter（夺宝）
)

const _minMatchCount = 3 // 最小中奖数量

const (
	_spinTypeBase = 1 // 普通
	_spinTypeFree = 2 // 免费
)
