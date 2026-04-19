package ycj

const GameID = 18926 // 印钞机
const _baseMultiplier = 20

const (
	_rowCount = 5 // 行数
	_colCount = 3 // 列数
	_midRow   = 2 // 中间行索引，仅此行参与 findWinInfos 判奖
)

// 符号定义 (连续ID设计)
const (
	_blank int64 = 0 // 空符号

	// 数字符号 (第0、2列)
	_num01 int64 = 1 // 0.1
	_num05 int64 = 2 // 0.5
	_num1  int64 = 3 // 1
	_num5  int64 = 4 // 5
	_num10 int64 = 5 // 10

	// 银行符号 (第1列)
	_bank int64 = 6 // 银行

	// 夺宝符号 (第1列) - 触发免费旋转
	_free5  int64 = 7 // 5次免费旋转
	_free10 int64 = 8 // 10次免费旋转
	_free20 int64 = 9 // 20次免费旋转

	// 翻倍符号 (第1列)
	_mul2   int64 = 10 // X2
	_mul3   int64 = 11 // X3
	_mul5   int64 = 12 // X5
	_mul10  int64 = 13 // X10
	_mul20  int64 = 14 // X20
	_mul30  int64 = 15 // X30
	_mul50  int64 = 16 // X50
	_mul100 int64 = 17 // X100
)

// 游戏阶段
const (
	_spinTypeBase = 1 // 普通模式
	_spinTypeFree = 2 // 免费模式
)

// 续步类型（Pend）：本步响应后客户端需连请求的补判种类
const (
	_pendNone   uint8 = 0
	_pendExtend uint8 = 1
	_pendRespin uint8 = 2
)

// 本扣费回合内已执行过的补判（Done 位掩码，initSpinSymbol 时清零）
const (
	_doneExtend uint8 = 1 << 0
	_doneRespin uint8 = 1 << 1
)
