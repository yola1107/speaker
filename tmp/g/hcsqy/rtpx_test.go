package hcsqy

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	testRounds       = 1e8
	progressInterval = 1e7
	// debugFileOpen>0 时写 logs 下详细每步日志（与购买无关）
	debugFileOpen = 0

	_enablePurchaseModeRtp2 = false // TestRtp2：是否注入购买
	_purchasePriceRtp2      = 75    // 购买价格倍数（扣费 = ×_baseMultiplier）
)

func TestRtp2(t *testing.T) {
	stats := &benchmarkStats{}

	// —— 购买 RTP 测试（与生产 req.Purchase 数学路径对齐，仍走 debug.open，无真实扣余额）——
	// 1) 外层每完成一次「大回合」(FreeNum 耗尽并 reset) 后，内层下一轮回合起点若满足条件则注入购买。
	// 2) 注入：IsPurchase、FreeNum、SetFreeNum、SetPurchaseAmount、Stage=_spinTypeBase（见下方注释，否则首手会被 syncGameStage 判成免费）。
	// 3) 扣费统计：会话内不记单笔 _baseMultiplier；FreeNum==0 重置前记一笔 _purchasePriceRtp2×_baseMultiplier，purchaseSessionWin 累计整段 stepMultiplier。
	// 4) 开关 _enablePurchaseModeRtp2=false 时完全不注入，日志里也不打 [购买跟踪]。
	var (
		purchaseSessionWin float64
		isInPurchaseRound  bool
	)

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

	for stats.BaseRounds < testRounds {
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

			// 购买注入：仅在大回合起点、当前非免费/非重转、无待执行免费、且上一笔购买已结算后执行。
			if _enablePurchaseModeRtp2 && !svc.isFreeRound && !svc.scene.IsRespinMode && svc.scene.FreeNum <= 0 && !isInPurchaseRound {
				svc.scene.IsPurchase = true
				svc.scene.PurchaseAmount = _purchasePriceRtp2 * _baseMultiplier
				//svc.client.ClientOfFreeGame.SetPurchaseAmount(_purchasePriceRtp2 * _baseMultiplier)
				svc.scene.Stage = _spinTypeBase
				svc.scene.NextStage = 0
				svc.isFreeRound = false
				isInPurchaseRound = true
				purchaseSessionWin = 0
			}

			beforeRespin := svc.scene.IsRespinMode
			beforeFree := svc.isFreeRound
			if err := svc.baseSpin(); err != nil {
				t.Fatalf("baseSpin failed: %v", err)
			}
			didRespin := svc.respinWildCol >= 0
			if didRespin {
				if beforeFree {
					stats.RespinStepsInFree++
				} else {
					stats.RespinStepsInBase++
				}
				if !beforeRespin {
					if beforeFree {
						stats.ResChainStartsInFree++
					} else {
						stats.ResChainStartsBase++
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
			if didRespin {
				stats.RespinTotalWin += stepWin
			}
			roundWin += stepWin
			if isInPurchaseRound {
				purchaseSessionWin += stepWin
			}

			// 统计奖金（必须在写日志之前，因为要设置 triggeringBaseRound）
			if isFree {
				stats.FreeWin += stepWin
				freeRoundWin += stepWin
				if svc.addFreeTime > 0 {
					stats.FreeTreasureInFree++
					stats.FreeExtraFreeRounds += svc.addFreeTime
				}
			} else {
				stats.BaseWin += stepWin
				// 与 rtp_test 一致：基础盘任意一次 addFreeTime>0 计一次触发（含重转至赢末手进免费，此时 isFirst=false）
				if svc.addFreeTime > 0 {
					stats.FreeTriggerCount++
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
					// 购买或重转末手进免费时，可能尚未计入 baseGameCount，仍为 0 → 原日志显示「第?局」
					if triggerRound == 0 {
						if baseGameCount > 0 {
							triggerRound = baseGameCount
						} else {
							triggerRound = 1
						}
					}
				}
				writeSpinDetail(fileBuf, svc, gameNum, isFree, triggerRound, stepWin, roundWin, respinStep, _enablePurchaseModeRtp2, isInPurchaseRound, purchaseSessionWin)
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
					// 基础模式回合结束
					baseGameCount++ // 回合结束时增加局号
					stats.BaseRounds++
					if roundWin > 0 {
						stats.BaseWinTimes++
					}
					if !(_enablePurchaseModeRtp2 && isInPurchaseRound) {
						stats.TotalBet += float64(_baseMultiplier)
					}
				}
				roundWin = 0

				// 只有当免费游戏完全结束时才重置服务并退出内层循环
				if svc.scene.FreeNum <= 0 {
					if _enablePurchaseModeRtp2 && isInPurchaseRound {
						stats.TotalBet += float64(_purchasePriceRtp2 * _baseMultiplier)
						stats.PurchaseCount++
						if purchaseSessionWin > 0 {
							stats.PurchaseWinTimes++
						}
						stats.PurchaseWin += purchaseSessionWin
						isInPurchaseRound = false
						purchaseSessionWin = 0
					}
					resetBetServiceForNextRound(svc)
					freeRoundIdx = 0
					if stats.BaseRounds%interval == 0 {
						stats.TotalWin = stats.BaseWin + stats.FreeWin
						// freeTime 与 rtp_test 一致：为基础模式触发免费次数（非 0，便于进度行与汇总对照）
						stats.FreeTime = stats.FreeTriggerCount
						printBenchmarkProgress(buf, stats, start)
						fmt.Print(buf.String())
					}
					break
				}
			} else if svc.addFreeTime > 0 && !isFree {
				// 基础模式触发免费时，这一局也算基础模式的一局
				// 计入 baseRounds（投入筹码的局数）和 baseGameCount
				// 注意：免费模式中触发额外免费不计入基础模式统计
				baseGameCount++
				stats.BaseRounds++
				if !(_enablePurchaseModeRtp2 && isInPurchaseRound) {
					stats.TotalBet += float64(_baseMultiplier)
				}
				if roundWin > 0 {
					stats.BaseWinTimes++
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

	stats.TotalWin = stats.BaseWin + stats.FreeWin
	stats.FreeTime = stats.FreeTriggerCount
	buf.Reset()
	printBenchmarkSummary(buf, stats, start, _enablePurchaseModeRtp2, _purchasePriceRtp2)
	// 保留购买段落，便于与 TestRtp 输出风格一致
	fprintf(buf, "\n[购买模式统计]\n")
	if !_enablePurchaseModeRtp2 {
		fprintf(buf, "开关: 关闭（本文件 const _enablePurchaseModeRtp2）\n")
	} else if stats.PurchaseCount == 0 {
		fprintf(buf, "开关: 已开启，但本次未产生购买结算（局数或条件不足）\n")
	} else {
		purchaseDenom := float64(stats.PurchaseCount) * float64(_purchasePriceRtp2*_baseMultiplier)
		purchaseRTP := safeDivFloat(stats.PurchaseWin*100, purchaseDenom)
		fprintf(buf, "购买次数: %d | 购买中奖率: %.2f%% | 购买RTP: %.2f%%\n", stats.PurchaseCount, safeDiv(stats.PurchaseWinTimes*100, stats.PurchaseCount), purchaseRTP)
		fprintf(buf, "平均每次购买对应免费总局数: %.2f（总免费步局数/购买次数；仅当每轮均购买时可作「每轮免费长度」参考）\n", safeDivFloat(float64(stats.FreeRounds), float64(stats.PurchaseCount)))
	}
	result := buf.String()
	fmt.Print(result)
	if debugFileOpen > 0 && fileBuf != nil {
		saveDebugFile(result, fileBuf.String(), start)
	}
}

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, gameNum int, isFree bool, triggeringBaseRound int, stepWin, roundWin float64, respinStep int, logPurchase bool, isInPurchaseRound bool, purchaseSessionWin float64) {
	if isFree {
		trigger := "?"
		if triggeringBaseRound > 0 {
			trigger = fmt.Sprintf("%d", triggeringBaseRound)
		}
		fprintf(buf, "\n=============[免费模式] 基础第%s局触发 - 免费第%d局 =============\n", trigger, gameNum)
	} else {
		if respinStep > 0 {
			fprintf(buf, "\n=============[基础模式] 第%d局 (重转至赢 Step%d) =============\n", gameNum, respinStep+1)
		} else {
			fprintf(buf, "\n=============[基础模式] 第%d局 =============\n", gameNum)
		}
	}
	if logPurchase && isInPurchaseRound {
		fprintf(buf, "[购买跟踪] isFreeRound=%v Stage=%d StepIsPurchase=%v SceneIsPurchase=%v  | 会话累计赢=%.2f(倍) FreeNum=%d client.purchaseAmt=%d\n",
			svc.isFreeRound, svc.scene.Stage, svc.stepIsPurchase, svc.scene.IsPurchase, purchaseSessionWin, svc.scene.FreeNum, svc.scene.PurchaseAmount)
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
	case svc.respinWildCol >= 0:
		longwild = "💎重转至赢"
	case svc.wildExpandCol >= 0:
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

	isRespin := 0
	if svc.scene.IsRespinMode {
		isRespin = 1
	}

	IsPurchase := 0
	if svc.stepIsPurchase {
		IsPurchase = 1
	}

	if svc.addFreeTime > 0 {
		if !isFree {
			fprintf(buf, "\t🚨🚨🚨 Scatter(全盘)=%d, 触发免费: +%d 次 |  当前剩余免费=%d | IsRespin=%v | IsPurchase=%v | index=%d %s\n", svc.scatterCount, svc.addFreeTime, svc.scene.FreeNum, isRespin, IsPurchase, svc.debug.mode, longwild)
		} else {
			fprintf(buf, "\tMode=%d Scatter(全盘)=%d | 当前剩余免费=%d nextStage=%d | IsRespin=%v |IsPurchase=%v | index=%d %s\n", model, svc.scatterCount, svc.scene.FreeNum, svc.scene.NextStage, isRespin, IsPurchase, svc.debug.mode, longwild)
		}
	} else {
		fprintf(buf, "\tMode=%d Scatter(全盘)=%d | nextStage=%d | IsRespin=%v | IsPurchase=%v | index=%d %s\n", model, svc.scatterCount, svc.scene.NextStage, isRespin, IsPurchase, svc.debug.mode, longwild)

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

func btoi(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
