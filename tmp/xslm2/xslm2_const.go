package xslm2

// 游戏基础配置
const (
	_gameID         = 18892 // 游戏ID（西施恋美2）
	_baseMultiplier = 20    // 基础倍数
)

// 网格配置
const (
	_rowCount int64 = 4 // 行数
	_colCount int64 = 5 // 列数
)

// 符号定义
const (
	_blank       int64 = 0  // 空白符号
	_            int64 = 1  // 普通符号1
	_            int64 = 2  // 普通符号2
	_            int64 = 3  // 普通符号3
	_            int64 = 4  // 普通符号4
	_            int64 = 5  // 普通符号5
	_            int64 = 6  // 普通符号6
	_femaleA     int64 = 7  // 女性符号A（可收集）
	_femaleB     int64 = 8  // 女性符号B（可收集）
	_femaleC     int64 = 9  // 女性符号C（可收集）
	_wildFemaleA int64 = 10 // Wild女性A
	_            int64 = 11
	_            int64 = 12
	_wild        int64 = 13 // Wild（百搭）
	_treasure    int64 = 14 // 夺宝符号（触发免费）
)

// 预设数据类型
const (
	_presetKindNormalBase = 0 // 普通模式（不带免费）
	_presetKindNormalFree = 1 // 普通模式（带免费）
)

// 游戏规则配置
const (
	_minMatchCount                       = 3    // 最少匹配列数
	_triggerTreasureCount                = 3    // 触发免费的夺宝数量
	_femaleSymbolCountForFullElimination = 10   // 女性符号全屏消除阈值
	_maxMultiplierForBaseOnly            = 5000 // 基础模式最大倍率
)

// Redis key模板
const (
	_presetDataKeyTpl = "%s:slot_xslm_data"     // 预设数据Hash key
	_presetIDKeyTpl   = "%s:slot_xslm_id:%d:%d" // 预设ID查询key（site:kind:multiplier）
)

// 调试配置（0=关闭，>0=使用指定预设ID）
const _presetID = int64(0)
