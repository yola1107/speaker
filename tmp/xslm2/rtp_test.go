package xslm2

import (
	"fmt"
	"strings"
	"sync"
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
	benchTestRounds       int64 = 1e8
	benchProgressInterval int64 = 1e7
)

var stateNames = []string{"base", "000", "001", "010", "100", "011", "101", "110", "111", "008"}

// rtpStats RTP统计数据结构
type rtpStats struct {
	// 基础模式统计
	baseRounds  int64
	baseWin     int64
	baseWinTime int64
	totalBet    float64

	// 免费模式统计
	freeRounds         int64
	freeWin            int64
	freeWinRounds      int64
	freeWinTime        int64
	totalFreeGameCount int64

	// 触发统计
	freeTime int64

	// 总统计
	totalWin int64

	// 女性符号状态统计
	freeFemaleStateCount [10]int64
	femaleKeyWins        [10]float64
}

func TestRtp(t *testing.T) {
	betService := newBerService()
	stats := &rtpStats{}
	start := time.Now()
	buf := &strings.Builder{}
	progressStep := int64(min(benchProgressInterval, benchTestRounds))

	var freeRoundWin float64
	var roundWin float64

	for stats.baseRounds < benchTestRounds {
		betService.isFirst = betService.scene.Steps == 0
		isFirst := betService.isFirst

		err := betService.baseSpin()
		if err != nil {
			panic(err)
		}

		isFree := betService.isFreeRound
		if isFirst && isFree {
			stats.freeFemaleStateCount[betService.scene.SymbolRoller[0].Real]++
		}

		stepWin := betService.stepMultiplier
		stats.totalWin += stepWin
		roundWin += float64(stepWin)

		if isFree {
			stats.freeWin += stepWin
			freeRoundWin += float64(stepWin)
			stats.totalFreeGameCount++

			if stepWin > 0 {
				stats.freeWinTime++
			}

			if betService.isRoundOver {
				stats.freeRounds++
				if freeRoundWin > 0 {
					stats.freeWinRounds++
				}
				stats.femaleKeyWins[betService.scene.SymbolRoller[0].Real] += roundWin
				freeRoundWin = 0
				roundWin = 0
			}
		} else {
			stats.baseWin += stepWin

			if betService.isRoundOver {
				stats.baseRounds++
				if roundWin > 0 {
					stats.baseWinTime++
				}
				if betService.newFreeRoundCount > 0 {
					stats.freeTime++
				}
				stats.totalBet += float64(_baseMultiplier)
				roundWin = 0
			}
		}

		if betService.isRoundOver && betService.scene.FreeNum <= 0 {
			resetBetServiceForNextRound(betService)
			freeRoundWin = 0

			if stats.baseRounds%progressStep == 0 {
				printBenchmarkProgress(buf, stats, start)
				fmt.Print(buf.String())
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
	avgFreePerTrigger := safeDivide(stats.freeRounds, stats.freeTime)
	buf.Reset()
	fprintf(buf, "\rRuntime=%d baseRtp=%.4f%%,baseWinRate=%.4f%% freeRtp=%.4f%% freeWinRate=%.4f%%, freeTriggerRate=%.4f%% avgFree=%.4f Rtp=%.4f%% \n",
		stats.baseRounds,
		safeDivide(stats.baseWin*100, stats.baseRounds*_baseMultiplier),
		safeDivide(stats.baseWinTime*100, stats.baseRounds),
		safeDivide(stats.freeWin*100, stats.baseRounds*_baseMultiplier),
		safeDivide(stats.freeWinRounds*100, freeRoundsSafe),
		safeDivide(stats.freeTime*100, stats.baseRounds),
		avgFreePerTrigger,
		safeDivide(stats.totalWin*100, stats.baseRounds*_baseMultiplier),
	)
	fprintf(buf, "\rtotalWin-%d freeWin=%d,baseWin=%d ,baseWinTime=%d ,freeTime=%d, freeRounds=%d ,freeWinRounds=%d, freeWinTime=%d, elapsed=%v\n",
		stats.totalWin, stats.freeWin, stats.baseWin, stats.baseWinTime, stats.freeTime,
		stats.freeRounds, stats.freeWinRounds, stats.freeWinTime, time.Since(start).Round(time.Second))
}

func printBenchmarkSummary(buf *strings.Builder, stats *rtpStats, start time.Time) {
	if stats.baseRounds == 0 || stats.totalBet == 0 {
		buf.WriteString("No data collected for RTP benchmark.\n")
		return
	}

	w := func(format string, args ...interface{}) { fprintf(buf, format, args...) }
	elapsed := time.Since(start)
	speed := safeDivide(stats.baseRounds, int64(elapsed.Seconds()))
	w("\n运行局数: %d，用时: %v，速度: %.0f 局/秒\n\n", stats.baseRounds, elapsed.Round(time.Second), speed)

	baseRTP := safeDivide(stats.baseWin*100, stats.baseRounds*_baseMultiplier)
	freeRTP := safeDivide(stats.freeWin*100, stats.baseRounds*_baseMultiplier)
	totalRTP := safeDivide(stats.totalWin*100, stats.baseRounds*_baseMultiplier)
	baseWinRate := safeDivide(stats.baseWinTime*100, stats.baseRounds)
	freeWinRate := safeDivide(stats.freeWinRounds*100, stats.freeRounds)
	freeTriggerRate := safeDivide(stats.freeTime*100, stats.baseRounds)
	avgFreePerRound := safeDivide(stats.freeRounds, stats.baseRounds)
	avgFreePerTrigger := safeDivide(stats.freeRounds, stats.freeTime)

	w("\n[总计]\n")
	w("总回报率(RTP): %.2f%%\n\n", totalRTP)

	w("[基础模式统计]\n")
	w("基础模式总游戏局数: %d\n", stats.baseRounds)
	w("基础模式总投注(倍数): %.2f\n", stats.totalBet)
	w("基础模式总奖金: %.2f\n", float64(stats.baseWin))
	w("基础模式RTP: %.2f%%\n", baseRTP)
	w("基础模式免费局触发次数: %d\n", stats.freeTime)
	w("基础模式触发免费局比例: %.2f%%\n", freeTriggerRate)
	w("基础模式平均每局免费次数: %.2f\n", avgFreePerRound)
	w("基础模式中奖率: %.2f%%\n", baseWinRate)
	w("基础模式中奖局数: %d\n", stats.baseWinTime)

	w("\n[免费模式统计]\n")
	w("免费模式总游戏局数: %d\n", stats.freeRounds)
	w("免费模式总游戏步数: %d\n", stats.totalFreeGameCount)
	w("免费模式总奖金: %.2f\n", float64(stats.freeWin))
	w("免费模式RTP: %.2f%%\n", freeRTP)
	w("免费模式中奖率: %.2f%%\n", freeWinRate)
	w("免费模式中奖局数: %d\n", stats.freeWinRounds)
	w("免费模式中奖步数: %d\n", stats.freeWinTime)
	if stats.freeRounds > 0 {
		printFemaleStateStats(w, stats.freeRounds, stats.freeFemaleStateCount, stats.femaleKeyWins)
	}

	w("\n[免费触发效率]\n")
	w("总免费游戏次数: %d | 总触发次数: %d\n", stats.freeRounds, stats.freeTime)
	w("平均每次触发获得免费次数: %.2f\n", avgFreePerTrigger)
}

func printFemaleStateStats(w func(string, ...interface{}), freeRounds int64, freeFemaleStateCount [10]int64, femaleKeyWins [10]float64) {
	w("\n[免费模式女性符号状态统计]\n")
	totalStateCount := int64(0)
	for i := 0; i < 10; i++ {
		totalStateCount += freeFemaleStateCount[i]
	}
	w("  总统计次数: %d (应该等于免费模式总游戏局数: %d)\n", totalStateCount, freeRounds)
	for i := 1; i < 9; i++ {
		count := freeFemaleStateCount[i]
		w("  状态 %s: %.4f%% (%d次)\n", stateNames[i], safeDivide(count*100, freeRounds), count)
	}
	w("\n[免费模式女性 key 赢分统计]\n")
	for i := 1; i < 9; i++ {
		winSum := femaleKeyWins[i]
		count := freeFemaleStateCount[i]
		avg := safeDivide(int64(winSum), count)
		avgBet := avg / float64(_baseMultiplier)
		w("  key=%s | 总赢分=%.2f | 次数=%d | 平均倍数=%.4f\n",
			stateNames[i], winSum, count, avgBet)
	}
}

func resetBetServiceForNextRound(s *betOrderService) {
	s.scene = &SpinSceneData{}
	s.stepMultiplier = 0
	s.lineMultiplier = 0
	s.treasureCount = 0
	s.newFreeRoundCount = 0
	s.isRoundOver = false
	s.client.IsRoundOver = false
	s.client.ClientOfFreeGame.Reset()
	s.client.ClientOfFreeGame.ResetGeneralWinTotal()
	s.client.ClientOfFreeGame.ResetRoundBonus()
	s.client.SetLastMaxFreeNum(0)
}

func newBerService() *betOrderService {
	s := &betOrderService{
		req: &request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     _gameID,
			BaseMoney:  1,
			Multiple:   1,
			Purchase:   0,
			Review:     0,
			Merchant:   "Jack23",
			Member:     "Jack23",
		},
		merchant: &merchant.Merchant{
			ID:       20020,
			Merchant: "Jack23",
		},
		member: &member.Member{
			ID:         1,
			MemberName: "Jack23",
			Balance:    10000000,
			Currency:   "USD",
		},
		game: &game.Game{
			ID: _gameID,
		},
		client: &client.Client{
			MemberId:         1,
			Member:           "Jack23",
			NickName:         "Jack23",
			Merchant:         "Jack23",
			GameId:           _gameID,
			Timestamp:        time.Now().Unix(),
			ActivityId:       0,
			Lock:             sync.Mutex{},
			BetLock:          sync.Mutex{},
			SyncLock:         sync.Mutex{},
			MaxFreeNum:       0,
			LastMaxFreeNum:   0,
			TreasureNum:      0,
			IsRoundOver:      false,
			ClientOfGame:     &client.ClientOfGame{},
			SliceSlow:        []int64{},
			ClientOfFreeGame: &client.ClientOfFreeGame{},
			ClientGameCache:  &client.ClientGameCache{ExpiredTime: 90 * 24 * 3600 * time.Second},
		},
		lastOrder:      nil,
		gameRedis:      nil,
		scene:          &SpinSceneData{},
		gameOrder:      nil,
		bonusAmount:    decimal.Decimal{},
		betAmount:      decimal.Decimal{},
		amount:         decimal.Decimal{},
		orderSN:        "",
		parentOrderSN:  "",
		freeOrderSN:    "",
		gameType:       0,
		stepMultiplier: 0,
		debug:          rtpDebugData{open: true},
	}
	s.initGameConfigs()
	return s
}

func fprintf(buf *strings.Builder, format string, args ...interface{}) {
	_, _ = fmt.Fprintf(buf, format, args...)
}

func safeDivide(numerator, denominator int64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
