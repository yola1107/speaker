package ycj

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	testRounds       = 1e3
	progressInterval = 1e8
	debugFileOpen    = 10
)

func TestRtp2(t *testing.T) {
	stats := &benchmarkStats{}
	start := time.Now()
	buf := &strings.Builder{}
	svc := newBerService()
	svc.initGameConfigs()
	baseGameCount, freeRoundIdx := 0, 0
	interval := int64(min(testRounds, progressInterval))

	var fileBuf *strings.Builder
	if debugFileOpen > 0 {
		fileBuf = &strings.Builder{}
	}

	for stats.ChargeCount < testRounds {
		var gameNum int
		var roundWin, freeRoundWin float64
		var triggeringBaseRound int
		var extendStep, respinStep int
		isFirst := true

		for {
			if isFirst {
				roundWin = 0
				freeRoundWin = 0
				extendStep = 0
				respinStep = 0
			} else {
				if svc.scene.IsExtendMode {
					extendStep++
				}
				if svc.scene.IsRespinMode {
					respinStep++
				}
			}

			beforeFree := svc.isFreeRound
			beforeExtend := svc.scene.IsExtendMode
			beforeRespin := svc.scene.IsRespinMode
			if err := svc.baseSpin(); err != nil {
				t.Fatalf("baseSpin failed: %v", err)
			}

			// 统计推展模式（首次进入）
			if svc.scene.IsExtendMode && !beforeExtend {
				if beforeFree {
					stats.ExtendStepsInFree++
				} else {
					stats.ExtendStepsInBase++
				}
			}

			// 统计重转模式（仅免费模式，首次进入）
			if svc.scene.IsRespinMode && !beforeRespin && beforeFree {
				stats.RespinStepsInFree++
			}

			isFree := svc.isFreeRound

			// 更新游戏计数
			if isFirst {
				if isFree {
					freeRoundIdx++
					gameNum = freeRoundIdx
					if triggeringBaseRound == 0 {
						triggeringBaseRound = baseGameCount
					}
				} else {
					gameNum = baseGameCount + 1
				}
			}

			stepWin := float64(svc.stepMultiplier)
			if svc.scene.IsExtendMode {
				stats.ExtendTotalWin += stepWin
				if isFree {
					stats.ExtendWinInFree += stepWin
				} else {
					stats.ExtendWinInBase += stepWin
				}
			}
			if svc.scene.IsRespinMode {
				stats.RespinTotalWin += stepWin
				if isFree {
					stats.RespinWinInFree += stepWin
				}
			}
			roundWin += stepWin
			stats.TotalWin += stepWin

			// 统计奖金
			if isFree {
				stats.FreeWin += stepWin
				freeRoundWin += stepWin
				if svc.addFreeTime > 0 {
					stats.FreeTreasureInFree++
					stats.FreeExtraFreeRounds += svc.addFreeTime
				}
			} else {
				stats.BaseWin += stepWin
				if svc.addFreeTime > 0 {
					stats.FreeTriggerCount++
					stats.FreeTime++
					if isFirst {
						triggeringBaseRound = baseGameCount + 1
					}
				}
			}

			// 调试日志
			if debugFileOpen > 0 && fileBuf != nil {
				triggerRound := 0
				if isFree {
					triggerRound = triggeringBaseRound
					if triggerRound == 0 && isFirst {
						triggerRound = baseGameCount
					}
				}
				writeSpinDetail(fileBuf, svc, gameNum, isFree, triggerRound, stepWin, roundWin, extendStep, respinStep, isFirst)
			}

			// Round 结束处理
			if svc.isRoundOver {
				if isFree {
					stats.FreeRounds++
					if freeRoundWin > 0 {
						stats.FreeWinTimes++
					}
					freeRoundWin = 0
				} else {
					baseGameCount++
					stats.BaseRounds++
					if roundWin > 0 {
						stats.BaseWinTimes++
					}
					stats.TotalBet += float64(_baseMultiplier)
					stats.ChargeCount++
				}
				roundWin = 0

				if svc.scene.FreeNum <= 0 {
					resetBetServiceForNextRound(svc)
					freeRoundIdx = 0
					if stats.ChargeCount > 0 && stats.ChargeCount%interval == 0 {
						printBenchmarkProgress(buf, stats, start)
						fmt.Print(buf.String())
					}
					break
				}
			} else if svc.addFreeTime > 0 && !isFree {
				baseGameCount++
				stats.BaseRounds++
				stats.TotalBet += float64(_baseMultiplier)
				stats.ChargeCount++
				if roundWin > 0 {
					stats.BaseWinTimes++
				}
			}

			if svc.isRoundOver {
				isFirst = true
			} else if svc.addFreeTime > 0 {
				isFirst = true
			} else {
				isFirst = false
			}
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

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, gameNum int, isFree bool, triggeringBaseRound int, stepWin, roundWin float64, extendStep, respinStep int, isFirst bool) {
	if isFree {
		trigger := "?"
		if triggeringBaseRound > 0 {
			trigger = fmt.Sprintf("%d", triggeringBaseRound)
		}
		fprintf(buf, "\n=============[基础模式] 第%s局 - 免费第%d局 =============\n", trigger, gameNum)
	} else {
		modeStr := ""
		if svc.scene.IsExtendMode {
			modeStr = fmt.Sprintf(" (推展模式 Step%d)", extendStep+1)
		}
		fprintf(buf, "\n=============[基础模式] 第%d局%s =============\n", gameNum, modeStr)
	}

	writeReelInfo(buf, svc.scene.SymbolRoller, svc)
	fprintf(buf, "Step1 初始盘面:\n")
	writeGridToBuilder(buf, &svc.symbolGrid, &svc.winGrid)

	fprintf(buf, "Step1 中奖详情:\n")
	if svc.stepMultiplier > 0 {
		fprintf(buf, "\t中奖倍数: %.2f\n", svc.stepMultiplier)
	} else {
		fprintf(buf, "\t未中奖\n")
	}

	stepMul := svc.stepMultiplier
	fprintf(buf, "\tMode=%d, RoundMul: %.2f, 累计中奖: %.2f\n", btoi(isFree), stepMul, roundWin)

	modeDesc := ""
	switch {
	case svc.scene.IsExtendMode:
		modeDesc = "| 💎 推展模式"
	case svc.scene.IsRespinMode:
		modeDesc = "| 💎 重转模式"
	}

	model := btoi(isFree)
	if svc.addFreeTime > 0 {
		if !isFree {
			fprintf(buf, "\t🚨🚨🚨 触发免费: +%d 次 | 当前剩余免费=%d %s\n", svc.addFreeTime, svc.scene.FreeNum, modeDesc)
		} else {
			fprintf(buf, "\tMode=%d 触发免费: +%d 次 | 当前剩余免费=%d, %s\n", model, svc.addFreeTime, svc.scene.FreeNum, modeDesc)
		}
	} else {
		fprintf(buf, "\tMode=%d | nextStage=%d | IsExtendMode=%v | IsRespinMode=%v | FreeNum=%d %s\n", model, svc.scene.NextStage, svc.scene.IsExtendMode, svc.scene.IsRespinMode, svc.scene.FreeNum, modeDesc)
	}

	fprintf(buf, "\n")
}

func writeReelInfo(buf *strings.Builder, rollers [_colCount]SymbolRoller, svc *betOrderService) {
	if len(rollers) == 0 {
		fprintf(buf, "滚轴配置Index: 0\n转轮信息长度/起始：未初始化\n")
		return
	}
	realIndex := rollers[0].Real
	fprintf(buf, "滚轴配置Index: %d\n", realIndex)
	fprintf(buf, "转轮信息长度/起始：")
	for c := 0; c < len(rollers); c++ {
		roller := rollers[c]
		reelLen := 0
		if svc.gameConfig != nil && roller.Real >= 0 && roller.Real < len(svc.gameConfig.RealData) &&
			roller.Col >= 0 && roller.Col < len(svc.gameConfig.RealData[roller.Real]) {
			reelLen = len(svc.gameConfig.RealData[roller.Real][roller.Col])
		}
		fprintf(buf, "%d[%d～%d]  ", reelLen, roller.Start, roller.Fall)
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
