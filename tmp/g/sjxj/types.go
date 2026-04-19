package sjxj

import "egame-grpc/game/sjxj/pb"

type int64Grid [_rowCount][_colCount]int64

type WinInfo struct {
	Symbol      int64     `json:"val"`     // 符号值
	SymbolCount int64     `json:"starNum"` // 连续相同符号的个数
	LineCount   int64     `json:"roadNum"` // 路数（支付线编号，从0开始）
	Odds        int64     `json:"odds"`    // 基础赔率
	WinGrid     int64Grid `json:"loc"`     // 中奖位置网格
}

type WinDetails struct {
	FreeWin      float64          `json:"freeWin"`
	TotalWin     float64          `json:"totalWin"`
	NewFreeTimes int64            `json:"newFreeTimes"`
	WinInfo      *pb.Sjxj_WinInfo `json:"winInfo"`
}

type rtpDebugData struct {
	open bool
}
