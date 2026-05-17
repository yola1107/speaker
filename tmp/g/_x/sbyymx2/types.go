package sbyymx2

import "egame-grpc/game/sbyymx2/pb"

// 3x3 符号网格
type int64Grid [_rowCount][_colCount]int64

type WinInfo struct {
	Symbol      int64     `json:"val"`     // 符号值
	SymbolCount int64     `json:"starNum"` // 连续相同符号的个数
	LineCount   int64     `json:"roadNum"` // 支付线编号
	Odds        int64     `json:"odds"`    // 赔率
	WinGrid     int64Grid `json:"loc"`     // 中奖位置
}

type WinDetails struct {
	TotalWin         float64 `json:"totalWin"`         // 本回合累计赢取（重转链内累加，存 scene.TotalWin）
	IsRespinUntilWin bool    `json:"isRespinUntilWin"` // 当前步是否处于重转至赢模式
	WildMultiplier   int64   `json:"wildMultiplier"`
	LineMultiplier   int64   `json:"lineMultiplier"`
	StepMultiple     int64   `json:"stepMultiple"`
	IsInstrumentWin  bool    `json:"isInstrumentWin"`
	IsRoundOver      bool    `json:"isRoundOver"`

	WinArr []*pb.Sbyymx_WinArr `json:"winArr"`
}

type rtpDebugData struct {
	open bool
	mode int // 测试日志用 spin 阶段
}
