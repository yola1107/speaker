package gcd

const GameID = 18989 // 鬼吹灯

const (
	_baseMultiplier int64 = 20
	_rowCount       int64 = 3
	_colCount       int64 = 5
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

const (
	_normalMode    = 0
	_normalModeEli = 11
	_freeMode      = 21
	_freeModeEli   = 22
)

const (
	_bonusStatePending  = 1 // 已触发免费，等待前端选档
	_bonusStateSelected = 2 // 已选档，可进入免费
)
