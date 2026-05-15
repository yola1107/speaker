package tmtg

import "egame-grpc/game/tmtg/pb"

type int64Grid [_rowCount][_colCount]int64

type WinInfo struct {
	Symbol      int64     `json:"symbol"`      // 符号值
	SymbolCount int64     `json:"symbolCount"` // 连续命中符号个数(含wild)
	Count       int64     `json:"count"`       // 数量
	Odds        int64     `json:"odds"`        // 基础赔率
	WinGrid     int64Grid `json:"loc"`         // 中奖位置网格
}

type WinDetails struct {
	State        int64             `json:"state"`
	NextStage    int64             `json:"nStage"`
	FreeNum      int64             `json:"freeNum"`
	FreeTimes    int64             `json:"freeTimes"`
	RoundWin     float64           `json:"roundWin"`
	TotalWin     float64           `json:"totalWin"`
	FreeWin      float64           `json:"freeWin"`
	NewFreeTimes int64             `json:"newFreeTimes"`
	IsRoundOver  bool              `json:"isRoundOver"`
	StepMul      int64             `json:"stepMul"`
	Limit        bool              `json:"limit"`
	WinArr       []*pb.Tmtg_WinArr `json:"winArr"`
}

type rtpDebugData struct {
	open bool
}
