package ajtm

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	testRounds       = 10000
	progressInterval = 1e7
	debugFileOpen    = 10
	freeModeLogOnly  = 0

	// debugTraceStderr：每步打一行到 stderr（无需 go test -v）；排查 Ways / 免费链时设为 1
	debugTraceStderr = 0
	// maxInnerCascadeSteps：同一「免费链」内连续连消步数上限（isRoundOver 且 FreeNum>0 会清零重计）
	maxInnerCascadeSteps = 2000
	// maxTotalSpins：整次 TestRtp2 的 baseSpin 调用上限。baseRounds 只在基础盘回合结束时 +1，
	// 若免费内反复加次导致 FreeNum 永不归零，外层 for baseRounds < testRounds 会永远进不去下一基础局，需此上限快速失败。
	maxTotalSpins = 5_000_000
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
					"[hcsqy2 TestRtp2] baseRounds=%d baseGame=%d freeIdx=%d cascade=%d isFirst=%v wasFree=%v nowFree=%v isRoundOver=%v freeNum=%d stage=%d steps=%d nWin=%d winCells=%d stepMul=%d addFree=%d sym=%s nextPre=%s\n",
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
						freeMul = int64(gameNum)
					} else {
						curRound = roundWin
						freeMul = 0
					}
					writeReportRoundSummary(reportBuf, totalWinAcc, freeMul, int64(curRound), isFree)
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
	writeGridToBuilder(buf, &svc.debug.originSymbolGrid, nil)
	//	writeGridToBuilder(buf, &svc.symbolGrid, nil)

	if len(svc.winInfos) > 0 {
		fprintf(buf, "Step%d 中奖标记:\n", step)
		// 中奖标记应基于「原始盘面」展示，避免与长符号转变后的 symbolGrid 混淆。
		writeGridToBuilder(buf, &svc.debug.originSymbolGrid, &svc.winGrid)
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
		//visibleCount := _rowCount
		//if c == 0 || c == _colCount-1 {
		//	visibleCount = _rowCount - 2
		//}
		//winStart := rc.OriStart
		//winEnd := (rc.OriStart + visibleCount - 1) % rc.Len
		//nextFill := rc.Start - 1
		//if nextFill < 0 {
		//	nextFill += rc.Len
		//}
		//// 注意：Fall 会被长符号写入过程调整，不等于原始可见窗口结束索引。
		//fprintf(buf, "%d[%d～%d|fill=%d|fall=%d]  ", rc.Len, winStart, winEnd, nextFill, rc.Fall)
	}
	fprintf(buf, "\n")
	fprintf(buf, "MysCount：%v \n ", svc.scene.DownCount)
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
				fprintf(buf, "\t本手触发免费: 夺宝=%d, +免费次数=%d, 剩余免费=%d, NextStage=%d, extMul=%d，stepmul=%d, enterfree 💎💎💎\n",
					treasureCount, svc.addFreeTime, svc.scene.FreeNum, svc.scene.NextStage, svc.extMul, svc.stepMultiplier)
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
		lineWin := 0.0
		if totalMultiplier > 0 {
			lineWin = stepWin * float64(elem.Multiplier) / float64(totalMultiplier)
		}
		fprintf(buf, "\t符号: %2d, 连线: %d, 路数：%d, 赔率: %d, sysMul: %d, 奖金: %4.2f\n",
			elem.Symbol, elem.SymbolCount, elem.LineCount, elem.Odds, svc.scene.MysMulTotal, lineWin)
	}

	isFreeMode := 0
	if svc.isFreeRound {
		isFreeMode = 1
	}
	fprintf(buf, "\tMode=%d, Stage=%d, Steps=%d, RoundMul=%d, stepMul=%d, lineMul=%d, sysMul=%d, 本回合累计 step 倍数: %.2f\n",
		isFreeMode, svc.scene.Stage, svc.scene.Steps, svc.scene.RoundMultiplier, svc.stepMultiplier, svc.lineMultiplier, svc.scene.MysMulTotal, roundWin)
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
			fprintf(buf, "\t免费模式结束 — RoundMul=%d, 本回合累计 step 倍数=%.2f\n", svc.scene.RoundMultiplier, roundWin)
		} else {
			fprintf(buf, "\t免费模式继续 — 剩余次数=%d, RoundMul=%d\n", svc.scene.FreeNum, svc.scene.RoundMultiplier)
		}
	} else if svc.addFreeTime > 0 {
		fprintf(buf, "\t基础盘触发免费 — 夺宝=%d, 剩余免费=%d, NextStage=%d, extMul=%d\n", treasureCount, svc.scene.FreeNum, svc.scene.NextStage, svc.extMul)
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

/*
基础模式第 1 局
realIndex-1,1,1
randomIndex-5,5,3
初始索引-2,1,119,36,12
totalWin-309
freeMultiple-0
stepMultiplier-309

基础模式第 1 局-免费模式第 1 局
realIndex2-1,1,1,1,1,1
realIndex3-1,1,1,1,2
realIndex4-1,1,1,1,1,1
freeAddMystery-2,4
初始索引-79,7,61,14,103
totalWin-309
freeMultiple-1
stepMultiplier-0
*/

func writeReportRoundHeader(buf *strings.Builder, svc *betOrderService, gameNum int, isFree bool, triggerRound int) {
	if isFree {
		fprintf(buf, "基础模式第 %d 局-免费模式第 %d 局\n", triggerRound, gameNum)
		for i := 0; i < 3; i++ {
			//s.gameConfig.BigSyMultiples[0]
			arr := []int64{1, 1, 1, 1, 1, 1}
			idx := svc.debug.freeRandomIndex[i]
			if idx > 0 {
				arr = svc.gameConfig.BigSyMultiples[0][idx-1]
			}
			fprintf(buf, "realIndex%d-%v\n", i+2, arr)
		}
		if svc.debug.freeAddMystery[0] != 0 { // [col,row]
			fprintf(buf, "freeAddMystery-%d,%d\n", svc.debug.freeAddMystery[0], svc.debug.freeAddMystery[1])
		}
	} else {
		fprintf(buf, "基础模式第 %d 局\n", gameNum)
		fprintf(buf, "realIndex-%d,%d,%d\n", svc.debug.realIndex[0], svc.debug.realIndex[1], svc.debug.realIndex[2])
		fprintf(buf, "randomIndex-%d,%d,%d\n", svc.debug.randomIndex[0], svc.debug.randomIndex[1], svc.debug.randomIndex[2])
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
