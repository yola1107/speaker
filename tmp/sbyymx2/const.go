package sbyymx2

const _gameID = 8917 // 桑巴与亚马逊
const GameID = 8917
const Name = "桑巴与亚马逊"

const _baseMultiplier = 10 // 与旧版一致：下注 = base * multiple * 10

const (
	_rowCount = 3
	_colCount = 3
)

// 符号：1-7 普通；9 空白（与旧版文档一致）；100 百搭，>100 为带倍率百搭(100+倍率)
const (
	_blank     int64 = 0
	_          int64 = 1 // 羽毛
	_          int64 = 2 // 龙鱼
	_          int64 = 3 // 棕榈树
	_          int64 = 4 // 巴西铃鼓
	_          int64 = 5 // 沙槌
	_          int64 = 6 // 古典吉他
	_          int64 = 7 // 鼓
	_blankCell int64 = 9 // 空白格（盘面展示）
	_wild      int64 = 100
)

// 中列倍率百搭 id 上限：策划 x2～x100 → 符号 id 最大 200；同时防止异常配置导致 int64 溢出
const _maxWildExtraMultiplier int64 = 100

const _slotName = "slot_sbyymx"
const _slotDetailsName = "slot_sbyymx_Details_A"
