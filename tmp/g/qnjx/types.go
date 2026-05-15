package qnjx

import "egame-grpc/game/qnjx/pb"

type int64Grid [_rowCount][_colCount]int64

type WinInfo struct {
	Symbol      int64     `json:"val"`     // 中奖符号 ID
	LineCount   int64     `json:"roadNum"` // 路数
	SymbolCount int64     `json:"starNum"` // 连续命中列数
	Odds        int64     `json:"odds"`    // 基础赔率
	Multiplier  int64     `json:"mul"`     // 结算倍数 = Odds * LineCount
	WinGrid     int64Grid `json:"loc"`     // 该符号的中奖网格

	Num int64 `json:"num"` // 命中的符号数量 （包含百搭）
}

type WinDetails struct {
	FreeWin      float64           `json:"freeWin"`      // 免费模式累计赢分
	RoundWin     float64           `json:"roundWin"`     // 当前 round 累计赢分
	IsRoundOver  bool              `json:"isRoundOver"`  // 当前 round 是否结束
	State        int64             `json:"state"`        // 当前阶段（与 gameOrder.State 结算标记不同）
	NewFreeTimes int64             `json:"newFreeTimes"` // 本步新增免费次数
	MysMul       int64             `json:"mysMul"`       // 当前长符号倍数
	Limit        bool              `json:"limit"`        // 是否触发最大可赢封顶
	ColorMul     []int64           `json:"colorMul"`     // 蓝/黄/绿收集倍数 绿：(1,4,7) 蓝: (2,5,8) 黄：(3,6,9)
	WinArr       []*pb.Qnjx_WinArr `json:"winArr"`       // 本步中奖条目
}

type rtpDebugData struct {
	open  bool  // 是否开启 RTP 调试
	mark  int32 // 调试日志对齐标记
	added [3]int64
}
