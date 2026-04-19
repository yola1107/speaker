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

	const loops = 100000000
	// 单注：仅作 RTP 分母，与 missWorld 原版一致（赔表/免费倍数为固定数值，不随 baseMultiplier 变）。
	const betPerSpin = 50.0
	// 生产环境若使用 bonusAmount = betAmount * mult / baseMultiplier，只影响真钱换算，与本模拟 RTP 无关。
	const baseMultiplier = 50

	var freeEnterCount int
	var baseTotalWin int
	var freeTotalWin int
	var baseGameWinCount int
	var freeStepsCount int
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
	baseRTP := float64(baseTotalWin) / totalBet
	freeRTP := float64(freeTotalWin) / totalBet
	totalRTP := float64(baseTotalWin+freeTotalWin) / totalBet

	baseGameWinRate := float64(baseGameWinCount) / float64(loops)
	freeTriggerRate := float64(freeEnterCount) / float64(loops)
	avgFreePerTrigger := 0.0
	if freeEnterCount > 0 {
		avgFreePerTrigger = float64(freeStepsCount) / float64(freeEnterCount)
	}

	fmt.Printf("模拟次数: %d\n", loops)
	fmt.Printf("单注(用于RTP): %.0f（与 baseMultiplier 脱钩；改 baseMultiplier 不改变下列 RTP）\n", betPerSpin)
	fmt.Printf("baseMultiplier(仅对照生产公式, 不参与本程序 RTP): %d\n", baseMultiplier)
	fmt.Printf("1) 进入免费游戏次数: %d\n", freeEnterCount)
	fmt.Printf("2) 普通游戏总中奖金额: %d\n", baseTotalWin)
	fmt.Printf("3) 普通游戏RTP: %.6f\n", baseRTP)
	fmt.Printf("4) baseGame中奖次数: %d, baseGame中奖比例: %.6f\n", baseGameWinCount, baseGameWinRate)
	fmt.Printf("5) 免费游戏总中奖金额: %d\n", freeTotalWin)
	fmt.Printf("6) 免费游戏RTP: %.6f\n", freeRTP)
	fmt.Printf("7) 总RTP: %.6f\n", totalRTP)
	fmt.Printf("8) 免费游戏最终解锁额外行数统计[0,1,2,3,4]: %v\n", freeUnlockRowsStats)
	fmt.Printf("9) 基础模式触发免费局比例: %.4f%% (%d/%d)\n", freeTriggerRate*100, freeEnterCount, loops)
	fmt.Printf("10) 平均每次触发获得免费步数: %.2f\n", avgFreePerTrigger)
	fmt.Printf("耗时: %s\n", time.Since(start))
}
