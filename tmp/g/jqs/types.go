package jqs

import "egame-grpc/game/jqs/pb"

type int64Grid [_rowCount][_colCount]int64

type WinInfo struct {
	Symbol      int64     `json:"val"`     // 符号值
	SymbolCount int64     `json:"starNum"` // 连续相同符号的个数
	LineCount   int64     `json:"roadNum"` // 支付线编号
	Odds        int64     `json:"odds"`    // 赔率
	WinGrid     int64Grid `json:"loc"`     // 中奖位置
}

type WinDetails struct {
	// Review：回包固定 0，不落库（与 dcdh gameOrderToResponse 一致）
	FreeWin  float64          `json:"freeWin"`
	TotalWin float64          `json:"totalWin"`
	Next     bool             `json:"next"`
	WinArr   []*pb.Jqs_WinArr `json:"winArr"`
}

type rtpDebugData struct {
	open bool
}
