package pjcd

// 文档地址 https://9x04qv.axshare.com/?g=14&id=8075wd&p=%E9%A1%B9%E7%9B%AE%E8%AF%B4%E6%98%8E_1&sc=3

const Name = "破茧成蝶"

const GameID = 18984

const _baseMultiplier = 20 // 基础倍数（20条赔付线）

// 网格规格
const (
	_rowCount = 3 // 总行数
	_colCount = 5 // 列数
)

// 符号常量
const (
	_blank      int64 = 0 // 空符号
	_clover     int64 = 1 // 三叶草
	_periwinkle int64 = 2 // 长春花
	_tulip      int64 = 3 // 郁金香
	_daisy      int64 = 4 // 雏菊
	_orchid     int64 = 5 // 兰花
	_lotus      int64 = 6 // 莲花
	_rose       int64 = 7 // 玫瑰
	_wild       int64 = 8 // 百搭符号
	_scatter    int64 = 9 // 夺宝符号
)

// 百搭形态（用于状态追踪，不影响符号ID）
// 毛虫 → 蝶茧 → 蝴蝶 → 消除并增加倍数
const (
	_wildStateNone        int8 = 0 // 无百搭
	_wildStateCaterpillar int8 = 1 // 毛虫（初始形态）
	_wildStateCocoon      int8 = 2 // 蝶茧
	_wildStateButterfly   int8 = 3 // 蝴蝶
)

// 游戏阶段
const (
	_spinTypeBase    int8 = 1  // 基础模式
	_spinTypeBaseEli int8 = 11 // 基础消除模式
	_spinTypeFree    int8 = 21 // 免费模式
	_spinTypeFreeEli int8 = 22 // 免费消除模式
)

// 中奖线相关
const (
	_minMatchCount = 3  // 最小中奖数量
	_maxWinLines   = 20 // 20条中奖线
)

// 轮轴相关
const (
	_reelLength = 100 // 轮轴长度
)
