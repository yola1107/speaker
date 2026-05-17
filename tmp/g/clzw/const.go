package clzw

const GameID = 18976 // 丛林之王
const _baseMultiplier = 20

const _buyFreeMultiple = 100 // 购买免费：扣费 = betAmount × 本值

const (
	_rowCount = 3
	_colCount = 5
)

const (
	_blank    int64 = 0  // 空
	_         int64 = 1  // 10
	_         int64 = 2  // J
	_         int64 = 3  // Q
	_         int64 = 4  // K
	_         int64 = 5  // A
	_         int64 = 6  // 土拨鼠
	_         int64 = 7  // 羚羊
	_         int64 = 8  // 斑马
	_         int64 = 9  // 野牛
	_         int64 = 10 // 河马
	_         int64 = 11 // 大象
	_wild     int64 = 12 // 鳄鱼
	_treasure int64 = 13 // 猎豹
	_lion     int64 = 14 // 狮子王
)

const _minMatchCount = 3

const (
	_spinTypeBase    = 1  // 普通
	_spinTypeBaseEli = 11 // 普通消除
	_spinTypeFree    = 21 // 免费
	_spinTypeFreeEli = 22 // 免费消除
)
