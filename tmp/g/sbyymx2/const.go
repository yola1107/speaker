package sbyymx2

const GameID = 8917 // 桑巴与亚马逊
const _baseMultiplier = 10

const (
	_rowCount = 3 // 行数
	_colCount = 3 // 列数
)

// 符号定义
const (
	_blank      int64 = 0 // 空白
	_feather    int64 = 1 // 羽毛
	_fish       int64 = 2 // 龙鱼
	_palm       int64 = 3 // 棕榈树
	_tambourine int64 = 4 // 巴西铃鼓
	_maraca     int64 = 5 // 沙槌
	_guitar     int64 = 6 // 古典吉他
	_drum       int64 = 7 // 鼓
	_wild       int64 = 8 // 百搭
)

const _minMatchCount = 3 // 最小中奖数量

const (
	_spinTypeBase       = 0
	_spinTypeBaseRespin = 1
)
