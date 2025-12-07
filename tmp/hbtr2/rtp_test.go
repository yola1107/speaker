package hbtr2

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

	var (
		err                                                    error
		baseRounds, freeRounds                                 int64
		baseWin, freeWin, totalBet, totalWin                   float64
		baseWinTimes, freeWinTimes, freeTriggerCount, freeTime int64
		freeRoundWin, roundWin                                 float64
	)

	for baseRounds < _benchmarkRounds {
		if err = svc.baseSpin(); err != nil {
			panic(err)
		}

		stepWin := float64(svc.stepMultiplier) // svc.bonusAmount.Round(2).InexactFloat64()
		roundWin += stepWin
		totalWin += stepWin

		if svc.isFreeRound {
			freeWin += stepWin
			freeRoundWin += stepWin
		} else {
			baseWin += stepWin
		}

		if svc.isRoundOver {
			if svc.isFreeRound {
				freeRounds++
				if freeRoundWin > 0 {
					freeWinTimes++
				}
				freeRoundWin = 0
			} else {
				baseRounds++
				if roundWin > 0 {
					baseWinTimes++
				}
				// 基础模式回合结束时，如果触发了免费游戏
				if svc.addFreeTime > 0 {
					freeTriggerCount++
					freeTime++
				}
				totalBet += float64(_baseMultiplier)
			}
			roundWin = 0
		}

		if svc.isRoundOver && svc.scene.FreeNum <= 0 {
			resetBetServiceForNextRound(svc)
			freeRoundWin = 0

			if baseRounds%progressStep == 0 {
				printBenchmarkProgress(buf, baseRounds, totalBet, baseWin, freeWin, totalWin, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, freeTime, start)
				fmt.Print(buf.String())
			}
		}
	}

	printBenchmarkSummary(buf, baseRounds, totalBet, baseWin, freeWin, totalWin, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, freeTime, start)
	fmt.Print(buf.String())
}

func printBenchmarkProgress(buf *strings.Builder, baseRounds int64, totalBet, baseWin, freeWin, totalWin float64, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, freeTime int64, start time.Time) {
	if baseRounds == 0 || totalBet == 0 {
		return
	}
	freeRoundsSafe := max(freeRounds, 1)
	avgFreePerTrigger := safeDiv(freeRounds, freeTriggerCount)
	buf.Reset()
	fprintf(buf, "\rRuntime=%d baseRtp=%.4f%%,baseWinRate=%.4f%% freeRtp=%.4f%% freeWinRate=%.4f%%, freeTriggerRate=%.4f%% avgFree=%.4f Rtp=%.4f%% \n",
		baseRounds,
		baseWin*100/totalBet,
		safeDiv(baseWinTimes*100, baseRounds),
		freeWin*100/totalBet,
		safeDiv(freeWinTimes*100, freeRoundsSafe),
		safeDiv(freeTriggerCount*100, baseRounds),
		avgFreePerTrigger,
		(baseWin+freeWin)*100/totalBet,
	)
	fprintf(buf, "\rtotalWin-%.0f freeWin=%.0f,baseWin-%.0f ,baseWinTime-%d ,freeTime-%d, freeRound-%d ,freeWinTime-%d, elapsed=%v\n",
		totalWin, freeWin, baseWin, baseWinTimes, freeTime, freeRounds, freeWinTimes, time.Since(start).Round(time.Second))
}

func printBenchmarkSummary(buf *strings.Builder, baseRounds int64, totalBet, baseWin, freeWin, totalWin float64, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, freeTime int64, start time.Time) {
	if baseRounds == 0 || totalBet == 0 {
		fprintf(buf, "No data collected for RTP benchmark.\n")
		return
	}
	w := func(format string, args ...interface{}) { fprintf(buf, format, args...) }
	elapsed := time.Since(start)
	speed := safeDiv(baseRounds, int64(elapsed.Seconds()))
	w("\n运行局数: %d，用时: %v，速度: %.0f 局/秒\n\n", baseRounds, elapsed.Round(time.Second), speed)

	baseRTP := baseWin * 100 / totalBet
	freeRTP := freeWin * 100 / totalBet
	totalRTP := (baseWin + freeWin) * 100 / totalBet
	baseWinRate := safeDiv(baseWinTimes*100, baseRounds)
	freeWinRate := safeDiv(freeWinTimes*100, freeRounds)
	freeTriggerRate := safeDiv(freeTriggerCount*100, baseRounds)
	avgFreePerRound := safeDiv(freeRounds, baseRounds)
	avgFreePerTrigger := safeDiv(freeRounds, freeTriggerCount)

	w("\n[基础模式统计]\n")
	w("基础模式总游戏局数: %d\n", baseRounds)
	w("基础模式总投注(倍数): %.2f\n", totalBet)
	w("基础模式总奖金: %.2f\n", baseWin)
	w("基础模式RTP: %.2f%%\n", baseRTP)
	w("基础模式免费局触发次数: %d\n", freeTime)
	w("基础模式触发免费局比例: %.2f%%\n", freeTriggerRate)
	w("基础模式平均每局免费次数: %.2f\n", avgFreePerRound)
	w("基础模式中奖率: %.2f%%\n", baseWinRate)
	w("基础模式中奖局数: %d\n", baseWinTimes)

	w("\n[免费模式统计]\n")
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

func newBerService() *betOrderService {
	return &betOrderService{
		req: &request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     _gameID,
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
			ID: _gameID,
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
	s.gameMultiple = 1
	s.stepMultiplier = 0
	s.lineMultiplier = 0
	s.scatterCount = 0
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
