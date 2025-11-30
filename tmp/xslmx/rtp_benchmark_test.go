package xslm2

import (
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

const (
	_benchmarkRounds           int64 = 1e7
	_benchmarkProgressInterval int64 = 1e4
)

func TestRtpBenchmark(t *testing.T) {
	svc := newRtpBetService()
	start := time.Now()
	progressStep := int64(min(_benchmarkProgressInterval, _benchmarkRounds))

	var (
		baseRounds, freeRounds                       int64
		baseWin, freeWin, totalBet                   float64
		baseWinTimes, freeWinTimes, freeTriggerCount int64
	)

	for baseRounds < _benchmarkRounds {
		// 第一次循环时初始化（与 rtp_test.go 保持一致）
		if baseRounds == 0 {
			svc.resetForNextRound(false)
		}

		isFree := svc.client.ClientOfFreeGame.GetFreeNum() > 0
		svc.isFreeRound = isFree

		var nextGrid *int64Grid
		var rollers *[_colCount]SymbolRoller
		var roundWin float64
		var hasWin bool
		cascadeSteps := 0

		for {
			isFirst := cascadeSteps == 0
			if isFirst {
				svc.spin.femaleCountsForFree = svc.scene.FemaleCountsForFree
				svc.spin.nextFemaleCountsForFree = svc.scene.FemaleCountsForFree
				// isFirst代表新的一局，新的一局里面有很多个step（连续消除有多个，没有消除就只有1个step）
				// prevStepTreasureCount 设置为 0，因为新的一局开始时没有上一 step 的夺宝数量
				// 注意：免费模式下，由于夺宝符号不会被消除，order_step.go 中直接使用 stepTreasureCount（最终盘面夺宝数量）作为新增免费次数
				svc.spin.prevStepTreasureCount = 0
				svc.betAmount = decimal.NewFromInt(_cnf.BaseBat)
				svc.client.ClientOfFreeGame.SetBetAmount(svc.betAmount.Round(2).InexactFloat64())
			} else {
				svc.spin.femaleCountsForFree = svc.spin.nextFemaleCountsForFree
				// 连消步骤中，从 scene 恢复上一 step 的夺宝数量
				// 注意：免费模式下，由于夺宝符号不会被消除，order_step.go 中直接使用 stepTreasureCount 作为新增免费次数，不再使用 prevStepTreasureCount
				// 但保留此设置不影响逻辑（可能用于其他场景或调试）
				svc.spin.prevStepTreasureCount = svc.scene.TreasureNum
				svc.betAmount = decimal.NewFromFloat(svc.client.ClientOfFreeGame.GetBetAmount())
			}

			svc.spin.baseSpin(isFree, isFirst, nextGrid, rollers)
			svc.updateStepResult()
			svc.updateScene(isFree)

			cascadeSteps++
			// bonusAmount = betAmount / BaseBat * stepMultiplier
			if stepWin := svc.bonusAmount.InexactFloat64(); stepWin > 0 {
				roundWin += stepWin
				hasWin = true
			}

			if svc.spin.isRoundOver {
				break
			}
			nextGrid, rollers = svc.scene.NextSymbolGrid, svc.scene.SymbolRollers
		}

		newFree := svc.spin.newFreeRoundCount
		svc.resetForNextRound(isFree)

		if isFree {
			freeRounds++
			freeWin += roundWin
			if hasWin {
				freeWinTimes++
			}

			// 免费模式完全结束时，清空场景数据（与 rtp_test.go 保持一致）
			if svc.client.ClientOfFreeGame.GetFreeNum() == 0 {
				svc.scene.FemaleCountsForFree = [3]int64{}
				svc.scene.NextSymbolGrid = nil
				svc.scene.SymbolRollers = nil
				svc.scene.TreasureNum = 0
				svc.scene.RollerKey = ""
			}
			continue
		}

		baseRounds++
		baseWin += roundWin
		totalBet += float64(_cnf.BaseBat)
		if hasWin {
			baseWinTimes++
		}
		if newFree > 0 {
			freeTriggerCount++
		}

		if progressStep > 0 && baseRounds%progressStep == 0 {
			printBenchmarkProgress(baseRounds, totalBet, baseWin, freeWin, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, start)
		}
	}

	fmt.Println()
	printBenchmarkSummary(baseRounds, totalBet, baseWin, freeWin, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, start)
}

func printBenchmarkProgress(baseRounds int64, totalBet, baseWin, freeWin float64, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount int64, start time.Time) {
	if baseRounds == 0 || totalBet == 0 {
		return
	}

	freeRoundsSafe := max(freeRounds, 1)
	elapsed := time.Since(start).Round(time.Second)
	avgFreePerTrigger := float64(0)
	if freeTriggerCount > 0 {
		avgFreePerTrigger = float64(freeRounds) / float64(freeTriggerCount)
	}

	fmt.Printf(
		"\rRuntime=%d baseRTP=%.4f%% baseWinRate=%.4f%% freeRTP=%.4f%% freeWinRate=%.4f%% freeTriggerRate=%.4f%% avgFree=%.4f totalRTP=%.4f%% elapsed=%v",
		baseRounds,
		baseWin*100/totalBet,
		float64(baseWinTimes)*100/float64(baseRounds),
		freeWin*100/totalBet,
		float64(freeWinTimes)*100/float64(freeRoundsSafe),
		float64(freeTriggerCount)*100/float64(baseRounds),
		avgFreePerTrigger,
		(baseWin+freeWin)*100/totalBet,
		elapsed,
	)
}

func printBenchmarkSummary(baseRounds int64, totalBet, baseWin, freeWin float64, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount int64, start time.Time) {
	if baseRounds == 0 || totalBet == 0 {
		fmt.Println("No data collected for RTP benchmark.")
		return
	}

	baseRTP := baseWin * 100 / totalBet
	freeRTP := freeWin * 100 / totalBet
	totalRTP := (baseWin + freeWin) * 100 / totalBet
	baseWinRate := float64(baseWinTimes) * 100 / float64(baseRounds)
	freeWinRate := float64(0)
	if freeRounds > 0 {
		freeWinRate = float64(freeWinTimes) * 100 / float64(freeRounds)
	}
	freeTriggerRate := float64(freeTriggerCount) * 100 / float64(baseRounds)
	avgFreePerRound := float64(0)
	if baseRounds > 0 {
		avgFreePerRound = float64(freeRounds) / float64(baseRounds)
	}
	avgFreePerTrigger := float64(0)
	if freeTriggerCount > 0 {
		avgFreePerTrigger = float64(freeRounds) / float64(freeTriggerCount)
	}

	fmt.Println("[基础模式统计]")
	fmt.Printf("基础模式总游戏局数: %d\n", baseRounds)
	fmt.Printf("基础模式总投注(倍数): %.2f\n", totalBet)
	fmt.Printf("基础模式总奖金: %.2f\n", baseWin)
	fmt.Printf("基础模式RTP: %.2f%%\n", baseRTP)
	fmt.Printf("基础模式免费局触发次数: %d\n", freeTriggerCount)
	fmt.Printf("基础模式触发免费局比例: %.2f%%\n", freeTriggerRate)
	fmt.Printf("基础模式平均每局免费次数: %.2f\n", avgFreePerRound)
	fmt.Printf("基础模式中奖率: %.2f%%\n", baseWinRate)
	fmt.Printf("基础模式中奖局数: %d\n", baseWinTimes)

	fmt.Println()
	fmt.Println("[免费模式统计]")
	fmt.Printf("免费模式总游戏局数: %d\n", freeRounds)
	fmt.Printf("免费模式总奖金: %.2f\n", freeWin)
	fmt.Printf("免费模式RTP: %.2f%%\n", freeRTP)
	fmt.Printf("免费模式中奖率: %.2f%%\n", freeWinRate)
	fmt.Printf("免费模式中奖局数: %d\n", freeWinTimes)

	fmt.Println()
	fmt.Println("[免费触发效率]")
	fmt.Printf("  总免费游戏次数: %d | 总触发次数: %d\n", freeRounds, freeTriggerCount)
	if freeTriggerCount > 0 {
		fmt.Printf("  平均每次触发获得免费次数: %.2f\n", avgFreePerTrigger)
	} else {
		fmt.Printf("  平均每次触发获得免费次数: 0\n")
	}

	fmt.Println()
	fmt.Println("[总计]")
	fmt.Printf("总回报率(RTP): %.2f%%\n", totalRTP)
}

/*
***************************************************************
***************************************************************
***************************************************************

 */

func TestFindWinInfos(t *testing.T) {
	s := &spin{}
	grid := int64Grid{
		{_wildFemaleA, _femaleA, _femaleA, 0, 0},
		{_wildFemaleB, _femaleB, _femaleB, 0, 0},
		{1, _femaleC, _wild, 0, 0},
		{0, 1, 0, 0, 0},
	}
	//grid := int64Grid{
	//	{_wildFemaleA, _wildFemaleA, 0, 0, 0},
	//	{_wildFemaleB, _wildFemaleA, 0, 0, 0},
	//	{_wildFemaleA, _wildFemaleA, _wild, 0, 0},
	//	{0, 0, 0, 0, 0},
	//}
	t.Logf("原始网格:%v", WGrid(&grid))

	s.symbolGrid = &grid
	s.findWinInfos()
	winInfos := s.winInfos
	for _, info := range winInfos {
		t.Logf("symbol=%d count=%d lines=%d 中奖符号网格=%v", info.Symbol, info.SymbolCount, info.LineCount, WGrid(&info.WinGrid))
	}
}

func TestFillElimFreePartialEliminatesFemaleWild(t *testing.T) {
	grid := int64Grid{
		//{_blocked, 0, 0, 0, _blocked},
		//{0, 1, 1, 0, 0},
		//{11, 1, 1, 0, 0},
		//{0, 9, 2, 0, 0},

		//{99, 13, 13, 13, 99},
		//{8, 8, 5, 8, 5},
		//{1, 8, 5, 8, 1},
		//{4, 14, 5, 14, 7},

		{99, 2, 8, 7, 99},
		{12, 12, 13, 4, 6},
		{6, 12, 7, 13, 6},
		{6, 1, 7, 8, 7},
	}
	s := &spin{
		symbolGrid: &grid,
	}

	isFree := true

	t.Logf("原始网格:%v", WGrid(&grid))

	if ok := s.findWinInfos(); !ok {
		t.Fatalf("expected female win infos")
	}

	s.updateStepResults()
	mode, nextGrid := s.execEliminateGrid(isFree)

	t.Logf("mode=%v 中奖网格=%v", mode, WGrid(s.winGrid))
	t.Logf("下个网格:%v", WGrid(nextGrid))

	//gridCopy := grid
	//eliminated := s.fillElimFreePartial(&gridCopy)
	//t.Logf("expected elimination:%v", eliminated)
	//if eliminated == 0 {
	//	t.Fatalf("expected elimination count > 0")
	//}

	//t.Logf("消除标记:%v", WGrid(s.winGrid))

	//if gridCopy[1][1] != _eliminated {
	//	t.Fatalf("expected wild female at (1,1) eliminated, got %d", gridCopy[1][1])
	//}

	//idx := _femaleC - _femaleA
	//if s.nextFemaleCountsForFree[idx] == 0 {
	//	t.Fatalf("expected female collection increment for index %d", idx)
	//}
}
