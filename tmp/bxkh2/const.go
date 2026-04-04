package bxkh2

const _gameID = 18965        // 巴西狂欢
const GameID_18965 = _gameID // 巴西狂欢
const _baseMultiplier = 20

const (
	_rowCount = 6
	_colCount = 6
)

const (
	_blank    int64 = 0
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
