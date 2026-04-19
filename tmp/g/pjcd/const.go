package pjcd

const GameID = 18984 // 破茧成蝶
const _baseMultiplier = 20

const (
	_rowCount = 3 // 行数
	_colCount = 5 // 列数
)

const (
	_blank    int64 = 0
	_         int64 = 1 //
	_         int64 = 2 //
	_         int64 = 3 //
	_         int64 = 4 //
	_         int64 = 5 //
	_         int64 = 6 //
	_         int64 = 7 //
	_wild     int64 = 8 // 百搭符号
	_treasure int64 = 9 // 夺宝符号
)

const _minMatchCount = 3 // 最小中奖数量

const (
	_spinTypeBase    = 1  // 普通
	_spinTypeBaseEli = 11 // 普通消除
	_spinTypeFree    = 21 // 免费
	_spinTypeFreeEli = 22 // 免费消除
)

const _mask = 10 // 百搭掩码
