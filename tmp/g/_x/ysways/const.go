package ys

const GameID = 18976 // 原神
const _baseMultiplier = 20

const (
	_rowCount = 5 // 行数
	_colCount = 6 // 列数
)

// 符号定义
const (
	_blank    int64 = 0
	_         int64 = 1 // Q
	_         int64 = 2 // K
	_         int64 = 3 // A
	_         int64 = 4 // 水神
	_         int64 = 5 // 火神
	_         int64 = 6 // 雷神
	_wild     int64 = 7 // 百搭符号
	_treasure int64 = 8 // 夺宝符号
)

const _minMatchCount = 3 // 最小中奖数量

// 游戏阶段
const (
	_spinTypeBase    = 1  // 普通
	_spinTypeBaseEli = 11 // 普通消除
	_spinTypeFree    = 21 // 免费
	_spinTypeFreeEli = 22 // 免费消除
)

var _checkList = []int64{1, 2, 3, 4, 5, 6}

/*

《原神》   18976   https://qr8tls.axshare.com/?g=1&id=agx1gr&p=%E4%B8%80%E3%80%81%E6%B8%B8%E6%88%8F%E7%AE%80%E4%BB%8B_1


3*5 way game
和原神类似的元素反应（水火、水电、电火），还有美少女爆衣关联玩法

只有3*5和消除比较类似，不过我们是way game
*/
