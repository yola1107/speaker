package tmtg

const GameID = 18976 // 甜蜜糖果
const _baseMultiplier = 20

const (
	_rowCount = 5
	_colCount = 6
)

const (
	_blank    int64 = 0  // 空白
	_         int64 = 1  // 香蕉
	_         int64 = 2  // 葡萄
	_         int64 = 3  // 西瓜
	_         int64 = 4  // 蓝莓
	_         int64 = 5  // 苹果
	_         int64 = 6  // 葡萄糖块
	_         int64 = 7  // 西瓜糖块
	_         int64 = 8  // 蓝莓糖块
	_         int64 = 9  // 苹果糖块
	_treasure int64 = 10 // scatter
	_wild     int64 = 11 // wild
	_bomb     int64 = 12 // 倍数炸弹
)

const _minMatchCount = 8

const (
	_spinTypeBase        = 1  // 普通
	_spinTypeBaseEli     = 11 // 普通消除
	_spinTypeFree        = 21 // 免费
	_spinTypeFreeEli     = 22 // 免费消除
	_spinTypeFreeBombEli = 33 // 免费十字消除
)
