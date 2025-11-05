package xslm2

// 游戏基础配置
const _gameID = 18892 // 游戏ID

// 网格配置
const (
	_rowCount int64 = 4 // 行数
	_colCount int64 = 5 // 列数
)

// 符号定义
const (
	_blank       int64 = 0  // 空白符号
	_diamond     int64 = 1  // 方块
	_club        int64 = 2  // 梅花
	_heart       int64 = 3  // 红桃
	_spade       int64 = 4  // 黑桃
	_stake       int64 = 5  // 尖头木桩
	_cross       int64 = 6  // 十字架
	_femaleA     int64 = 7  // 女性A（可收集）
	_femaleB     int64 = 8  // 女性B（可收集）
	_femaleC     int64 = 9  // 女性C（可收集）
	_wildFemaleA int64 = 10 // 女性A百搭
	_wildFemaleB int64 = 11 // 女性B百搭
	_wildFemaleC int64 = 12 // 女性C百搭
	_wild        int64 = 13 // 百搭(吸血鬼王)
	_treasure    int64 = 14 // 夺宝(血月)
)

// 游戏规则配置
const (
	_minMatchCount                       = 3  // 最少匹配列数
	_femaleSymbolCountForFullElimination = 10 // 女性符号收集阈值（达到此数量触发全屏消除）
)
