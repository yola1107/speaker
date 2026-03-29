package sbyymx2

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
	// debugFileOpen>0 时写 logs 下详细每步日志
	debugFileOpen = 10
)

func TestRtp2(t *testing.T) {
	stats := &benchmarkStats{}

	start := time.Now()
	buf := &strings.Builder{}
	svc := newBerService()
	svc.initGameConfigs()
	gameCount := 0
	interval := int64(min(testRounds, progressInterval))

	var fileBuf *strings.Builder
	if debugFileOpen > 0 {
		fileBuf = &strings.Builder{}
	}

	for stats.TotalRounds < testRounds {
		var roundWin float64
		var respinStep int // 重转至赢步数
		isFirst := true    // 每个回合只有第一次 spin 为 true

		for {
			if isFirst {
				roundWin = 0
				respinStep = 0
			} else {
				respinStep++ // 重转至赢步数递增
			}

			beforeRespin := svc.scene.IsRespinMode
			if err := svc.baseSpin(); err != nil {
				t.Fatalf("baseSpin failed: %v", err)
			}
			didRespin := svc.respinWildCol >= 0
			if didRespin {
				stats.RespinSteps++
				if !beforeRespin {
					stats.ResChainStarts++
				}
			}

			// 更新游戏计数（只有第一次 spin 才计数）
			if isFirst {
				gameCount++
			}

			stepWin := float64(svc.stepMultiplier)
			if didRespin {
				stats.RespinTotalWin += stepWin
			}
			roundWin += stepWin

			// 统计奖金
			stats.TotalWin += stepWin

			// 调试日志
			if debugFileOpen > 0 && fileBuf != nil {
				writeSpinDetail(fileBuf, svc, gameCount, stepWin, roundWin, respinStep)
			}

			// Round 结束处理
			if svc.isRoundOver {
				stats.TotalRounds++
				stats.TotalBet += float64(_baseMultiplier)
				if roundWin > 0 {
					stats.WinTimes++
				}
				roundWin = 0

				resetBetServiceForNextRound(svc)
				if stats.TotalRounds%interval == 0 {
					printBenchmarkProgress(buf, stats, start)
					fmt.Print(buf.String())
				}
				break
			}

			// 更新 isFirst：
			// - 如果回合结束（isRoundOver=true），下一局是新回合的第一局
			// - 如果必赢重转（isRoundOver=false），下一局是重转，不是第一局
			isFirst = false
		}
	}

	buf.Reset()
	printBenchmarkSummary(buf, stats, start)

	result := buf.String()
	fmt.Print(result)
	if debugFileOpen > 0 && fileBuf != nil {
		saveDebugFile(result, fileBuf.String(), start)
	}
}

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, gameNum int, stepWin, roundWin float64, respinStep int) {
	if respinStep > 0 {
		fprintf(buf, "\n=============[基础模式] 第%d局 (重转至赢 Step%d) =============\n", gameNum, respinStep+1)
	} else {
		fprintf(buf, "\n=============[基础模式] 第%d局 =============\n", gameNum)
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
	fprintf(buf, "\tRoundMul: %d, lineMul: %d, wildMul: %d, 累计中奖: %.2f\n", stepMul, lineMul, wildMul, roundWin)

	if wildMul != 0 && lineMul*wildMul != stepMul {
		fprintf(buf, "\t      (! stepMul 应等于 lineMul×wildMul，请查 processWinInfos)\n")
	}

	longwild := ""
	switch {
	case svc.respinWildCol >= 0:
		longwild = "💎重转至赢"
	case svc.wildExpandCol >= 0:
		longwild = "💎百搭变大"
	}

	isRespin := 0
	if svc.scene.IsRespinMode {
		isRespin = 1
	}

	fprintf(buf, "\tIsRespin=%v | index=%d %s\n", isRespin, svc.debug.mode, longwild)
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
