package sjxj

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	testRounds       = 1e2
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
			wasFreeBeforeSpin := svc.isFreeRound
			roundWin = 0
			freeRoundWin = 0

			_ = svc.baseSpin()
			isFree := svc.isFreeRound

			// 从基础模式切换到免费模式时，重置 cascadeCount
			if !wasFreeBeforeSpin && isFree {
				cascadeCount = 0
			}

			// 更新游戏计数
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

			cascadeCount++
			stepWin := float64(svc.stepMultiplier)
			roundWin += stepWin

			// 更新最大免费次数
			remFree := int64(svc.scene.FreeNum)
			if isFree && remFree > freeMaxFreeStreak {
				freeMaxFreeStreak = remFree
			}

			// 调试日志
			if debugFileOpen > 0 && fileBuf != nil && (freeModeLogOnly == 0 || isFree) {
				triggerRound := 0
				if isFree {
					triggerRound = triggeringBaseRound
					if triggerRound == 0 {
						triggerRound = baseGameCount
					}
				}
				writeSpinDetail(fileBuf, svc, gameNum, cascadeCount, isFree, triggerRound, stepWin, roundWin)
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
					if svc.addFreeTime > 0 {
						baseFreeTriggered++
					}
					if svc.isFreeRound {
						triggeringBaseRound = baseGameCount
					}
				}
				roundWin = 0

				// 只有当免费游戏完全结束时才重置服务并退出内层循环
				if svc.scene.FreeNum == 0 {
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
		saveDebugFile(result, fileBuf.String())
	}
}

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, gameNum, step int, isFree bool, triggeringBaseRound int, stepWin, roundWin float64) {
	if step == 1 {
		writeRoundHeader(buf, svc, gameNum, isFree, triggeringBaseRound)
	} else {
		writeReelInfo(buf, svc)
	}
	fprintf(buf, "Step%d 初始盘面:\n", step)
	writeGridToBuilderWithDynamicSplit(buf, svc, &svc.symbolGrid, &svc.winGrid)
	writeStepSummary(buf, svc, step, isFree, stepWin, roundWin)
	fprintf(buf, "\n")
}

func writeGridToBuilderWithDynamicSplit(buf *strings.Builder, svc *betOrderService, grid *int64Grid, winGrid *int64Grid) {
	if grid == nil {
		buf.WriteString("(空)\n")
		return
	}

	// 默认按基础模式分割：上4行 / 下4行
	// 免费模式按当前已解锁行动态分割
	splitAfterRow := _rowCountReward - 1
	if svc != nil && svc.isFreeRound {
		if lockedRows := _rowCount - svc.scene.UnlockedRows; lockedRows > 0 && lockedRows < _rowCount {
			splitAfterRow = lockedRows - 1
		} else {
			splitAfterRow = -1
		}
	}

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
		if r == splitAfterRow {
			buf.WriteString("--------------------------------\n")
		}
	}
}

func dumpFreeUnlockedScatterMuls(buf *strings.Builder, svc *betOrderService) (sum int64) {
	if svc == nil || svc.scene == nil {
		return 0
	}
	unlockedStartRow := max(0, _rowCount-svc.scene.UnlockedRows)

	var parts []string
	for r := unlockedStartRow; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if svc.symbolGrid[r][c] == _treasure {
				mul := svc.scene.ScatterLock[r][c]
				sum += mul
				parts = append(parts, fmt.Sprintf("(%d,%d)=%d", r, c, mul))
			}
		}
	}

	if len(parts) == 0 {
		fprintf(buf, "\t       [Free-核对] 已解锁区内无夺宝(Scatter)格子，sumMul=0\n")
		return sum
	}
	fprintf(buf, "\t       [Free-核对] 已解锁区 scatter倍数明细=%v, sumMul=%d, stepMultiplier=%d\n", parts, sum, svc.stepMultiplier)
	return sum
}

func writeReelInfo(buf *strings.Builder, svc *betOrderService) {
	if svc.scene == nil {
		fprintf(buf, "滚轴配置Index: 0\n转轮信息长度/起始：未初始化\n")
		return
	}
	fprintf(buf, "滚轴配置Index: %d\n转轮信息长度/起始：", svc.scene.SymbolRoller[0].Real)
	for c := 0; c < len(svc.scene.SymbolRoller); c++ {
		rc := svc.scene.SymbolRoller[c]
		fprintf(buf, "%d[%d-%d]  ", rc.Len, rc.Start, rc.Fall)
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
	totalTreasureCount := countTreasuresInGrid(&svc.symbolGrid)
	unlockedStartRow := max(0, _rowCount-svc.scene.UnlockedRows)
	scatterLockCount := countScatterLock(svc.scene)

	if len(svc.winInfos) == 0 {
		fprintf(buf, "\t未中奖\n")
	} else {
		// 单次遍历计算 totalMultiplier 并收集 winInfo
		totalMultiplier := int64(0)
		for _, elem := range svc.winInfos {
			totalMultiplier += elem.Odds
		}
		for _, elem := range svc.winInfos {
			lineWin := stepWin * float64(elem.Odds) / float64(max(totalMultiplier, 1))
			fprintf(buf, "\t符号:%2d, 支付线:%2d, 乘积: %d, 赔率: %4.2f, 下注: %g*%d, 奖金: %4.2f\n",
				elem.Symbol, elem.LineCount+1, elem.SymbolCount, float64(elem.Odds), svc.req.BaseMoney, svc.req.Multiple, lineWin)
		}
		fprintf(buf, "\tMode=%d, RoundMul: %d, lineMul: %d, 累计中奖: %.2f\n", btoi(svc.isFreeRound), svc.stepMultiplier, svc.stepMultiplier, roundWin)
	}

	if !svc.isRoundOver {
		fprintf(buf, "\t连消继续 -> Step%d\n", step+1)
		return
	}

	if isFree {
		writeFreeRoundState(buf, svc, treasureCount, totalTreasureCount, unlockedStartRow, scatterLockCount)
		if svc.scene.FreeNum == 0 {
			fprintf(buf, "\t免费模式结束 - RoundMultiplier: %d, 总奖金: %.2f\n", svc.stepMultiplier, roundWin)
			dumpFreeUnlockedScatterMuls(buf, svc)
		} else {
			fprintf(buf, "\t免费模式继续 - 剩余次数: %d, 本局结算倍数: %d\n", svc.scene.FreeNum, svc.stepMultiplier)
		}
	} else {
		writeBaseRoundState(buf, svc, treasureCount, totalTreasureCount, scatterLockCount)
	}
}

// countScatterLock 统计 ScatterLock 中非零元素个数
func countScatterLock(scene *SpinSceneData) int64 {
	if scene == nil {
		return 0
	}
	var count int64
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if scene.ScatterLock[r][c] != 0 {
				count++
			}
		}
	}
	return count
}

// countTreasuresInGrid 统计网格中 treasure 符号个数
func countTreasuresInGrid(grid *int64Grid) int64 {
	if grid == nil {
		return 0
	}
	var count int64
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if (*grid)[r][c] == _treasure {
				count++
			}
		}
	}
	return count
}

func writeBaseRoundState(buf *strings.Builder, svc *betOrderService, unlockedTreasureCount, totalTreasureCount, scatterLockTreasureCount int64) {
	fprintf(buf, "\tMode=0 Scatter(底4行)=%d | Scatter(全盘)=%d | nextStage=%d\n",
		unlockedTreasureCount, totalTreasureCount, svc.scene.NextStage)

	if svc.scene.NextStage == _spinTypeFree {
		fprintf(buf, "\t💎💎💎触发免费: +%d 次 | 当前剩余免费=%d | 解锁行=%d\n",
			svc.addFreeTime, svc.scene.FreeNum, svc.scene.UnlockedRows)
		fprintf(buf, "\t   已锁定Scatter格=%d(用于免费模式固定位置+固定倍数)\n", scatterLockTreasureCount)
	}
}

func writeFreeRoundState(buf *strings.Builder, svc *betOrderService, unlockedTreasureCount, totalTreasureCount int64, unlockedStartRow int, scatterLockTreasureCount int64) {
	newUnlockedRows := svc.scene.UnlockedRows - svc.scene.PrevUnlockedRows
	nextThreshold, remainToNext := getNextUnlockProgress(svc)

	fprintf(buf, "\t[Free] Mode=1, Scatter(已解锁区 row[%d..%d])=%d | Scatter(全盘)=%d\n",
		unlockedStartRow, _rowCount-1, unlockedTreasureCount, totalTreasureCount)
	fprintf(buf, "\t       解锁行: %d -> %d (本局新增=%d) | addFree=%d | 剩余免费=%d\n",
		svc.scene.PrevUnlockedRows, svc.scene.UnlockedRows, newUnlockedRows, svc.addFreeTime, svc.scene.FreeNum)

	if nextThreshold > 0 {
		fprintf(buf, "\t       下一档阈值: S>=%d | 当前还差=%d\n", nextThreshold, remainToNext)
	} else {
		fprintf(buf, "\t       下一档阈值: 已全部解锁(8行)\n")
	}

	fprintf(buf, "\t       nextStage=%d | lockScatter格数=%d\n", svc.scene.NextStage, scatterLockTreasureCount)
}

func getNextUnlockProgress(svc *betOrderService) (nextThreshold int64, remain int64) {
	if svc == nil || svc.gameConfig == nil || svc.scene.UnlockedRows >= _rowCount {
		return 0, 0
	}
	thresholds := svc.gameConfig.FreeUnlockThresholds
	if len(thresholds) <= svc.scene.UnlockedRows {
		return 0, 0
	}
	nextThreshold = thresholds[svc.scene.UnlockedRows]
	if nextThreshold <= 0 || svc.scatterCount >= nextThreshold {
		return nextThreshold, 0
	}
	return nextThreshold, nextThreshold - svc.scatterCount
}

func saveDebugFile(statsResult, detailResult string) {
	_ = os.MkdirAll("logs", 0755)
	filename := fmt.Sprintf("logs/%s.txt", time.Now().Format("20060102_150405"))
	_ = os.WriteFile(filename, []byte(statsResult+detailResult), 0644)
	fmt.Printf("\n调试信息已保存到: %s\n", filename)
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
