package xslm3

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"egame-grpc/global"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	testRounds       = 1e5
	progressInterval = 1e6
	debugFileOpen    = 10
	freeModeLogOnly  = 0
)

var stateNames = []string{"base", "000", "001", "010", "011", "100", "101", "110", "111", "008"}

func init() {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	cfg.DisableStacktrace = true
	cfg.EncoderConfig.EncodeCaller = zapcore.FullCallerEncoder
	logger, _ := cfg.Build()
	global.GVA_LOG = logger
}

func TestRtp(t *testing.T) {
	// 基础模式统计
	var baseRounds int64
	var baseWinRounds int64
	var baseCascadeSteps int64
	var baseFreeTriggered int64
	var baseTotalWin float64
	var baseFemaleSymbolWin float64
	var baseFemaleWildWin float64
	var baseMaxCascadeSteps int

	// 免费模式统计
	var freeRounds int64
	var freeWinRounds int64
	var freeCascadeSteps int64
	var freeFullElimination int64
	var freeTreasureInFree int64
	var freeExtraFreeRounds int64
	var freeMaxFreeStreak int64
	var freeTotalWin float64
	var freeFemaleSymbolWin float64
	var freeFemaleWildWin float64
	var freeMaxCascadeSteps int
	var freeFemaleStateCount [10]int64

	totalBet := 0.0
	start := time.Now()
	buf := &strings.Builder{}

	svc := newBerService()
	baseGameCount, freeRoundIdx := 0, 0

	var fileBuf *strings.Builder
	if debugFileOpen > 0 {
		fileBuf = &strings.Builder{}
	}

	for baseRounds < testRounds {
		// 根据 FreeNum 判断是否是免费模式（与 xslm2 保持一致）
		isFree := svc.scene.FreeNum > 0
		svc.isFreeRound = isFree

		var (
			cascadeCount            int
			roundWin                float64
			roundStartFemaleCounts  [3]int64
			roundHasFullElimination bool
			gameNum                 int
		)

		for {
			isFirst := cascadeCount == 0

			// 使用 scene.FreeNum 判断 isFree（与用户要求一致）
			isFree = svc.scene.FreeNum > 0
			svc.isFreeRound = isFree

			if isFirst {
				roundStartFemaleCounts = svc.scene.FemaleCountsForFree
				svc.femaleCountsForFree = roundStartFemaleCounts
				svc.nextFemaleCountsForFree = roundStartFemaleCounts
				roundWin = 0
				roundHasFullElimination = false
				// 显式重置状态标志字段，提高代码可读性
				svc.hasFemaleWin = false
				svc.hasFemaleWildWin = false
				svc.enableFullElimination = false
				svc.betAmount = decimal.NewFromInt(_baseMultiplier)
				svc.client.ClientOfFreeGame.SetBetAmount(svc.betAmount.Round(2).InexactFloat64())
				if isFree {
					freeRoundIdx++
					gameNum = freeRoundIdx
					if len(svc.scene.SymbolRoller) > 0 {
						if stateKey := svc.scene.SymbolRoller[0].Real; stateKey >= 0 && stateKey < 10 {
							freeFemaleStateCount[stateKey]++
						}
					}
				} else {
					baseGameCount++
					gameNum = baseGameCount
				}
			} else {
				svc.femaleCountsForFree = svc.nextFemaleCountsForFree
				svc.betAmount = decimal.NewFromFloat(svc.client.ClientOfFreeGame.GetBetAmount())
			}

			stepStartFemaleCounts := svc.femaleCountsForFree
			if err := svc.baseSpin(); err != nil {
				panic(err)
			}

			// baseSpin() 内部会通过 handleStageTransition() 设置 isFreeRound
			isFree = svc.isFreeRound

			cascadeCount++
			stepWin := svc.stepMultiplier
			roundWin += float64(stepWin)

			if isFree {
				if remainingFree := svc.scene.FreeNum; remainingFree > freeMaxFreeStreak {
					freeMaxFreeStreak = remainingFree
				}
				if svc.enableFullElimination && svc.hasFemaleWildWin {
					roundHasFullElimination = true
				}
			}

			if debugFileOpen > 0 && fileBuf != nil && (freeModeLogOnly == 0 || isFree) {
				triggerRound := 0
				if isFree {
					triggerRound = baseGameCount
				}
				writeSpinDetail(fileBuf, svc, gameNum, cascadeCount, isFree, triggerRound, stepStartFemaleCounts, roundStartFemaleCounts, float64(stepWin), roundWin)
			}

			if isFree {
				freeTotalWin += float64(stepWin)
				updateWinStats(svc.winResults, &freeFemaleSymbolWin, &freeFemaleWildWin)
			} else {
				baseTotalWin += float64(stepWin)
				updateWinStats(svc.winResults, &baseFemaleSymbolWin, &baseFemaleWildWin)
			}

			if svc.isRoundOver {
				if isFree {
					freeCascadeSteps += int64(cascadeCount)
					if cascadeCount > freeMaxCascadeSteps {
						freeMaxCascadeSteps = cascadeCount
					}
					if svc.newFreeRoundCount > 0 {
						freeTreasureInFree++
						freeExtraFreeRounds += svc.newFreeRoundCount
					}
					if roundHasFullElimination {
						freeFullElimination++
					}
				} else {
					baseCascadeSteps += int64(cascadeCount)
					if cascadeCount > baseMaxCascadeSteps {
						baseMaxCascadeSteps = cascadeCount
					}
				}
				break
			}
		}

		if isFree {
			freeRounds++
			if roundWin > 0 {
				freeWinRounds++
			}
			if svc.scene.FreeNum == 0 {
				// 不要手动清零，让游戏逻辑自己处理
				// svc.scene.FemaleCountsForFree 会在游戏逻辑中自动清零
				freeRoundIdx = 0
			}
		} else {
			baseRounds++
			if roundWin > 0 {
				baseWinRounds++
			}
			totalBet += float64(_baseMultiplier)
			if svc.newFreeRoundCount > 0 {
				baseFreeTriggered++
			}
		}

		interval := progressInterval
		if testRounds < progressInterval {
			interval = testRounds
		}
		if baseRounds%int64(interval) == 0 {
			printProgress(buf, baseRounds, totalBet, baseTotalWin, freeTotalWin, time.Since(start), baseWinRounds, freeWinRounds, baseFreeTriggered, freeRounds)
			fmt.Print(buf.String())
		}
	}

	printFinalStats(buf, baseRounds, baseTotalWin, baseWinRounds, baseFemaleSymbolWin, baseFemaleWildWin, baseCascadeSteps, baseMaxCascadeSteps, baseFreeTriggered,
		freeRounds, freeTotalWin, freeWinRounds, freeFemaleSymbolWin, freeFemaleWildWin, freeCascadeSteps, freeMaxCascadeSteps, freeFullElimination, freeTreasureInFree, freeExtraFreeRounds, freeMaxFreeStreak, freeFemaleStateCount,
		totalBet, start)
	result := buf.String()
	fmt.Print(result)
	if debugFileOpen > 0 && fileBuf != nil {
		saveDebugFile(result, fileBuf.String(), start)
	}
}

func updateWinStats(winResults []*winResult, femaleSymbolWin, femaleWildWin *float64) {
	for _, wr := range winResults {
		gain := float64(wr.TotalMultiplier)
		if wr.Symbol >= _femaleA && wr.Symbol <= _femaleC {
			*femaleSymbolWin += gain
		} else if wr.Symbol >= _wildFemaleA && wr.Symbol <= _wildFemaleC {
			*femaleWildWin += gain
		}
	}
}

func (s *betOrderService) GetReelLength(realIdx, col int) int {
	if s.gameConfig == nil || realIdx < 0 || realIdx >= len(s.gameConfig.RealData) {
		return 0
	}
	if col < 0 || col >= len(s.gameConfig.RealData[realIdx]) {
		return 0
	}
	return len(s.gameConfig.RealData[realIdx][col])
}

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, gameNum, step int, isFree bool, triggeringBaseRound int, stepStartFemaleCounts [3]int64, roundStartFemaleCounts [3]int64, stepWin float64, roundWin float64) {
	if step == 1 {
		writeRoundHeader(buf, svc, gameNum, isFree, triggeringBaseRound, stepStartFemaleCounts, 0)
	} else {
		// 每个 step 前都打印转轮坐标信息
		writeReelInfo(buf, svc)
	}
	// 在初始盘面之前打印当前女性收集状态（这一步开始时的状态）
	if isFree {
		buf.WriteString(fmt.Sprintf("女性收集状态: %v\n", stepStartFemaleCounts))
	}
	buf.WriteString(fmt.Sprintf("Step%d 初始盘面:\n", step))
	printGridToBuf(buf, svc.symbolGrid, nil)
	if len(svc.winResults) > 0 {
		buf.WriteString(fmt.Sprintf("Step%d 中奖标记:\n", step))
		printGridToBuf(buf, svc.symbolGrid, svc.winGrid)
	}
	if !svc.isRoundOver && svc.nextSymbolGrid != nil {
		buf.WriteString(fmt.Sprintf("Step%d 下一盘面预览（实际消除+下落+填充结果）:\n", step))
		printGridToBuf(buf, svc.nextSymbolGrid, nil)
	}
	writeStepSummary(buf, svc, step, isFree, stepStartFemaleCounts, roundStartFemaleCounts, stepWin, roundWin)
	buf.WriteString("\n")
}

// writeReelInfo 打印转轮坐标信息
func writeReelInfo(buf *strings.Builder, svc *betOrderService) {
	//buf.WriteString("────────────────────────────────────────────────────────\n")
	//buf.WriteString("[转轮坐标信息]\n")
	if svc.scene != nil && len(svc.scene.SymbolRoller) >= int(_colCount) {
		buf.WriteString(fmt.Sprintf("滚轴配置Index: %d\n", svc.scene.SymbolRoller[0].Real))
		buf.WriteString("转轮信息长度/起始：")
		for c := int64(0); c < _colCount; c++ {
			if c > 0 {
				buf.WriteString("， ")
			}
			length := svc.GetReelLength(svc.scene.SymbolRoller[c].Real, int(c))
			start := svc.scene.SymbolRoller[c].Start
			fall := svc.scene.SymbolRoller[c].Fall
			if length > 0 {
				buf.WriteString(fmt.Sprintf("%d[%d～%d]", length, start, fall))
			} else {
				buf.WriteString("0[0～0]")
			}
		}
		buf.WriteString("\n")
	} else {
		buf.WriteString("滚轴配置Index: 0\n转轮信息长度/起始：未初始化\n")
	}
}

func writeRoundHeader(buf *strings.Builder, svc *betOrderService, gameNum int, isFree bool, triggeringBaseRound int, femaleStart [3]int64, _ int64) {
	if isFree {
		buf.WriteString(fmt.Sprintf("\n=============[基础模式] 第%d局 - 免费第%d局 =============\n", triggeringBaseRound, gameNum))
	} else {
		buf.WriteString(fmt.Sprintf("\n=============[基础模式] 第%d局 =============\n", gameNum))
	}
	writeReelInfo(buf, svc)
	if isFree {
		if svc.enableFullElimination {
			buf.WriteString("🎯 全屏消除模式已激活（三种女性符号均>=10）\n")
		}
	}
}

// getTreasureCountFromGrid 从指定的 grid 中计算夺宝数量
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

func writeStepSummary(buf *strings.Builder, svc *betOrderService, step int, isFree bool, stepStartFemaleCounts [3]int64, roundStartFemaleCounts [3]int64, stepWin float64, roundWin float64) {
	buf.WriteString(fmt.Sprintf("Step%d 中奖详情:\n", step))
	// 动态获取夺宝数量：
	// - 如果 isRoundOver 且有 nextSymbolGrid，使用 nextSymbolGrid（消除后的最终盘面）
	// - 否则使用 symbolGrid（当前盘面）
	var finalTreasureCount int64
	if svc.isRoundOver && svc.nextSymbolGrid != nil {
		finalTreasureCount = getTreasureCountFromGrid(svc.nextSymbolGrid)
	} else {
		finalTreasureCount = svc.getTreasureCount()
	}
	// 对于"有夺宝"的显示，使用当前盘面（消除前）的夺宝数量
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
		!isFree, svc.hasFemaleWin, svc.hasFemaleWildWin,
		svc.hasWildSymbol(), svc.enableFullElimination,
		currentTreasureCount > 0, currentTreasureCount))

	// 使用 svc 的真实数据
	// 基础模式不应该收集女性符号，所以只显示免费模式的女性收集信息
	if isFree {
		final := svc.nextFemaleCountsForFree
		stepDelta := [3]int64{final[0] - stepStartFemaleCounts[0], final[1] - stepStartFemaleCounts[1], final[2] - stepStartFemaleCounts[2]}
		buf.WriteString(fmt.Sprintf("\t女性收集: 上一步=%v → 当前=%v (本步=%v)\n",
			stepStartFemaleCounts, final, stepDelta))
	} else {
		// 基础模式下，验证女性符号计数应该始终为 [0 0 0]
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
		extra := ""
		//if isFree && svc.newFreeRoundCount > 0 {
		//	extra = fmt.Sprintf(" | 新增免费次数=%d ⭐", svc.newFreeRoundCount)
		//}
		buf.WriteString(fmt.Sprintf("\t🛑 连消结束（%s）%s\n\n", stopReason, extra))
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

func printGridToBuf(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
	if grid == nil {
		buf.WriteString("(空)\n")
		return
	}
	rGrid := reverseGridRows(grid)
	rWinGrid := reverseGridRows(winGrid)
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbol := rGrid[r][c]
			isWin := rWinGrid[r][c] != _blank && rWinGrid[r][c] != _blocked
			if isWin {
				if symbol == _blank {
					buf.WriteString("   *|")
				} else {
					fmt.Fprintf(buf, " %2d*|", symbol)
				}
			} else {
				if symbol == _blank {
					buf.WriteString("    |")
				} else {
					fmt.Fprintf(buf, " %2d |", symbol)
				}
			}
			if c < _colCount-1 {
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

func printProgress(buf *strings.Builder, rounds int64, totalBet, baseWin, freeWin float64, elapsed time.Duration, baseWinRounds, freeWinRounds, baseFreeTriggered, freeRounds int64) {
	if totalBet <= 0 {
		return
	}
	buf.Reset()
	baseRtp := calculateRtp(int64(baseWin), rounds, _baseMultiplier)
	baseWinRate := calculateRtp(baseWinRounds, rounds, 1)
	freeRtp := calculateRtp(int64(freeWin), rounds, _baseMultiplier)
	freeWinRateDenominator := freeRounds
	if freeWinRateDenominator == 0 {
		freeWinRateDenominator = 1
	}
	freeWinRate := calculateRtp(freeWinRounds, freeWinRateDenominator, 1)
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
	freeRounds int64, freeTotalWin float64, freeWinRounds int64, freeFemaleSymbolWin float64, freeFemaleWildWin float64, freeCascadeSteps int64, freeMaxCascadeSteps int, freeFullElimination int64, freeTreasureInFree int64, freeExtraFreeRounds int64, freeMaxFreeStreak int64, freeFemaleStateCount [10]int64,
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
	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	/*	// =================== 数据验证 ===================
		w("\n===== 数据验证 =====\n")

		// 验证1: 总奖金 = 基础奖金 + 免费奖金
		if abs(baseTotalWin+freeTotalWin-totalWin) > 0.01 {
			w("❌ 警告: 总奖金(%.2f) != 基础奖金(%.2f) + 免费奖金(%.2f)，差值: %.2f\n",
				totalWin, baseTotalWin, freeTotalWin, totalWin-(baseTotalWin+freeTotalWin))
		} else {
			w("✅ 验证通过: 总奖金 = 基础奖金 + 免费奖金 (%.2f = %.2f + %.2f)\n",
				totalWin, baseTotalWin, freeTotalWin)
		}

		// 验证2: 基础局数应该等于测试局数（每局都有基础模式）
		testRoundsInt := int64(testRounds)
		if baseRounds != testRoundsInt {
			w("⚠️  注意: 基础局数(%d) != 测试局数(%d)，差值: %d\n",
				baseRounds, testRoundsInt, baseRounds-testRoundsInt)
			w("   说明: 可能存在连续免费模式的情况（基础模式触发免费后，免费模式又触发免费）\n")
		} else {
			w("✅ 验证通过: 基础局数 = 测试局数 (%d = %d)\n", baseRounds, testRoundsInt)
		}

		// 验证3: 免费触发次数应该 <= 基础局数
		if baseFreeTriggered > baseRounds {
			w("❌ 警告: 免费触发次数(%d) > 基础局数(%d)，这是不可能的\n",
				baseFreeTriggered, baseRounds)
		} else if baseFreeTriggered == baseRounds {
			w("✅ 验证通过: 免费触发次数 = 基础局数 (%d = %d)，说明每局基础模式都触发了免费\n",
				baseFreeTriggered, baseRounds)
		} else {
			w("✅ 验证通过: 免费触发次数(%d) <= 基础局数(%d)，触发率: %.2f%%\n",
				baseFreeTriggered, baseRounds, float64(baseFreeTriggered)*100/float64(baseRounds))
		}

		// 验证4: 免费局数应该 >= 免费触发次数（因为一次触发可能产生多局免费）
		if freeRounds < baseFreeTriggered {
			w("❌ 警告: 免费局数(%d) < 免费触发次数(%d)，这是不可能的\n",
				freeRounds, baseFreeTriggered)
		} else if freeRounds == baseFreeTriggered {
			w("✅ 验证通过: 免费局数 = 免费触发次数 (%d = %d)，说明每次触发只产生1局免费\n",
				freeRounds, baseFreeTriggered)
		} else {
			w("✅ 验证通过: 免费局数(%d) >= 免费触发次数(%d)，平均每次触发产生 %.2f 局免费\n",
				freeRounds, baseFreeTriggered, float64(freeRounds)/float64(baseFreeTriggered))
		}

		// 验证5: 免费模式中奖局数应该 <= 免费局数
		if freeWinRounds > freeRounds {
			w("❌ 警告: 免费模式中奖局数(%d) > 免费局数(%d)，这是不可能的\n",
				freeWinRounds, freeRounds)
		} else {
			w("✅ 验证通过: 免费模式中奖局数(%d) <= 免费局数(%d)\n",
				freeWinRounds, freeRounds)
		}

		// 验证6: 基础模式中奖局数应该 <= 基础局数
		if baseWinRounds > baseRounds {
			w("❌ 警告: 基础模式中奖局数(%d) > 基础局数(%d)，这是不可能的\n",
				baseWinRounds, baseRounds)
		} else {
			w("✅ 验证通过: 基础模式中奖局数(%d) <= 基础局数(%d)\n",
				baseWinRounds, baseRounds)
		}

		// 验证7: 女性符号状态统计总数应该等于免费局数
		// 注意：如果某些免费局开始时 SymbolRoller 为空或 stateKey 无效，可能不会统计状态
		totalStateCount := int64(0)
		for i := 0; i < 10; i++ {
			totalStateCount += freeFemaleStateCount[i]
		}
		if totalStateCount != freeRounds {
			w("⚠️  注意: 女性符号状态统计总数(%d) != 免费局数(%d)，差值: %d\n",
				totalStateCount, freeRounds, totalStateCount-freeRounds)
			w("   说明: 可能存在某些免费局开始时 SymbolRoller 为空或 stateKey 无效的情况\n")
			w("   这是正常的，因为状态统计需要 SymbolRoller 已初始化\n")
		} else {
			w("✅ 验证通过: 女性符号状态统计总数 = 免费局数 (%d = %d)\n",
				totalStateCount, freeRounds)
		}

		// 验证8: RTP 计算验证
		if totalBet > 0 {
			calculatedRtp := totalWin * 100 / totalBet
			manualRtp := (baseTotalWin + freeTotalWin) * 100 / totalBet
			if abs(calculatedRtp-manualRtp) > 0.01 {
				w("❌ 警告: RTP 计算不一致，calculatedRtp=%.4f%%, manual=%.4f%%\n",
					calculatedRtp, manualRtp)
			} else {
				w("✅ 验证通过: RTP 计算一致 (%.4f%%)\n", calculatedRtp)
			}
		}

		// 验证9: 免费模式额外增加局数应该 <= 免费局数
		if freeExtraFreeRounds > freeRounds {
			w("❌ 警告: 免费模式额外增加局数(%d) > 免费局数(%d)，这是不可能的\n",
				freeExtraFreeRounds, freeRounds)
		} else {
			w("✅ 验证通过: 免费模式额外增加局数(%d) <= 免费局数(%d)\n",
				freeExtraFreeRounds, freeRounds)
		}
	*/
	w("\n")
}

// abs 返回浮点数的绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
