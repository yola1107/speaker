package main

type Matrix [Rows][Cols]int

type Position struct {
	Row int
	Col int
}

type WinDetail struct {
	LineIndex int
	SymbolID  int
	LineLen   int
	Win       int
}

type SpinResult struct {
	Grid     Matrix
	Details  []WinDetail
	TotalWin int
	Steps    int
}

type MultiplierMatrix [Rows][Cols]int

type FreeSpinStep struct {
	Index           int
	Grid            Matrix
	MultiplierGrid  MultiplierMatrix
	UnlockedRows    int
	ScatterInUnlock int
	RespinsBefore   int
	RespinsAfter    int
	NewScatter      int
	NewUnlockRows   int
	TotalMultiplier int
}

type BaseWithFreeResult struct {
	Base                SpinResult
	TriggeredFreeGame   bool
	TriggerScatterCount int
	FreeSteps           []FreeSpinStep
}
