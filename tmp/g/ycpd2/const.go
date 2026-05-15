package ycpd

const GameID = 18981 // 泳池派对
const _baseMultiplier = 20

const (
	_rowCount = 5 // 行数
	_colCount = 5 // 列数
)

const (
	_blank    int64 = 0
	_         int64 = 1  //9
	_         int64 = 2  //10
	_         int64 = 3  //J
	_         int64 = 4  //Q
	_         int64 = 5  //K
	_         int64 = 6  //A
	_         int64 = 7  //夏日冷饮
	_         int64 = 8  //浴巾
	_         int64 = 9  //水枪
	_         int64 = 10 //泳镜
	_         int64 = 11 //泳装帅哥
	_         int64 = 12 //泳装美女
	_treasure int64 = 13 //夺宝scatter
	_wild     int64 = 14 //百搭wild
	_kong     int64 = 20 //空位
)

const _minMatchCount = 3 // 最小中奖数量

var _lineNumber = [5]int{3, 4, 5, 4, 3} // 每列扑克数目

const (
	_spinTypeBase    = 1  //普通
	_spinTypeBaseEli = 2  //普通消除
	_spinTypeFree    = 11 //免费
	_spinTypeFreeEli = 12 //免费消除
)
