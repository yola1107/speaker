package bxkh2

import "egame-grpc/game/bxkh2/pb"

type int64Grid [_rowCount][_colCount]int64

// winInfo 中奖信息
type winInfo struct {
	Symbol      int64
	SymbolCount int64
	LineCount   int64
	Odds        int64
	Multiplier  int64
}

type WinDetails struct {
	RoundWin     float64            `json:"roundWin"`
	TotalWin     float64            `json:"totalWin"`
	FreeTotalWin float64            `json:"freeTotalWin"`
	FreeMultiple int64              `json:"freeMultiple"`
	IsRoundOver  bool               `json:"isRoundOver"`
	AddFreeTime  int64              `json:"addFreeTime"`
	WinArr       []*pb.Bxkh2_WinArr `json:"winArr"`
}

type rtpDebugData struct {
	open bool
	//originSymbolGrid int64Grid
}
