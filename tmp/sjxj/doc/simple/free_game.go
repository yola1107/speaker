package main

import "math/rand"

func PlayBaseWithFreeGame() BaseWithFreeResult {
	base := PlayBaseSpin()
	triggerScatter := countScatterInBottomRows(base.Grid, 4)
	if triggerScatter < FreeGameScatterMin {
		return BaseWithFreeResult{
			Base:                base,
			TriggeredFreeGame:   false,
			TriggerScatterCount: triggerScatter,
		}
	}

	steps := playFreeGame(base.Grid)
	return BaseWithFreeResult{
		Base:                base,
		TriggeredFreeGame:   true,
		TriggerScatterCount: triggerScatter,
		FreeSteps:           steps,
	}
}

func playFreeGame(start Matrix) []FreeSpinStep {
	grid := start
	var multiplierGrid MultiplierMatrix
	assigned := make(map[Position]bool)

	// 对齐sjxj：ScatterLock保存夺宝位置和倍数
	// 触发免费时：为整个盘面的Scatter分配倍数并锁定
	var scatterLock [Rows][Cols]int
	for r := 0; r < Rows; r++ {
		for c := 0; c < Cols; c++ {
			if grid[r][c] == Scatter {
				mul := randomMultiplierForRow(r)
				multiplierGrid[r][c] = mul
				scatterLock[r][c] = mul
				assigned[Position{Row: r, Col: c}] = true
			}
		}
	}

	unlockedRows := 4
	// 进入免费：先按已解锁区域（从4行起）级联统计 scatter，直到无法再解锁更多行。
	unlockedRows = settleUnlockedRows(grid, unlockedRows)

	respins := FreeGameTimes

	steps := make([]FreeSpinStep, 0, 16)
	for respins > 0 {
		// sjxj 的 baseSpin() 在进入 processWinInfos() 前会先做 FreeNum--。
		respinsBefore := respins
		respins-- // 本步消耗 1 次免费次数

		// 对齐sjxj：在填充盘面之前记录当前解锁行数（等价 PrevUnlockedRows）
		beforeUnlockRows := unlockedRows

		// 对齐sjxj：使用 ScatterLock 锁定夺宝位置，其余从 free reel 重新填充整盘
		rerollWithScatterLock(&grid, &multiplierGrid, &scatterLock, unlockedRows)

		// 每步 spin 后：在扩大后的解锁区内重数 scatter，级联解锁直到稳定。
		unlockedRows = settleUnlockedRows(grid, unlockedRows)
		newUnlockRows := unlockedRows - beforeUnlockRows

		scatterInUnlock := countScatterInBottomRows(grid, unlockedRows)

		// 对齐sjxj 的 calcCurrentFreeGameMul：
		// 为新生成的夺宝分配倍数并写入 scatterLock（ScatterLock[r][c] == 0 表示尚未分配的夺宝）
		for r := 0; r < Rows; r++ {
			for c := 0; c < Cols; c++ {
				if grid[r][c] == Scatter && scatterLock[r][c] == 0 {
					pos := Position{Row: r, Col: c}
					if !assigned[pos] {
						mul := randomMultiplierForRow(r)
						multiplierGrid[r][c] = mul
						scatterLock[r][c] = mul
						assigned[pos] = true
					}
				}
			}
		}

		// 对齐sjxj：isFullScatter（已解锁区是否全是夺宝）
		isFullScatter := checkFullScatterInUnlocked(grid, unlockedRows)

		freeGameMul := totalMultiplier(multiplierGrid, unlockedRows)

		// 对齐sjxj：解锁新行且未满屏夺宝时，才补齐到 FreeUnlockResetSpins（注意 gate 条件 !isFullScatter）
		if !isFullScatter && newUnlockRows > 0 {
			if respins < FreeUnlockResetSpins {
				respins = FreeUnlockResetSpins
			}
		}

		steps = append(steps, FreeSpinStep{
			Index:           len(steps) + 1,
			Grid:            grid,
			MultiplierGrid:  multiplierGrid,
			UnlockedRows:    unlockedRows,
			ScatterInUnlock: scatterInUnlock,
			RespinsBefore:   respinsBefore,
			RespinsAfter:    respins,
			NewScatter:      0,
			NewUnlockRows:   newUnlockRows,
			TotalMultiplier: freeGameMul,
		})

		// 对齐sjxj：当 FreeNum<=0 或 isFullScatter 时，本步结算倍数后免费结束
		if isFullScatter || respins <= 0 {
			break
		}
	}

	return steps
}

// rerollWithScatterLock 对齐sjxj的免费盘面生成逻辑
// 对齐bet_order_configs.go的getSceneSymbolFree：
// - ScatterLock锁定的位置保持夺宝不变
// - 其余位置从free reel连续段填充
// - 处理整张8x5盘面
func rerollWithScatterLock(grid *Matrix, multiplierGrid *MultiplierMatrix, scatterLock *[Rows][Cols]int, unlockedRows int) {
	for col := 0; col < Cols; col++ {
		// 收集需要填充的行（非锁定位置）
		needRows := []int{}
		for row := Rows - 1; row >= 0; row-- {
			if (*scatterLock)[row][col] != 0 {
				// 锁定的夺宝位置保持不变
				(*grid)[row][col] = Scatter
				if (*multiplierGrid)[row][col] == 0 {
					(*multiplierGrid)[row][col] = (*scatterLock)[row][col]
				}
			} else {
				needRows = append(needRows, row)
			}
		}

		// 从free reel连续段填充需要的位置
		if len(needRows) > 0 {
			reel := FreeReels[col]
			dataLen := len(reel)
			start := rand.Intn(dataLen)
			for i, row := range needRows {
				sym := reel[(start+i)%dataLen]
				(*grid)[row][col] = sym
				if sym != Scatter {
					(*multiplierGrid)[row][col] = 0
				}
			}
		}
	}
}

func rerollNonScatterInUnlockedRows(grid *Matrix, unlockedRows int) int {
	newScatter := 0
	rowStart := Rows - unlockedRows
	for col := 0; col < Cols; col++ {
		for row := rowStart; row < Rows; row++ {
			if (*grid)[row][col] == Scatter {
				continue
			}
			next := randomFreeReelSymbol(col)
			(*grid)[row][col] = next
			if next == Scatter {
				newScatter++
			}
		}
	}
	return newScatter
}

func randomFreeReelSymbol(col int) int {
	idx := rand.Intn(ReelLen)
	return FreeReels[col][idx]
}

func settleUnlockedRows(grid Matrix, currentUnlockedRows int) int {
	// 用当前解锁高度数 scatter → 映射目标行数；若变高则在更大区域内重数，
	// 直到目标不再超过当前解锁行数（与 missWorld 级联逻辑一致）。
	unlockedRows := currentUnlockedRows
	for unlockedRows < Rows {
		count := countScatterInBottomRows(grid, unlockedRows)
		targetUnlockedRows := calcUnlockedRowsByScatterCount(count)
		if targetUnlockedRows <= unlockedRows {
			break
		}
		unlockedRows = targetUnlockedRows
	}
	return unlockedRows
}

func calcUnlockedRowsByScatterCount(scatterCount int) int {
	// Prefer config thresholds when provided, e.g. [0,0,0,0,8,12,16,20].
	// Index means unlocked row count.
	if len(FreeUnlockThresholds) >= Rows {
		unlocked := 4
		for rows := 5; rows <= Rows; rows++ {
			if scatterCount >= FreeUnlockThresholds[rows-1] {
				unlocked = rows
			}
		}
		return unlocked
	}

	// Fallback: 4 scatters is base amount for 4 rows, then every extra 4 unlocks 1 row.
	if scatterCount < 4 {
		return 4
	}
	unlocked := 4 + (scatterCount-4)/4
	if unlocked > Rows {
		return Rows
	}
	return unlocked
}

func countScatterInBottomRows(grid Matrix, rowsCount int) int {
	if rowsCount <= 0 {
		return 0
	}
	if rowsCount > Rows {
		rowsCount = Rows
	}
	start := Rows - rowsCount
	total := 0
	for row := start; row < Rows; row++ {
		for col := 0; col < Cols; col++ {
			if grid[row][col] == Scatter {
				total++
			}
		}
	}
	return total
}

func assignScatterMultipliers(multiplierGrid *MultiplierMatrix, assigned *map[Position]bool, grid Matrix) {
	for row := 0; row < Rows; row++ {
		for col := 0; col < Cols; col++ {
			if grid[row][col] != Scatter {
				continue
			}
			pos := Position{Row: row, Col: col}
			if (*assigned)[pos] {
				continue
			}
			(*multiplierGrid)[row][col] = randomMultiplierForRow(row)
			(*assigned)[pos] = true
		}
	}
}

func randomMultiplierForRow(row int) int {
	if row < 0 {
		row = 0
	}
	if row >= len(FreeScatterMultiplierByRow) {
		row = len(FreeScatterMultiplierByRow) - 1
	}
	values := FreeScatterMultiplierByRow[row]
	return values[rand.Intn(len(values))]
}

func totalMultiplier(m MultiplierMatrix, unlockedRows int) int {
	total := 0
	startRow := Rows - unlockedRows
	if startRow < 0 {
		startRow = 0
	}
	for row := startRow; row < Rows; row++ {
		for col := 0; col < Cols; col++ {
			total += m[row][col]
		}
	}
	return total
}

// checkFullScatterInUnlocked 检查已解锁区是否全部是夺宝
// 对齐sjxj的calcCurrentFreeGameMul中的isFullScatter判定
func checkFullScatterInUnlocked(grid Matrix, unlockedRows int) bool {
	startRow := Rows - unlockedRows
	if startRow < 0 {
		startRow = 0
	}
	for row := startRow; row < Rows; row++ {
		for col := 0; col < Cols; col++ {
			if grid[row][col] != Scatter {
				return false
			}
		}
	}
	return true
}
