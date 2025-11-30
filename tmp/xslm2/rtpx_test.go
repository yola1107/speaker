package xslm2

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

const (
	testRounds       = 1e8
	progressInterval = 1e7
	debugFileOpen    = 0
	freeModeLogOnly  = 0
)

func TestRtp2(t *testing.T) {
	var (
		baseRounds, baseWinRounds, baseCascadeSteps, baseFreeTriggered   int64
		baseTotalWin, baseFemaleSymbolWin, baseFemaleWildWin             float64
		baseMaxCascadeSteps                                              int
		freeRounds, freeWinRounds, freeCascadeSteps, freeFullElimination int64
		freeTreasureInFree, freeExtraFreeRounds, freeMaxFreeStreak       int64
		freeTotalWin, freeFemaleSymbolWin, freeFemaleWildWin             float64
		freeMaxCascadeSteps                                              int
		freeFemaleStateCount                                             [10]int64
		femaleKeyWins                                                    [10]float64
	)

	totalBet, start := 0.0, time.Now()
	buf := &strings.Builder{}
	svc := newBerService()
	baseGameCount, freeRoundIdx := 0, 0
	interval := int64(min(testRounds, progressInterval))

	var fileBuf *strings.Builder
	if debugFileOpen > 0 {
		fileBuf = &strings.Builder{}
	}

	for baseRounds < testRounds {
		var cascadeCount, gameNum int
		var roundWin, freeRoundWin float64
		var triggeringBaseRound int

		for {
			isFirst := svc.scene.Steps == 0
			wasFreeBeforeSpin := svc.isFreeRound
			svc.isFirst = isFirst

			if isFirst {
				roundWin = 0
				freeRoundWin = 0
			}

			if err := svc.baseSpin(); err != nil {
				panic(err)
			}

			isFree := svc.isFreeRound

			if isFirst && !wasFreeBeforeSpin && isFree {
				cascadeCount = 0
			}

			if isFirst {
				if isFree {
					freeRoundIdx++
					gameNum = freeRoundIdx
					if triggeringBaseRound == 0 {
						triggeringBaseRound = baseGameCount
					}
					freeFemaleStateCount[svc.scene.SymbolRoller[0].Real]++
				} else {
					baseGameCount++
					gameNum = baseGameCount
				}
			}

			cascadeCount++
			stepWin := float64(svc.stepMultiplier)
			roundWin += stepWin

			if isFree {
				if svc.scene.FreeNum > freeMaxFreeStreak {
					freeMaxFreeStreak = svc.scene.FreeNum
				}
			}

			if debugFileOpen > 0 && fileBuf != nil && (freeModeLogOnly == 0 || isFree) {
				triggerRound := 0
				if isFree {
					triggerRound = triggeringBaseRound
					if triggerRound == 0 && isFirst {
						triggerRound = baseGameCount
					}
				}
				writeSpinDetail(fileBuf, svc, gameNum, cascadeCount, isFree, triggerRound, stepWin, roundWin, isFirst)
			}

			if isFree {
				freeTotalWin += stepWin
				freeRoundWin += stepWin
				updateWinStats(svc.winResults, &freeFemaleSymbolWin, &freeFemaleWildWin)
				if svc.newFreeRoundCount > 0 {
					freeTreasureInFree++
					freeExtraFreeRounds += svc.newFreeRoundCount
				}
			} else {
				baseTotalWin += stepWin
				updateWinStats(svc.winResults, &baseFemaleSymbolWin, &baseFemaleWildWin)
			}

			if svc.isRoundOver {
				if isFree {
					freeCascadeSteps += int64(cascadeCount)
					if cascadeCount > freeMaxCascadeSteps {
						freeMaxCascadeSteps = cascadeCount
					}
					freeRounds++
					if freeRoundWin > 0 {
						freeWinRounds++
					}
					femaleKeyWins[svc.scene.SymbolRoller[0].Real] += roundWin
					freeRoundWin = 0
				} else {
					baseCascadeSteps += int64(cascadeCount)
					if cascadeCount > baseMaxCascadeSteps {
						baseMaxCascadeSteps = cascadeCount
					}
					baseRounds++
					if roundWin > 0 {
						baseWinRounds++
					}
					totalBet += float64(_baseMultiplier)
					if svc.newFreeRoundCount > 0 {
						baseFreeTriggered++
					}
					if svc.isFreeRound {
						triggeringBaseRound = baseGameCount
					}
				}
				roundWin = 0

				if svc.scene.FreeNum <= 0 {
					resetBetServiceForNextRound(svc)
					freeRoundIdx = 0
					if baseRounds%interval == 0 {
						printProgress(buf, baseRounds, totalBet, baseTotalWin, freeTotalWin, time.Since(start), baseWinRounds, freeWinRounds, baseFreeTriggered, freeRounds)
						fmt.Print(buf.String())
					}
					break
				}
				cascadeCount = 0
			}
		}
	}

	printFinalStats(buf, baseRounds, baseTotalWin, baseWinRounds, baseFemaleSymbolWin, baseFemaleWildWin,
		baseCascadeSteps, baseMaxCascadeSteps, baseFreeTriggered, freeRounds, freeTotalWin, freeWinRounds,
		freeFemaleSymbolWin, freeFemaleWildWin, freeCascadeSteps, freeMaxCascadeSteps, freeFullElimination,
		freeTreasureInFree, freeExtraFreeRounds, freeMaxFreeStreak, freeFemaleStateCount, femaleKeyWins, totalBet, start)
	result := buf.String()
	fmt.Print(result)
	if debugFileOpen > 0 && fileBuf != nil {
		saveDebugFile(result, fileBuf.String(), start)
	}
}

func updateWinStats(winResults []*winResult, femaleSymbolWin, femaleWildWin *float64) {
	for _, wr := range winResults {
		gain := float64(wr.TotalMultiplier)
		switch {
		case wr.Symbol >= _femaleA && wr.Symbol <= _femaleC:
			*femaleSymbolWin += gain
		case wr.Symbol >= _wildFemaleA && wr.Symbol <= _wildFemaleC:
			*femaleWildWin += gain
		}
	}
}

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, gameNum, step int, isFree bool, triggeringBaseRound int, stepWin, roundWin float64, isFirstStep bool) {
	if step == 1 {
		writeRoundHeader(buf, svc, gameNum, isFree, triggeringBaseRound)
	} else {
		writeReelInfo(buf, svc)
	}

	if isFree && isFirstStep {
		buf.WriteString(fmt.Sprintf("女性收集状态（上一局结算/控制滚轴）: %v\n", svc.scene.RoundFemaleCountsForFree))
	}
	if isFree {
		buf.WriteString(fmt.Sprintf("女性收集状态（每步开始）: %v\n", svc.scene.FemaleCountsForFree))
	}

	buf.WriteString(fmt.Sprintf("Step%d 初始盘面:\n", step))
	writeGridToBuilder(buf, svc.symbolGrid, nil)

	if len(svc.winResults) > 0 {
		buf.WriteString(fmt.Sprintf("Step%d 中奖标记:\n", step))
		writeGridToBuilder(buf, svc.symbolGrid, svc.winGrid)
	}

	if !svc.isRoundOver && svc.nextSymbolGrid != nil {
		buf.WriteString(fmt.Sprintf("Step%d 下一盘面预览（实际消除+下落+填充结果）:\n", step))
		writeGridToBuilder(buf, svc.nextSymbolGrid, nil)
	}

	writeStepSummary(buf, svc, step, isFree, stepWin, roundWin)
	buf.WriteString("\n")
}

func writeReelInfo(buf *strings.Builder, svc *betOrderService) {
	if svc.scene == nil {
		buf.WriteString("滚轴配置Index: 0\n转轮信息长度/起始：未初始化\n")
		return
	}
	buf.WriteString(fmt.Sprintf("滚轴配置Index: %d\n", svc.scene.SymbolRoller[0].Real))
	buf.WriteString("转轮信息长度/起始：")
	for c := int64(0); c < _colCount; c++ {
		if c > 0 {
			buf.WriteString("， ")
		}

		length := len(svc.gameConfig.RealData[svc.scene.SymbolRoller[c].Real][int(c)])
		start := svc.scene.SymbolRoller[c].Start
		fall := svc.scene.SymbolRoller[c].Fall
		if length > 0 {
			buf.WriteString(fmt.Sprintf("%d[%d～%d]", length, start, fall))
		} else {
			buf.WriteString("0[0～0]")
		}
	}
	buf.WriteString("\n")
}

func writeRoundHeader(buf *strings.Builder, svc *betOrderService, gameNum int, isFree bool, triggeringBaseRound int) {
	if isFree {
		if triggeringBaseRound == 0 {
			buf.WriteString(fmt.Sprintf("\n=============[基础模式] 第?局 - 免费第%d局 =============\n", gameNum))
		} else {
			buf.WriteString(fmt.Sprintf("\n=============[基础模式] 第%d局 - 免费第%d局 =============\n", triggeringBaseRound, gameNum))
		}
	} else {
		buf.WriteString(fmt.Sprintf("\n=============[基础模式] 第%d局 =============\n", gameNum))
	}
	writeReelInfo(buf, svc)
	if isFree && svc.enableFullElimination {
		buf.WriteString("🎯 全屏消除模式已激活（三种女性符号均>=10）\n")
	}
}

func getTreasureCountFromGrid(grid *int64Grid) int64 {
	if grid == nil {
		return 0
	}
	count := int64(0)
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if grid[r][c] == _treasure {
				count++
			}
		}
	}
	return count
}

func writeStepSummary(buf *strings.Builder, svc *betOrderService, step int, isFree bool, stepWin, roundWin float64) {
	buf.WriteString(fmt.Sprintf("Step%d 中奖详情:\n", step))

	var finalTreasureCount int64
	if svc.isRoundOver && svc.nextSymbolGrid != nil {
		finalTreasureCount = getTreasureCountFromGrid(svc.nextSymbolGrid)
	} else {
		finalTreasureCount = svc.getTreasureCount()
	}
	currentTreasureCount := svc.getTreasureCount()

	if len(svc.winResults) == 0 {
		buf.WriteString("\t未中奖\n")
		if svc.isRoundOver {
			if isFree && finalTreasureCount > 0 {
				buf.WriteString(fmt.Sprintf("\t💎 当前盘面夺宝数量: %d \n", finalTreasureCount))
			}
			if !isFree && svc.newFreeRoundCount >= 3 {
				buf.WriteString(fmt.Sprintf("\t💎 基础模式。 夺宝=%d 免费次数=%d\n", finalTreasureCount, svc.newFreeRoundCount))
			}
		}
		return
	}

	buf.WriteString(fmt.Sprintf("\t基础=%v, 触发: 女性中奖=%v, 女性百搭参与=%v, 有百搭=%v, 全屏=%v, 有夺宝=%v, (%d)\n",
		!isFree, svc.hasFemaleWin, svc.hasFemaleWildWin, svc.hasWildSymbol(), svc.enableFullElimination, currentTreasureCount > 0, currentTreasureCount))

	if isFree {
		final := svc.nextFemaleCountsForFree
		stepDelta := [3]int64{final[0] - svc.scene.FemaleCountsForFree[0], final[1] - svc.scene.FemaleCountsForFree[1], final[2] - svc.scene.FemaleCountsForFree[2]}
		buf.WriteString(fmt.Sprintf("\t女性收集: 上一步=%v → 当前=%v (本步=%v)\n", svc.scene.FemaleCountsForFree, final, stepDelta))
	} else {
		final := svc.nextFemaleCountsForFree
		if final[0] != 0 || final[1] != 0 || final[2] != 0 {
			buf.WriteString(fmt.Sprintf("\t⚠️ 警告: 基础模式不应该收集女性符号，但检测到收集=%v\n", final))
		}
	}

	reason := "无女性中奖"
	if svc.hasFemaleWin {
		if isFree && svc.enableFullElimination {
			reason = "女性中奖且全屏消除启动"
		} else if isFree {
			reason = "女性中奖触发部分消除"
		} else {
			reason = "女性中奖与百搭触发"
		}
	}

	if !svc.isRoundOver {
		buf.WriteString(fmt.Sprintf("\t🔁 连消继续 → Step%d (%s)\n\n", step+1, reason))
	} else {
		stopReason := "无后续可消除"
		if svc.hasFemaleWin {
			if svc.enableFullElimination {
				stopReason = "全屏消除已完成"
			} else {
				stopReason = "女性连消在本步结束"
			}
		}
		buf.WriteString(fmt.Sprintf("\t🛑 连消结束（%s）\n\n", stopReason))
	}

	lineBet := svc.betAmount.Div(decimal.NewFromInt(_baseMultiplier))
	for _, wr := range svc.winResults {
		amount := lineBet.Mul(decimal.NewFromInt(wr.TotalMultiplier)).Round(2).InexactFloat64()
		buf.WriteString(fmt.Sprintf("\t符号: %d(%d), 连线: %d, 乘积: %d, 赔率: %.2f, 下注: %g×%d, 奖金: %g\n",
			wr.Symbol, wr.Symbol, wr.SymbolCount, wr.LineCount, float64(wr.BaseLineMultiplier),
			svc.req.BaseMoney, svc.req.Multiple, amount))
	}
	buf.WriteString(fmt.Sprintf("\t累计中奖: %.2f\n", roundWin))

	if isFree && svc.isRoundOver && finalTreasureCount > 0 {
		buf.WriteString(fmt.Sprintf("\t💎 当前盘面夺宝数量: %d \n", finalTreasureCount))
	}
	if !isFree && svc.isRoundOver && svc.newFreeRoundCount > 0 {
		buf.WriteString(fmt.Sprintf("\t基础模式。 夺宝=%d 免费次数=%d\n", finalTreasureCount, svc.newFreeRoundCount))
	}
}

func saveDebugFile(statsResult, detailResult string, start time.Time) {
	_ = os.MkdirAll("logs", 0755)
	filename := fmt.Sprintf("logs/%s.txt", time.Now().Format("20060102_150405"))
	_ = os.WriteFile(filename, []byte(statsResult+detailResult), 0644)
	fmt.Printf("\n📄 调试信息已保存到: %s\n", filename)
}

func printProgress(buf *strings.Builder, rounds int64, totalBet, baseWin, freeWin float64, elapsed time.Duration, baseWinRounds, freeWinRounds, baseFreeTriggered, freeRounds int64) {
	if totalBet <= 0 {
		return
	}
	buf.Reset()
	baseRtp := calculateRtp(int64(baseWin), rounds, _baseMultiplier)
	baseWinRate := calculateRtp(baseWinRounds, rounds, 1)
	freeRtp := calculateRtp(int64(freeWin), rounds, _baseMultiplier)
	freeWinRate := calculateRtp(freeWinRounds, max(freeRounds, 1), 1)
	freeTriggerRate := calculateRtp(baseFreeTriggered, rounds, 1)
	totalRtp := calculateRtp(int64(baseWin+freeWin), rounds, _baseMultiplier)
	fmt.Fprintf(buf, "\rRuntime=%d baseRtp=%.4f%%,baseWinRate=%.4f%% freeRtp=%.4f%% freeWinRate=%.4f%%, freeTriggerRate=%.4f%% Rtp=%.4f%% 用时=%v\n",
		rounds, baseRtp, baseWinRate, freeRtp, freeWinRate, freeTriggerRate, totalRtp, elapsed.Round(time.Second))
}

func printWinContribution(w func(string, ...interface{}), femaleSymbolWin, femaleWildWin, totalWin float64) {
	if totalWin > 0 {
		w("  女性符号中奖贡献: %.2f (%.2f%%)\n", femaleSymbolWin, femaleSymbolWin*100/totalWin)
		w("  女性百搭中奖贡献: %.2f (%.2f%%)\n", femaleWildWin, femaleWildWin*100/totalWin)
	} else {
		w("  女性符号中奖贡献: %.2f\n", femaleSymbolWin)
		w("  女性百搭中奖贡献: %.2f\n", femaleWildWin)
	}
}

func printFinalStats(buf *strings.Builder,
	baseRounds int64, baseTotalWin float64, baseWinRounds int64, baseFemaleSymbolWin float64, baseFemaleWildWin float64, baseCascadeSteps int64, baseMaxCascadeSteps int, baseFreeTriggered int64,
	freeRounds int64, freeTotalWin float64, freeWinRounds int64, freeFemaleSymbolWin float64, freeFemaleWildWin float64, freeCascadeSteps int64, freeMaxCascadeSteps int, freeFullElimination int64, freeTreasureInFree int64, freeExtraFreeRounds int64, freeMaxFreeStreak int64, freeFemaleStateCount [10]int64, femaleKeyWins [10]float64,
	totalBet float64, start time.Time) {
	w := func(s string, args ...interface{}) { buf.WriteString(fmt.Sprintf(s, args...)) }

	w("\n===== 详细统计汇总 =====\n")
	w("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	w("\n[基础模式统计]\n")
	w("基础模式总游戏局数: %d\n", baseRounds)
	w("基础模式总投注(倍数): %.2f\n", totalBet)
	w("基础模式总奖金: %.2f\n", baseTotalWin)
	if totalBet > 0 {
		w("基础模式RTP: %.2f%% (基础模式奖金/基础模式投注)\n", baseTotalWin*100/totalBet)
	}
	w("基础模式免费局触发次数: %d\n", baseFreeTriggered)
	if baseRounds > 0 {
		w("基础模式触发免费局比例: %.2f%%\n", float64(baseFreeTriggered)*100/float64(baseRounds))
		w("基础模式平均每局免费次数: %.2f\n", float64(freeRounds)/float64(baseRounds))
		w("基础模式中奖率: %.2f%%\n", float64(baseWinRounds)*100/float64(baseRounds))
		w("基础模式平均连消步数: %.2f\n", float64(baseCascadeSteps)/float64(baseRounds))
		w("基础模式最大连消步数: %d\n", baseMaxCascadeSteps)
	}
	w("基础模式中奖局数: %d\n", baseWinRounds)
	w("\n[基础模式中奖贡献分析]\n")
	printWinContribution(w, baseFemaleSymbolWin, baseFemaleWildWin, baseTotalWin)

	w("\n[免费模式统计]\n")
	w("免费模式总游戏局数: %d\n", freeRounds)
	w("免费模式总奖金: %.2f\n", freeTotalWin)
	if totalBet > 0 {
		w("免费模式RTP: %.2f%% (免费模式奖金/基础模式投注，因为免费模式不投注)\n", freeTotalWin*100/totalBet)
	}
	w("免费模式额外增加局数: %d\n", freeExtraFreeRounds)
	w("免费模式最大连续局数: %d\n", freeMaxFreeStreak)
	w("免费模式中奖局数: %d\n", freeWinRounds)
	if freeRounds > 0 {
		w("免费模式中奖率: %.2f%%\n", float64(freeWinRounds)*100/float64(freeRounds))
		w("免费模式全屏消除次数: %d (%.2f%%)\n", freeFullElimination, float64(freeFullElimination)*100/float64(freeRounds))
		w("免费模式出现夺宝的次数: %d (%.2f%%)\n", freeTreasureInFree, float64(freeTreasureInFree)*100/float64(freeRounds))
		w("免费模式平均连消步数: %.2f\n", float64(freeCascadeSteps)/float64(freeRounds))
		w("免费模式最大连消步数: %d\n", freeMaxCascadeSteps)
		w("\n[免费模式女性符号状态统计]\n")
		totalStateCount := int64(0)
		for i := 0; i < 10; i++ {
			totalStateCount += freeFemaleStateCount[i]
		}
		w("  总统计次数: %d (应该等于免费模式总游戏局数: %d)\n", totalStateCount, freeRounds)
		for i := 1; i < 9; i++ {
			count := freeFemaleStateCount[i]
			w("  状态 %s: %.4f%% (%d次)\n", stateNames[i], float64(count)*100/float64(freeRounds), count)
		}
		w("\n[免费模式女性 key 赢分统计]\n")
		for i := 0; i < len(femaleKeyWins); i++ {
			winSum := femaleKeyWins[i]
			count := freeFemaleStateCount[i]
			avg := 0.0
			if count > 0 {
				avg = winSum / float64(count)
			}
			avgBet := avg / float64(_baseMultiplier)
			w("  key=%s | 总赢分=%.2f | 次数=%d | 平均倍数=%.4f\n",
				stateNames[i], winSum, count, avgBet)
		}
	}
	w("\n[免费模式中奖贡献分析]\n")
	printWinContribution(w, freeFemaleSymbolWin, freeFemaleWildWin, freeTotalWin)

	totalWin := baseTotalWin + freeTotalWin
	w("\n[免费触发效率]\n")
	w("  总免费游戏次数: %d (真实的游戏局数，包含中途增加的免费次数)\n", freeRounds)
	w("  总触发次数: %d (基础模式触发免费游戏的次数)\n", baseFreeTriggered)
	if baseFreeTriggered > 0 {
		w("  平均1次触发获得免费游戏: %.2f次 (总免费游戏次数 / 总触发次数)\n", float64(freeRounds)/float64(baseFreeTriggered))
	} else {
		w("  平均1次触发获得免费游戏: 0 (未触发)\n")
	}
	w("\n[总计]\n")
	w("  总投注(倍数): %.2f (仅基础模式投注，免费模式不投注)\n", totalBet)
	w("  总奖金: %.2f (基础模式奖金 + 免费模式奖金)\n", totalWin)
	if totalBet > 0 {
		w("  总回报率(RTP): %.2f%% (总奖金/总投注 = %.2f/%.2f)\n", totalWin*100/totalBet, totalWin, totalBet)
	}
	if totalWin > 0 {
		w("  基础贡献: %.2f%% | 免费贡献: %.2f%%\n", baseTotalWin*100/totalWin, freeTotalWin*100/totalWin)
	}
	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}
