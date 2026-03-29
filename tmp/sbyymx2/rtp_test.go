package sbyymx2

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
	// 游戏局数
	TotalRounds int64

	// 中奖局数
	WinTimes int64

	// 奖励与投注
	TotalWin float64
	TotalBet float64

	// 重转统计
	RespinTotalWin     float64
	RespinSteps        int64
	ResChainStarts     int64
	WildExpandTriggers int64
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
		err          error
		roundWin     float64
		beforeRespin bool
	)

	for stats.TotalRounds < _benchmarkRounds {
		beforeRespin = svc.scene.IsRespinMode

		if err = svc.baseSpin(); err != nil {
			panic(err)
		}

		if svc.respinWildCol >= 0 {
			stats.RespinSteps++
			if !beforeRespin {
				stats.ResChainStarts++
			}
		}

		if svc.wildExpandCol >= 0 {
			stats.WildExpandTriggers++
		}

		stepWin := float64(svc.stepMultiplier)
		roundWin += stepWin
		stats.TotalWin += stepWin

		if svc.respinWildCol >= 0 {
			stats.RespinTotalWin += stepWin
		}

		if svc.isRoundOver {
			stats.TotalRounds++
			stats.TotalBet += float64(_baseMultiplier)
			if roundWin > 0 {
				stats.WinTimes++
			}
			roundWin = 0
		}

		if svc.isRoundOver {
			resetBetServiceForNextRound(svc)

			if stats.TotalRounds%progressStep == 0 {
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
	if stats.TotalRounds == 0 || stats.TotalBet == 0 {
		return
	}
	buf.Reset()
	fprintf(buf, "\rRounds=%d RTP=%.4f%% WinRate=%.4f%% elapsed=%v\n",
		stats.TotalRounds,
		stats.TotalWin*100/stats.TotalBet,
		safeDiv(stats.WinTimes*100, stats.TotalRounds),
		time.Since(start).Round(time.Second),
	)
}

func printBenchmarkSummary(buf *strings.Builder, stats *benchmarkStats, start time.Time) {
	if stats.TotalRounds == 0 || stats.TotalBet == 0 {
		fprintf(buf, "No data collected for RTP benchmark.\n")
		return
	}
	w := func(format string, args ...interface{}) { fprintf(buf, format, args...) }
	elapsed := time.Since(start)
	speed := safeDivFloat(float64(stats.TotalRounds), elapsed.Seconds())
	w("\n运行局数: %d，用时: %v，速度: %.2f 局/秒\n\n", stats.TotalRounds, elapsed.Round(time.Second), speed)

	totalRTP := stats.TotalWin * 100 / stats.TotalBet
	winRate := safeDiv(stats.WinTimes*100, stats.TotalRounds)

	w("[游戏统计]\n")
	w("总游戏局数: %d\n", stats.TotalRounds)
	w("总扣费(倍数): %.2f\n", stats.TotalBet)
	w("总奖金: %.2f\n", stats.TotalWin)
	w("总RTP: %.2f%%\n", totalRTP)
	w("中奖率: %.2f%%\n", winRate)
	w("中奖局数: %d\n", stats.WinTimes)

	w("\n[重转至赢统计]\n")
	w("重转触发次数: %d\n", stats.ResChainStarts)
	w("重转触发率: %.2f%%\n", safeDiv(stats.ResChainStarts*100, stats.TotalRounds))
	w("重转总步数: %d\n", stats.RespinSteps)
	w("重转总奖金: %.2f\n", stats.RespinTotalWin)
	w("平均每次重转触发奖金倍数: %.4f\n", safeDivFloat(stats.RespinTotalWin, float64(stats.ResChainStarts)*float64(_baseMultiplier)))

	w("\n[百搭变大统计]\n")
	w("百搭变大触发次数: %d\n", stats.WildExpandTriggers)
	w("百搭变大触发率: %.2f%%\n", safeDiv(stats.WildExpandTriggers*100, stats.TotalRounds))

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
	s.isRoundOver = false
	s.scene = &SpinSceneData{}
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
