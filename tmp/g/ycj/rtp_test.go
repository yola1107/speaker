package ycj

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
)

type benchmarkStats struct {
	// ===== 全局统计 =====
	ChargeCount int64
	TotalBet    float64
	TotalWin    float64

	// ===== 基础模式（base） =====
	BaseRounds   int64
	BaseWinTimes int64
	BaseWin      float64

	// ===== 免费模式（free） =====
	FreeRounds          int64
	FreeWinTimes        int64
	FreeWin             float64
	FreeTriggerCount    int64
	FreeTime            int64
	FreeTreasureInFree  int64
	FreeExtraFreeRounds int64

	// ===== 推展模式（extend） =====
	ExtendStepsInBase int64
	ExtendStepsInFree int64
	ExtendTotalWin    float64
	ExtendWinInBase   float64
	ExtendWinInFree   float64

	// ===== 重转模式（respin） =====
	RespinStepsInFree int64
	RespinTotalWin    float64
	RespinWinInFree   float64
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
		baseRoundOpen          bool
	)

	for stats.ChargeCount < _benchmarkRounds {
		beforePend := svc.scene.Pend
		beforeFree := svc.isFreeRound
		wasPaidBaseStart := !svc.isFreeRound && !svc.scene.pendingSpin()
		if wasPaidBaseStart {
			baseRoundOpen = true
		}
		if err = svc.baseSpin(); err != nil {
			panic(err)
		}

		// 统计推展模式（首次进入）
		if svc.scene.Pend == _pendExtend && beforePend != _pendExtend {
			if beforeFree {
				stats.ExtendStepsInFree++
			} else {
				stats.ExtendStepsInBase++
			}
		}

		// 统计重转模式（仅免费模式，首次进入）
		if svc.scene.Pend == _pendRespin && beforePend != _pendRespin && beforeFree {
			stats.RespinStepsInFree++
		}

		stepWin := float64(svc.stepMultiplier)
		roundWin += stepWin
		stats.TotalWin += stepWin

		if svc.scene.Pend == _pendExtend {
			stats.ExtendTotalWin += stepWin
			if svc.isFreeRound {
				stats.ExtendWinInFree += stepWin
			} else {
				stats.ExtendWinInBase += stepWin
			}
		}
		if svc.scene.Pend == _pendRespin {
			stats.RespinTotalWin += stepWin
			if svc.isFreeRound {
				stats.RespinWinInFree += stepWin
			}
		}

		if svc.isFreeRound {
			stats.FreeWin += stepWin
			freeRoundWin += stepWin
			if svc.addFreeTime > 0 {
				stats.FreeTreasureInFree++
				stats.FreeExtraFreeRounds += svc.addFreeTime
			}
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
			} else if baseRoundOpen {
				stats.BaseRounds++
				if roundWin > 0 {
					stats.BaseWinTimes++
				}
				stats.TotalBet += float64(_baseMultiplier)
				stats.ChargeCount++
				baseRoundOpen = false
			}
			roundWin = 0
		} else if !svc.isFreeRound && svc.addFreeTime > 0 && baseRoundOpen {
			stats.BaseRounds++
			stats.TotalBet += float64(_baseMultiplier)
			stats.ChargeCount++
			if roundWin > 0 {
				stats.BaseWinTimes++
			}
			stats.FreeTriggerCount++
			stats.FreeTime++
			roundWin = 0
			baseRoundOpen = false
		}

		if svc.isRoundOver && svc.scene.FreeNum == 0 {
			resetBetServiceForNextRound(svc)
			freeRoundWin = 0

			if stats.ChargeCount > 0 && stats.ChargeCount%progressStep == 0 {
				printBenchmarkProgress(buf, stats, start)
				fmt.Print(buf.String())
			}
		}
	}

	buf.Reset()
	printBenchmarkSummary(buf, stats, start)
	fmt.Print(buf.String())
}

func printBenchmarkProgress(buf *strings.Builder, stats *benchmarkStats, start time.Time) {
	if stats.ChargeCount == 0 || stats.TotalBet == 0 {
		return
	}
	freeRoundsSafe := max(stats.FreeRounds, 1)
	avgFreePerTrigger := safeDiv(stats.FreeRounds, stats.FreeTriggerCount)
	buf.Reset()
	fprintf(buf, "\rRuntime=%d baseRtp=%.4f%%,baseWinRate=%.4f%% freeRtp=%.4f%% freeWinRate=%.4f%%, freeTriggerRate=%.4f%% avgFree=%.4f Rtp=%.4f%% \n",
		stats.ChargeCount,
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

func printBenchmarkSummary(buf *strings.Builder, stats *benchmarkStats, start time.Time) {
	if stats.ChargeCount == 0 || stats.TotalBet == 0 {
		fprintf(buf, "No data collected for RTP benchmark.\n")
		return
	}
	w := func(format string, args ...interface{}) { fprintf(buf, format, args...) }
	elapsed := time.Since(start)
	speed := safeDivFloat(float64(stats.ChargeCount), elapsed.Seconds())
	w("\n运行扣费次数: %d，用时: %v，速度: %.2f 次/秒\n\n", stats.ChargeCount, elapsed.Round(time.Second), speed)

	baseRTP := stats.BaseWin * 100 / stats.TotalBet
	freeRTP := stats.FreeWin * 100 / stats.TotalBet
	totalRTP := (stats.BaseWin + stats.FreeWin) * 100 / stats.TotalBet
	baseWinRate := safeDiv(stats.BaseWinTimes*100, stats.BaseRounds)
	freeWinRate := safeDiv(stats.FreeWinTimes*100, stats.FreeRounds)
	freeTriggerRate := safeDiv(stats.FreeTriggerCount*100, stats.BaseRounds)
	avgFreePerRound := safeDiv(stats.FreeRounds, stats.BaseRounds)
	avgFreePerTrigger := safeDiv(stats.FreeRounds, stats.FreeTriggerCount)

	w("\n[基础模式统计]\n")
	w("基础模式总游戏局数: %d\n", stats.BaseRounds)
	w("基础模式总投注(倍数): %.2f\n", stats.TotalBet)
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
	w("免费模式夺宝触发次数: %d\n", stats.FreeTreasureInFree)
	w("免费模式额外增加局数: %d\n", stats.FreeExtraFreeRounds)

	w("\n[免费触发效率]\n")
	w("  总免费游戏次数: %d | 总触发次数: %d\n", stats.FreeRounds, stats.FreeTriggerCount)
	w("  平均每次触发获得免费次数: %.2f\n", avgFreePerTrigger)

	w("\n[推展模式统计]\n")
	w("推展模式·基础: 步%d | 率%.4f%% | 总赢%.2f\n", stats.ExtendStepsInBase, safeDiv(stats.ExtendStepsInBase*100, stats.BaseRounds), stats.ExtendWinInBase)
	w("推展模式·免费: 步%d | 率%.4f%% | 总赢%.2f\n", stats.ExtendStepsInFree, safeDiv(stats.ExtendStepsInFree*100, max(stats.FreeRounds, 1)), stats.ExtendWinInFree)
	w("推展模式·合计总赢: %.2f\n", stats.ExtendTotalWin)

	w("\n[重转模式统计]\n")
	w("重转模式·免费: 步%d | 率%.4f%% | 总赢%.2f\n", stats.RespinStepsInFree, safeDiv(stats.RespinStepsInFree*100, max(stats.FreeRounds, 1)), stats.RespinWinInFree)
	w("重转模式·合计总赢: %.2f\n", stats.RespinTotalWin)

	w("\n[总计]\n")
	w("总回报率(RTP): %.2f%%\n", totalRTP)
	w("总扣费笔数: %d\n", stats.ChargeCount)
	w("总投注金额: %.2f\n", stats.TotalBet)
	w("总奖金金额: %.2f\n\n", stats.TotalWin)
}

func newBerService() *betOrderService {
	return &betOrderService{
		req: &request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     GameID,
			BaseMoney:  1,
			Multiple:   1,
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
	s.isFreeRound = false
	s.scene.Reset()
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
