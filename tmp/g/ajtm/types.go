package ajtm

import (
	"egame-grpc/game/ajtm/pb"
)

// int64Grid 表示 6x5 盘面网格。
type int64Grid [_rowCount][_colCount]int64

type WinInfo struct {
	Symbol      int64     `json:"val"`     // 中奖符号 ID
	LineCount   int64     `json:"roadNum"` // 路数
	SymbolCount int64     `json:"starNum"` // 连续命中列数
	Odds        int64     `json:"odds"`    // 基础赔率
	Multiplier  int64     `json:"mul"`     // 结算倍数 = Odds * LineCount
	WinGrid     int64Grid `json:"loc"`     // 该符号的中奖网格
}

type Block struct {
	Col       int64 `json:"col"`       // 列号
	HeadRow   int64 `json:"headRow"`   // 长符号头部行号
	TailRow   int64 `json:"tailRow"`   // 长符号尾部行号，固定等于 HeadRow+1
	OldSymbol int64 `json:"oldSymbol"` // 转变前符号
	NewSymbol int64 `json:"newSymbol"` // 转变后符号
}

type WinDetails struct {
	// BetAmount / IsFree / ScatterCount / StepMul：由 gameOrder（Base*Multiple、IsFree、HuNum、LineMultiple）在 gameOrderToResponse 中推导，不落库
	FreeWin      float64           `json:"freeWin"`      // 免费模式累计赢分
	RoundWin     float64           `json:"roundWin"`     // 当前 round 累计赢分
	IsRoundOver  bool              `json:"isRoundOver"`  // 当前 round 是否结束
	State        int64             `json:"state"`        // 当前阶段（与 gameOrder.State 结算标记不同）
	NewFreeTimes int64             `json:"newFreeTimes"` // 本步新增免费次数
	WinArr       []*pb.Ajtm_WinArr `json:"winArr"`       // 本步中奖条目
	WinMys       []*pb.Ajtm_WinMys `json:"winMys"`       // 长符号转变事件
	MysMul       int64             `json:"mysMul"`       // 当前神秘符号倍数
	Limit        bool              `json:"limit"`        // 是否触发最大可赢封顶
}

type rtpDebugData struct {
	open             bool      // 是否开启 RTP 调试
	mark             int32     // 调试日志对齐标记
	originSymbolGrid int64Grid //

	//基础模式下统计
	realIndex   [3]int // 1 2 3轴取的长符号个数// -1,1,1
	randomIndex [3]int // 1 2 3轴随机到的布局索引+1

	// 免费模式下统计
	freeAddMystery [2]int64 // 新增长符号 [col,row]
}
