package sbyymx2

// int64Grid 3x3 符号网格
type int64Grid [_rowCount][_colCount]int64

// winInfo 中间行中奖候选（赔付符号为左格）
type winInfo struct {
	Symbol int64
}

// winResult 单线中奖摘要（写入订单 WinDetails）
// JSON 字段保持与旧版 sbyymx 兼容：element/mul
type winResult struct {
	Symbol     int64 `json:"element"`
	Multiplier int64 `json:"mul"`
}

// rtpDebugData RTP 调试数据
type rtpDebugData struct {
	open bool // 是否开启调试模式（用于 RTP 测试：跳过 Redis 配置、固定扣费基数、不累计奖金等）
}
