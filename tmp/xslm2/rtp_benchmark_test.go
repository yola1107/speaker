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

type roundOutcome struct {
	isFree          bool
	roundWin        float64
	cascadeSteps    int
	hasWin          bool
	treasureCount   int64
	newFree         int64
	fullElimination bool
	femaleCollect   [3]int64
	femaleSymbolWin float64
	femaleWildWin   float64
}

func TestRtpBenchmark(t *testing.T) {
	baseStats := &rtpStats{}
	freeStats := &rtpStats{}
	totalBet := 0.0
	start := time.Now()
	buf := &strings.Builder{}

	svc := newRtpBetService()
	progressStep := int64(min(_benchmarkProgressInterval, _benchmarkRounds))

	for baseStats.rounds < _benchmarkRounds {
		outcome := playRound(svc)

		if outcome.isFree {
			collectFreeStats(freeStats, outcome)
			continue
		}

		collectBaseStats(baseStats, outcome)
		totalBet += float64(_cnf.BaseBat)

		if baseStats.rounds > 0 && baseStats.rounds%progressStep == 0 {
			printProgress(buf, baseStats.rounds, totalBet, baseStats.totalWin, freeStats.totalWin, time.Since(start))
			if buf.Len() > 0 {
				t.Log(buf.String())
			}
			buf.Reset()
		}
	}

	printFinalStats(buf, baseStats, freeStats, totalBet, start)
	t.Log(buf.String())
	t.Logf("女性符号贡献：基础=%.2f 免费=%.2f；女性百搭贡献：基础=%.2f 免费=%.2f",
		baseStats.femaleSymbolWin, freeStats.femaleSymbolWin,
		baseStats.femaleWildWin, freeStats.femaleWildWin)
}

func playRound(svc *betOrderService) roundOutcome {
	isFree := svc.client.ClientOfFreeGame.GetFreeNum() > 0
	svc.isFreeRound = isFree

	var (
		nextGrid *int64Grid
		rollers  *[_colCount]SymbolRoller

		cascadeSteps    int
		roundWin        float64
		hasWin          bool
		fullScreen      bool
		femaleDelta     [3]int64
		femaleSymbolWin float64
		femaleWildWin   float64
	)

	for {
		if len(svc.spin.winResults) > 0 {
			logWinDetails(svc, cascadeSteps, isFree)
		}
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

		for _, wr := range svc.spin.winResults {
			switch {
			case wr.Symbol >= _femaleA && wr.Symbol <= _femaleC:
				femaleSymbolWin += float64(wr.TotalMultiplier)
			case wr.Symbol >= _wildFemaleA && wr.Symbol <= _wildFemaleC:
				femaleWildWin += float64(wr.TotalMultiplier)
			}
		}

		if isFree {
			for i := 0; i < 3; i++ {
				delta := svc.spin.nextFemaleCountsForFree[i] - svc.spin.femaleCountsForFree[i]
				if delta > 0 {
					femaleDelta[i] += delta
				}
			}
		}

		if svc.spin.enableFullElimination {
			fullScreen = true
		}

		if svc.spin.isRoundOver {
			break
		}

		nextGrid = svc.scene.NextSymbolGrid
		rollers = svc.scene.SymbolRollers
	}

	treasureCount := svc.spin.treasureCount
	newFree := svc.spin.newFreeRoundCount

	svc.resetForNextRound(isFree)

	return roundOutcome{
		isFree:          isFree,
		roundWin:        roundWin,
		cascadeSteps:    cascadeSteps,
		hasWin:          hasWin,
		treasureCount:   treasureCount,
		newFree:         newFree,
		fullElimination: fullScreen,
		femaleCollect:   femaleDelta,
		femaleSymbolWin: femaleSymbolWin,
		femaleWildWin:   femaleWildWin,
	}
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

func collectBaseStats(base *rtpStats, outcome roundOutcome) {
	base.rounds++
	base.totalWin += outcome.roundWin
	base.femaleSymbolWin += outcome.femaleSymbolWin
	base.femaleWildWin += outcome.femaleWildWin
	if outcome.hasWin {
		base.winRounds++
	}
	base.cascadeSteps += int64(outcome.cascadeSteps)
	if outcome.cascadeSteps > base.maxCascadeSteps {
		base.maxCascadeSteps = outcome.cascadeSteps
	}
	if outcome.cascadeSteps < len(base.cascadeDistrib) {
		base.cascadeDistrib[outcome.cascadeSteps]++
	}
	if outcome.treasureCount >= 1 && outcome.treasureCount <= 5 {
		base.treasureCount[outcome.treasureCount]++
	}
	if outcome.newFree > 0 {
		base.freeTriggered++
		base.totalFreeGiven += outcome.newFree
	}
}

func collectFreeStats(free *rtpStats, outcome roundOutcome) {
	free.rounds++
	free.totalWin += outcome.roundWin
	free.femaleSymbolWin += outcome.femaleSymbolWin
	free.femaleWildWin += outcome.femaleWildWin
	if outcome.hasWin {
		free.winRounds++
		free.freeWithCascade++
	} else {
		free.freeNoCascade++
	}
	free.cascadeSteps += int64(outcome.cascadeSteps)
	if outcome.cascadeSteps > free.maxCascadeSteps {
		free.maxCascadeSteps = outcome.cascadeSteps
	}
	if outcome.cascadeSteps < len(free.cascadeDistrib) {
		free.cascadeDistrib[outcome.cascadeSteps]++
	}
	if outcome.treasureCount > 0 {
		free.treasureInFree++
		free.extraFreeRounds += outcome.newFree
	}
	if outcome.fullElimination {
		free.fullElimination++
	}
	for i := 0; i < 3; i++ {
		free.femaleCollect[i] += outcome.femaleCollect[i]
	}
}
