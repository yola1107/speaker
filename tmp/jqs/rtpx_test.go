package jqs

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	testRounds       = 1000
	progressInterval = 1e4
	debugFileOpen    = 1 // 0=关闭调试输出，>0=开启
	freeModeLogOnly  = 0 // 0=记录所有，1=只记录免费模式
)

func TestRtp2(t *testing.T) {
	// 基础模式统计
	var baseRounds, baseWinRounds, baseFreeTriggered int64
	var baseTotalWin float64

	// 免费模式统计
	var freeRounds, freeWinRounds, freeContinueCount int64
	var freeTotalWin float64
	var freeMaxContinue int64

	totalBet, start := 0.0, time.Now()
	buf := &strings.Builder{}
	svc := newTestService()
	svc.initGameConfigs()
	baseGameCount := 0 // 基础模式从第0局开始，第一次回合结束时变成第1局
	interval := int64(min(testRounds, progressInterval))

	var fileBuf *strings.Builder
	if debugFileOpen > 0 {
		fileBuf = &strings.Builder{}
	}

	for baseRounds < testRounds {
		var continueCount int64
		var roundWin, freeRoundWin float64
		var triggeringBaseRound int // 初始为0，参考sgz

		for {
			isFirst := svc.scene.Stage == _spinTypeBase
			wasFreeBeforeSpin := svc.scene.Stage == _spinTypeFree

			if isFirst {
				roundWin = 0
				freeRoundWin = 0
			}

			if err := svc.baseSpin(); err != nil {
				panic(err)
			}

			// 参考jqt实现，在baseSpin后调用updateBonusAmount计算奖金
			svc.updateBonusAmount()

			isFree := svc.scene.Stage == _spinTypeFree

			// 从基础模式切换到免费模式时，重置 continueCount
			if isFirst && !wasFreeBeforeSpin && isFree {
				continueCount = 0
			}

			// 更新游戏计数（参考sgz第64-76行逻辑）
			if isFirst {
				if isFree {
					// 免费模式下，记录触发局数
					if triggeringBaseRound == 0 {
						triggeringBaseRound = baseGameCount
					}
				} else {
					// 基础模式下，递增基础局数
					baseGameCount++
				}
			}

			continueCount++
			stepWin := float64(svc.stepMultiplier) // 与jqt保持一致，使用stepMultiplier
			roundWin += stepWin

			// 更新最大连续次数
			if isFree && continueCount > freeMaxContinue {
				freeMaxContinue = continueCount
			}

			// 调试日志
			if debugFileOpen > 0 && fileBuf != nil && (freeModeLogOnly == 0 || isFree) {
				triggerRound := 0
				currentFreeNum := 1
				if isFree {
					triggerRound = triggeringBaseRound
					currentFreeNum = int(continueCount)
				}
				actualBaseNum := baseGameCount
				if isFree {
					actualBaseNum = triggeringBaseRound
				}
				writeSpinDetail(fileBuf, svc, actualBaseNum, currentFreeNum, continueCount, isFree, triggerRound, stepWin, roundWin, isFirst)
			}

			// 统计奖金
			if isFree {
				freeTotalWin += stepWin
				freeRoundWin += stepWin
			} else {
				baseTotalWin += stepWin
			}

			// Round 结束处理
			if svc.isRoundCompleted {
				// 统计连续次数和胜负
				if isFree {
					freeContinueCount += continueCount
					freeRounds++
					// 只有在免费round结束时才判断胜负和重置
					if svc.scene.NextStage == _spinTypeBase {
						if freeRoundWin > 0 {
							freeWinRounds++
						}
						freeRoundWin = 0 // 在免费round结束时重置
					}
				} else {
					baseRounds++
					if roundWin > 0 {
						baseWinRounds++
					}
					totalBet += float64(_baseMultiplier)
					// 基础模式回合结束时，如果触发了免费游戏
					if svc.scene.NextStage == _spinTypeFree {
						baseFreeTriggered++
						// 记录触发免费游戏的基础局数（参考sgz第138-141行）
						triggeringBaseRound = baseGameCount
					}
				}
				roundWin = 0

				// 只有当免费游戏完全结束时才重置服务并退出内层循环
				if !isFree || svc.scene.NextStage == _spinTypeBase {
					svc = newTestService()
					svc.initGameConfigs()
					if baseRounds%interval == 0 {
						totalWin := baseTotalWin + freeTotalWin
						printBenchmarkProgress(buf, baseRounds, totalBet, baseTotalWin, freeTotalWin, totalWin, baseWinRounds, freeWinRounds, freeRounds, baseFreeTriggered, 0, start)
						fmt.Print(buf.String())
					}
					break
				}
				continueCount = 0
			}
		}
	}

	printFinalStats(buf, baseRounds, baseTotalWin, baseWinRounds, baseFreeTriggered,
		freeRounds, freeTotalWin, freeWinRounds, freeContinueCount, freeMaxContinue, totalBet, start)
	result := buf.String()
	fmt.Print(result)
	if debugFileOpen > 0 && fileBuf != nil {
		saveDebugFile(result, fileBuf.String(), start)
	}
}

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, baseGameNum, freeGameNum int, step int64, isFree bool, triggeringBaseRound int, stepWin, roundWin float64, isFirstStep bool) {
	if step == 1 {
		writeRoundHeader(buf, svc, baseGameNum, freeGameNum, isFree, triggeringBaseRound)
	}

	fprintf(buf, "Step%d 初始盘面:\n", step)
	writeGridToBuilder(buf, svc.symbolGrid)

	if len(svc.winResults) > 0 {
		fprintf(buf, "Step%d 中奖标记:\n", step)
		writeWinGridToBuilder(buf, svc.symbolGrid, svc.winGrid)
	}

	writeStepSummary(buf, svc, step, isFree, stepWin, roundWin)
	fprintf(buf, "\n")
}

func writeRoundHeader(buf *strings.Builder, svc *betOrderService, baseGameNum, freeGameNum int, isFree bool, triggeringBaseRound int) {
	if isFree {
		trigger := "?"
		if triggeringBaseRound > 0 {
			trigger = fmt.Sprintf("%d", triggeringBaseRound)
		}
		fprintf(buf, "\n=============[基础模式] 第%s局 - 免费第%d局 =============\n", trigger, freeGameNum)
	} else {
		fprintf(buf, "\n=============[基础模式] 第%d局 =============\n", baseGameNum)
	}
}

func writeStepSummary(buf *strings.Builder, svc *betOrderService, step int64, isFree bool, stepWin, roundWin float64) {
	fprintf(buf, "Step%d 中奖详情:\n", step)

	if len(svc.winResults) == 0 {
		fprintf(buf, "\t未中奖\n")
		if svc.scene.NextStage == _spinTypeBase {
			if isFree {
				if svc.isAllWild() {
					fprintf(buf, "\t💎 九个位置全百搭，获得%d倍奖励\n", svc.gameConfig.MaxPayMultiple)
				}
			} else if svc.scene.NextStage == _spinTypeFree {
				fprintf(buf, "\t🔄 基础模式触发Re-spin，进入免费游戏\n")
			}
		}
	}

	for _, result := range svc.winResults {
		// 按比例分配lineWin，确保lineWin之和等于stepWin（参考sgz实现）
		var lineWin float64
		if roundWin > 0 && svc.stepMultiplier > 0 {
			lineWin = roundWin * float64(result.TotalMultiplier) / float64(svc.stepMultiplier)
		}
		fprintf(buf, "\t符号:%2d, 支付线:%2d, 连续: %d, 赔率: %4.2f, 倍数: %d, 奖金: %4.2f\n",
			result.Symbol, result.LineNo, result.SymbolCount, float64(result.BaseLineMultiplier), result.TotalMultiplier, lineWin)
	}

	mode := 0
	if isFree {
		mode = 1
	}
	fprintf(buf, "\tMode=%d, stepMul: %d, lineMul: %d, 累计中奖: %.2f\n",
		mode, svc.stepMultiplier, svc.lineMultiplier, roundWin)

	if svc.scene.NextStage == _spinTypeBase {
		if isFree {
			fprintf(buf, "\t🛑 免费模式结束 - 总奖金: %.2f\n", roundWin)
		} else if svc.scene.NextStage == _spinTypeFree {
			fprintf(buf, "\t🔄 基础模式触发Re-spin\n")
		}
	} else {
		fprintf(buf, "\t🔁 免费模式继续\n")
	}
}

func writeGridToBuilder(buf *strings.Builder, grid int64Grid) {
	for row := 0; row < _rowCount; row++ {
		for col := 0; col < _colCount; col++ {
			symbol := grid[row][col]
			if symbol == 0 {
				buf.WriteString("    |")
			} else {
				_, _ = fmt.Fprintf(buf, " %2d |", symbol)
			}
			if col < _colCount-1 {
				buf.WriteString(" ")
			}
		}
		buf.WriteString("\n")
	}
}

func writeWinGridToBuilder(buf *strings.Builder, symbolGrid, winGrid int64Grid) {
	for row := 0; row < _rowCount; row++ {
		for col := 0; col < _colCount; col++ {
			symbol := symbolGrid[row][col]
			isWin := winGrid[row][col] > 0
			if symbol == 0 {
				if isWin {
					buf.WriteString("   *|")
				} else {
					buf.WriteString("    |")
				}
			} else {
				if isWin {
					_, _ = fmt.Fprintf(buf, " %2d*|", symbol)
				} else {
					_, _ = fmt.Fprintf(buf, " %2d |", symbol)
				}
			}
			if col < _colCount-1 {
				buf.WriteString(" ")
			}
		}
		buf.WriteString("\n")
	}
}

func saveDebugFile(statsResult, detailResult string, start time.Time) {
	_ = os.MkdirAll("logs", 0755)
	filename := fmt.Sprintf("logs/%s.txt", time.Now().Format("20060102_150405"))
	_ = os.WriteFile(filename, []byte(statsResult+detailResult), 0644)
	fmt.Printf("\n📄 调试信息已保存到: %s\n", filename)
}

func printFinalStats(buf *strings.Builder, baseRounds int64, baseTotalWin float64, baseWinRounds int64,
	baseFreeTriggered int64, freeRounds int64, freeTotalWin float64,
	freeWinRounds int64, freeContinueCount int64, freeMaxContinue int64, totalBet float64, start time.Time) {

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
	w("基础模式RTP: %.2f%% (基础模式奖金/基础模式投注)\n", safeDiv(int64(baseTotalWin*100), int64(totalBet)))
	w("基础模式Re-spin触发次数: %d\n", baseFreeTriggered)
	w("基础模式触发Re-spin比例: %.2f%%\n", safeDiv(baseFreeTriggered*100, baseRounds))
	w("基础模式平均每局免费次数: %.2f\n", safeDiv(freeRounds, baseRounds))
	w("基础模式中奖率: %.2f%%\n", safeDiv(baseWinRounds*100, baseRounds))
	w("基础模式中奖局数: %d\n", baseWinRounds)

	w("\n[免费模式统计]\n")
	w("免费模式总游戏局数: %d\n", freeRounds)
	w("免费模式总奖金: %.2f\n", freeTotalWin)
	w("免费模式RTP: %.2f%% (免费模式奖金/基础模式投注，因为免费模式不投注)\n", safeDiv(int64(freeTotalWin*100), int64(totalBet)))
	w("免费模式中奖局数: %d\n", freeWinRounds)
	w("免费模式中奖率: %.2f%%\n", safeDiv(freeWinRounds*100, freeRounds))
	w("免费模式总连续次数: %d\n", freeContinueCount)
	w("免费模式平均连续次数: %.2f\n", safeDiv(freeContinueCount, freeRounds))
	w("免费模式最大连续次数: %d\n", freeMaxContinue)

	totalWin := baseTotalWin + freeTotalWin
	w("\n[Re-spin触发效率]\n")
	w("  总免费游戏次数: %d (真实的免费游戏局数)\n", freeRounds)
	w("  总触发次数: %d (基础模式触发Re-spin的次数)\n", baseFreeTriggered)
	w("  平均1次触发获得免费游戏: %.2f次\n", safeDiv(freeRounds, baseFreeTriggered))

	w("\n[总计]\n")
	w("  总投注(倍数): %.2f (仅基础模式投注，免费模式不投注)\n", totalBet)
	w("  总奖金: %.2f (基础模式奖金 + 免费模式奖金)\n", totalWin)
	totalRTP := safeDiv(int64(totalWin*100), int64(totalBet))
	w("  总回报率(RTP): %.2f%% (总奖金/总投注 = %.2f/%.2f)\n", totalRTP, totalWin, totalBet)
	w("  基础贡献: %.2f%% | 免费贡献: %.2f%%\n", safeDiv(int64(baseTotalWin*100), int64(totalWin)), safeDiv(int64(freeTotalWin*100), int64(totalWin)))
	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}
