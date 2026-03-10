package pjcd

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
	var freeScatterInFree, freeExtraFreeRounds, freeMaxFreeStreak int64
	var freeTotalWin float64
	var freeMaxCascadeSteps int

	totalBet, start := 0.0, time.Now()
	buf := &strings.Builder{}
	svc := newBetService()
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
			isFirst := svc.scene.Steps == 0
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
			stepWin := float64(svc.stepMultiplier)
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
					freeScatterInFree++
					freeExtraFreeRounds += svc.addFreeTime
				}
			} else {
				baseTotalWin += stepWin
			}

			// Round 结束处理
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
		freeRounds, freeTotalWin, freeWinRounds, freeCascadeSteps, freeMaxCascadeSteps, freeScatterInFree, freeExtraFreeRounds, freeMaxFreeStreak, totalBet, start)
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
	writeGridToBuilder(buf, &svc.symbolGrid, nil)

	if len(svc.winInfos) > 0 {
		fprintf(buf, "Step%d 中奖标记:\n", step)
		writeGridToBuilder(buf, &svc.symbolGrid, &svc.winGrid)
	}

	if !svc.isRoundOver {
		fprintf(buf, "Step%d 下一盘面预览（实际消除+下落+填充结果）:\n", step)
		writeGridToBuilder(buf, &svc.nextSymbolGrid, nil)
	}
	writeStepSummary(buf, svc, step, isFree, stepWin, roundWin)
	fprintf(buf, "\n")
}

func writeReelInfo(buf *strings.Builder, svc *betOrderService) {
	if svc.scene == nil || len(svc.scene.SymbolRoller) == 0 {
		fprintf(buf, "滚轴配置Index: 0\n转轮信息长度/起始：未初始化\n")
		return
	}
	fprintf(buf, "滚轴配置: ")
	for c := 0; c < len(svc.scene.SymbolRoller); c++ {
		rc := svc.scene.SymbolRoller[c]
		fprintf(buf, "len=%d,pos=%d  ", rc.Length, rc.Position)
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
	scatterCount := svc.getScatterCount()

	if len(svc.winInfos) == 0 {
		fprintf(buf, "\t未中奖\n")
		if svc.isRoundOver {
			if isFree && scatterCount > 0 {
				fprintf(buf, "\t夺宝数量: %d, 增加免费次数: %d\n", scatterCount, svc.addFreeTime)
			} else if !isFree && svc.scene.NextStage == _spinTypeFree {
				fprintf(buf, "\t夺宝=%d 触发免费游戏=%d\n", scatterCount, svc.scene.FreeNum)
			}
		}
		return
	}

	for _, elem := range svc.winInfos {
		fprintf(buf, "\t符号:%2d, 支付线:%2d, 连数: %d, 赔率: %d\n",
			elem.Symbol, elem.LineIndex+1, elem.SymbolCount, elem.Odds)
	}

	fprintf(buf, "\tStepMul: %d, RoundMul: %d, 累计中奖: %.2f\n",
		svc.stepMultiplier, svc.scene.RoundMultiplier, roundWin)

	if !svc.isRoundOver {
		fprintf(buf, "\t连消继续 → Step%d\n", step+1)
		return
	}

	fprintf(buf, "\t连消结束\n")
	if isFree && scatterCount > 0 {
		fprintf(buf, "\t夺宝数量: %d, 增加免费次数: %d\n", scatterCount, svc.addFreeTime)
	}
	if isFree {
		if svc.scene.FreeNum == 0 {
			fprintf(buf, "\t免费模式结束 - RoundMultiplier: %d, 总奖金: %.2f\n", svc.scene.RoundMultiplier, roundWin)
		} else {
			fprintf(buf, "\t免费模式继续 - 剩余次数: %d, RoundMultiplier: %d\n", svc.scene.FreeNum, svc.scene.RoundMultiplier)
		}
	} else if svc.isFreeRound {
		fprintf(buf, "\t夺宝=%d 触发免费游戏=%d\n", scatterCount, svc.scene.FreeNum)
	}
}

func saveDebugFile(statsResult, detailResult string, start time.Time) {
	_ = os.MkdirAll("logs", 0755)
	filename := fmt.Sprintf("logs/%s.txt", time.Now().Format("20060102_150405"))
	_ = os.WriteFile(filename, []byte(statsResult+detailResult), 0644)
	fmt.Printf("\n调试信息已保存到: %s\n", filename)
}

func printFinalStats(buf *strings.Builder, baseRounds int64, baseTotalWin float64, baseWinRounds int64,
	baseCascadeSteps int64, baseMaxCascadeSteps int, baseFreeTriggered int64, freeRounds int64, freeTotalWin float64,
	freeWinRounds int64, freeCascadeSteps int64, freeMaxCascadeSteps int, freeScatterInFree int64,
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
	w("基础模式RTP: %.2f%%\n", safeDiv(int64(baseTotalWin)*100, int64(totalBet)))
	w("基础模式免费局触发次数: %d\n", baseFreeTriggered)
	w("基础模式触发免费局比例: %.2f%%\n", safeDiv(baseFreeTriggered*100, baseRounds))
	w("基础模式中奖率: %.2f%%\n", safeDiv(baseWinRounds*100, baseRounds))
	w("基础模式平均连消步数: %.2f\n", safeDiv(baseCascadeSteps, baseRounds))
	w("基础模式最大连消步数: %d\n", baseMaxCascadeSteps)

	w("\n[免费模式统计]\n")
	w("免费模式总游戏局数: %d\n", freeRounds)
	w("免费模式总奖金: %.2f\n", freeTotalWin)
	w("免费模式RTP: %.2f%%\n", safeDiv(int64(freeTotalWin)*100, int64(totalBet)))
	w("免费模式中奖率: %.2f%%\n", safeDiv(freeWinRounds*100, freeRounds))

	totalWin := baseTotalWin + freeTotalWin
	w("\n[总计]\n")
	w("总回报率(RTP): %.2f%%\n", safeDiv(int64(totalWin)*100, int64(totalBet)))
	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}
