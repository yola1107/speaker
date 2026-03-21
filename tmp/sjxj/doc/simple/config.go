package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type missWorldJSON struct {
	PayTable                   [][]int         `json:"pay_table"`
	Lines                      [][]int         `json:"lines"`
	RealData                   [][][]int       `json:"real_data"`
	RollCfg                    json.RawMessage `json:"roll_cfg,omitempty"`
	FreeGameScatterMin         int             `json:"free_game_scatter_min"`
	FreeGameTimes              int             `json:"free_game_times"`
	FreeScatterMultiplierByRow [][]int         `json:"free_scatter_multiplier_by_row"`
	FreeUnlockThresholds       []int           `json:"free_unlock_thresholds"`
	FreeUnlockResetSpins       int             `json:"free_unlock_reset_spins"`
}

var (
	PayTableJSON [][]int // [9][5]
	LinesJSON    [][]int // [Lines][Cols] (each item is 5 indexes)
	BaseReels    [][]int // [Cols][ReelLen]
	FreeReels    [][]int // [Cols][ReelLen] (reserved for future free-game)

	FreeGameScatterMin         int
	FreeGameTimes              int
	FreeScatterMultiplierByRow [][]int
	FreeUnlockThresholds       []int
	FreeUnlockResetSpins       int
)

func loadMissWorldConfig() {
	// Resolve JSON path relative to this source file, not current working directory.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller failed")
	}
	jsonPath := filepath.Join(filepath.Dir(thisFile), "missworld.json")

	b, err := os.ReadFile(jsonPath)
	if err != nil {
		panic(err)
	}

	var cfg missWorldJSON
	if err := json.Unmarshal(b, &cfg); err != nil {
		panic(err)
	}

	PayTableJSON = cfg.PayTable
	LinesJSON = cfg.Lines
	FreeGameScatterMin = cfg.FreeGameScatterMin
	FreeGameTimes = cfg.FreeGameTimes
	FreeScatterMultiplierByRow = cfg.FreeScatterMultiplierByRow
	FreeUnlockThresholds = cfg.FreeUnlockThresholds
	FreeUnlockResetSpins = cfg.FreeUnlockResetSpins

	// real_data[0] => base reels, real_data[1] => free reels.
	if len(cfg.RealData) < 2 {
		panic("missworld.json real_data requires at least 2 reel sets: base and free")
	}
	BaseReels = cfg.RealData[0]
	FreeReels = cfg.RealData[1]

	// Basic sanity checks to catch obvious config mistakes early.
	if len(PayTableJSON) < 9 || len(PayTableJSON[0]) < 5 {
		panic("missworld.json pay_table shape mismatch")
	}
	if len(LinesJSON) != Lines {
		panic("missworld.json lines count mismatch")
	}
	for i := 0; i < Lines; i++ {
		if len(LinesJSON[i]) != Cols {
			panic("missworld.json lines[?] shape mismatch")
		}
	}
	if len(BaseReels) != Cols || len(FreeReels) != Cols {
		panic("missworld.json real_data cols mismatch")
	}
	for col := 0; col < Cols; col++ {
		if len(BaseReels[col]) != ReelLen || len(FreeReels[col]) != ReelLen {
			panic("missworld.json real_data reel length mismatch")
		}
	}
	if FreeGameScatterMin <= 0 {
		FreeGameScatterMin = 4
	}
	if FreeGameTimes <= 0 {
		FreeGameTimes = 3
	}
	if len(FreeScatterMultiplierByRow) == 0 {
		FreeScatterMultiplierByRow = [][]int{
			{32, 64, 96, 128}, // row 0 (top)
			{16, 32, 48, 64},  // row 1
			{8, 16, 24, 32},   // row 2
			{4, 8, 12, 16},    // row 3
			{1, 2, 3, 4},      // row 4
			{1, 2, 3, 4},      // row 5
			{1, 2, 3, 4},      // row 6
			{1, 2, 3, 4},      // row 7 (bottom)
		}
	}
	if len(FreeScatterMultiplierByRow) != Rows {
		panic("missworld.json free_scatter_multiplier_by_row rows mismatch")
	}
	for row := 0; row < Rows; row++ {
		if len(FreeScatterMultiplierByRow[row]) == 0 {
			panic("missworld.json free_scatter_multiplier_by_row has empty row")
		}
	}
	if FreeUnlockResetSpins <= 0 {
		FreeUnlockResetSpins = 3
	}
}

func init() {
	loadMissWorldConfig()
}
