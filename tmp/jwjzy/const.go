package jwjzy

const GameID = 18926 // 鸡尾酒之约

// 虚拟中奖线倍数（对应策划：base_bet）
const _baseMultiplier = 20

const (
	_rowCount = 5 // 行数
	_colCount = 6 // 列数
)

// 符号ID（策划/前端口径：1~15）
const (
	_blank         int64 = 0
	_ten           int64 = 1  // 牌10
	_jack          int64 = 2  // 牌J
	_queen         int64 = 3  // 牌Q
	_king          int64 = 4  // 牌K
	_ace           int64 = 5  // 牌A
	_cocktailMulti int64 = 6  // 杂色鸡尾酒
	_cocktailPink  int64 = 7  // 粉红鸡尾酒
	_cocktailGreen int64 = 8  // 绿色鸡尾酒
	_cocktailBlue  int64 = 9  // 蓝色鸡尾
	_xoCup         int64 = 10 // 一杯XO
	_cigar         int64 = 11 // 雪茄（非必要）
	_xoBottle      int64 = 12 // 一瓶XO
	_wild          int64 = 13 // 百搭（必须是喝酒动作）
	_treasure      int64 = 14 // 夺宝（Scatter）
	_goldFrame     int64 = 15 // 金色框（中奖金色背景符号消除后变为百搭）
)

const _minMatchCount = 3 // WayGame 最小中奖数量（3+）

// 兼容历史代码的“形态标记掩码”（旧逻辑中用于把 wildForm 从 symbolGrid 标记出来）
// 后续在实现金色背景->百搭与长符号时可移除对该常量的依赖。
const _mask int64 = 10

const (
	_spinTypeBase    = 1  // 普通
	_spinTypeBaseEli = 11 // 普通消除
	_spinTypeFree    = 21 // 免费
	_spinTypeFreeEli = 22 // 免费消除
)

// 购买免费（策划：buy_free_game_multiple）
const _buyFreeGameMultiple int64 = 75

//// 免费模式配置（策划：free_game_times / free_game_scatter_min / free_game_add_times_per_scatter）
//const (
//	_freeGameTimes              int64 = 10
//	_freeGameScatterMin         int64 = 4
//	_freeGameAddTimesPerScatter int64 = 2
//)
