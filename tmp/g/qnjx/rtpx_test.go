package qnjx

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	testRounds       = 1e2 //500000
	progressInterval = 1e7
	debugFileOpen    = 10
	freeModeLogOnly  = 0

	// debugTraceStderr：每步打一行到 stderr（无需 go test -v）；排查 Ways / 免费链时设为 1
	debugTraceStderr = 0
	// maxInnerCascadeSteps：同一「免费链」内连续连消步数上限（isRoundOver 且 FreeNum>0 会清零重计）
	maxInnerCascadeSteps = 2000
	// maxTotalSpins：整次 TestRtp2 的 baseSpin 调用上限。baseRounds 只在基础盘回合结束时 +1，
	// 若免费内反复加次导致 FreeNum 永不归零，外层 for baseRounds < testRounds 会永远进不去下一基础局，需此上限快速失败。
	maxTotalSpins = 50_000_000
)

func TestRtp2(t *testing.T) {
	// 基础模式统计
	var baseRounds, baseWinRounds, baseCascadeSteps, baseFreeTriggered int64
	var baseTotalWin float64
	var baseMaxCascadeSteps int

	// 免费模式统计
	var freeRounds, freeWinRounds, freeCascadeSteps int64
	var freeTreasureInFree, freeExtraFreeRounds, freeMaxFreeStreak int64
	var freeTotalWin float64
	var freeMaxCascadeSteps int

	totalBet, start := 0.0, time.Now()
	buf := &strings.Builder{}
	svc := newBerService()
	svc.initGameConfigs()
	baseGameCount, freeRoundIdx := 0, 0
	interval := int64(min(testRounds, progressInterval))
	var totalSpins int64

	var fileBuf *strings.Builder
	var reportBuf *strings.Builder
	if debugFileOpen > 0 {
		fileBuf = &strings.Builder{}
		reportBuf = &strings.Builder{}
	}

	for baseRounds < testRounds {
		var cascadeCount, gameNum int
		var roundWin, freeRoundWin float64
		var triggeringBaseRound int

		for {
			isFirst := svc.scene.Steps == 0
			wasFreeBeforeSpin := svc.isFreeRound

			if isFirst {
				roundWin = 0
				freeRoundWin = 0
			}

			_ = svc.baseSpin()
			totalSpins++
			if totalSpins > maxTotalSpins {
				t.Fatalf("TestRtp2: 全局 spin 超过 %d（疑似免费次数只加不减、无法结束）。baseRounds=%d freeRounds=%d FreeNum=%d isFreeRound=%v stage=%d",
					maxTotalSpins, baseRounds, freeRounds, svc.scene.FreeNum, svc.isFreeRound, svc.scene.Stage)
			}
			isFree := svc.isFreeRound

			// 从基础模式切换到免费模式时，重置 cascadeCount
			if isFirst && !wasFreeBeforeSpin && isFree {
				cascadeCount = 0
			}

			// 更新游戏计数
			if isFirst {
				if isFree {
					freeRoundIdx++
					gameNum = freeRoundIdx
					if triggeringBaseRound == 0 {
						triggeringBaseRound = baseGameCount
					}
				} else {
					baseGameCount++
					gameNum = baseGameCount
				}

				if reportBuf != nil {
					tr := 0
					if isFree {
						tr = triggeringBaseRound
						if tr == 0 {
							tr = baseGameCount
						}
					}
					writeReportRoundHeader(reportBuf, svc, gameNum, isFree, tr)
				}
			}

			cascadeCount++
			stepMul := svc.stepMultiplier
			stepWin := float64(stepMul) // svc.bonusAmount.Round(2).InexactFloat64()
			roundWin += stepWin

			if debugTraceStderr != 0 {
				fmt.Fprintf(os.Stderr,
					"[TestRtp2] baseRounds=%d baseGame=%d freeIdx=%d cascade=%d isFirst=%v wasFree=%v nowFree=%v isRoundOver=%v freeNum=%d stage=%d steps=%d nWin=%d winCells=%d stepMul=%d addFree=%d sym=%s nextPre=%s\n",
					baseRounds, baseGameCount, freeRoundIdx, cascadeCount, isFirst, wasFreeBeforeSpin, isFree,
					svc.isRoundOver, svc.scene.FreeNum, svc.scene.Stage, svc.scene.Steps, len(svc.winInfos),
					winGridNonZeroCount(&svc.winGrid), stepMul, svc.addFreeTime,
					gridFingerprint(&svc.symbolGrid), gridFingerprint(&svc.nextSymbolGrid))
			}

			if cascadeCount > maxInnerCascadeSteps {
				var dump strings.Builder
				writeSpinDetail(&dump, svc, gameNum, cascadeCount, isFree, triggeringBaseRound, stepWin, roundWin, isFirst)
				t.Fatalf("内层步数超过上限 %d（疑似死循环）。freeNum=%d stage=%d steps=%v isRoundOver=%v nWin=%d winCells=%d\n%s",
					maxInnerCascadeSteps, svc.scene.FreeNum, svc.scene.Stage, svc.scene.Steps, svc.isRoundOver,
					len(svc.winInfos), winGridNonZeroCount(&svc.winGrid), dump.String())
			}

			// 更新最大免费次数
			if isFree && svc.scene.FreeNum > freeMaxFreeStreak {
				freeMaxFreeStreak = svc.scene.FreeNum
			}

			// 调试日志
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

			// 统计奖金
			if isFree {
				freeTotalWin += stepWin
				freeRoundWin += stepWin
				if svc.addFreeTime > 0 {
					freeTreasureInFree++
					freeExtraFreeRounds += svc.addFreeTime
				}
			} else {
				baseTotalWin += stepWin
			}

			// Round 结束处理
			if svc.isRoundOver {
				// 与 hbtr2 一致：本步 stepWin 已累进 base/freeTotalWin 后再写 report
				if reportBuf != nil {
					totalWinAcc := baseTotalWin + freeTotalWin
					var curRound float64
					var freeMul int64
					if isFree {
						curRound = freeRoundWin
						//freeMul = int64(gameNum)
						freeMul = svc.mysMul

						if freeMul == 0 {
							freeMul = 1
						}
					} else {
						curRound = roundWin
						freeMul = 0
					}

					writeReportRoundSummary(reportBuf, totalWinAcc, freeMul, int64(curRound), isFree)
					//writeReportRoundSummary(reportBuf, totalWinAcc, freeMul, int64(curRound), isFree)
				}

				// 统计连消步数
				if isFree {
					freeCascadeSteps += int64(cascadeCount)
					if cascadeCount > freeMaxCascadeSteps {
						freeMaxCascadeSteps = cascadeCount
					}
					freeRounds++
					if freeRoundWin > 0 {
						freeWinRounds++
					}
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
					// 基础模式回合结束时，如果触发了免费游戏
					if svc.addFreeTime > 0 {
						baseFreeTriggered++
					}
					// 记录触发免费游戏的基础局数
					if svc.isFreeRound {
						triggeringBaseRound = baseGameCount
					}
				}
				roundWin = 0

				// 只有当免费游戏完全结束时才重置服务并退出内层循环
				if svc.scene.FreeNum <= 0 {
					resetBetServiceForNextRound(svc)
					freeRoundIdx = 0
					if baseRounds%interval == 0 {
						totalWin := baseTotalWin + freeTotalWin
						printBenchmarkProgress(buf, baseRounds, totalBet, baseTotalWin, freeTotalWin, totalWin, baseWinRounds, freeWinRounds, freeRounds, baseFreeTriggered, 0, start)
						fmt.Print(buf.String())
					}
					break
				}
				cascadeCount = 0
			}
		}
	}

	printFinalStats(buf, baseRounds, baseTotalWin, baseWinRounds, baseCascadeSteps, baseMaxCascadeSteps, baseFreeTriggered,
		freeRounds, freeTotalWin, freeWinRounds, freeCascadeSteps, freeMaxCascadeSteps, freeTreasureInFree, freeExtraFreeRounds, freeMaxFreeStreak, totalBet, start)
	result := buf.String()
	fmt.Print(result)
	if debugFileOpen > 0 {
		saveDebugFiles(result, fileBuf, reportBuf, start)
	}
}

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, gameNum, step int, isFree bool, triggeringBaseRound int, stepWin, roundWin float64, isFirstStep bool) {
	if step == 1 {
		writeRoundHeader(buf, svc, gameNum, isFree, triggeringBaseRound)
	} else {
		writeReelInfo(buf, svc)
	}
	fprintf(buf, "Step%d 初始盘面:\n", step)
	writeGridToBuilder(buf, &svc.symbolGrid, nil)

	if len(svc.winInfos) > 0 {
		fprintf(buf, "Step%d 中奖标记:\n", step)
		writeGridToBuilder(buf, &svc.symbolGrid, &svc.winGrid)
	}

	if !svc.isRoundOver {
		// nextSymbolGrid = moveSymbols 结果（已清中奖格并下落）；fallingWinSymbols 里 ring 补位写回的是 SymbolRoller，此处不含补位后符号
		fprintf(buf, "Step%d 消除并下落后（ring 从滚轴补位前）:\n", step)
		writeGridToBuilder(buf, &svc.nextSymbolGrid, nil)
	}
	writeStepSummary(buf, svc, step, isFree, stepWin, roundWin)
	fprintf(buf, "\n")
}

func writeReelInfo(buf *strings.Builder, svc *betOrderService) {
	if svc.scene == nil {
		fprintf(buf, "滚轴配置Index: 0\n转轮信息长度/起始：未初始化\n")
		return
	}
	fprintf(buf, "滚轴配置Index: %d\n转轮信息长度/起始：", svc.scene.SymbolRoller[0].Real)
	for c := 0; c < len(svc.scene.SymbolRoller); c++ {
		rc := svc.scene.SymbolRoller[c]
		fprintf(buf, "%d[%d～%d]  ", rc.Len, rc.OriStart, rc.Fall)
	}
	fprintf(buf, "\n")
	fprintf(buf, "curr=%d mul=%v count=%v add=%v\n ", svc.mysMul, svc.scene.ColorMul, svc.scene.ColorCount, svc.debug.added)
}

func writeRoundHeader(buf *strings.Builder, svc *betOrderService, gameNum int, isFree bool, triggeringBaseRound int) {
	if isFree {
		trigger := "?"
		if triggeringBaseRound > 0 {
			trigger = fmt.Sprintf("%d", triggeringBaseRound)
		}
		fprintf(buf, "\n=============[基础模式] 第%s局 - 免费第%d局 =============\n", trigger, gameNum)
	} else {
		fprintf(buf, "\n=============[基础模式] 第%d局 =============\n", gameNum)
	}
	writeReelInfo(buf, svc)
}

func writeStepSummary(buf *strings.Builder, svc *betOrderService, step int, isFree bool, stepWin, roundWin float64) {
	fprintf(buf, "Step%d 中奖详情:\n", step)
	treasureCount := svc.getScatterCount()

	if len(svc.winInfos) == 0 {
		fprintf(buf, "\t未中奖\n")
		if svc.isRoundOver {
			if isFree && treasureCount > 0 {
				fprintf(buf, "\t夺宝数量(当前盘面): %d\n", treasureCount)
			} else if !isFree && svc.addFreeTime > 0 {
				fprintf(buf, "\t本手触发免费: 夺宝=%d, +免费次数=%d, 剩余免费=%d, NextStage=%d，stepmul=%d, enterfree 💎💎💎\n",
					treasureCount, svc.addFreeTime, svc.scene.FreeNum, svc.scene.NextStage, svc.stepMultiplier)
			}
		}
		return
	}

	// 按真实结算倍数分摊每条中奖信息。
	totalMultiplier := int64(0)
	for _, elem := range svc.winInfos {
		totalMultiplier += elem.Multiplier
	}

	for _, elem := range svc.winInfos {
		//lineWin := 0.0
		//if totalMultiplier > 0 {
		//	lineWin = stepWin * float64(elem.Multiplier) / float64(totalMultiplier)
		//}
		lineWin := elem.LineCount * elem.Odds * svc.mysMul
		fprintf(buf, "\t符号: %2d, 连线: %d, 路数：%d, 赔率: %d, Num：%d, mysMul: %d, 奖金:%2d\n",
			elem.Symbol, elem.SymbolCount, elem.LineCount, elem.Odds, elem.Num, svc.mysMul, lineWin)
	}

	isFreeMode := 0
	if svc.isFreeRound {
		isFreeMode = 1
	}
	fprintf(buf, "\tMode=%d, Stage=%d, Steps=%d, stepMul=%d, lineMul=%d, mysMul=%d, limit=%v, 本回合累计 step 倍数: %.2f\n",
		isFreeMode, svc.scene.Stage, svc.scene.Steps, svc.stepMultiplier, svc.lineMultiplier, svc.mysMul, btoi(svc.limit), roundWin)
	if !svc.isRoundOver {
		fprintf(buf, "\t连消继续 → 下一请求 Step%d（Stage 将为 Eli）\n", step+1)
		return
	}

	fprintf(buf, "\t连消结束（本回合无更多可消除）\n\n")
	if isFree {
		if treasureCount > 0 && svc.addFreeTime > 0 {
			fprintf(buf, "\t免费内再触发: 夺宝=%d, +次数=%d, 剩余免费=%d\n", treasureCount, svc.addFreeTime, svc.scene.FreeNum)
		}
		if svc.scene.FreeNum == 0 {
			fprintf(buf, "\t免费模式结束 — 本回合累计 step 倍数=%.2f\n", roundWin)
		} else {
			fprintf(buf, "\t免费模式继续 — 剩余次数=%d\n", svc.scene.FreeNum)
		}
	} else if svc.addFreeTime > 0 {
		fprintf(buf, "\t基础盘触发免费 — 夺宝=%d, 剩余免费=%d, NextStage=%d\n", treasureCount, svc.scene.FreeNum, svc.scene.NextStage)
	}
}

// saveDebugFiles 参考 game/hbtr2/rtpx_test.go：详情日志 + 报告行
func saveDebugFiles(statsResult string, fileBuf, reportBuf *strings.Builder, start time.Time) {
	_ = start
	ts := time.Now().Format("20060102_150405")
	_ = os.MkdirAll("logs", 0755)
	detail := ""
	if fileBuf != nil {
		detail = fileBuf.String()
	}
	debugPath := fmt.Sprintf("logs/%s.txt", ts)
	_ = os.WriteFile(debugPath, []byte(statsResult+detail), 0644)
	fmt.Printf("\n调试详情已保存: %s\n", debugPath)

	if reportBuf != nil && reportBuf.Len() > 0 {
		reportPath := fmt.Sprintf("logs/%s_report.txt", ts)
		_ = os.WriteFile(reportPath, []byte(reportBuf.String()), 0644)
		fmt.Printf("调试报告已保存: %s\n", reportPath)
	}
}

func writeReportRoundHeader(buf *strings.Builder, svc *betOrderService, gameNum int, isFree bool, triggerRound int) {
	if isFree {
		fprintf(buf, "基础模式第 %d 局-免费模式第 %d 局\n", triggerRound, gameNum)
		fprintf(buf, "reelSetId-%d\n", svc.scene.SymbolRoller[0].Real)
	} else {
		fprintf(buf, "基础模式第 %d 局\n", gameNum)
		fprintf(buf, "reelSetId-%d\n", svc.scene.SymbolRoller[0].Real)
	}

	fprintf(buf, "初始索引-")
	for c := 0; c < _colCount; c++ {
		if c > 0 {
			fprintf(buf, ",")
		}
		if svc.scene != nil && c < len(svc.scene.SymbolRoller) {
			fprintf(buf, "%d", svc.scene.SymbolRoller[c].OriStart)
			//fprintf(buf, "%d", svc.scene.SymbolRoller[c].Start)
		} else {
			fprintf(buf, "0")
		}
	}
	fprintf(buf, "\n")
}

func writeReportRoundSummary(buf *strings.Builder, totalWin float64, freeMultiple int64, stepMultiplier int64, isFree bool) {
	_ = isFree
	fprintf(buf, "totalWin-%d\n", int64(totalWin))
	fprintf(buf, "freeMultiple-%d\n", freeMultiple)
	fprintf(buf, "stepMultiplier-%d\n", stepMultiplier)
}

func gridFingerprint(g *int64Grid) string {
	var b strings.Builder
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if b.Len() > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "%d", (*g)[r][c])
		}
	}
	return b.String()
}

func winGridNonZeroCount(w *int64Grid) int {
	n := 0
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if (*w)[r][c] != 0 {
				n++
			}
		}
	}
	return n
}

func printFinalStats(buf *strings.Builder, baseRounds int64, baseTotalWin float64, baseWinRounds int64,
	baseCascadeSteps int64, baseMaxCascadeSteps int, baseFreeTriggered int64, freeRounds int64, freeTotalWin float64,
	freeWinRounds int64, freeCascadeSteps int64, freeMaxCascadeSteps int, freeTreasureInFree int64,
	freeExtraFreeRounds int64, freeMaxFreeStreak int64, totalBet float64, start time.Time) {
	w := func(format string, args ...interface{}) { fprintf(buf, format, args...) }
	elapsed := time.Since(start)
	speed := safeDiv(baseRounds, int64(elapsed.Seconds()))
	w("\n运行局数: %d，用时: %v，速度: %.0f 局/秒\n", baseRounds, elapsed.Round(time.Second), speed)

	w("\n===== 详细统计汇总 =====\n")
	w("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	w("\n[基础模式统计]\n")
	w("基础模式总游戏局数: %d\n", baseRounds)
	w("基础模式总投注(倍数): %.2f\n", totalBet)
	w("基础模式总奖金: %.2f\n", baseTotalWin)
	w("基础模式RTP: %.2f%% (基础模式奖金/基础模式投注)\n", safeDivFloat(baseTotalWin*100, totalBet))
	w("基础模式免费局触发次数: %d\n", baseFreeTriggered)
	w("基础模式触发免费局比例: %.2f%%\n", safeDiv(baseFreeTriggered*100, baseRounds))
	w("基础模式平均每局免费次数: %.2f\n", safeDiv(freeRounds, baseRounds))
	w("基础模式中奖率: %.2f%%\n", safeDiv(baseWinRounds*100, baseRounds))
	w("基础模式平均连消步数: %.2f\n", safeDiv(baseCascadeSteps, baseRounds))
	w("基础模式最大连消步数: %d\n", baseMaxCascadeSteps)
	w("基础模式中奖局数: %d\n", baseWinRounds)

	w("\n[免费模式统计]\n")
	w("免费模式总游戏局数: %d\n", freeRounds)
	w("免费模式总奖金: %.2f\n", freeTotalWin)
	w("免费模式RTP: %.2f%% (免费模式奖金/基础模式投注，因为免费模式不投注)\n", safeDivFloat(freeTotalWin*100, totalBet))

	w("免费模式额外增加局数: %d\n", freeExtraFreeRounds)
	w("免费模式最大连续局数: %d\n", freeMaxFreeStreak)
	w("免费模式中奖局数: %d\n", freeWinRounds)
	w("免费模式中奖率: %.2f%%\n", safeDiv(freeWinRounds*100, freeRounds))
	w("免费模式出现夺宝的次数: %d (%.2f%%)\n", freeTreasureInFree, safeDiv(freeTreasureInFree*100, freeRounds))
	w("免费模式平均连消步数: %.2f\n", safeDiv(freeCascadeSteps, freeRounds))
	w("免费模式最大连消步数: %d\n", freeMaxCascadeSteps)

	totalWin := baseTotalWin + freeTotalWin
	w("\n[免费触发效率]\n")
	w("  总免费游戏次数: %d (真实的游戏局数，包含中途增加的免费次数)\n", freeRounds)
	w("  总触发次数: %d (基础模式触发免费游戏的次数)\n", baseFreeTriggered)
	w("  平均1次触发获得免费游戏: %.2f次 (总免费游戏次数 / 总触发次数)\n", safeDiv(freeRounds, baseFreeTriggered))

	w("\n[总计]\n")
	w("  总投注(倍数): %.2f (仅基础模式投注，免费模式不投注)\n", totalBet)
	w("  总奖金: %.2f (基础模式奖金 + 免费模式奖金)\n", totalWin)
	totalRTP := safeDivFloat(totalWin*100, totalBet)
	w("  总回报率(RTP): %.2f%% (总奖金/总投注 = %.2f/%.2f)\n", totalRTP, totalWin, totalBet)
	w("  基础贡献: %.2f%% | 免费贡献: %.2f%%\n", safeDivFloat(baseTotalWin*100, totalWin), safeDivFloat(freeTotalWin*100, totalWin))
	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}

// writeGridToBuilder 打印 3×3 盘面；winGrid 非 nil 时在命中格后加 *（rtp/rtpx 调试共用）
func writeGridToBuilder(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
	if grid == nil {
		buf.WriteString("(空)\n")
		return
	}
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			symbol := (*grid)[r][c]
			isWin := winGrid != nil && (*winGrid)[r][c] != 0
			if symbol == 0 {
				// 空位不打印 0，与「%2d |」列宽大致对齐
				if isWin {
					fprintf(buf, "     *|")
				} else {
					fprintf(buf, "      |")
				}
				continue
			}
			if isWin {
				fprintf(buf, " %4d*|", symbol)
			} else {
				fprintf(buf, " %4d |", symbol)
			}
		}
		buf.WriteByte('\n')
	}
}

/*
func TestCollectWinningSymbolsUpdatesColorMultipliers(t *testing.T) {
	svc := &betOrderService{
		scene: &SpinSceneData{
			ColorMul:   [3]int64{1, 1, 1},
			ColorCount: [3]int64{4, 0, 0},
		},
		gameConfig: &gameConfigJson{
			ColorMultiplierAdd: []int64{1, 2, 3},
		},
	}

	svc.symbolGrid[0][0] = 1 // green
	svc.symbolGrid[0][1] = 2 // blue
	svc.symbolGrid[0][2] = 3 // yellow
	svc.symbolGrid[0][3] = _wild
	svc.symbolGrid[1][0] = 4 // long green head
	svc.symbolGrid[2][0] = _longSymbol + 4

	svc.winGrid[0][0] = 1
	svc.winGrid[0][1] = 2
	svc.winGrid[0][2] = 3
	svc.winGrid[0][3] = _wild
	svc.winGrid[1][0] = 4
	svc.winGrid[2][0] = _longSymbol + 4

	svc.collectWinningSymbols()

	if svc.scene.ColorMul != [3]int64{2, 1, 1} {
		t.Fatalf("ColorMul = %v, want [2 1 1]", svc.scene.ColorMul)
	}
	if svc.scene.ColorCount != [3]int64{2, 2, 2} {
		t.Fatalf("ColorCount = %v, want [2 2 2]", svc.scene.ColorCount)
	}
}

func TestCalcNewFreeGameNumUsesScatterConfig(t *testing.T) {
	svc := &betOrderService{
		gameConfig: &gameConfigJson{
			FreeGameScatter: 4,
			FreeGameTimes:   12,
			AddFreeTimes:    2,
		},
	}

	tests := []struct {
		scatter int64
		want    int64
	}{
		{scatter: 3, want: 0},
		{scatter: 4, want: 12},
		{scatter: 5, want: 14},
		{scatter: 6, want: 16},
	}

	for _, tt := range tests {
		if got := svc.calcNewFreeGameNum(tt.scatter); got != tt.want {
			t.Fatalf("calcNewFreeGameNum(%d) = %d, want %d", tt.scatter, got, tt.want)
		}
	}

	svc.isFreeRound = true
	if got := svc.calcNewFreeGameNum(5); got != 14 {
		t.Fatalf("free calcNewFreeGameNum(5) = %d, want 14", got)
	}
}
*/
/*
_treasure int64 = 11 // 夺宝符号

board     [2,1,9,4,6]
pattern   [1,1,2,1],
newBoard  [2,1,9,1009,4]

board     [2,1,11,4,6]
pattern   [2,1,2],
newBoard  [2,1002,1, 11, 1011]
*/

//func TestWriteLongToBoard(t *testing.T) {
//	pattern := []int64{2, 1, 2}
//	board := [_rowCount]int64{2, 1, 9, 4, 6}
//	val, newBoard, write := writeLongToBoard(pattern, board)
//	t.Logf("val=%v, newBoard=%v, write=%v", val, newBoard, write)
//}

func TestWinSymbolsWild(t *testing.T) {
	t.Log("开始测试")
	buf := &strings.Builder{}
	svc := &betOrderService{}
	svc.debug.open = true
	svc.initGameConfigs()
	grid1 := int64Grid{
		{4, 9, 1, 7, 2, 1},
		{1004, 7, 5, 1007, 1002, 1001},
		{4, 5, 6, 1007, 7, 8},
		{1004, 7, 1006, 1007, 1007, 1008},
		{7, 1007, 7, 8, 4, 1008},
	}

	svc.symbolGrid = grid1 // 必须：判奖读的是 svc.symbolGrid，未赋值前为零盘面，最下行再对也不会进中奖
	svc.findWinInfos()
	writeGridToBuilder(buf, &svc.symbolGrid, &svc.winGrid)
	t.Logf("中奖信息:\n%s\n", buf.String())
	for _, elem := range svc.winInfos {
		t.Logf("\t符号:%2d, 支付线:%2d, 连线: %d, 赔率: %2d, \n", elem.Symbol, elem.LineCount+1, elem.SymbolCount, elem.Odds)
	}
}

func TestMatchPattern(t *testing.T) {
	//pattern := []int64{2, 3} //ok=true, write=2 newTailIndex=12, board=[6 1006 4 1004 1004], reelLen=15
	pattern := []int64{2, 1, 2} // ok=true, write=3 newTailIndex=11, board=[8 1008 6 4 1004], reelLen=15
	reel := []int64{2, 6, 2, 3, 7, 2, 6, 8, 6, 11, 3, 8, 6, 4, 3}
	empty := 5
	reelLen := len(reel)
	tailIndex := reelLen - 1

	ok, write, newTailIndex, board := matchPattern(pattern, empty, tailIndex, len(reel), reel)
	t.Logf("ok=%v, write=%d newTailIndex=%v, board=%v, reelLen=%d", ok, write, newTailIndex, board, reelLen)
}
