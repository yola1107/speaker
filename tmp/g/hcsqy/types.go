package hcsqy

import "egame-grpc/game/hcsqy/pb"

// 3x3 符号网格
type int64Grid [_rowCount][_colCount]int64

type WinInfo struct {
	Symbol      int64     `json:"val"`     // 符号值
	SymbolCount int64     `json:"starNum"` // 连续相同符号的个数
	LineCount   int64     `json:"roadNum"` // 支付线编号
	Odds        int64     `json:"odds"`    // 赔率
	WinGrid     int64Grid `json:"loc"`     // 中奖位置
}

type WinDetails struct {
	// BetAmount / IsFree：由 gameOrder 在 gameOrderToResponse 中推导
	FreeWin          float64            `json:"freeWin"`          // 免费游戏累计赢取
	TotalWin         float64            `json:"totalWin"`         // 本回合总赢取
	IsRoundOver      bool               `json:"isRoundOver"`      // 是否本局结束
	NewFreeTimes     int64              `json:"newFreeTimes"`     // 本步新增的免费旋转次数
	IsPurchase       bool               `json:"isPurchase"`       // 当前大回合是否购买免费触发
	Next             bool               `json:"next"`             // 是否需要继续请求（重转至赢）
	IsRespinUntilWin bool               `json:"isRespinUntilWin"` // 是否在重转至赢模式中 (有炸胡动画)
	RespinWildCol    int32              `json:"respinWildCol"`    // 重转至赢模式长条百搭列 (0-2)，非重转模式为-1
	WildMultiplier   int64              `json:"wildMultiplier"`   // 长条百搭倍数 (x2-x20)
	LineMultiplier   int64              `json:"lineMultiplier"`   // 线赔率合计（各中奖线基础赔率之和，未乘长条百搭）
	ScatterCount     int64              `json:"scatterCount"`     // 夺宝符个数
	WinArr           []*pb.Hcsqy_WinArr `json:"winArr"`           // 本步各条中奖线的详情
}

type rtpDebugData struct {
	open bool
	mode spinExecMode
}
