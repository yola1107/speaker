package sbyymx2

const GameID = 8917 // 桑巴与亚马逊
const _baseMultiplier = 10

const (
	_rowCount = 3 // 行数
	_colCount = 3 // 列数
)

// 符号定义
const (
	_blank int64 = 0 // 空白
	_      int64 = 1 // 羽毛
	_      int64 = 2 // 龙鱼
	_      int64 = 3 // 棕榈树
	_      int64 = 4 // 巴西铃鼓
	_      int64 = 5 // 沙槌
	_      int64 = 6 // 古典吉他
	_      int64 = 7 // 鼓
	_wild  int64 = 8 // 百搭
)

const _minMatchCount = 3 // 最小中奖数量
