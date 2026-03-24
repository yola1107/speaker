package hcsqy

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
)

func TestRtp2(t *testing.T) {
	// 基础模式统计
	var baseRounds, baseWinRounds, baseFreeTriggered int64
	var baseTotalWin float64

	// 免费模式统计
	var freeRounds, freeWinRounds, freeTreasureInFree, freeExtraFreeRounds int64
	var freeTotalWin float64

	totalBet, start := 0.0, time.Now()
	buf := &strings.Builder{}
	svc := newBerService()
	svc.initGameConfigs()
	var respinStepsInBase, respinStepsInFree int64
	var resChainStartsBase, resChainStartsInFree int64
	baseGameCount, freeRoundIdx := 0, 0
	interval := int64(min(testRounds, progressInterval))

	var fileBuf *strings.Builder
	if debugFileOpen > 0 {
		fileBuf = &strings.Builder{}
	}

	for baseRounds < testRounds {
		var gameNum int
		var roundWin, freeRoundWin float64
		var triggeringBaseRound int
		var respinStep int // 重转至赢步数
		isFirst := true    // 每个回合只有第一次 spin 为 true

		for {
			if isFirst {
				roundWin = 0
				freeRoundWin = 0
				respinStep = 0
			} else {
				respinStep++ // 重转至赢步数递增
			}

			beforeRespin := svc.scene.IsRespinMode
			beforeFree := svc.isFreeRound
			_ = svc.baseSpin()
			if svc.respinWildCol >= 0 {
				if beforeFree {
					respinStepsInFree++
				} else {
					respinStepsInBase++
				}
				if !beforeRespin {
					if beforeFree {
						resChainStartsInFree++
					} else {
						resChainStartsBase++
					}
				}
			}
			isFree := svc.isFreeRound

			// 更新游戏计数（只有第一次 spin 才计数）
			if isFirst {
				if isFree {
					freeRoundIdx++
					gameNum = freeRoundIdx
					if triggeringBaseRound == 0 {
						triggeringBaseRound = baseGameCount
					}
				} else {
					// 基础模式：使用当前 baseGameCount 作为局号
					// baseGameCount 在回合结束时才增加
					gameNum = baseGameCount + 1
				}
			}

			stepWin := float64(svc.stepMultiplier)
			roundWin += stepWin

			// 统计奖金（必须在写日志之前，因为要设置 triggeringBaseRound）
			if isFree {
				freeTotalWin += stepWin
				freeRoundWin += stepWin
				if svc.addFreeTime > 0 {
					freeTreasureInFree++
					freeExtraFreeRounds += svc.addFreeTime
				}
			} else {
				baseTotalWin += stepWin
				// 与 rtp_test 一致：基础盘任意一次 addFreeTime>0 计一次触发（含重转至赢末手进免费，此时 isFirst=false）
				if svc.addFreeTime > 0 {
					baseFreeTriggered++
					if isFirst {
						triggeringBaseRound = baseGameCount + 1 // 日志局号：常见为首手触发
					}
				}
			}

			// 调试日志（在统计之后，使用正确的 triggeringBaseRound）
			if debugFileOpen > 0 && fileBuf != nil {
				triggerRound := 0
				if isFree {
					triggerRound = triggeringBaseRound
					if triggerRound == 0 && isFirst {
						triggerRound = baseGameCount
					}
				}
				writeSpinDetail(fileBuf, svc, gameNum, isFree, triggerRound, stepWin, roundWin, respinStep, isFirst)
			}

			// Round 结束处理
			if svc.isRoundOver {
				if isFree {
					freeRounds++
					if freeRoundWin > 0 {
						freeWinRounds++
					}
					freeRoundWin = 0
				} else {
					// 基础模式回合结束
					baseGameCount++ // 回合结束时增加局号
					baseRounds++
					if roundWin > 0 {
						baseWinRounds++
					}
					totalBet += float64(_baseMultiplier)
				}
				roundWin = 0

				// 只有当免费游戏完全结束时才重置服务并退出内层循环
				if svc.scene.FreeNum <= 0 {
					resetBetServiceForNextRound(svc)
					freeRoundIdx = 0
					if baseRounds%interval == 0 {
						totalWin := baseTotalWin + freeTotalWin
						// freeTime 与 rtp_test 一致：为基础模式触发免费次数（非 0，便于进度行与汇总对照）
						printBenchmarkProgress(buf, baseRounds, totalBet, baseTotalWin, freeTotalWin, totalWin, baseWinRounds, freeWinRounds, freeRounds, baseFreeTriggered, baseFreeTriggered, start)
						fmt.Print(buf.String())
					}
					break
				}
			} else if svc.addFreeTime > 0 && !isFree {
				// 基础模式触发免费时，这一局也算基础模式的一局
				// 计入 baseRounds（投入筹码的局数）和 baseGameCount
				// 注意：免费模式中触发额外免费不计入基础模式统计
				baseGameCount++
				baseRounds++
				totalBet += float64(_baseMultiplier)
				if roundWin > 0 {
					baseWinRounds++
				}
			}

			// 更新 isFirst：
			// - 如果回合结束（isRoundOver=true），下一局是新回合的第一局
			// - 如果触发免费（addFreeTime>0），下一局是免费模式第一局
			// - 如果必赢重转（isRoundOver=false 且非触发免费），下一局是重转，不是第一局
			if svc.isRoundOver {
				isFirst = true
			} else if svc.addFreeTime > 0 {
				isFirst = true // 触发免费，免费模式第一局
			} else {
				isFirst = false // 必赢重转
			}
		}
	}

	printFinalStats(buf, baseRounds, baseTotalWin, baseWinRounds, baseFreeTriggered,
		freeRounds, freeTotalWin, freeWinRounds, freeTreasureInFree, freeExtraFreeRounds, totalBet, start,
		respinStepsInBase, respinStepsInFree, resChainStartsBase, resChainStartsInFree)
	result := buf.String()
	fmt.Print(result)
	if debugFileOpen > 0 && fileBuf != nil {
		saveDebugFile(result, fileBuf.String(), start)
	}
}

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, gameNum int, isFree bool, triggeringBaseRound int, stepWin, roundWin float64, respinStep int, isFirst bool) {
	if isFree {
		trigger := "?"
		if triggeringBaseRound > 0 {
			trigger = fmt.Sprintf("%d", triggeringBaseRound)
		}
		fprintf(buf, "\n=============[基础模式] 第%s局 - 免费第%d局 =============\n", trigger, gameNum)
	} else {
		if respinStep > 0 {
			fprintf(buf, "\n=============[基础模式] 第%d局 (重转至赢 Step%d) =============\n", gameNum, respinStep+1)
		} else {
			fprintf(buf, "\n=============[基础模式] 第%d局 =============\n", gameNum)
		}
	}
	writeReelInfo(buf, svc)
	fprintf(buf, "Step1 初始盘面:\n")
	writeGridToBuilder(buf, &svc.symbolGrid, &svc.winGrid)

	fprintf(buf, "Step1 中奖详情:\n")
	if len(svc.winInfos) == 0 {
		fprintf(buf, "\t未中奖\n")
	} else {
		for _, elem := range svc.winInfos {
			fprintf(buf, "\t符号:%2d, 支付线:%2d, 赔率:%d\n", elem.Symbol, elem.LineCount+1, elem.Odds)
		}
	}

	lineMul := svc.lineMultiplier
	wildMul := svc.wildMultiplier
	stepMul := svc.stepMultiplier
	fprintf(buf, "\tMode=%d, RoundMul: %d, lineMul: %d, wildMul: %d, 累计中奖: %.2f\n", btoi(isFree), stepMul, lineMul, wildMul, roundWin)

	if wildMul != 0 && lineMul*wildMul != stepMul {
		fprintf(buf, "\t      (! stepMul 应等于 lineMul×wildMul，请查 processWinInfos)\n")
	}

	longwild := ""
	switch {
	case svc.scene.IsRespinMode && svc.respinWildCol >= 0:
		//fprintf(buf, "\t触发长条(重转至赢), col=%d mul=%d\n", svc.respinWildCol, wildMul)
		longwild = "💎重转至赢"
	case svc.wildExpandCol >= 0:
		//fprintf(buf, "\t触发长条(变大), col=%d mul=%d\n", svc.wildExpandCol, wildMul)
		longwild = "💎长条变大"
	}

	model := btoi(isFree)
	//treasureCount := svc.getScatterCount()
	//fprintf(buf, "\tMode=%d Stage=%d, nSt=%d, S=%d | FreeNum=%d CliFreeTimes=%d | Over=%v Next=%v MW=%v addFree=%d %s\n",
	//	btoi(isFree), svc.scene.Stage, svc.scene.NextStage, treasureCount,
	//	svc.scene.FreeNum, svc.client.ClientOfFreeGame.GetFreeTimes(),
	//	svc.isRoundOver, svc.next, svc.scene.IsRespinUntilWin, svc.addFreeTime, longwild)

	//switch {
	//case svc.scene.IsRespinUntilWin && svc.respinWildCol >= 0:
	//	fprintf(buf, "\t触发长条(重转至赢), col=%d mul=%d\n", svc.respinWildCol, wildMul)
	//case svc.wildExpandCol >= 0:
	//	fprintf(buf, "\t触发长条(变大), col=%d mul=%d\n", svc.wildExpandCol, wildMul)
	//	case wildMul > 1:
	//		fprintf(buf, "\t长条: ×%d\n", wildMul)
	//	default:
	//		fprintf(buf, "\t长条: -\n")
	//}

	if svc.addFreeTime > 0 {
		if !isFree {
			fprintf(buf, "\t🚨🚨🚨 Scatter(全盘)=%d, 触发免费: +%d 次 |  当前剩余免费=%d  %s\n", svc.scatterCount, svc.addFreeTime, svc.scene.FreeNum, longwild)
		} else {
			fprintf(buf, "\tMode=%d Scatter(全盘)=%d | 当前剩余免费=%d nextStage=%d, %s\n", model, svc.scatterCount, svc.scene.FreeNum, svc.scene.NextStage, longwild)
		}
	} else {
		fprintf(buf, "\tMode=%d Scatter(全盘)=%d | nextStage=%d %s\n", model, svc.scatterCount, svc.scene.NextStage, longwild)

	}

	//if !isFree && svc.addFreeTime > 0 {
	//	fprintf(buf, "\t🚨 触发免费: +%d 次 | 当前剩余免费=%d\n", svc.addFreeTime, svc.scene.FreeNum)
	//} else if svc.addFreeTime > 0 {
	//	fprintf(buf, "\t免费: +%d 次 (剩余 Free=%d)\n", svc.addFreeTime, svc.scene.FreeNum)
	//}
	fprintf(buf, "\n")
}

func writeReelInfo(buf *strings.Builder, svc *betOrderService) {
	if svc.scene == nil || len(svc.scene.SymbolRoller) == 0 {
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

func writeGridToBuilder(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			symbol := (*grid)[r][c]
			isWin := winGrid != nil && (*winGrid)[r][c] != 0
			if isWin {
				fprintf(buf, " %2d*|", symbol)
			} else {
				fprintf(buf, " %2d |", symbol)
			}
		}
		buf.WriteString("\n")
	}
}

func saveDebugFile(statsResult, detailResult string, start time.Time) {
	_ = os.MkdirAll("logs", 0755)
	filename := fmt.Sprintf("logs/%s.txt", time.Now().Format("20060102_150405"))
	_ = os.WriteFile(filename, []byte(statsResult+detailResult), 0644)
	fmt.Printf("\n调试信息已保存到: %s\n", filename)
}

func printFinalStats(buf *strings.Builder, baseRounds int64, baseTotalWin float64, baseWinRounds int64,
	baseFreeTriggered int64, freeRounds int64, freeTotalWin float64,
	freeWinRounds int64, freeTreasureInFree int64, freeExtraFreeRounds int64, totalBet float64, start time.Time,
	respinStepsInBase, respinStepsInFree, resChainStartsBase, resChainStartsInFree int64) {
	w := func(format string, args ...interface{}) { fprintf(buf, format, args...) }
	elapsed := time.Since(start)
	speed := safeDiv(baseRounds, int64(elapsed.Seconds()))
	w("\n运行局数: %d，用时: %v，速度: %.0f 局/秒\n", baseRounds, elapsed.Round(time.Second), speed)

	w("\n===== 详细统计汇总 =====\n")
	w("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	// 与 rtp_test（printBenchmarkSummary）一致：浮点口径，避免先转 int64 丢小数
	baseRTP := safeDivFloat(baseTotalWin*100, totalBet)
	freeRTP := safeDivFloat(freeTotalWin*100, totalBet)
	totalWin := baseTotalWin + freeTotalWin
	totalRTP := safeDivFloat(totalWin*100, totalBet)
	baseWinRate := safeDiv(baseWinRounds*100, baseRounds)
	freeWinRate := safeDiv(freeWinRounds*100, max(freeRounds, 1))
	freeTriggerRate := safeDiv(baseFreeTriggered*100, baseRounds)
	avgFreePerTrigger := safeDiv(freeRounds, baseFreeTriggered)
	baseContrib := safeDivFloat(baseTotalWin*100, totalWin)
	freeContrib := safeDivFloat(freeTotalWin*100, totalWin)
	resStartRateBase := safeDiv(resChainStartsBase*100, baseRounds)
	resStartRateFree := safeDiv(resChainStartsInFree*100, max(freeRounds, 1))
	resChainTotal := resChainStartsBase + resChainStartsInFree
	avgRespinStepsPerChain := safeDiv(respinStepsInBase+respinStepsInFree, resChainTotal)

	w("\n[基础模式统计]\n")
	w("基础模式总游戏局数: %d\n", baseRounds)
	w("基础模式总投注(倍数): %.2f\n", totalBet)
	w("基础模式总奖金: %.2f\n", baseTotalWin)
	w("基础模式RTP: %.2f%% (基础模式奖金/基础模式投注)\n", baseRTP)
	w("基础模式免费局触发次数: %d\n", baseFreeTriggered)
	w("基础模式触发免费局比例: %.2f%%\n", freeTriggerRate)
	w("基础模式中奖率: %.2f%%\n", baseWinRate)
	w("基础模式中奖局数: %d\n", baseWinRounds)
	w("重转至赢·基础: 步%d | 链%d | 率%.2f%% | 均%.4f步/局\n",
		respinStepsInBase, resChainStartsBase, resStartRateBase, safeDivFloat(float64(respinStepsInBase), float64(baseRounds)))

	w("\n[免费模式统计]\n")
	w("免费模式总游戏局数: %d\n", freeRounds)
	w("免费模式总奖金: %.2f\n", freeTotalWin)
	w("免费模式RTP: %.2f%% (免费模式奖金/基础模式投注，因为免费模式不投注)\n", freeRTP)
	w("免费模式中奖率: %.2f%%\n", freeWinRate)
	w("免费模式中奖局数: %d\n", freeWinRounds)
	w("免费模式额外增加局数: %d\n", freeExtraFreeRounds)
	w("免费模式出现夺宝的次数: %d (%.2f%%)\n", freeTreasureInFree, safeDiv(freeTreasureInFree*100, max(freeRounds, 1)))
	w("重转至赢·免费: 步%d | 链%d | 率%.2f%% | 均%.4f步/局\n",
		respinStepsInFree, resChainStartsInFree, resStartRateFree, safeDivFloat(float64(respinStepsInFree), float64(max(freeRounds, 1))))

	w("\n[免费触发效率]\n")
	w("  总免费游戏次数: %d (真实的游戏局数，包含中途增加的免费次数)\n", freeRounds)
	w("  总触发次数: %d (基础模式触发免费游戏的次数)\n", baseFreeTriggered)
	w("  平均1次触发获得免费游戏: %.2f次 (总免费游戏次数 / 总触发次数)\n", avgFreePerTrigger)
	w("  重转至赢·合计: %d步 | %d链 | %.2f 步/链\n", respinStepsInBase+respinStepsInFree, resChainTotal, avgRespinStepsPerChain)

	w("\n[总计]\n")
	w("  总投注(倍数): %.2f (仅基础模式投注，免费模式不投注)\n", totalBet)
	w("  总奖金: %.2f (基础模式奖金 + 免费模式奖金)\n", totalWin)
	w("  总回报率(RTP): %.2f%% (总奖金/总投注 = %.2f/%.2f)\n", totalRTP, totalWin, totalBet)
	w("  基础贡献: %.2f%% | 免费贡献: %.2f%%\n", baseContrib, freeContrib)

	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}
