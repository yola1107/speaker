package hbtr2

const _gameID = 8918 // 霍比特人2
const GameId = 8918

const _baseMultiplier = 20 // 虚拟中奖线倍数

const (
	_rowCount = 5 // 行数
	_colCount = 6 // 列数
)

const (
	_rollerRowCount = 5             // 滚轮行数
	_rollerColCount = 7             // 滚轮列数
	_boardSize      = _rowCount - 1 // 滚轮盘面符号个数（每列4个符号）
)

// 符号定义
const (
	_blank    int64 = 0  // 空
	_diamond  int64 = 1  // 方块
	_club     int64 = 2  // 梅花
	_heart    int64 = 3  // 红桃
	_spade    int64 = 4  // 黑桃
	_shield   int64 = 5  // 圆盾
	_hat      int64 = 6  // 巫师帽
	_ring     int64 = 7  // 魔戒
	_thorin   int64 = 8  // 索林
	_elfQueen int64 = 9  // 精灵女王
	_gandalf  int64 = 10 // 甘道夫
	_wild     int64 = 12 // 百搭
	_scatter  int64 = 11 // 夺宝          (基础模式的夺宝符号)
	_freePlus int64 = 13 // 免费次数+1     (免费模式的夺宝符号)
	_scaWild  int64 = 14 // 夺宝+百搭      (基础模式，wild移动到夺宝上展示 12+11->14)
	_freeWild int64 = 15 // 免费次数+1+百搭 (免费模式，wild移动到夺宝上展示 12+13->15)           //
)

const _minMatchCount = 3 // 最小中奖数量

const (
	_spinTypeBase    = 1  // 普通
	_spinTypeBaseEli = 11 // 普通消除
	_spinTypeFree    = 21 // 免费
	_spinTypeFreeEli = 22 // 免费消除
)
