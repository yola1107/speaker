package pjcd

import "egame-grpc/game/pjcd/pb"

type int64Grid [_rowCount][_colCount]int64

type WinInfo struct {
	Symbol      int64     `json:"val"`     // 符号值
	SymbolCount int64     `json:"starNum"` // 连续相同符号的个数
	LineCount   int64     `json:"roadNum"` // 路数（支付线编号，从0开始）
	Odds        int64     `json:"odds"`    // 符号赔率
	WinGrid     int64Grid `json:"loc"`     // 中奖位置网格
}

type WinDetails struct {
	TotalWin          float64           `json:"totalWin"`          // 本回合总赢取
	FreeWin           float64           `json:"freeWin"`           // 免费游戏累计赢取
	RoundWin          float64           `json:"roundWin"`          // 一局所有回合总赢
	IsRoundOver       bool              `json:"isRoundOver"`       // 是否本局结束
	NewFreeTimes      int64             `json:"newFreeTimes"`      // 增加的免费次数
	MulIndex          int64             `json:"mulIndex"`          // 当前消除轮次索引
	BaseMultipliers   []int64           `json:"baseMultipliers"`   // 轮次倍数列表 [1,2,3,5]
	FreeMultipliers   []int64           `json:"freeMultipliers"`   // 轮次倍数列表 [3,6,9,15]
	StepMultiplier    int64             `json:"stepMultiplier"`    // 本步倍数
	WildEliCount      int64             `json:"wildEliCount"`      // 本步蝴蝶百搭个数
	TotalWildEliCount int64             `json:"totalWildEliCount"` // 累积蝴蝶百搭个数
	WildEliMultiple   int64             `json:"wildEliMultiple"`   // 蝴蝶百搭倍数
	WinArr            []*pb.Pjcd_WinArr `json:"winArr"`            // 本步各条中奖线的详情
}

type rtpDebugData struct {
	open bool
}
