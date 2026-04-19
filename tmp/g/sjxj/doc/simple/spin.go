package main

import "math/rand"

type SpinSource struct {
	Grid         Matrix
	ReelCursors  [Cols]int
	StartIndices [Cols]int
}

func GenerateGrid() SpinSource {
	var start [Cols]int
	var cursor [Cols]int
	var grid Matrix

	for col := 0; col < Cols; col++ {
		reel := BaseReels[col]
		reelLen := len(reel)
		start[col] = rand.Intn(reelLen)
		// 与生产 sjxj getSceneSymbolBase 一致：BoardSymbol[Rows-1-r] = reel[(start+r)%reelLen]
		// 即最底行对应轮带起点 start，向上依次为 start+1 …（grid[row] 为 row=0 顶、row=7 底）
		for row := 0; row < Rows; row++ {
			grid[row][col] = reel[(start[col]+(Rows-1-row))%reelLen]
		}
		cursor[col] = (start[col] + Rows) % reelLen
	}

	return SpinSource{
		Grid:         grid,
		ReelCursors:  cursor,
		StartIndices: start,
	}
}

func PlayBaseSpin() SpinResult {
	source := GenerateGrid()
	grid := source.Grid
	details, totalWin, _ := CheckWin(grid)

	return SpinResult{
		Grid:     grid,
		Details:  details,
		TotalWin: totalWin,
		Steps:    1,
	}
}
