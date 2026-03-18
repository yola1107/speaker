package sjxj

const _gameID = 18969
const GameID = 18969
const _baseMultiplier = 50

const (
	_rowCount       = 8 // 基础模式行数
	_colCount       = 5 // 列数
	_rowCountReward = 4 // 奖励行数（用于中奖检测）
)

const (
	_blank     int64 = 0  // 空白
	_10        int64 = 1  // 10
	_J         int64 = 2  // J
	_Q         int64 = 3  // Q
	_K         int64 = 4  // K
	_A         int64 = 5  // A
	_HighHeels int64 = 6  // 高跟鞋
	_Bandage   int64 = 7  // 绷带
	_Scepter   int64 = 8  // 权杖
	_Crown     int64 = 9  // 头冠
	_wild      int64 = 10 // Wild（百搭符号）
	_treasure  int64 = 11 // Scatter（夺宝）
)

const _minMatchCount = 3 // 最小中奖数量

const (
	_spinTypeBase = 1 // 普通
	_spinTypeFree = 2 // 免费
)
