package sbyymx2

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// 短跑诊断：局数、进度间隔、是否写详细盘面日志到 logs/（与 game/hcsqy/rtpx_test.go 一致）
const (
	testRounds       = int64(1e2)
	progressInterval = int64(1e7)
	debugFileOpen    = 10
)

func TestRtp2(t *testing.T) {
	var baseRounds, baseWinRounds int64
	var baseTotalWin float64
	totalBet := 0.0
	start := time.Now()
	buf := &strings.Builder{}
	svc := newBerService()
	interval := int64(min(testRounds, progressInterval))

	var fileBuf *strings.Builder
	if debugFileOpen > 0 {
		fileBuf = &strings.Builder{}
	}

	for baseRounds < testRounds {
		if err := svc.baseSpin(); err != nil {
			t.Fatal(err)
		}
		stepWin := float64(svc.stepMultiplier)
		roundWin := stepWin
		gameNum := int(baseRounds) + 1

		baseTotalWin += stepWin

		if debugFileOpen > 0 && fileBuf != nil {
			writeSpinDetailSbyymx2(fileBuf, svc, gameNum, stepWin, roundWin)
		}

		baseRounds++
		if roundWin > 0 {
			baseWinRounds++
		}
		totalBet += float64(_baseMultiplier)

		if baseRounds%interval == 0 {
			totalWin := baseTotalWin
			printRtpProgressHcsqyStyle(buf, baseRounds, totalBet, baseTotalWin, totalWin, baseWinRounds, start)
			fmt.Print(buf.String())
		}
	}

	printFinalStatsSbyymx2(buf, baseRounds, baseTotalWin, baseWinRounds, 0, 0, 0, 0, 0, 0, totalBet, start)
	result := buf.String()
	fmt.Print(result)
	if debugFileOpen > 0 && fileBuf != nil {
		saveDebugFileSbyymx2(result, fileBuf.String(), start)
	}
}

func writeSpinDetailSbyymx2(buf *strings.Builder, svc *betOrderService, gameNum int, stepWin, roundWin float64) {
	fprintf(buf, "\n=============[基础模式] 第%d局 =============\n", gameNum)
	fprintf(buf, "Step1 初始盘面:\n")
	writeGridToBuilderSbyymx2(buf, &svc.symbolGrid, &svc.winGrid)

	fprintf(buf, "Step1 中奖详情:\n")
	if len(svc.winInfos) == 0 {
		fprintf(buf, "\t未中奖\n")
	} else {
		for _, wr := range svc.winResults {
			fprintf(buf, "\t符号:%2d, 线倍数(含百搭): %d\n", wr.Symbol, wr.Multiplier)
		}
	}

	left := svc.symbolGrid[1][0]
	mid := svc.symbolGrid[1][1]
	right := svc.symbolGrid[1][2]
	fprintf(buf, "\t中间行 L|C|R: %d | %d | %d\n", left, mid, right)
	fprintf(buf, "\tlineMul: %d, stepMul: %d, 累计中奖(本局): %.2f, 本步倍数: %.2f\n",
		svc.lineMultiplier, svc.stepMultiplier, roundWin, stepWin)
	if svc.lineMultiplier != 0 && svc.lineMultiplier != svc.stepMultiplier {
		fprintf(buf, "\t      (! lineMul 与 stepMul 不一致，请查 processWinInfos)\n")
	}
	fprintf(buf, "\t（本游戏无 Scatter/免费；仅中间行判奖）\n")
	fprintf(buf, "\n")
}

func writeGridToBuilderSbyymx2(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
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

func saveDebugFileSbyymx2(statsResult, detailResult string, _ time.Time) {
	_ = os.MkdirAll("logs", 0o755)
	filename := fmt.Sprintf("logs/%s.txt", time.Now().Format("20060102_150405"))
	_ = os.WriteFile(filename, []byte(statsResult+detailResult), 0o644)
	fmt.Printf("\n调试信息已保存到: %s\n", filename)
}

func printFinalStatsSbyymx2(buf *strings.Builder, baseRounds int64, baseTotalWin float64, baseWinRounds int64,
	baseFreeTriggered int64, freeRounds int64, freeTotalWin float64,
	freeWinRounds int64, freeTreasureInFree int64, freeExtraFreeRounds int64, totalBet float64, start time.Time) {
	w := func(format string, args ...interface{}) { fprintf(buf, format, args...) }
	elapsed := time.Since(start)
	speed := safeDiv(baseRounds, int64(elapsed.Seconds()))
	w("\n运行局数: %d，用时: %v，速度: %.0f 局/秒\n", baseRounds, elapsed.Round(time.Second), speed)

	w("\n===== 详细统计汇总 =====\n")
	w("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	baseRTP := safeDiv(int64(baseTotalWin)*100, int64(totalBet))
	freeRTP := safeDiv(int64(freeTotalWin)*100, int64(totalBet))
	totalWin := baseTotalWin + freeTotalWin
	totalRTP := safeDiv(int64(totalWin)*100, int64(totalBet))
	baseWinRate := safeDiv(baseWinRounds*100, baseRounds)
	freeWinRate := safeDiv(freeWinRounds*100, max(freeRounds, 1))
	freeTriggerRate := safeDiv(baseFreeTriggered*100, baseRounds)
	avgFreePerTrigger := safeDiv(freeRounds, baseFreeTriggered)
	baseContrib := safeDivFloat(baseTotalWin*100, totalWin)
	freeContrib := safeDivFloat(freeTotalWin*100, totalWin)

	w("\n[基础模式统计]\n")
	w("基础模式总游戏局数: %d\n", baseRounds)
	w("基础模式总投注(倍数): %.2f\n", totalBet)
	w("基础模式总奖金: %.2f\n", baseTotalWin)
	w("基础模式RTP: %.2f%% (基础模式奖金/基础模式投注)\n", baseRTP)
	w("基础模式免费局触发次数: %d\n", baseFreeTriggered)
	w("基础模式触发免费局比例: %.2f%%\n", freeTriggerRate)
	w("基础模式中奖率: %.2f%%\n", baseWinRate)
	w("基础模式中奖局数: %d\n", baseWinRounds)

	w("\n[免费模式统计]（sbyymx2 无免费，下列为 0）\n")
	w("免费模式总游戏局数: %d\n", freeRounds)
	w("免费模式总奖金: %.2f\n", freeTotalWin)
	w("免费模式RTP: %.2f%% (免费模式奖金/基础模式投注，因为免费模式不投注)\n", freeRTP)
	w("免费模式中奖率: %.2f%%\n", freeWinRate)
	w("免费模式中奖局数: %d\n", freeWinRounds)
	w("免费模式额外增加局数: %d\n", freeExtraFreeRounds)
	w("免费模式出现夺宝的次数: %d (%.2f%%)\n", freeTreasureInFree, safeDiv(freeTreasureInFree*100, max(freeRounds, 1)))

	w("\n[免费触发效率]\n")
	w("  总免费游戏次数: %d (真实的游戏局数，包含中途增加的免费次数)\n", freeRounds)
	w("  总触发次数: %d (基础模式触发免费游戏的次数)\n", baseFreeTriggered)
	w("  平均1次触发获得免费游戏: %.2f次 (总免费游戏次数 / 总触发次数)\n", avgFreePerTrigger)

	w("\n[总计]\n")
	w("  总投注(倍数): %.2f (仅基础模式投注，免费模式不投注)\n", totalBet)
	w("  总奖金: %.2f (基础模式奖金 + 免费模式奖金)\n", totalWin)
	w("  总回报率(RTP): %.2f%% (总奖金/总投注 = %.2f/%.2f)\n", totalRTP, totalWin, totalBet)
	w("  基础贡献: %.2f%% | 免费贡献: %.2f%%\n", baseContrib, freeContrib)

	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}

func safeDivFloat(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}
