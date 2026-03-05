package sgz

const _gameID = 18931
const GameID = 18931
const _baseMultiplier = 20

const (
	_rowCount = 4 // 总行数
	_colCount = 5 // 列数

	_rowCountReward = 3 // 奖励行数（用于中奖检测，不包含缓冲区）
)

const (
	_blank    int64 = 0
	_         int64 = 1 // 盾牌  Q符号
	_         int64 = 2 // 长矛  K符号
	_         int64 = 3 // 弓箭  A符号
	_         int64 = 4 // 战马  藏宝图
	_         int64 = 5 // 冲车  指南针
	_         int64 = 6 // 井栏  枪
	_         int64 = 7 // 投石车  女王
	_wild     int64 = 8 // 百搭符号
	_treasure int64 = 9 // 夺宝符号
)

const (
	_heroID1 = 1 // 刘备
	_heroID2 = 2 // 张飞
	_heroID3 = 3 // 关羽
	_heroID4 = 4 // 赵云
	_heroID5 = 5 // 诸葛亮
	_heroID6 = 6 // 黄忠
	_heroID7 = 7 // 马超
	_heroID8 = 8 // 君主
)

const (
	_spinTypeBase    = 1  // 普通
	_spinTypeBaseEli = 11 // 普通消除
	_spinTypeFree    = 21 // 免费
	_spinTypeFreeEli = 22 // 免费消除
)

const _minMatchCount = 3 // 最小中奖数量

const _maxWinLines = 20 // 20条中奖线
