package jqs

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

func init() {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	cfg.DisableStacktrace = true
	cfg.EncoderConfig.EncodeCaller = zapcore.FullCallerEncoder
	logger, _ := cfg.Build()
	global.GVA_LOG = logger
}

const (
	_benchmarkRounds           int64 = 1e8
	_benchmarkProgressInterval int64 = 1e7
)

// rtpStats RTP统计数据结构
type rtpStats struct {
	baseRounds       int64   // 基础模式总游戏局数
	freeRounds       int64   // 免费模式总游戏局数
	baseWin          int64   // 基础模式总奖金（倍数）
	freeWin          int64   // 免费模式总奖金（倍数）
	totalBet         float64 // 总投注金额（倍数）
	totalWin         int64   // 总奖金（倍数）
	baseWinTimes     int64   // 基础模式中奖局数
	freeWinTimes     int64   // 免费模式中奖局数
	freeTriggerCount int64   // Re-spin触发次数
	freeTime         int64   // 免费模式触发次数
	freeContinueSum  int64   // 免费模式连续spin总次数
	freeMaxContinue  int64   // 免费模式最大连续spin次数
}

func TestRtp(t *testing.T) {
	start := time.Now()
	buf := &strings.Builder{}
	svc := newTestService()
	svc.initGameConfigs()
	stats := &rtpStats{}
	maxBench := _benchmarkRounds
	if testing.Short() {
		maxBench = 100_000
	}
	progressStep := int64(min(_benchmarkProgressInterval, maxBench))

	// 恢复双层循环结构，局间复用 svc（见 resetBetServiceForNextRound）
	for stats.baseRounds+stats.freeRounds < maxBench {
		var continueCount int64

		for {
			isFirst := svc.scene.Stage == _spinTypeBase
			wasFreeBeforeSpin := svc.scene.Stage == _spinTypeFree

			if err := svc.baseSpin(); err != nil {
				panic(err)
			}

			// 更新奖金金额（RTP模式下已优化，跳过decimal计算）
			svc.updateBonusAmount()

			isFree := svc.scene.Stage == _spinTypeFree

			// 从基础模式切换到免费模式时，重置计数并累加投注
			if isFirst && !wasFreeBeforeSpin && isFree {
				continueCount = 0
				stats.freeTriggerCount++
				stats.totalBet += float64(_baseMultiplier)
			}

			continueCount++
			stepWin := svc.stepMultiplier
			stats.totalWin += stepWin

			// 更新最大连续次数
			if isFree && continueCount > stats.freeMaxContinue {
				stats.freeMaxContinue = continueCount
			}

			// 统计奖金
			if isFree {
				stats.freeWin += stepWin
			} else {
				stats.baseWin += stepWin
			}

			if svc.roundEndedThisSpin() {
				if isFree {
					stats.freeContinueSum += continueCount
					stats.freeRounds++
					if stepWin > 0 {
						stats.freeWinTimes++
					}
				} else {
					stats.baseRounds++
					if stepWin > 0 {
						stats.baseWinTimes++
					}
					stats.totalBet += float64(_baseMultiplier)
				}

				if !isFree || svc.scene.NextStage == _spinTypeBase {
					resetBetServiceForNextRound(svc)
					if stats.baseRounds%progressStep == 0 {
						printBenchmarkProgress(buf, stats, start)
						fmt.Print(buf.String())
					}
					break
				}
				continueCount = 0
			}
		}
	}

	printBenchmarkSummary(buf, stats, start)
	fmt.Print(buf.String())
}

func printBenchmarkProgress(buf *strings.Builder, stats *rtpStats, start time.Time) {
	if stats.baseRounds == 0 || stats.totalBet == 0 {
		return
	}
	freeRoundsSafe := max(stats.freeRounds, 1)
	avgFreePerTrigger := safeDiv(stats.freeRounds, stats.freeTriggerCount)
	buf.Reset()
	fprintf(buf, "\rRuntime=%d baseRtp=%.4f%%,baseWinRate=%.4f%% freeRtp=%.4f%% freeWinRate=%.4f%%, freeTriggerRate=%.4f%% avgFree=%.4f Rtp=%.4f%% \n",
		stats.baseRounds,
		float64(stats.baseWin)*100/stats.totalBet,
		safeDiv(stats.baseWinTimes*100, stats.baseRounds),
		float64(stats.freeWin)*100/stats.totalBet,
		safeDiv(stats.freeWinTimes*100, freeRoundsSafe),
		safeDiv(stats.freeTriggerCount*100, stats.baseRounds),
		avgFreePerTrigger,
		float64(stats.totalWin)*100/stats.totalBet,
	)
	fprintf(buf, "\rtotalWin-%d freeWin=%d,baseWin=%d ,baseWinTime-%d ,freeTime=%d, freeRound-%d ,freeWinTime-%d, elapsed=%v\n",
		stats.totalWin, stats.freeWin, stats.baseWin, stats.baseWinTimes, stats.freeTime, stats.freeRounds, stats.freeWinTimes, time.Since(start).Round(time.Second))
}

func printBenchmarkSummary(buf *strings.Builder, stats *rtpStats, start time.Time) {
	if stats.baseRounds == 0 || stats.totalBet == 0 {
		fprintf(buf, "No data collected for RTP benchmark.\n")
		return
	}
	w := func(format string, args ...interface{}) { fprintf(buf, format, args...) }
	elapsed := time.Since(start)
	speed := safeDiv(stats.baseRounds, int64(elapsed.Seconds()))
	w("\n运行局数: %d，用时: %v，速度: %.0f 局/秒\n\n", stats.baseRounds, elapsed.Round(time.Second), speed)

	baseRTP := float64(stats.baseWin) * 100 / stats.totalBet
	freeRTP := float64(stats.freeWin) * 100 / stats.totalBet
	totalRTP := float64(stats.totalWin) * 100 / stats.totalBet
	baseWinRate := safeDiv(stats.baseWinTimes*100, stats.baseRounds)
	freeWinRate := safeDiv(stats.freeWinTimes*100, stats.freeRounds)
	freeTriggerRate := safeDiv(stats.freeTriggerCount*100, stats.baseRounds)
	avgFreePerRound := safeDiv(stats.freeRounds, stats.baseRounds)
	avgFreePerTrigger := safeDiv(stats.freeRounds, stats.freeTriggerCount)

	w("\n[基础模式统计]\n")
	w("基础模式总游戏局数: %d\n", stats.baseRounds)
	w("基础模式总投注(倍数): %.2f\n", stats.totalBet)
	w("基础模式总奖金: %.2f\n", float64(stats.baseWin))
	w("基础模式RTP: %.2f%%\n", baseRTP)
	w("基础模式Re-spin触发次数: %d\n", stats.freeTime)
	w("基础模式触发Re-spin比例: %.2f%%\n", freeTriggerRate)
	w("基础模式平均每局免费次数: %.2f\n", avgFreePerRound)
	w("基础模式中奖率: %.2f%%\n", baseWinRate)
	w("基础模式中奖局数: %d\n", stats.baseWinTimes)

	w("\n[免费模式统计]\n")
	w("免费模式总游戏局数: %d\n", stats.freeRounds)
	w("免费模式总奖金: %.2f\n", float64(stats.freeWin))
	w("免费模式RTP: %.2f%%\n", freeRTP)
	w("免费模式中奖率: %.2f%%\n", freeWinRate)
	w("免费模式中奖局数: %d\n", stats.freeWinTimes)
	w("\n[Re-spin触发效率]\n")
	w("  总免费游戏次数: %d | 总触发次数: %d\n", stats.freeRounds, stats.freeTriggerCount)
	w("  平均每次触发获得免费次数: %.2f\n", avgFreePerTrigger)

	w("\n[总计]\n")
	w("总回报率(RTP): %.2f%%\n", totalRTP)
	w("总投注金额: %.2f\n", stats.totalBet)
	w("总奖金金额: %.2f\n\n", float64(stats.totalWin))
}

func newTestService() *betOrderService {
	return &betOrderService{
		req: &request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     GameID,
			BaseMoney:  1,
			Multiple:   1,
			Purchase:   0,
			Review:     0,
			Merchant:   "TestMerchant",
			Member:     "TestMember",
		},
		merchant: &merchant.Merchant{
			ID:       20020,
			Merchant: "TestMerchant",
		},
		member: &member.Member{
			ID:         1,
			MemberName: "TestMember",
			Balance:    10000000,
			Currency:   "USD",
		},
		game: &game.Game{
			ID: GameID,
		},
		client: &client.Client{
			MemberId:   1,
			Member:     "TestMember",
			GameId:     GameID,
			Timestamp:  time.Now().Unix(),
			ActivityId: 0,
			ClientOfGame: &client.ClientOfGame{
				HeadMultipleTimes: []uint64{},
				TimeList:          []int{},
			},
			SliceSlow:        []int64{},
			ClientOfFreeGame: &client.ClientOfFreeGame{},
			ClientGameCache: &client.ClientGameCache{
				ExpiredTime: 90 * 24 * 3600 * time.Second,
			},
		},
		lastOrder:      nil,
		scene:          &SpinSceneData{Stage: _spinTypeBase},
		gameOrder:      nil,
		bonusAmount:    decimal.Decimal{},
		betAmount:      decimal.Decimal{},
		amount:         decimal.Decimal{},
		stepMultiplier: 0,
		debug:          rtpDebugData{open: true},
	}
}

// resetBetServiceForNextRound 在 RTP 长测每局结束后重置会话状态，保留已解析的 gameConfig。
// TestRtp2 若每局 newTestService+initGameConfigs 会重复 JSON 解析，耗时数量级变差。
func resetBetServiceForNextRound(s *betOrderService) {
	s.scene.Stage = _spinTypeBase
	s.scene.NextStage = 0
	s.scene.SceneFreeGame.Reset()
	s.stepMultiplier = 0
	s.isFreeRound = false
	s.state = 0
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
