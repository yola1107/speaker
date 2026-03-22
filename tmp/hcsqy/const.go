package hcsqy

const _gameID = 18956
const GameID = 18956
const Name = "横财三千亿"
const _baseMultiplier = 5

const (
	_rowCount = 3 // 行数
	_colCount = 3 // 列数
)

// 符号定义
const (
	_blank    int64 = 0 // 空白
	_         int64 = 1 // K
	_         int64 = 2 // A
	_         int64 = 3 // 鸡尾酒
	_         int64 = 4 // 钞票
	_         int64 = 5 // 金条
	_         int64 = 6 // 游艇
	_         int64 = 7 // 飞机
	_wild     int64 = 8 // 美女百搭
	_treasure int64 = 9 // 黑卡夺宝
)

const _minMatchCount = 3 // 最小中奖数量

// 游戏阶段
const (
	_spinTypeBase    = 1  // 普通模式
	_spinTypeFree    = 2  // 免费模式
	_spinTypeBuyBase = 11 // 购买免费基础模式
	_spinTypeBuyFree = 12 // 购买免费免费模式
)

var (
	_supportedBetSizes     = []float64{0.04, 0.40, 4.00}
	_supportedBetMultiples = []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
)
