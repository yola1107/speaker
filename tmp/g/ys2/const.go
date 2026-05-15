package ys2

const GameID = 18976 // 原神
const _baseMultiplier = 20

const (
	_rowCount = 3
	_colCount = 5
)

const (
	_blank    int64 = 0
	_         int64 = 1
	_         int64 = 2
	_         int64 = 3
	_         int64 = 4
	_         int64 = 5
	_         int64 = 6
	_         int64 = 7
	_wild     int64 = 8
	_treasure int64 = 9
)

const _minMatchCount = 3

const (
	_spinTypeBase    = 1  // 普通
	_spinTypeBaseEli = 11 // 普通消除
	_spinTypeFree    = 21 // 免费
	_spinTypeFreeEli = 22 // 免费消除
)

const (
	_bonusStatePending  = 1 // 已触发免费，等待前端选档
	_bonusStateSelected = 2 // 已选档，可进入免费
)
