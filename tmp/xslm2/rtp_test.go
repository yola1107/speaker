package xslm2

import (
	"fmt"
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
				printBenchmarkProgress(stats, start)
			}
		}
	}

	fmt.Println()
	printBenchmarkSummary(stats, start)
}

func printBenchmarkProgress(stats *rtpStats, start time.Time) {
	if stats.baseRounds == 0 || stats.totalBet == 0 {
		return
	}

	freeRoundsSafe := max(stats.freeRounds, 1)
	avgFreePerTrigger := float64(0)
	if stats.freeTime > 0 {
		avgFreePerTrigger = float64(stats.freeRounds) / float64(stats.freeTime)
	}

	fmt.Printf("Runtime-%d baseRtp=%.4f%%,baseWinRate=%.4f%% freeRtp=%.4f%% freeWinRate=%.4f%%, freeTriggerRate=%.4f%% avgFree=%.4f Rtp=%.4f%%\n",
		stats.baseRounds,
		calculateRtp(stats.baseWin, stats.baseRounds, _baseMultiplier),
		calculateRtp(stats.baseWinTime, stats.baseRounds, 1),
		calculateRtp(stats.freeWin, stats.baseRounds, _baseMultiplier),
		float64(stats.freeWinRounds)*100/float64(freeRoundsSafe),
		float64(stats.freeTime)*100/float64(stats.baseRounds),
		avgFreePerTrigger,
		calculateRtp(stats.totalWin, stats.baseRounds, _baseMultiplier),
	)
	fmt.Printf("\rtotalWin-%d freeWin=%d,baseWin=%d ,baseWinTime=%d ,freeTime=%d, freeRounds=%d ,freeWinRounds=%d, freeWinTime=%d, elapsed=%v\n",
		stats.totalWin, stats.freeWin, stats.baseWin, stats.baseWinTime, stats.freeTime,
		stats.freeRounds, stats.freeWinRounds, stats.freeWinTime, time.Since(start).Round(time.Second))
}

func printBenchmarkSummary(stats *rtpStats, start time.Time) {
	if stats.baseRounds == 0 || stats.totalBet == 0 {
		fmt.Println("No data collected for RTP benchmark.")
		return
	}

	elapsed := time.Since(start)
	fmt.Printf("运行局数: %d，用时: %v，速度: %.0f 局/秒\n\n",
		stats.baseRounds, elapsed.Round(time.Second), float64(stats.baseRounds)/elapsed.Seconds())

	baseRTP := calculateRtp(stats.baseWin, stats.baseRounds, _baseMultiplier)
	freeRTP := calculateRtp(stats.freeWin, stats.baseRounds, _baseMultiplier)
	totalRTP := calculateRtp(stats.totalWin, stats.baseRounds, _baseMultiplier)
	baseWinRate := float64(0)
	if stats.baseRounds > 0 {
		baseWinRate = float64(stats.baseWinTime) * 100 / float64(stats.baseRounds)
	}
	freeWinRate := float64(0)
	if stats.freeRounds > 0 {
		freeWinRate = float64(stats.freeWinRounds) * 100 / float64(stats.freeRounds)
	}
	freeTriggerRate := float64(0)
	if stats.baseRounds > 0 {
		freeTriggerRate = float64(stats.freeTime) * 100 / float64(stats.baseRounds)
	}
	avgFreePerRound := float64(0)
	if stats.baseRounds > 0 {
		avgFreePerRound = float64(stats.freeRounds) / float64(stats.baseRounds)
	}
	avgFreePerTrigger := float64(0)
	if stats.freeTime > 0 {
		avgFreePerTrigger = float64(stats.freeRounds) / float64(stats.freeTime)
	}

	fmt.Println("[基础模式统计]")
	fmt.Printf("基础模式总游戏局数: %d\n", stats.baseRounds)
	fmt.Printf("基础模式总投注(倍数): %.2f\n", stats.totalBet)
	fmt.Printf("基础模式总奖金: %.2f\n", float64(stats.baseWin))
	fmt.Printf("基础模式RTP: %.2f%%\n", baseRTP)
	fmt.Printf("基础模式免费局触发次数: %d\n", stats.freeTime)
	fmt.Printf("基础模式触发免费局比例: %.2f%%\n", freeTriggerRate)
	fmt.Printf("基础模式平均每局免费次数: %.2f\n", avgFreePerRound)
	fmt.Printf("基础模式中奖率: %.2f%%\n", baseWinRate)
	fmt.Printf("基础模式中奖局数: %d\n", stats.baseWinTime)

	fmt.Println()
	fmt.Println("[免费模式统计]")
	fmt.Printf("免费模式总游戏局数: %d\n", stats.freeRounds)
	fmt.Printf("免费模式总游戏步数: %d\n", stats.totalFreeGameCount)
	fmt.Printf("免费模式总奖金: %.2f\n", float64(stats.freeWin))
	fmt.Printf("免费模式RTP: %.2f%%\n", freeRTP)
	fmt.Printf("免费模式中奖率: %.2f%%\n", freeWinRate)
	fmt.Printf("免费模式中奖局数: %d\n", stats.freeWinRounds)
	fmt.Printf("免费模式中奖步数: %d\n", stats.freeWinTime)
	if stats.totalFreeGameCount > 0 {
		fmt.Printf("免费模式中奖率(按步): %.2f%%\n", float64(stats.freeWinTime)*100/float64(stats.totalFreeGameCount))
	}

	if stats.freeRounds > 0 {
		fmt.Println()
		printFemaleStateStats(stats)
	}

	fmt.Println()
	fmt.Println("[免费触发效率]")
	fmt.Printf("总免费游戏次数: %d | 总触发次数: %d\n", stats.freeRounds, stats.freeTime)
	if stats.freeTime > 0 {
		fmt.Printf("平均每次触发获得免费次数: %.2f\n", avgFreePerTrigger)
	} else {
		fmt.Printf("平均每次触发获得免费次数: 0\n")
	}

	fmt.Println()
	fmt.Println("[总计]")
	fmt.Printf("总回报率(RTP): %.2f%%\n", totalRTP)
}

func printFemaleStateStats(stats *rtpStats) {
	fmt.Println("[免费模式女性符号状态统计]")
	totalStateCount := int64(0)
	for i := 0; i < 10; i++ {
		totalStateCount += stats.freeFemaleStateCount[i]
	}
	fmt.Printf("总统计次数: %d (应该等于免费模式总游戏局数: %d)\n", totalStateCount, stats.freeRounds)
	for i := 1; i < 9; i++ {
		count := stats.freeFemaleStateCount[i]
		percentage := float64(0)
		if stats.freeRounds > 0 {
			percentage = float64(count) * 100 / float64(stats.freeRounds)
		}
		fmt.Printf("状态 %s: %.2f%% (%d次)\n", stateNames[i], percentage, count)
	}
	fmt.Println()
	fmt.Println("[免费模式女性 key 赢分统计]")
	for i := 0; i < len(stats.femaleKeyWins); i++ {
		winSum := stats.femaleKeyWins[i]
		count := stats.freeFemaleStateCount[i]
		avg := float64(0)
		if count > 0 {
			avg = winSum / float64(count)
		}
		avgBet := avg / float64(_baseMultiplier)
		fmt.Printf("key=%s | 总赢分=%.2f | 次数=%d | 平均倍数=%.4f\n",
			stateNames[i], winSum, count, avgBet)
	}
}

func calculateRtp(win, rounds, multiplier int64) float64 {
	if rounds == 0 || multiplier == 0 {
		return 0
	}
	return float64(win) / float64(rounds*multiplier) * 100.0
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
