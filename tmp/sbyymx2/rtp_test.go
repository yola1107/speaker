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
	_benchmarkRounds           int64 = 1e7
	_benchmarkProgressInterval int64 = 1e6
)

func init() {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	cfg.DisableStacktrace = true
	cfg.EncoderConfig.EncodeCaller = zapcore.FullCallerEncoder
	logger, _ := cfg.Build()
	global.GVA_LOG = logger
}

// TestRtp 大规模模拟中间行倍数（debug 下 bonusAmount 为 0，用 stepMultiplier 统计理论赔付倍数）
func TestRtp(t *testing.T) {
	rounds := _benchmarkRounds
	progressStep := int64(min(_benchmarkProgressInterval, rounds))
	if testing.Short() {
		rounds = 5000
		progressStep = 1000
	}

	svc := newBerService()
	start := time.Now()
	buf := &strings.Builder{}

	var (
		totalBet  float64
		totalWin  float64
		winRounds int64
		completed int64
		err       error
	)

	for completed < rounds {
		if err = svc.baseSpin(); err != nil {
			t.Fatal(err)
		}
		stepWin := float64(svc.stepMultiplier)
		totalBet += float64(_baseMultiplier)
		totalWin += stepWin
		if stepWin > 0 {
			winRounds++
		}
		completed++

		if completed%progressStep == 0 {
			printRtpProgressHcsqyStyle(buf, completed, totalBet, totalWin, totalWin, winRounds, start)
			fmt.Print(buf.String())
		}
	}

	printRtpSummaryHcsqyStyle(buf, completed, totalBet, totalWin, totalWin, winRounds, start)
	fmt.Print(buf.String())
}

// printRtpProgressHcsqyStyle 与 game/hcsqy rtp_test 中 printBenchmarkProgress 格式一致（本游戏无免费模式，免费相关统计为 0）
func printRtpProgressHcsqyStyle(buf *strings.Builder, baseRounds int64, totalBet, baseWin, totalWin float64, baseWinTimes int64, start time.Time) {
	if baseRounds == 0 || totalBet == 0 {
		return
	}
	var (
		freeWinTimes, freeRounds, freeTriggerCount, freeTime int64
	)
	freeWinF := 0.0
	freeRoundsSafe := max(freeRounds, 1)
	avgFreePerTrigger := safeDiv(freeRounds, freeTriggerCount)
	buf.Reset()
	fprintf(buf, "\rRuntime=%d baseRtp=%.4f%%,baseWinRate=%.4f%% freeRtp=%.4f%% freeWinRate=%.4f%%, freeTriggerRate=%.4f%% avgFree=%.4f Rtp=%.4f%% \n",
		baseRounds,
		baseWin*100/totalBet,
		safeDiv(baseWinTimes*100, baseRounds),
		freeWinF*100/totalBet,
		safeDiv(freeWinTimes*100, freeRoundsSafe),
		safeDiv(freeTriggerCount*100, baseRounds),
		avgFreePerTrigger,
		(totalWin)*100/totalBet,
	)
	fprintf(buf, "\rtotalWin-%.0f freeWin=%.0f,baseWin-%.0f ,baseWinTime-%d ,freeTime-%d, freeRound-%d ,freeWinTime-%d, elapsed=%v\n",
		totalWin, freeWinF, baseWin, baseWinTimes, freeTime, freeRounds, freeWinTimes, time.Since(start).Round(time.Second))
}

// printRtpSummaryHcsqyStyle 与 game/hcsqy rtp_test 中 printBenchmarkSummary 结构一致（免费块为 0，并注明本游戏无免费）
func printRtpSummaryHcsqyStyle(buf *strings.Builder, baseRounds int64, totalBet, baseWin, totalWin float64, baseWinTimes int64, start time.Time) {
	if baseRounds == 0 || totalBet == 0 {
		buf.Reset()
		fprintf(buf, "No data collected for RTP benchmark.\n")
		return
	}
	w := func(format string, args ...interface{}) { fprintf(buf, format, args...) }
	buf.Reset()
	elapsed := time.Since(start)
	speed := safeDiv(baseRounds, int64(elapsed.Seconds()))
	w("\n运行局数: %d，用时: %v，速度: %.0f 局/秒\n\n", baseRounds, elapsed.Round(time.Second), speed)

	baseRTP := baseWin * 100 / totalBet
	freeRTP := 0.0
	totalRTP := totalWin * 100 / totalBet
	baseWinRate := safeDiv(baseWinTimes*100, baseRounds)
	var freeRounds, freeWinTimes, freeTriggerCount, freeTime int64
	freeWin := 0.0
	freeWinRate := 0.0
	avgFreePerRound := safeDiv(freeRounds, baseRounds)
	avgFreePerTrigger := safeDiv(freeRounds, freeTriggerCount)

	w("\n[基础模式统计]\n")
	w("基础模式总游戏局数: %d\n", baseRounds)
	w("基础模式总投注(倍数): %.2f\n", totalBet)
	w("基础模式总奖金: %.2f\n", baseWin)
	w("基础模式RTP: %.2f%%\n", baseRTP)
	w("基础模式免费局触发次数: %d\n", freeTime)
	w("基础模式触发免费局比例: %.2f%%\n", safeDiv(freeTriggerCount*100, baseRounds))
	w("基础模式平均每局免费次数: %.2f\n", avgFreePerRound)
	w("基础模式中奖率: %.2f%%\n", baseWinRate)
	w("基础模式中奖局数: %d\n", baseWinTimes)

	w("\n[免费模式统计]（本游戏 sbyymx2 无免费模式，下列为 0）\n")
	w("免费模式总游戏局数: %d\n", freeRounds)
	w("免费模式总奖金: %.2f\n", freeWin)
	w("免费模式RTP: %.2f%%\n", freeRTP)
	w("免费模式中奖率: %.2f%%\n", freeWinRate)
	w("免费模式中奖局数: %d\n", freeWinTimes)
	w("\n[免费触发效率]\n")
	w("  总免费游戏次数: %d | 总触发次数: %d\n", freeRounds, freeTriggerCount)
	w("  平均每次触发获得免费次数: %.2f\n", avgFreePerTrigger)

	w("\n[总计]\n")
	w("总回报率(RTP): %.2f%%\n", totalRTP)
	w("总投注金额: %.2f\n", totalBet)
	w("总奖金金额: %.2f\n\n", totalWin)
}

// newBerService 构造 RTP 测试用服务：debug.open = true（与 hcsqy rtp_test 中 newBerService 位置一致）
func newBerService() *betOrderService {
	s := &betOrderService{
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
		},
		game: &game.Game{
			ID:       GameID,
			GameName: Name,
		},
		client: &client.Client{
			ClientOfFreeGame: &client.ClientOfFreeGame{},
		},
		scene: &SpinSceneData{},
		debug: rtpDebugData{open: true},
	}
	s.initGameConfigs()
	return s
}

func TestRtpDebug_baseSpin(t *testing.T) {
	s := newBerService()
	if err := s.baseSpin(); err != nil {
		t.Fatal(err)
	}
	if s.gameConfig == nil {
		t.Fatal("gameConfig is nil")
	}
	if !s.bonusAmount.Equal(decimal.Zero) {
		t.Fatalf("debug mode: bonusAmount want 0, got %v", s.bonusAmount)
	}
	if !s.betAmount.Equal(decimal.NewFromInt(_baseMultiplier)) {
		t.Fatalf("debug mode: betAmount want %d, got %v", _baseMultiplier, s.betAmount)
	}
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
