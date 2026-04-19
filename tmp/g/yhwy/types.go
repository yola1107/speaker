package yhwy

import "egame-grpc/game/yhwy/pb"

// int64Grid 4x5 盘面
type int64Grid [_rowCount][_colCount]int64

type WinInfo struct {
	Symbol      int64     `json:"sy"`   // 符号值
	SymbolCount int64     `json:"sc"`   // 连续相同符号的个数
	LineCount   int64     `json:"lc"`   // 支付线编号
	Odds        int64     `json:"odds"` // 赔率
	WinGrid     int64Grid `json:"loc"`  // 中奖位置
}

type WinDetails struct {
	RoundWin     float64           `json:"roundWin"`
	TotalWin     float64           `json:"totalWin"`
	FreeWin      float64           `json:"freeWin"`
	NewFreeTimes int64             `json:"newFreeTimes"`
	MysGrid      []int64           `json:"mysGrid"`
	CollectLevel int64             `json:"collectLevel"`
	WinArr       []*pb.Yhwy_WinArr `json:"winArr"`
}

type rtpDebugData struct {
	open      bool      // true 表示启用测试模式，不走线上缓存/扣费逻辑
	origin    int64Grid // 原始符号
	sakuraCol int       // 樱吹雪替换到的最远列（3/4/5） 默认-1
	mysSymbol int64     // 百变樱花本次统一揭示出的目标符号
}
