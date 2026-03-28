package hcsqy

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	_benchmarkRounds           int64 = 1e8
	_benchmarkProgressInterval int64 = 1e7

	_enablePurchaseModeRtp = false // TestRtp：是否注入购买
	_purchasePriceRtp      = 75    // 购买价格倍数（扣费 = ×_baseMultiplier）
)

type benchmarkStats struct {
	// 基础/免费局数
	BaseRounds int64
	FreeRounds int64

	// 中奖局数
	BaseWinTimes int64
	FreeWinTimes int64

	// 免费触发
	FreeTriggerCount int64
	FreeTime         int64

	// 奖励与投注
	BaseWin  float64
	FreeWin  float64
	TotalWin float64
	TotalBet float64

	// 重转统计
	RespinTotalWin       float64
	RespinStepsInBase    int64
	RespinStepsInFree    int64
	ResChainStartsBase   int64
	ResChainStartsInFree int64

	// 购买统计
	PurchaseCount    int64
	PurchaseWinTimes int64
	PurchaseWin      float64

	// 免费中免费
	FreeTreasureInFree  int64
	FreeExtraFreeRounds int64
}

func init() {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	cfg.DisableStacktrace = true
	cfg.EncoderConfig.EncodeCaller = zapcore.FullCallerEncoder
	logger, _ := cfg.Build()
	global.GVA_LOG = logger
}

func TestRtp(t *testing.T) {
	svc := newBerService()
	svc.initGameConfigs()
	start := time.Now()
	buf := &strings.Builder{}
	progressStep := int64(min(_benchmarkProgressInterval, _benchmarkRounds))
	stats := &benchmarkStats{}

	var (
		err                    error
		freeRoundWin, roundWin float64
		// 重转至赢：respinWildCol>=0 表示本步执行了 processRespinUntilWin（与 wildExpandCol 互斥）
		// 购买模式统计
		purchaseSessionWin float64 // purchaseSessionWin：一次购买会话内各步 stepMultiplier 累计（与生产「整段购买免费」口径一致）
		isInPurchaseRound  bool    // 标记当前是否处于购买会话（从注入到 FreeNum 耗尽）
	)

	for stats.BaseRounds < _benchmarkRounds {
		// 购买模式：每次新回合开始时触发购买
		if _enablePurchaseModeRtp && !svc.isFreeRound && !svc.scene.IsRespinMode && svc.scene.FreeNum <= 0 && !isInPurchaseRound {
			svc.scene.IsPurchase = true
			//svc.scene.FreeNum = svc.gameConfig.Free.FreeTimes
			//svc.client.ClientOfFreeGame.SetFreeNum(uint64(svc.gameConfig.Free.FreeTimes))
			svc.client.ClientOfFreeGame.SetPurchaseAmount(_purchasePriceRtp * _baseMultiplier)
			// baseSpin 内先 syncGameStage：若 Stage==0 且 FreeNum>0 会强制 Stage=免费并 isFreeRound=true；debug 下 initFirstStep 又不会写回 isFreeRound=false（生产会）。
			// 显式 Stage=基础，首手才与生产一致：BuyBase、且不在 baseSpin 开头 Decr FreeNum。
			svc.scene.Stage = _spinTypeBase
			svc.scene.NextStage = 0
			svc.isFreeRound = false
			isInPurchaseRound = true
			purchaseSessionWin = 0
			roundWin = 0
		}

		beforeRespin := svc.scene.IsRespinMode
		beforeFree := svc.isFreeRound
		if err = svc.baseSpin(); err != nil {
			panic(err)
		}
		if svc.respinWildCol >= 0 {
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

		stepWin := float64(svc.stepMultiplier)
		roundWin += stepWin
		stats.TotalWin += stepWin
		if isInPurchaseRound {
			purchaseSessionWin += stepWin
		}
		if svc.respinWildCol >= 0 {
			stats.RespinTotalWin += stepWin
		}

		if svc.isFreeRound {
			stats.FreeWin += stepWin
			freeRoundWin += stepWin
		} else {
			stats.BaseWin += stepWin
		}

		if svc.isRoundOver {
			if svc.isFreeRound {
				stats.FreeRounds++
				if freeRoundWin > 0 {
					stats.FreeWinTimes++
				}
				freeRoundWin = 0
			} else {
				stats.BaseRounds++
				// 购买会话未结束前不记单笔底注；整笔 75× 在会话结束（FreeNum 耗尽）时记入，见下方 reset 前逻辑
				if !(_enablePurchaseModeRtp && isInPurchaseRound) {
					stats.TotalBet += float64(_baseMultiplier)
				}
				if roundWin > 0 {
					stats.BaseWinTimes++
				}
			}
			roundWin = 0
		} else if !svc.isFreeRound && svc.addFreeTime > 0 {
			// 基础盘打出夺宝进免费：本手 isRoundOver=false，但已对应一次完整基础下注（与 TestRtp2 / 真实扣费口径一致）
			stats.BaseRounds++
			if !(_enablePurchaseModeRtp && isInPurchaseRound) {
				stats.TotalBet += float64(_baseMultiplier)
			}
			if roundWin > 0 {
				stats.BaseWinTimes++
			}
			stats.FreeTriggerCount++
			stats.FreeTime++
			roundWin = 0
		}

		if svc.isRoundOver && svc.scene.FreeNum <= 0 {
			// 购买统计：与 processWinInfos 在 FreeNum<=0 时结束购买一致，整段免费产出计入 purchaseWin
			if _enablePurchaseModeRtp && isInPurchaseRound {
				stats.TotalBet += float64(_purchasePriceRtp * _baseMultiplier)
				stats.PurchaseCount++
				if purchaseSessionWin > 0 {
					stats.PurchaseWinTimes++
				}
				stats.PurchaseWin += purchaseSessionWin
				isInPurchaseRound = false
				purchaseSessionWin = 0
			}
			resetBetServiceForNextRound(svc)
			freeRoundWin = 0

			if stats.BaseRounds%progressStep == 0 {
				printBenchmarkProgress(buf, stats, start)
				fmt.Print(buf.String())
			}
		}
	}

	buf.Reset()
	printBenchmarkSummary(buf, stats, start, _enablePurchaseModeRtp, _purchasePriceRtp)

	// 购买模式额外统计
	fprintf(buf, "\n[购买模式统计]\n")
	if !_enablePurchaseModeRtp {
		fprintf(buf, "开关: 关闭（本文件 const _enablePurchaseModeRtp）\n")
	} else if stats.PurchaseCount == 0 {
		fprintf(buf, "开关: 已开启，但本次未产生购买结算（局数或条件不足）\n")
	} else {
		purchaseRTP := safeDivFloat(stats.PurchaseWin*100, float64(stats.PurchaseCount)*float64(_purchasePriceRtp*_baseMultiplier))
		fprintf(buf, "购买次数: %d | 购买中奖率: %.2f%% | 购买RTP: %.2f%%\n", stats.PurchaseCount, safeDiv(stats.PurchaseWinTimes*100, stats.PurchaseCount), purchaseRTP)
		fprintf(buf, "平均每次购买对应免费总局数: %.2f（总免费步局数/购买次数；全开购买时≈每轮免费长度）\n", safeDivFloat(float64(stats.FreeRounds), float64(stats.PurchaseCount)))
	}

	fmt.Print(buf.String())
}

func printBenchmarkProgress(buf *strings.Builder, stats *benchmarkStats, start time.Time) {
	if stats.BaseRounds == 0 || stats.TotalBet == 0 {
		return
	}
	freeRoundsSafe := max(stats.FreeRounds, 1)
	avgFreePerTrigger := safeDiv(stats.FreeRounds, stats.FreeTriggerCount)
	buf.Reset()
	fprintf(buf, "\rRuntime=%d baseRtp=%.4f%%,baseWinRate=%.4f%% freeRtp=%.4f%% freeWinRate=%.4f%%, freeTriggerRate=%.4f%% avgFree=%.4f Rtp=%.4f%% \n",
		stats.BaseRounds,
		stats.BaseWin*100/stats.TotalBet,
		safeDiv(stats.BaseWinTimes*100, stats.BaseRounds),
		stats.FreeWin*100/stats.TotalBet,
		safeDiv(stats.FreeWinTimes*100, freeRoundsSafe),
		safeDiv(stats.FreeTriggerCount*100, stats.BaseRounds),
		avgFreePerTrigger,
		(stats.BaseWin+stats.FreeWin)*100/stats.TotalBet,
	)
	fprintf(buf, "\rtotalWin-%.0f freeWin=%.0f,baseWin-%.0f ,baseWinTime-%d ,freeTime-%d, freeRound-%d ,freeWinTime-%d, elapsed=%v\n",
		stats.TotalWin, stats.FreeWin, stats.BaseWin, stats.BaseWinTimes, stats.FreeTime, stats.FreeRounds, stats.FreeWinTimes, time.Since(start).Round(time.Second))
}

func printBenchmarkSummary(buf *strings.Builder, stats *benchmarkStats, start time.Time, purchaseEnabled bool, purchasePrice int64) {
	if stats.BaseRounds == 0 || stats.TotalBet == 0 {
		fprintf(buf, "No data collected for RTP benchmark.\n")
		return
	}
	w := func(format string, args ...interface{}) { fprintf(buf, format, args...) }
	elapsed := time.Since(start)
	speed := safeDivFloat(float64(stats.BaseRounds), elapsed.Seconds())
	w("\n运行局数: %d，用时: %v，速度: %.2f 局/秒\n\n", stats.BaseRounds, elapsed.Round(time.Second), speed)

	baseRTP := stats.BaseWin * 100 / stats.TotalBet
	freeRTP := stats.FreeWin * 100 / stats.TotalBet
	totalRTP := (stats.BaseWin + stats.FreeWin) * 100 / stats.TotalBet
	baseWinRate := safeDiv(stats.BaseWinTimes*100, stats.BaseRounds)
	freeWinRate := safeDiv(stats.FreeWinTimes*100, stats.FreeRounds)
	freeTriggerRate := safeDiv(stats.FreeTriggerCount*100, stats.BaseRounds)
	avgFreePerRound := safeDiv(stats.FreeRounds, stats.BaseRounds)
	avgFreePerTrigger := safeDiv(stats.FreeRounds, stats.FreeTriggerCount)
	resStartRateBase := safeDiv(stats.ResChainStartsBase*100, stats.BaseRounds)
	resStartRateFree := safeDiv(stats.ResChainStartsInFree*100, max(stats.FreeRounds, 1))
	resChainTotal := stats.ResChainStartsBase + stats.ResChainStartsInFree
	avgRespinStepsPerChain := safeDiv(stats.RespinStepsInBase+stats.RespinStepsInFree, resChainTotal)

	w("\n[基础模式统计]\n")
	w("基础模式总游戏局数: %d\n", stats.BaseRounds)
	w("总扣费(倍数，含购买): %.2f\n", stats.TotalBet)
	w("基础模式总奖金: %.2f\n", stats.BaseWin)
	w("基础模式RTP: %.2f%%\n", baseRTP)
	w("基础模式免费局触发次数: %d\n", stats.FreeTime)
	w("基础模式触发免费局比例: %.2f%%\n", freeTriggerRate)
	w("基础模式平均每局免费次数: %.2f\n", avgFreePerRound)
	w("基础模式中奖率: %.2f%%\n", baseWinRate)
	w("基础模式中奖局数: %d\n", stats.BaseWinTimes)

	w("\n[免费模式统计]\n")
	w("免费模式总游戏局数: %d\n", stats.FreeRounds)
	w("免费模式总奖金: %.2f\n", stats.FreeWin)
	w("免费模式RTP: %.2f%%\n", freeRTP)
	w("免费模式中奖率: %.2f%%\n", freeWinRate)
	w("免费模式中奖局数: %d\n", stats.FreeWinTimes)

	w("\n[免费触发效率]\n")
	w("  总免费游戏次数: %d | 总触发次数: %d\n", stats.FreeRounds, stats.FreeTriggerCount)
	w("  平均每次触发获得免费次数: %.2f\n", avgFreePerTrigger)
	if stats.FreeExtraFreeRounds > 0 || stats.FreeTreasureInFree > 0 {
		w("  免费中夺宝触发次数: %d | 免费中额外局数: %d\n", stats.FreeTreasureInFree, stats.FreeExtraFreeRounds)
	}

	w("\n[respin统计]\n")
	w("重转至赢·基础: 步%d | 链%d | 率%.2f%% | 均%.4f步/局\n", stats.RespinStepsInBase, stats.ResChainStartsBase, resStartRateBase, safeDivFloat(float64(stats.RespinStepsInBase), float64(stats.BaseRounds)))
	w("重转至赢·免费: 步%d | 链%d | 率%.2f%% | 均%.4f步/局\n", stats.RespinStepsInFree, stats.ResChainStartsInFree, resStartRateFree, safeDivFloat(float64(stats.RespinStepsInFree), float64(max(stats.FreeRounds, 1))))
	w("重转至赢·合计: %d步 | %d链 | %.2f 步/链\n", stats.RespinStepsInBase+stats.RespinStepsInFree, resChainTotal, avgRespinStepsPerChain)
	// 平均一轮 respin win 倍数：总 win / 触发次数 / bet
	w("  平均每次重转至赢触发的胜出倍数: %.4f (总重转win: %.2f)\n", safeDivFloat(stats.RespinTotalWin, float64(resChainTotal)*float64(_baseMultiplier)), stats.RespinTotalWin)

	w("\n[总计]\n")
	w("总回报率(RTP): %.2f%%\n", totalRTP)
	w("总扣费金额(含购买): %.2f\n", stats.TotalBet)
	w("总奖金金额: %.2f\n", stats.TotalWin)
	if purchaseEnabled && stats.PurchaseCount > 0 {
		purchaseRTP := safeDivFloat(stats.PurchaseWin*100, float64(stats.PurchaseCount)*float64(purchasePrice*_baseMultiplier))
		w("购买RTP: %.2f%%\n", purchaseRTP)
	}
	w("\n")

}

func newBerService() *betOrderService {
	return &betOrderService{
		req: &request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     GameID,
			BaseMoney:  1,
			Multiple:   1,
			Purchase:   0,
		},
		merchant: &merchant.Merchant{
			ID:       20020,
			Merchant: "TestMerchant",
		},
		member: &member.Member{
			ID:         1,
			MemberName: "TestUser",
			Balance:    10000000,
			Currency:   "USD",
		},
		game: &game.Game{
			ID: GameID,
		},
		client: &client.Client{
			ClientOfFreeGame: &client.ClientOfFreeGame{},
			ClientGameCache:  &client.ClientGameCache{},
		},
		scene:       &SpinSceneData{},
		bonusAmount: decimal.Decimal{},
		betAmount:   decimal.Decimal{},
		amount:      decimal.Decimal{},
		debug:       rtpDebugData{open: true},
	}
}

func resetBetServiceForNextRound(s *betOrderService) {
	s.stepMultiplier = 0
	s.scatterCount = 0
	s.isFreeRound = false
	s.scene = &SpinSceneData{} // 完全重置场景状态
	s.client.IsRoundOver = false
	s.client.ClientOfFreeGame.Reset()
	s.client.ClientOfFreeGame.ResetGeneralWinTotal()
	s.client.ClientOfFreeGame.ResetRoundBonus()
	s.client.SetLastMaxFreeNum(0)
}

func fprintf(buf *strings.Builder, format string, args ...interface{}) {
	_, _ = fmt.Fprintf(buf, format, args...)
}

func safeDiv(numerator, denominator int64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func safeDivFloat(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}
