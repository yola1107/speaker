package bxkh2

const GameID = 18965 // 巴西狂欢
const _baseMultiplier = 20

const (
	_rowCount = 6 // 行数
	_colCount = 6 // 列数
)

const (
	_blank    int64 = 0
	_         int64 = 1  //
	_         int64 = 2  //
	_         int64 = 3  //
	_         int64 = 4  //
	_         int64 = 5  //
	_         int64 = 6  //
	_         int64 = 7  //
	_         int64 = 8  //
	_         int64 = 9  //
	_         int64 = 10 //
	_         int64 = 11 //
	_treasure int64 = 12 // 夺宝(Scatter)
	_wild     int64 = 13 // 百搭(Wild)
)

const _minMatchCount = 3

const (
	_spinTypeBase    = 1  // 普通
	_spinTypeBaseEli = 11 // 普通消除
	_spinTypeFree    = 21 // 免费
	_spinTypeFreeEli = 22 // 免费消除
)

const (
	_longSymbol   = 1000 // 长符号
	_silverSymbol = 100  // 银符号
	_goldenSymbol = 200  // 金符号
)
