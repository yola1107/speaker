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
	for col := 0; col < Cols; col++ {
		start[col] = rand.Intn(ReelLen)
		cursor[col] = start[col]
	}

	var grid Matrix
	for col := 0; col < Cols; col++ {
		for row := 0; row < Rows; row++ {
			grid[row][col] = BaseReels[col][cursor[col]]
			cursor[col] = (cursor[col] + 1) % ReelLen
		}
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
