package tmtg

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
	debugFileOpen    = 10 // >0：写 logs/*.txt（详情见 writeSpinDetail）
	freeModeLogOnly  = 0  // >0 仅记录免费 spin

	debugTraceStderr            = 0    // 1：每 baseSpin 一行 stderr（很慢）
	maxInnerCascadeSteps        = 2000 // 未完免费链内 spin 数上限，防死循环
	maxAverageSpinsPerBaseRound = 100  // 整场 spin 硬顶 = testRounds×本值
	maxTotalSpins               = testRounds * maxAverageSpinsPerBaseRound
)

func TestRtp2(t *testing.T) {
	var baseRounds, baseWinRounds, baseCascadeSteps, baseFreeTriggered int64
	var baseTotalWin float64
	var baseMaxCascadeSteps int
	var freeRounds, freeWinRounds, freeCascadeSteps int64
	var freeTreasureInFree, freeExtraFreeRounds, freeMaxFreeStreak int64
	var freeTotalWin float64
	var freeMaxCascadeSteps int

	totalBet, start := 0.0, time.Now()
	buf := &strings.Builder{}
	s := newRtpBetService()
	s.initGameConfigs()
	baseGameCount, freeRoundIdx := 0, 0
	interval := int64(min(testRounds, progressInterval))
	var totalSpins int64

	var fileBuf *strings.Builder
	if debugFileOpen > 0 {
		fileBuf = &strings.Builder{}
	}

	for baseRounds < testRounds {
		var cascadeCount, gameNum int
		var roundWin, freeRoundWin float64
		var triggeringBaseRound int

		for {
			isFirst := s.scene.Steps == 0
			wasFreeBeforeSpin := s.isFreeRound
			if isFirst {
				roundWin, freeRoundWin = 0, 0
			}
			if err := s.baseSpin(); err != nil {
				t.Fatalf("baseSpin failed: %v", err)
			}
			totalSpins++
			if totalSpins > maxTotalSpins {
				t.Fatalf("TestRtp2: baseSpin 次数超过整场硬上限 %d（= testRounds×%d）。baseRounds=%d/%d freeRounds=%d FreeNum=%d FreeTimes=%d isFreeRound=%v stage=%d steps=%d isRoundOver=%v nWin=%d",
					int64(maxTotalSpins), int64(maxAverageSpinsPerBaseRound), baseRounds, int64(testRounds), freeRounds,
					s.scene.FreeNum, s.scene.FreeTimes, s.isFreeRound, s.scene.Stage, s.scene.Steps,
					s.isRoundOver, len(s.winInfos))
			}
			isFree := s.isFreeRound

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
				} else {
					baseGameCount++
					gameNum = baseGameCount
				}
			}

			cascadeCount++
			stepWin := float64(s.stepMultiplier)
			roundWin += stepWin

			if debugTraceStderr != 0 {
				fmt.Fprintf(os.Stderr,
					"[TestRtp2] baseRounds=%d baseGame=%d freeIdx=%d cascade=%d isFirst=%v wasFree=%v nowFree=%v isRoundOver=%v freeNum=%d stage=%d steps=%d nWin=%d winCells=%d stepMul=%d addFree=%d sym=%s nextPre=%s\n",
					baseRounds, baseGameCount, freeRoundIdx, cascadeCount, isFirst, wasFreeBeforeSpin, isFree,
					s.isRoundOver, s.scene.FreeNum, s.scene.Stage, s.scene.Steps, len(s.winInfos),
					winGridNonZeroCount(&s.winGrid), s.stepMultiplier, s.addFreeTime,
					gridFingerprint(&s.symbolGrid), gridFingerprint(&s.nextSymbolGrid))
			}
			if cascadeCount > maxInnerCascadeSteps {
				var dump strings.Builder
				tr := rtpXDetailTriggerBase(isFree, isFirst, triggeringBaseRound, baseGameCount)
				writeSpinDetail(&dump, s, gameNum, cascadeCount, isFree, tr, roundWin)
				t.Fatalf("内层步数超过上限 %d（疑似死循环）。freeNum=%d stage=%d steps=%v isRoundOver=%v nWin=%d winCells=%d\n%s",
					maxInnerCascadeSteps, s.scene.FreeNum, s.scene.Stage, s.scene.Steps, s.isRoundOver,
					len(s.winInfos), winGridNonZeroCount(&s.winGrid), dump.String())
			}
			if isFree && s.scene.FreeNum > freeMaxFreeStreak {
				freeMaxFreeStreak = s.scene.FreeNum
			}
			if debugFileOpen > 0 && fileBuf != nil && (freeModeLogOnly == 0 || isFree) {
				tr := rtpXDetailTriggerBase(isFree, isFirst, triggeringBaseRound, baseGameCount)
				writeSpinDetail(fileBuf, s, gameNum, cascadeCount, isFree, tr, roundWin)
			}

			if isFree {
				freeTotalWin += stepWin
				freeRoundWin += stepWin
				if s.addFreeTime > 0 {
					freeTreasureInFree++
					freeExtraFreeRounds += s.addFreeTime
				}
			} else {
				baseTotalWin += stepWin
			}

			if !s.isRoundOver {
				continue
			}

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
				if s.isFreeRound {
					triggeringBaseRound = baseGameCount
				}
			}
			roundWin = 0

			if s.scene.FreeNum <= 0 {
				resetRtpBetCounters(s)
				freeRoundIdx = 0
				if baseRounds%interval == 0 {
					tw := baseTotalWin + freeTotalWin
					printBenchmarkProgress(buf, baseRounds, totalBet, baseTotalWin, freeTotalWin, tw,
						baseWinRounds, freeWinRounds, freeRounds, baseFreeTriggered, start)
					fmt.Print(buf.String())
				}
				break
			}
			cascadeCount = 0
		}
	}

	printFinalStats(buf, baseRounds, baseTotalWin, baseWinRounds, baseCascadeSteps, baseMaxCascadeSteps, baseFreeTriggered,
		freeRounds, freeTotalWin, freeWinRounds, freeCascadeSteps, freeMaxCascadeSteps, freeTreasureInFree, freeExtraFreeRounds, freeMaxFreeStreak, totalBet, start)
	out := buf.String()
	fmt.Print(out)
	if debugFileOpen > 0 {
		saveRtpXDebugFiles(out, fileBuf)
	}
}

func writeSpinDetail(buf *strings.Builder, s *betOrderService, gameNum, step int, isFree bool, triggeringBaseRound int, roundWin float64) {
	if step == 1 {
		writeRoundHeader(buf, s, gameNum, isFree, triggeringBaseRound)
	} else {
		writeReelInfo(buf, s)
	}
	fprintf(buf, "Step%d 初始盘面:\n", step)
	writeGridToBuilder(buf, &s.symbolGrid, nil)
	if len(s.winInfos) > 0 {
		fprintf(buf, "Step%d 中奖标记:\n", step)
		writeGridToBuilder(buf, &s.symbolGrid, &s.winGrid)
	}
	if !s.isRoundOver {
		fprintf(buf, "Step%d 消除并下落后（ring 从滚轴补位前）:\n", step)
		writeGridToBuilder(buf, &s.nextSymbolGrid, nil)
	}
	writeStepSummary(buf, s, step, isFree, roundWin)
	fprintf(buf, "\n")
}

func writeReelInfo(buf *strings.Builder, s *betOrderService) {
	if s.scene == nil {
		fprintf(buf, "滚轴配置Index: 0\n转轮信息长度/起始：未初始化\n")
		return
	}
	fprintf(buf, "滚轴配置Index: %d\n转轮信息长度/起始：", s.scene.SymbolRoller[0].Real)
	for c := 0; c < len(s.scene.SymbolRoller); c++ {
		rc := s.scene.SymbolRoller[c]
		fprintf(buf, "%d[%d～%d]  ", rc.Len, rc.OriStart, rc.Fall)
	}
	fprintf(buf, "\n")
}

func writeRoundHeader(buf *strings.Builder, s *betOrderService, gameNum int, isFree bool, triggeringBaseRound int) {
	if isFree {
		trigger := "?"
		if triggeringBaseRound > 0 {
			trigger = fmt.Sprintf("%d", triggeringBaseRound)
		}
		fprintf(buf, "\n=============[基础模式] 第%s局 - 免费第%d局 =============\n", trigger, gameNum)
	} else {
		fprintf(buf, "\n=============[基础模式] 第%d局 =============\n", gameNum)
	}
	writeReelInfo(buf, s)
}

func writeStepSummary(buf *strings.Builder, s *betOrderService, step int, isFree bool, roundWin float64) {
	fprintf(buf, "Step%d 中奖详情:\n", step)
	treasureCount := s.getScatterCount()
	if len(s.winInfos) == 0 {
		fprintf(buf, "\t未中奖\n")
		if s.isRoundOver {
			//if s.scene.BonusState == _bonusStatePending {
			//	fprintf(buf, "\t💎  等待选档: BonusState=%d, s=%d\n", _bonusStatePending, s.scatterCount)
			//}
			if isFree && s.addFreeTime > 0 {
				fprintf(buf, "\t💎 当前盘面夺宝数量: %d\n", treasureCount)
			} else if !isFree && s.addFreeTime > 0 {
				fprintf(buf, "\t本手触发免费: 夺宝=%d, +免费次数=%d, 剩余免费=%d, NextStage=%d，stepmul=%d, enterfree 💎💎💎\n",
					s.scatterCount, s.addFreeTime, s.scene.FreeNum, s.scene.NextStage, s.stepMultiplier)
			}
		}
		return
	}
	for _, elem := range s.winInfos {
		fprintf(buf, "\t符号:%2d, 命中: %d, 赔率: %4.2f, 奖金: %4.2f\n",
			elem.Symbol, elem.SymbolCount, float64(elem.Odds), float64(elem.Odds))
	}
	isFreeMode := 0
	if s.isFreeRound {
		isFreeMode = 1
	}
	fprintf(buf, "\tMode=%d, Stage=%d, Steps=%d, stepMul=%d, lineMul=%d, 本回合累计 step 倍数: %.2f\n",
		isFreeMode, s.scene.Stage, s.scene.Steps, s.stepMultiplier, s.lineMultiplier, roundWin)
	if !s.isRoundOver {
		fprintf(buf, "\t连消继续 → 下一请求 Step%d\n", step+1)
		return
	}
	fprintf(buf, "\t连消结束（本回合无更多可消除）\n\n")
	if isFree {
		if treasureCount > 0 && s.addFreeTime > 0 {
			fprintf(buf, "\t免费内再触发: 夺宝=%d, +次数=%d, 剩余免费=%d\n", treasureCount, s.addFreeTime, s.scene.FreeNum)
		}
		if s.scene.FreeNum == 0 {
			fprintf(buf, "\t免费模式结束 — 本回合累计 step 倍数=%.2f\n", roundWin)
		} else {
			fprintf(buf, "\t免费模式继续 — 剩余次数=%d\n", s.scene.FreeNum)
		}
	} else if s.addFreeTime > 0 {
		fprintf(buf, "\t基础盘触发免费 — 夺宝=%d, 剩余免费=%d, NextStage=%d\n", treasureCount, s.scene.FreeNum, s.scene.NextStage)
	}
}

func saveRtpXDebugFiles(statsResult string, fileBuf *strings.Builder) {
	ts := time.Now().Format("20060102_150405")
	_ = os.MkdirAll("logs", 0755)
	detail := ""
	if fileBuf != nil {
		detail = fileBuf.String()
	}
	debugPath := fmt.Sprintf("logs/%s.txt", ts)
	_ = os.WriteFile(debugPath, []byte(statsResult+detail), 0644)
	fmt.Printf("\n调试详情已保存: %s\n", debugPath)
}

// rtpXDetailTriggerBase 详单日志用：免费链对应的基础局序号（与 writeSpinDetail / 原 fileBuf 分支一致）
func rtpXDetailTriggerBase(isFree, isFirstStep bool, triggeringBaseRound, baseGameCount int) int {
	if !isFree {
		return 0
	}
	if triggeringBaseRound > 0 {
		return triggeringBaseRound
	}
	if isFirstStep {
		return baseGameCount
	}
	return 0
}

func gridFingerprint(g *int64Grid) string {
	var b strings.Builder
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if b.Len() > 0 {
				b.WriteByte(',')
			}
			fprintf(&b, "%d", (*g)[r][c])
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
	elapsed := time.Since(start)
	speed := safeDiv(baseRounds, int64(elapsed.Seconds()))
	w := func(format string, args ...any) { fprintf(buf, format, args...) }
	w("\n运行局数: %d，用时: %v，速度: %.0f 局/秒\n", baseRounds, elapsed.Round(time.Second), speed)
	w("\n===== 详细统计汇总 =====\n")
	w("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	w("\n[基础模式统计]\n")
	w("基础模式总游戏局数: %d\n", baseRounds)
	w("基础模式总投注(倍数): %.2f\n", totalBet)
	w("基础模式总奖金: %.2f\n", baseTotalWin)
	w("基础模式RTP: %.4f%% (基础模式奖金/基础模式投注)\n", safeDiv(baseTotalWin*100, totalBet))
	w("基础模式免费局触发次数: %d\n", baseFreeTriggered)
	w("基础模式触发免费局比例: %.4f%%\n", safeDiv(baseFreeTriggered*100, baseRounds))
	w("基础模式平均每局免费次数: %.4f\n", safeDiv(freeRounds, baseRounds))
	w("基础模式中奖率: %.4f%%\n", safeDiv(baseWinRounds*100, baseRounds))
	w("基础模式平均连消步数: %.4f\n", safeDiv(baseCascadeSteps, baseRounds))
	w("基础模式最大连消步数: %d\n", baseMaxCascadeSteps)
	w("基础模式中奖局数: %d\n", baseWinRounds)

	w("\n[免费模式统计]\n")
	w("免费模式总游戏局数: %d\n", freeRounds)
	w("免费模式总奖金: %.4f\n", freeTotalWin)
	w("免费模式RTP: %.4f%% (免费模式奖金/基础模式投注，因为免费模式不投注)\n", safeDiv(freeTotalWin*100, totalBet))
	w("免费模式额外增加局数: %d\n", freeExtraFreeRounds)
	w("免费模式最大连续局数: %d\n", freeMaxFreeStreak)
	w("免费模式中奖局数: %d\n", freeWinRounds)
	w("免费模式中奖率: %.4f%%\n", safeDiv(freeWinRounds*100, freeRounds))
	w("免费模式出现夺宝的次数: %d (%.4f%%)\n", freeTreasureInFree, safeDiv(freeTreasureInFree*100, freeRounds))
	w("免费模式平均连消步数: %.4f\n", safeDiv(freeCascadeSteps, freeRounds))
	w("免费模式最大连消步数: %d\n", freeMaxCascadeSteps)
	totalWin := baseTotalWin + freeTotalWin
	w("\n[免费触发效率]\n")
	w("  总免费游戏次数: %d (真实的游戏局数，包含中途增加的免费次数)\n", freeRounds)
	w("  总触发次数: %d (基础模式触发免费游戏的次数)\n", baseFreeTriggered)
	w("  平均1次触发获得免费游戏: %.4f次 (总免费游戏次数 / 总触发次数)\n", safeDiv(freeRounds, baseFreeTriggered))

	w("\n[总计]\n")
	w("  总投注(倍数): %.2f (仅基础模式投注，免费模式不投注)\n", totalBet)
	w("  总奖金: %.2f (基础模式奖金 + 免费模式奖金)\n", totalWin)
	totalRTP := safeDiv(totalWin*100, totalBet)
	w("  总回报率(RTP): %.4f%% (总奖金/总投注 = %.2f/%.2f)\n", totalRTP, totalWin, totalBet)
	w("  基础贡献: %.4f%% | 免费贡献: %.4f%%\n", safeDiv(baseTotalWin*100, totalWin), safeDiv(freeTotalWin*100, totalWin))
	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}

func writeGridToBuilder(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
	if grid == nil {
		buf.WriteString("(空)\n")
		return
	}
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			symbol := (*grid)[r][c]
			isWin := winGrid != nil && (*winGrid)[r][c] != 0
			switch {
			case symbol == 0 && isWin:
				fprintf(buf, "   *|")
			case symbol == 0:
				fprintf(buf, "    |")
			case isWin:
				fprintf(buf, " %2d*|", symbol)
			default:
				fprintf(buf, " %2d |", symbol)
			}
		}
		buf.WriteByte('\n')
	}
}
