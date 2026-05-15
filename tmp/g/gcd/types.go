package gcd

import "egame-grpc/game/gcd/pb"

type Int64Grid = [_rowCount][_colCount]int64

type WinInfo struct {
	Symbol      int64
	SymbolCount int64
	LineCount   int64
	Odds        int64
	LineNo      int
	Positions   Int64Grid
}

type WinResult struct {
	Symbol             int64     `json:"symbol"`
	SymbolCount        int64     `json:"symbolCount"`
	LineCount          int64     `json:"lineCount"`
	BaseLineMultiplier int64     `json:"baseLineMultiplier"`
	TotalMultiplier    int64     `json:"totalMultiplier"`
	LineNo             int64     `json:"line_no"`
	Position           Int64Grid `json:"position"`
}

type SymbolRoller struct {
	Index  int              `json:"i"`
	Symbol [_rowCount]int64 `json:"s"`
}

type WinDetails struct {
	StepIndex    int64           `json:"stepIndex"`
	IsRoundOver  bool            `json:"isRoundOver"`
	Stage        int64           `json:"stage"`
	NextStage    int64           `json:"n_stage"`
	FreeNum      int64           `json:"freeNum"`
	FreeTimes    int64           `json:"freeTimes"`
	RoundWin     float64         `json:"roundWin"`
	FreeWin      float64         `json:"freeWin"`
	TotalWin     float64         `json:"totalWin"`
	WinArr       []*pb.WinResult `json:"winArr"`
	RoundMulti   int64           `json:"roundMulti"`
	BonusState   int64           `json:"bonusState"`
	FreeType     int64           `json:"freeType"`
	NewFreeTimes int64           `json:"newFreeTimes"`
}
