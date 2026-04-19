package pjcd

// 文档地址 https://9x04qv.axshare.com/?g=14&id=8075wd&p=%E9%A1%B9%E7%9B%AE%E8%AF%B4%E6%98%8E_1&sc=3

const Name = "破茧成蝶"

const _gameID = 18984
const GameID = 18984
const _baseMultiplier = 20

const (
	_rowCount = 3 // 总行数
	_colCount = 5 // 列数

	_rowCountReward = 3 // 奖励行数（用于中奖检测，不包含缓冲区）
)

const (
	_blank    int64 = 0
	_         int64 = 1 //
	_         int64 = 2 //
	_         int64 = 3 //
	_         int64 = 4 //
	_         int64 = 5 //
	_         int64 = 6 //
	_         int64 = 7 //
	_wild     int64 = 8 // 百搭符号
	_treasure int64 = 9 // 夺宝符号
)

const _minMatchCount = 3 // 最小中奖数量

const (
	_spinTypeBase    = 1  // 普通
	_spinTypeBaseEli = 11 // 普通消除
	_spinTypeFree    = 21 // 免费
	_spinTypeFreeEli = 22 // 免费消除
)

const _mask = 10 // 百搭掩码
