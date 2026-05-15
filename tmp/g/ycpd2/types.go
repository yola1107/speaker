package ycpd

import "egame-grpc/game/ycpd2/pb"

type int64Grid [_rowCount][_colCount]int64

type WinInfo struct {
	Symbol      int64
	SymbolCount int64
	LineCount   int64
	Odds        int64
	Multiplier  int64
}

type WinDetails struct {
	State        int64             `json:"state"`
	FreeNum      int64             `json:"freeNum"`
	FreeTimes    int64             `json:"freeTimes"`
	TotalWin     float64           `json:"totalWin"`
	FreeWin      float64           `json:"freeWin"`
	RoundWin     float64           `json:"roundWin"`
	TotalFree    int64             `json:"totalFree"`
	NewFreeTimes int64             `json:"newFreeTimes"`
	IsRoundOver  bool              `json:"isRoundOver"`
	IsSpinOver   bool              `json:"isSpinOver"`
	IsFree       bool              `json:"isFree"`
	LineMul      int64             `json:"lineMul"`
	StepMul      int64             `json:"stepMul"`
	GameMul      int64             `json:"gameMul"`
	RemoveMul    []int64           `json:"RemoveMul"`
	CurGameMul   []int64           `json:"curGameMul"`
	WinArr       []*pb.Ycpd_WinArr `json:"winArr"`
}

type rtpDebugData struct {
	open bool
}
