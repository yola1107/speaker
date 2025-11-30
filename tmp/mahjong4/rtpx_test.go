package mahjong

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	testRounds       = 1e3
	progressInterval = 1e6
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
			isFirst := svc.scene.Steps == 0
			wasFreeBeforeSpin := svc.isFreeRound

			if isFirst {
				roundWin = 0
				freeRoundWin = 0
			}

			res, _ := svc.baseSpin()
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
			stepWin := res.stepWin
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
				writeSpinDetail(fileBuf, svc, res, gameNum, cascadeCount, isFree, triggerRound, stepWin, roundWin, isFirst)
			}

			// 统计奖金
			if isFree {
				freeTotalWin += stepWin
				freeRoundWin += stepWin
				if res.addFreeTime > 0 {
					freeTreasureInFree++
					freeExtraFreeRounds += res.addFreeTime
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
					if !wasFreeBeforeSpin && res.winInfo.State == runStateFreeGame {
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
						printProgress(buf, baseRounds, totalBet, baseTotalWin, freeTotalWin, time.Since(start), baseWinRounds, freeWinRounds, baseFreeTriggered, freeRounds)
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

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, result *BaseSpinResult,
	gameNum, step int, isFree bool, triggeringBaseRound int, stepWin, roundWin float64, isFirstStep bool) {
	if step == 1 {
		writeRoundHeader(buf, svc, gameNum, isFree, triggeringBaseRound)
	} else {
		writeReelInfo(buf, svc)
	}
	buf.WriteString(fmt.Sprintf("Step%d 初始盘面:\n", step))
	printGridToBuf(buf, &result.cards, nil)

	if len(result.winInfo.WinArr) > 0 {
		buf.WriteString(fmt.Sprintf("Step%d 中奖标记:\n", step))
		fullWinGrid := winGridRewardToFull(result.winGrid)
		printGridToBuf(buf, &result.cards, &fullWinGrid)
	}

	if !svc.isRoundOver {
		buf.WriteString(fmt.Sprintf("Step%d 下一盘面预览（实际消除+下落+填充结果）:\n", step))
		printGridToBuf(buf, &result.nextSymbolGrid, nil)
	}
	writeStepSummary(buf, svc, result, step, isFree, stepWin, roundWin)
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
		realIdx := svc.scene.SymbolRoller[c].Real
		length := len(svc.gameConfig.RealData[realIdx][c])
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
}

func writeStepSummary(buf *strings.Builder, svc *betOrderService, ret *BaseSpinResult, step int, isFree bool, stepWin, roundWin float64) {
	buf.WriteString(fmt.Sprintf("Step%d 中奖详情:\n", step))
	treasureCount := ret.scatterCount
	isRoundOver := svc.isRoundOver

	if len(ret.winInfo.WinArr) == 0 {
		buf.WriteString("\t未中奖\n")
		if isRoundOver {
			if isFree && treasureCount > 0 {
				buf.WriteString(fmt.Sprintf("\t💎 当前盘面夺宝数量: %d\n", treasureCount))
			} else if !isFree && svc.scene.NextStage == _spinTypeFree {
				buf.WriteString(fmt.Sprintf("\t💎💎💎 基础模式。 夺宝=%d 触发免费游戏=%d\n", treasureCount, svc.scene.FreeNum))
			}
		}
		return
	}

	// 计算总倍数并打印每条中奖线
	totalMultiplier := int64(0)
	for _, win := range ret.winInfo.WinArr {
		totalMultiplier += win.Mul * ret.gameMultiple
	}
	for _, win := range ret.winInfo.WinArr {
		lineMultiplier := win.Mul * ret.gameMultiple
		lineWin := float64(0)
		if totalMultiplier > 0 {
			lineWin = ret.stepWin * float64(lineMultiplier) / float64(totalMultiplier)
		}
		buf.WriteString(fmt.Sprintf("\t符号: %2d, 支付线: %2d, 乘积: %d, 赔率: %4.2f, 下注: %g×%d, 奖金: %4.2f\n",
			win.Val, win.RoadNum+1, win.StarNum, float64(win.Odds), svc.req.BaseMoney, svc.req.Multiple, lineWin))
	}
	buf.WriteString(fmt.Sprintf("\tisFreeMode=%d, RoundMultiplier: %d, stepMultiplier: %d, lineMultiplier: %d, gameMultiple: %d, ContinueNum: %d\n",
		ret.winInfo.State, svc.scene.RoundMultiplier, ret.stepMultiplier, ret.lineMultiplier, ret.gameMultiple, svc.scene.ContinueNum))

	buf.WriteString(fmt.Sprintf("\t累计中奖: %.2f \n", roundWin))

	if isRoundOver {
		buf.WriteString("\t🛑 连消结束（无后续可消除）\n\n")
		if isFree {
			if treasureCount > 0 {
				buf.WriteString(fmt.Sprintf("\t💎 当前盘面夺宝数量: %d, 增加免费次数: %d\n", treasureCount, ret.addFreeTime))
			}
			if svc.scene.FreeNum == 0 {
				buf.WriteString(fmt.Sprintf("\t🎉 免费模式结束 - RoundMultiplier: %d, 总奖金: %.2f\n", svc.scene.RoundMultiplier, roundWin))
			} else {
				buf.WriteString(fmt.Sprintf("\t➡️ 免费模式继续 - 剩余次数: %d, RoundMultiplier: %d\n", svc.scene.FreeNum, svc.scene.RoundMultiplier))
			}
		} else if svc.isFreeRound {
			buf.WriteString(fmt.Sprintf("\t💎💎💎 基础模式。 夺宝=%d 触发免费游戏=%d\n", treasureCount, svc.scene.FreeNum))
		}
	} else {
		buf.WriteString(fmt.Sprintf("\t🔁 连消继续 → Step%d\n", step+1))
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

func printFinalStats(buf *strings.Builder, baseRounds int64, baseTotalWin float64, baseWinRounds int64,
	baseCascadeSteps int64, baseMaxCascadeSteps int, baseFreeTriggered int64, freeRounds int64, freeTotalWin float64,
	freeWinRounds int64, freeCascadeSteps int64, freeMaxCascadeSteps int, freeTreasureInFree int64,
	freeExtraFreeRounds int64, freeMaxFreeStreak int64, totalBet float64, start time.Time) {
	w := func(format string, args ...interface{}) { buf.WriteString(fmt.Sprintf(format, args...)) }

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
		w("免费模式出现夺宝的次数: %d (%.2f%%)\n", freeTreasureInFree, float64(freeTreasureInFree)*100/float64(freeRounds))
		w("免费模式平均连消步数: %.2f\n", float64(freeCascadeSteps)/float64(freeRounds))
		w("免费模式最大连消步数: %d\n", freeMaxCascadeSteps)
	}

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

func calculateRtp(win, rounds, multiplier int64) float64 {
	if rounds == 0 || multiplier == 0 {
		return 0
	}
	return float64(win) * 100 / float64(rounds*multiplier)
}
