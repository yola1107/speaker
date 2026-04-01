package yhwy

// 文档地址 https://gt7xqi.axshare.com/

const GameID = 18959
const Name = "樱花物语"
const _baseMultiplier = 25

const (
	_rowCount = 4
	_colCount = 5
)

// 符号定义从 0 开始。
const (
	_blank   int64 = -1
	_10      int64 = 0
	_J       int64 = 1
	_Q       int64 = 2
	_K       int64 = 3
	_A       int64 = 4
	_Geta    int64 = 5
	_Fan     int64 = 6
	_Bell    int64 = 7
	_Ninja   int64 = 8
	_Miko    int64 = 9
	_Scatter int64 = 10
	_Mystery int64 = 11
	_Wild    int64 = 12
)

const (
	_symbolCount   = 13
	_minMatchCount = 3
)

const (
	_spinTypeBase = 1
	_spinTypeFree = 21
)

const (
	_resetDirectionNone int64 = 0
	_resetDirectionUp   int64 = 1
	_resetDirectionDown int64 = 2
)
