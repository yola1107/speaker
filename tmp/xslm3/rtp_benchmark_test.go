package xslm2

import (
	"fmt"
	"strings"
	"testing"
	"time"
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
		baseRounds       int64
		freeRounds       int64
		baseWin          float64
		freeWin          float64
		baseWinTimes     int64
		freeWinTimes     int64
		freeTriggerCount int64
		totalFreeGiven   int64
		totalBet         float64
	)

	for baseRounds < _benchmarkRounds {
		isFree, roundWin, hasWin, newFree := playRound(svc)

		if isFree {
			freeRounds++
			freeWin += roundWin
			if hasWin {
				freeWinTimes++
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
			totalFreeGiven += newFree
		}

		if progressStep > 0 && baseRounds%progressStep == 0 {
			printBenchmarkProgress(baseRounds, totalBet, baseWin, freeWin, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, totalFreeGiven, start)
		}
	}

	fmt.Println()
	printBenchmarkSummary(baseRounds, totalBet, baseWin, freeWin, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, totalFreeGiven, start)
}

func playRound(svc *betOrderService) (bool, float64, bool, int64) {
	isFree := svc.client.ClientOfFreeGame.GetFreeNum() > 0
	svc.isFreeRound = isFree

	var (
		nextGrid     *int64Grid
		rollers      *[_colCount]SymbolRoller
		cascadeSteps int
		roundWin     float64
		hasWin       bool
	)

	for {
		// 调试日志过于冗长，默认关闭
		// if len(svc.spin.winResults) > 0 {
		// 	logWinDetails(svc, cascadeSteps, isFree)
		// }
		isFirst := cascadeSteps == 0
		if isFirst {
			svc.spin.femaleCountsForFree = svc.scene.FemaleCountsForFree
			svc.spin.nextFemaleCountsForFree = svc.scene.FemaleCountsForFree
		} else {
			svc.spin.femaleCountsForFree = svc.spin.nextFemaleCountsForFree
		}

		svc.spin.baseSpin(isFree, isFirst, nextGrid, rollers)
		svc.updateStepResult()
		svc.updateScene(isFree)

		cascadeSteps++
		stepWin := float64(svc.spin.stepMultiplier)
		roundWin += stepWin
		if stepWin > 0 {
			hasWin = true
		}

		if svc.spin.isRoundOver {
			break
		}

		nextGrid = svc.scene.NextSymbolGrid
		rollers = svc.scene.SymbolRollers
	}

	newFree := svc.spin.newFreeRoundCount

	svc.resetForNextRound(isFree)

	return isFree, roundWin, hasWin, newFree
}

func logWinDetails(svc *betOrderService, step int, isFree bool) {
	mode := "BASE"
	if isFree {
		mode = "FREE"
	}
	builder := &strings.Builder{}
	builder.WriteString("\n---- RTP DEBUG ----\n")
	builder.WriteString(
		fmt.Sprintf("%s Step=%d, RoundOver=%v, StepWin=%d\n",
			mode, step, svc.spin.isRoundOver, svc.spin.stepMultiplier))
	for _, wr := range svc.spin.winResults {
		builder.WriteString(fmt.Sprintf("Symbol=%d Count=%d Lines=%d Total=%d\n",
			wr.Symbol, wr.SymbolCount, wr.LineCount, wr.TotalMultiplier))
	}
	fmt.Println(builder.String())
}

func printBenchmarkProgress(baseRounds int64, totalBet, baseWin, freeWin float64, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, totalFreeGiven int64, start time.Time) {
	if baseRounds == 0 || totalBet == 0 {
		return
	}

	freeRoundsSafe := max64(freeRounds, 1)
	elapsed := time.Since(start).Round(time.Second)

	fmt.Printf(
		"\rRuntime=%d baseRTP=%.4f%% baseWinRate=%.4f%% freeRTP=%.4f%% freeWinRate=%.4f%% freeTriggerRate=%.4f%% avgFree=%.4f totalRTP=%.4f%% elapsed=%v",
		baseRounds,
		baseWin*100/totalBet,
		float64(baseWinTimes)*100/float64(baseRounds),
		freeWin*100/totalBet,
		float64(freeWinTimes)*100/float64(freeRoundsSafe),
		float64(freeTriggerCount)*100/float64(baseRounds),
		float64(totalFreeGiven)/float64(max64(freeTriggerCount, 1)),
		(baseWin+freeWin)*100/totalBet,
		elapsed,
	)
}

func printBenchmarkSummary(baseRounds int64, totalBet, baseWin, freeWin float64, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, totalFreeGiven int64, start time.Time) {
	if baseRounds == 0 || totalBet == 0 {
		fmt.Println("No data collected for RTP benchmark.")
		return
	}

	freeRoundsSafe := max64(freeRounds, 1)
	elapsed := time.Since(start).Round(time.Second)
	baseRTP := baseWin * 100 / totalBet
	freeRTP := freeWin * 100 / totalBet
	totalRTP := (baseWin + freeWin) * 100 / totalBet
	baseWinRate := float64(baseWinTimes) * 100 / float64(baseRounds)
	freeWinRate := float64(freeWinTimes) * 100 / float64(freeRoundsSafe)
	freeTriggerRate := float64(freeTriggerCount) * 100 / float64(baseRounds)
	avgFreePerTrigger := float64(totalFreeGiven) / float64(max64(freeTriggerCount, 1))

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Benchmark Rounds: %d | Free Rounds: %d | Elapsed: %v\n", baseRounds, freeRounds, elapsed)
	fmt.Printf("Base RTP: %.4f%% | Free RTP: %.4f%% | Total RTP: %.4f%%\n", baseRTP, freeRTP, totalRTP)
	fmt.Printf("Base Win Rate: %.4f%% | Free Win Rate: %.4f%%\n", baseWinRate, freeWinRate)
	fmt.Printf("Free Trigger Rate: %.4f%% | Avg Free/Trigger: %.4f\n", freeTriggerRate, avgFreePerTrigger)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
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

		{99, 13, 13, 13, 99},
		{8, 8, 5, 8, 5},
		{1, 8, 5, 8, 1},
		{4, 14, 5, 14, 7},
	}
	s := &spin{
		symbolGrid: &grid,
	}
	isFree := false

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
