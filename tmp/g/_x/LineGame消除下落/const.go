package hcsqy

const GameID = 18956 // 横财三千亿
const _baseMultiplier = 5

// 购买逻辑已移除

const (
	_rowCount = 5 // 行数
	_colCount = 6 // 列数
)

// 符号定义
const (
	_blank    int64 = 0 // 空白
	_         int64 = 1 // K
	_         int64 = 2 // A
	_         int64 = 3 // 鸡尾酒
	_         int64 = 4 // 钞票
	_         int64 = 5 // 金条
	_         int64 = 6 // 游艇
	_         int64 = 7 // 飞机
	_wild     int64 = 8 // 美女百搭
	_treasure int64 = 9 // 黑卡夺宝
)

const _minMatchCount = 3 // 最小中奖数量

// 游戏阶段
const (
	_spinTypeBase    = 1  // 普通
	_spinTypeBaseEli = 11 // 普通消除
	_spinTypeFree    = 21 // 免费
	_spinTypeFreeEli = 22 // 免费消除
)
