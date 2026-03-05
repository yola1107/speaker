package jqs

import "egame-grpc/game/common/pb"

// 中奖信息
type winInfo struct {
	Symbol      int64
	SymbolCount int64
	LineCount   int64
	Odds        int64
	LineNo      int
	Positions   int64Grid
}

// 中奖结果
type winResult struct {
	Symbol             int64     `json:"symbol"`
	SymbolCount        int64     `json:"symbolCount"`
	LineCount          int64     `json:"lineCount"`
	BaseLineMultiplier int64     `json:"baseLineMultiplier"`
	TotalMultiplier    int64     `json:"totalMultiplier"`
	LineNo             int64     `json:"line_no"`
	Position           int64Grid `json:"position"`
}

type WinArr struct {
	Val     int64     `json:"val"`
	RoadNum int64     `json:"roadNum"`
	StarNum int64     `json:"starNum"`
	Odds    int64     `json:"odds"`
	Mul     int64     `json:"mul"`
	LineNo  int64     `json:"lineNo"`
	Loc     int64Grid `json:"loc"`
}

type WinDetails struct {
	FreeWin  float64          `json:"freeWin"`
	TotalWin float64          `json:"totalWin"`
	State    int64            `json:"state"`
	WinArr   []*pb.Jqs_WinArr `json:"winArr"`
	WinGrid  *pb.Board        `json:"winGrid"`
}
