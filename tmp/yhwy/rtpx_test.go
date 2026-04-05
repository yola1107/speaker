package yhwy

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	testRounds       = 1e4
	progressInterval = 1e7
	debugFileOpen    = 10
	freeModeLogOnly  = 0
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

	var fileBuf *strings.Builder
	if debugFileOpen > 0 {
		fileBuf = &strings.Builder{}
	}

	for baseRounds < testRounds {
		var cascadeCount, gameNum int
		var roundWin, freeRoundWin float64
		var triggeringBaseRound int

		for {
			isFirst := true // svc.scene.Steps == 0
			wasFreeBeforeSpin := svc.isFreeRound

			if isFirst {
				roundWin = 0
				freeRoundWin = 0
			}

			_ = svc.baseSpin()
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
			}

			cascadeCount++
			stepWin := float64(svc.stepMultiplier) // svc.bonusAmount.Round(2).InexactFloat64()
			roundWin += stepWin

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
	if debugFileOpen > 0 && fileBuf != nil {
		saveDebugFile(result, fileBuf.String(), start)
	}
}

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, gameNum, step int, isFree bool, triggeringBaseRound int, stepWin, roundWin float64, isFirstStep bool) {
	if step == 1 {
		writeRoundHeader(buf, svc, gameNum, isFree, triggeringBaseRound)
	} else {
		writeReelInfo(buf, svc)
	}
	fprintf(buf, "Step%d 初始盘面:\n", step)
	//writeGridToBuilder(buf, &svc.symbolGrid, nil)
	writeGridToBuilder(buf, &svc.debug.origin, &svc.mysteryGrid)

	if len(svc.winInfos) > 0 {
		fprintf(buf, "Step%d 中奖标记:\n", step)
		writeGridToBuilder(buf, &svc.symbolGrid, &svc.winGrid)
	}

	//if !svc.isRoundOver {
	//	fprintf(buf, "Step%d 下一盘面预览（实际消除+下落+填充结果）:\n", step)
	//	writeGridToBuilder(buf, nil, nil)
	//	//writeGridToBuilder(buf, &svc.nextSymbolGrid, nil)
	//}
	writeStepSummary(buf, svc, step, isFree, stepWin, roundWin)
	fprintf(buf, "\n")
}

func writeGridToBuilder(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
	if grid == nil {
		buf.WriteString("(空)\n")
		return
	}

	//// 默认按基础模式分割：上4行 / 下4行
	//// 免费模式按当前已解锁行动态分割
	//splitAfterRow := _rowCountReward - 1
	//if svc != nil && svc.isFreeRound {
	//	if lockedRows := _rowCount - svc.scene.UnlockedRows; lockedRows > 0 && lockedRows < _rowCount {
	//		splitAfterRow = lockedRows - 1
	//	} else {
	//		splitAfterRow = -1
	//	}
	//}

	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			symbol := (*grid)[r][c]
			isWin := winGrid != nil && (*winGrid)[r][c] != 0
			switch {
			case symbol == 0 && isWin:
				buf.WriteString("   *|")
			case symbol == 0:
				buf.WriteString("    |")
			case isWin:
				_, _ = fmt.Fprintf(buf, " %2d*|", symbol)
			default:
				_, _ = fmt.Fprintf(buf, " %2d |", symbol)
			}
			if c < _colCount-1 {
				buf.WriteString(" ")
			}
		}
		buf.WriteString("\n")
		//if r == splitAfterRow {
		//	buf.WriteString("--------------------------------\n")
		//}
	}
}

func writeReelInfo(buf *strings.Builder, svc *betOrderService) {
	if svc.scene == nil {
		fprintf(buf, "滚轴配置Index: 0\n转轮信息长度/起始：未初始化\n")
		return
	}
	fprintf(buf, "滚轴配置Index: %d\n转轮信息长度/起始：", svc.scene.SymbolRoller[0].Real)
	for c := 0; c < len(svc.scene.SymbolRoller); c++ {
		rc := svc.scene.SymbolRoller[c]
		fprintf(buf, "%d[%d～%d]  ", rc.Len, rc.Start, rc.Fall)
	}
	fprintf(buf, "\n")
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
				fprintf(buf, "\t💎 当前盘面夺宝数量: %d\n", treasureCount)
			} else if !isFree && svc.scene.NextStage == _spinTypeFree {
				fprintf(buf, "\t💎💎💎 基础模式。 夺宝=%d 触发免费游戏=%d\n", treasureCount, svc.scene.FreeNum)
			}
		}
		return
	}

	gameMultiple := int64(1) // svc.gameMultiple
	totalMultiplier := int64(0)
	for _, elem := range svc.winInfos {
		totalMultiplier += elem.Odds * gameMultiple
	}

	for _, elem := range svc.winInfos {
		lineWin := 0.0
		if totalMultiplier > 0 {
			lineWin = stepWin * float64(elem.Odds*gameMultiple) / float64(totalMultiplier)
		}
		fprintf(buf, "\t符号:%2d, 支付线:%2d, 乘积: %d, 赔率: %2d, 奖金: %4.2f\n",
			elem.Symbol, elem.LineCount+1, elem.SymbolCount, elem.Odds, lineWin)
	}

	isFreeMode := 0
	if svc.isFreeRound {
		isFreeMode = 1
	}
	//	sakuraCol int   // 樱吹雪替换到的最远列（3/4/5） 默认-1
	//	mysSymbol int64 // 百变樱花本次统一揭示出的目标符号
	fprintf(buf, "\tMode=%d,  stepMul: %d, lineMul: %d, sakuraCol: %d, mysSymbol: %d, roundWin: %.2f\n",
		isFreeMode, svc.stepMultiplier, svc.lineMultiplier, svc.debug.sakuraCol, svc.debug.mysSymbol, roundWin)
	//if svc.isFreeRound {
	//	fprintf(buf, "\tHeroID=%d, MulList:%v, ContinueNum: %d, gameMul: %d, CNum=%d\n",
	//		svc.scene.FreeHeroID, svc.gameConfig.FreeMultipleMap[svc.scene.FreeHeroID], svc.scene.ContinueNum, svc.gameMultiple, svc.scene.CityValue)
	//}
	if !svc.isRoundOver {
		fprintf(buf, "\t🔁 连消继续 → Step%d\n", step+1)
		return
	}

	//fprintf(buf, "\t🛑 连消结束（无后续可消除）\n\n")
	if isFree {
		if treasureCount > 0 {
			fprintf(buf, "\t💎 当前盘面夺宝数量: %d, 增加免费次数: %d\n", treasureCount, svc.addFreeTime)
		}
		if svc.scene.FreeNum == 0 {
			fprintf(buf, "\t🎉 免费模式结束 - RoundMultiplier: %d, 总奖金: %.2f\n", svc.stepMultiplier, roundWin)
		} else {
			fprintf(buf, "\t➡️ 免费模式继续 - 剩余次数: %d, RoundMultiplier: %d\n", svc.scene.FreeNum, svc.stepMultiplier)
		}
	} else if svc.isFreeRound {
		fprintf(buf, "\t💎💎💎 基础模式。 夺宝=%d 触发免费游戏=%d\n", treasureCount, svc.scene.FreeNum)
	}
}

func saveDebugFile(statsResult, detailResult string, start time.Time) {
	_ = os.MkdirAll("logs", 0755)
	filename := fmt.Sprintf("logs/%s.txt", time.Now().Format("20060102_150405"))
	_ = os.WriteFile(filename, []byte(statsResult+detailResult), 0644)
	fmt.Printf("\n📄 调试信息已保存到: %s\n", filename)
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
	w("基础模式RTP: %.2f%% (基础模式奖金/基础模式投注)\n", safeDiv(int64(baseTotalWin)*100, int64(totalBet)))
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
	w("免费模式RTP: %.2f%% (免费模式奖金/基础模式投注，因为免费模式不投注)\n", safeDiv(int64(freeTotalWin)*100, int64(totalBet)))

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
	totalRTP := safeDiv(int64(totalWin)*100, int64(totalBet))
	w("  总回报率(RTP): %.2f%% (总奖金/总投注 = %.2f/%.2f)\n", totalRTP, totalWin, totalBet)
	w("  基础贡献: %.2f%% | 免费贡献: %.2f%%\n", safeDiv(int64(baseTotalWin)*100, int64(totalWin)), safeDiv(int64(freeTotalWin)*100, int64(totalWin)))
	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}

func TestWinSymbolsWild(t *testing.T) {
	t.Log("开始测试")
	svc := &betOrderService{}
	svc.debug.open = true
	svc.initGameConfigs()
	grid1 := int64Grid{
		{0, 0, 0, 0, 0},                 // row0 墙格
		{0, 1, 0, 0, 0},                 // row1
		{0, 2, 0, 0, 0},                 // row2
		{_wild, _wild, _wild, 3, _wild}, // row3：支付线4 [15..19] 为 11+11+11+3+11 → 符号3五连 赔率10
	}
	t.Logf("初始状态:\n%s\n", GridToString(&grid1, nil))
	svc.symbolGrid = grid1 // 必须：判奖读的是 svc.symbolGrid，未赋值前为零盘面，最下行再对也不会进中奖
	svc.checkSymbolGridWin()
	t.Logf("中奖信息:\n%s\n", GridToString(&grid1, &svc.winGrid))
	for _, elem := range svc.winInfos {
		fmt.Printf("\t符号:%2d, 支付线:%2d, 连线: %d, 赔率: %2d, \n", elem.Symbol, elem.LineCount+1, elem.SymbolCount, elem.Odds)
	}
}

func GridToString(grid *int64Grid, winGrid *int64Grid) string {
	if grid == nil {
		return "(空)\n"
	}
	var buf strings.Builder
	buf.Grow(512)
	writeGridToBuilder(&buf, grid, winGrid)
	return buf.String()
}
