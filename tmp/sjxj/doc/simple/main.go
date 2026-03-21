package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

func printGrid(grid Matrix) {
	for row := 0; row < Rows; row++ {
		fmt.Println(grid[row])
	}
}

func printMultiplierGrid(grid MultiplierMatrix) {
	for row := 0; row < Rows; row++ {
		fmt.Println(grid[row])
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "fmt-json" {
		if err := RewriteMissWorldJSONFormat(); err != nil {
			panic(err)
		}
		fmt.Println("已整理 missworld.json 格式（未修改数据）")
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "gen-reels" {
		seed, err := GenerateAndWriteReelsToJSON()
		if err != nil {
			panic(err)
		}
		fmt.Printf("已按权重生成并写入 real_data，seed=%d\n", seed)
		return
	}

	rand.Seed(time.Now().UnixNano())

	start := time.Now()

	// 模拟局数
	const loops = 100000000
	// 基础乘数（对齐sjxj生产版：奖金 = 倍数 / baseMultiplier）
	const baseMultiplier = 50
	// 单次普通游戏下注额（按 baseMultiplier 计算 RTP）
	const betPerSpin = float64(baseMultiplier)

	var freeEnterCount int
	var baseTotalWin int // 倍数总和
	var freeTotalWin int // 倍数总和
	var baseGameWinCount int
	var freeStepsCount int // 免费游戏总步数
	// 0~4 分别表示在免费游戏里最终额外解锁 0~4 行的次数。
	var freeUnlockRowsStats [5]int

	for i := 0; i < loops; i++ {
		result := PlayBaseWithFreeGame()
		baseTotalWin += result.Base.TotalWin
		if result.Base.TotalWin > 0 {
			baseGameWinCount++
		}

		if !result.TriggeredFreeGame {
			continue
		}

		freeEnterCount++
		freeStepsCount += len(result.FreeSteps)

		maxUnlockedRows := 4
		for _, step := range result.FreeSteps {
			if step.UnlockedRows > maxUnlockedRows {
				maxUnlockedRows = step.UnlockedRows
			}
		}
		// Free game pays once when respins reach 0: sum all scatter-cell multipliers on the final grid.
		// TotalMultiplier on each step is the running total on the board; summing steps double-counts.
		if n := len(result.FreeSteps); n > 0 {
			freeTotalWin += result.FreeSteps[n-1].TotalMultiplier
		}

		extraUnlockedRows := maxUnlockedRows - 4
		if extraUnlockedRows < 0 {
			extraUnlockedRows = 0
		}
		if extraUnlockedRows > 4 {
			extraUnlockedRows = 4
		}
		freeUnlockRowsStats[extraUnlockedRows]++
	}

	totalBet := float64(loops) * betPerSpin
	// 对齐sjxj：奖金直接就是stepMultiplier（倍数）
	// 因为：bonusAmount = betAmount * stepMultiplier / baseMultiplier
	//            = (baseMoney * multiple * 50) * stepMultiplier / 50
	//            = baseMoney * multiple * stepMultiplier
	// 当 baseMoney=1, multiple=1 时，bonusAmount = stepMultiplier
	baseActualWin := float64(baseTotalWin)
	freeActualWin := float64(freeTotalWin)
	totalActualWin := baseActualWin + freeActualWin

	baseRTP := baseActualWin / totalBet
	freeRTP := freeActualWin / totalBet
	totalRTP := totalActualWin / totalBet

	baseGameWinRate := float64(baseGameWinCount) / float64(loops)
	freeTriggerRate := float64(freeEnterCount) / float64(loops)
	avgFreePerTrigger := float64(0)
	if freeEnterCount > 0 {
		avgFreePerTrigger = float64(freeStepsCount) / float64(freeEnterCount)
	}

	fmt.Printf("模拟次数: %d\n", loops)
	fmt.Printf("基础乘数: %d (下注额 = baseMoney * multiple * %d)\n", baseMultiplier, baseMultiplier)
	fmt.Printf("1) 进入免费游戏次数: %d\n", freeEnterCount)
	fmt.Printf("2) 普通游戏总倍数: %d, 实际奖金: %.2f\n", baseTotalWin, baseActualWin)
	fmt.Printf("3) 普通游戏RTP: %.6f\n", baseRTP)
	fmt.Printf("4) baseGame中奖次数: %d, baseGame中奖比例: %.6f\n", baseGameWinCount, baseGameWinRate)
	fmt.Printf("5) 免费游戏总倍数: %d, 实际奖金: %.2f\n", freeTotalWin, freeActualWin)
	fmt.Printf("6) 免费游戏RTP: %.6f\n", freeRTP)
	fmt.Printf("7) 总RTP: %.6f\n", totalRTP)
	fmt.Printf("8) 免费游戏最终解锁额外行数统计[0,1,2,3,4]: %v\n", freeUnlockRowsStats)
	fmt.Printf("9) 基础模式触发免费局比例: %.4f%% (%d/%d)\n", freeTriggerRate*100, freeEnterCount, loops)
	fmt.Printf("10) 平均每次触发获得免费游戏: %.2f 次\n", avgFreePerTrigger)

	fmt.Printf("耗时: %s\n", time.Since(start))
}
