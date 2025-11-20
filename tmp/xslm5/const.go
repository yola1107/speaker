package xslm3

const _gameID = 18892
const _baseMultiplier = 20

const (
	_rowCount int64 = 4
	_colCount int64 = 5
)

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

const (
	_presetKindNormalBase = 0
	_presetKindNormalFree = 1
)

const _minMatchCount = 3
const _triggerTreasureCount = 3
const _femaleFullCount = 10

const _maxMultiplierForBaseOnly = 5000

const _presetDataKeyTpl = "%s:slot_xslm_data"
const _presetIDKeyTpl = "%s:slot_xslm_id:%d:%d"

const _presetID = int64(0)

const (
	_spinTypeBase    = 1 //普通
	_spinTypeBaseEli = 2 //普通消除
	_spinTypeFree    = 3 //免费
	_spinTypeFreeEli = 4 //免费消除
)

// 特殊符号
const (
	_eliminated int64 = -1 // 消除标识（用于展示日志/调试）
	_blocked    int64 = 99 // 墙格标记（左下角和右下角）
)
