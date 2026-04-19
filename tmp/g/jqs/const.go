package jqs

const GameID = 18972      // 金钱鼠
const _baseMultiplier = 5 // 虚拟中奖线倍数

const (
	_rowCount int = 3
	_colCount int = 3
)

const (
	_blank int64 = 0 // 空
	_            = 1 // 花生
	_      int64 = 2 // 年桔
	_      int64 = 3 // 鞭炮
	_      int64 = 4 // 福袋
	_      int64 = 5 // 红包
	_      int64 = 6 // 福字
	_wild  int64 = 7 // 鼠(百搭)
)

const _minMatchCount = 3 // 最小中奖数量

const (
	_spinTypeBase = 1
	_spinTypeFree = 2
)
