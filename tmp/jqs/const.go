package jqs

const _gameID = 18972     //18972     // 金钱鼠
const GameID = _gameID    // 金钱鼠
const _baseMultiplier = 5 // 虚拟中奖线倍数

const (
	_rowCount int = 3 // 行数
	_colCount int = 3 // 列数
)

type int64Grid [_rowCount][_colCount]int64

const (
	_lock     int64 = -1 // 锁定
	_blank    int64 = 0  // 空
	_wild     int64 = 1  // 鼠(百搭)
	_fuzi     int64 = 2  // 福字
	_hongbao  int64 = 3  // 红包
	_fudai    int64 = 4  // 福袋
	_bianpao  int64 = 5  // 鞭炮
	_nianju   int64 = 6  // 年桔
	_huasheng int64 = 7  // 花生
)

const (
	_spinTypeBase = 1
	_spinTypeFree = 2
)
