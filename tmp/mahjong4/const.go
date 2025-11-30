package mahjong

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
	_         int64 = 1 // Q符号
	_         int64 = 2 // K符号
	_         int64 = 3 // A符号
	_         int64 = 4 // 藏宝图
	_         int64 = 5 // 指南针
	_         int64 = 6 // 枪
	_         int64 = 7 // 女王
	_treasure int64 = 8 // 夺宝符号
	_wild     int64 = 9 // 百搭符号
)

const _minMatchCount = 3 // 最小中奖数量

const (
	_bonusNum1 = 1
	_bonusNum2 = 2
	_bonusNum3 = 3
)

const (
	_spinTypeBase    = 1  // 普通
	_spinTypeBaseEli = 11 // 普通消除
	_spinTypeFree    = 21 // 免费
	_spinTypeFreeEli = 22 // 免费消除
)

const runStateNormal = int8(0)   //普通
const runStateFreeGame = int8(1) //免费

// 免费游戏选择状态
const (
	_bonusStatePending  = 1 // 等待客户端选择免费游戏类型
	_bonusStateSelected = 2 // 已选择免费游戏类型
)
