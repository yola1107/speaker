package ycj

const GameID = 18926 // 印钞机
const _baseMultiplier = 20

const (
	_rowCount = 1 // 行数 (1行)
	_colCount = 3 // 列数 (3列)
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

	// 翻倍符号 (第1列)
	_mul2   int64 = 7  // X2
	_mul3   int64 = 8  // X3
	_mul5   int64 = 9  // X5
	_mul10  int64 = 10 // X10
	_mul20  int64 = 11 // X20
	_mul30  int64 = 12 // X30
	_mul50  int64 = 13 // X50
	_mul100 int64 = 14 // X100

	// 夺宝符号 (第1列) - 触发免费旋转
	_free5  int64 = 15 // 5次免费旋转
	_free10 int64 = 16 // 10次免费旋转
	_free20 int64 = 17 // 20次免费旋转
)

// 符号类型判断
func isNumberSymbol(s int64) bool     { return s >= 1 && s <= 5 }
func isMultiplierSymbol(s int64) bool { return s >= 7 && s <= 14 }
func isFreeSpinSymbol(s int64) bool   { return s >= 15 && s <= 17 }

// 游戏阶段
const (
	_spinTypeBase = 1 // 普通模式
	_spinTypeFree = 2 // 免费模式
)

// 最大倍数
const _maxMultiplier = 1000
