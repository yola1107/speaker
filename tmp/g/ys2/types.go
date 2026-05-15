package ys2

import "egame-grpc/game/ys2/pb"

type int64Grid [_rowCount][_colCount]int64

type WinInfo struct {
	Symbol      int64     `json:"symbol"`      // 符号值
	SymbolCount int64     `json:"symbolCount"` // 连续相同符号的个数
	LineCount   int64     `json:"lineCount"`   // 路数
	Odds        int64     `json:"odds"`        // 基础赔率
	Multiplier  int64     `json:"mul"`         // 倍数
	WinGrid     int64Grid `json:"loc"`         // 中奖位置网格
}

type WinDetails struct {
	State        int64           `json:"state"`
	NextStage    int64           `json:"nStage"`
	FreeNum      int64           `json:"freeNum"`
	FreeTimes    int64           `json:"freeTimes"`
	RoundWin     float64         `json:"roundWin"`
	TotalWin     float64         `json:"totalWin"`
	FreeWin      float64         `json:"freeWin"`
	BonusState   int64           `json:"bonusState"`
	BonusNum     int64           `json:"bonusNum"`
	NewFreeTimes int64           `json:"newFreeTimes"`
	ScatterCount int64           `json:"scatterCount"`
	IsRoundOver  bool            `json:"isRoundOver"`
	StepMul      int64           `json:"stepMul"`
	Limit        bool            `json:"limit"`
	WinArr       []*pb.Ys_WinArr `json:"winArr"`
}

type rtpDebugData struct {
	open bool
}
