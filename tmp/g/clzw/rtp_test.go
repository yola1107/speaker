package clzw

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"

	"github.com/shopspring/decimal"
	"golang.org/x/exp/constraints"
)

const (
	_benchmarkRounds           int64 = 1e8
	_benchmarkProgressInterval int64 = 1e7

	_enablePurchaseModeRtp = false
	_purchaseAmountRtp     = 1 * 1 * _baseMultiplier * _buyFreeMultiple
)

func TestRtp(t *testing.T) {
	s := newRtpBetService()
	s.initGameConfigs()
	start := time.Now()
	buf := &strings.Builder{}
	progressStep := int64(min(_benchmarkProgressInterval, _benchmarkRounds))

	var (
		err                                          error
		baseRounds, freeRounds                       int64
		baseWin, freeWin, totalBet, totalWin         float64
		baseWinTimes, freeWinTimes, freeTriggerCount int64
		freeRoundWin, roundWin                       float64
	)

	for {
		if err = s.syncGameStage(); err != nil {
			t.Fatal(err)
		}
		s.autoPurchase(_enablePurchaseModeRtp, _purchaseAmountRtp)
		if err = s.baseSpin(); err != nil {
			t.Fatal(err)
		}

		stepWin := float64(s.stepMultiplier)
		roundWin += stepWin
		totalWin += stepWin
		if s.isFreeRound {
			freeWin += stepWin
			freeRoundWin += stepWin
		} else {
			baseWin += stepWin
		}

		if s.isRoundOver {
			if s.isFreeRound {
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
				if s.addFreeTime > 0 {
					freeTriggerCount++
				}
				if _enablePurchaseModeRtp && s.req.Purchase > 0 {
					totalBet += float64(_purchaseAmountRtp)
				} else {
					totalBet += float64(_baseMultiplier)
				}
			}
			roundWin = 0
		}

		if s.isRoundOver && s.scene.FreeNum <= 0 {
			resetRtpBetCounters(s)

			if baseRounds%progressStep == 0 {
				printBenchmarkProgress(buf, baseRounds, totalBet, baseWin, freeWin, totalWin, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, start)
				fmt.Print(buf.String())
			}
			if baseRounds >= _benchmarkRounds {
				break
			}
		}
	}

	printBenchmarkSummary(buf, baseRounds, totalBet, baseWin, freeWin, totalWin, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, start)
	fmt.Print(buf.String())
}

func printBenchmarkProgress(buf *strings.Builder, baseRounds int64, totalBet, baseWin, freeWin, totalWin float64, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount int64, start time.Time) {
	if baseRounds == 0 || totalBet == 0 {
		return
	}
	buf.Reset()
	fprintf(buf, "\rRuntime=%d baseRtp=%.4f%%,baseWinRate=%.4f%% freeRtp=%.4f%% freeWinRate=%.4f%%, freeTriggerRate=%.4f%% avgFree=%.4f Rtp=%.4f%% \n",
		baseRounds,
		baseWin*100/totalBet,
		safeDiv(baseWinTimes*100, baseRounds),
		freeWin*100/totalBet,
		safeDiv(freeWinTimes*100, freeRounds),
		safeDiv(freeTriggerCount*100, baseRounds),
		safeDiv(freeRounds, freeTriggerCount),
		(baseWin+freeWin)*100/totalBet,
	)
	fprintf(buf, "\rtotalWin-%.0f freeWin=%.0f,baseWin-%.0f ,baseWinTime-%d ,freeTime-%d, freeRound-%d ,freeWinTime-%d, elapsed=%v\n",
		totalWin, freeWin, baseWin, baseWinTimes, freeTriggerCount, freeRounds, freeWinTimes, time.Since(start).Round(time.Second))
}

func printBenchmarkSummary(buf *strings.Builder, baseRounds int64, totalBet, baseWin, freeWin, totalWin float64, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount int64, start time.Time) {
	if baseRounds == 0 || totalBet == 0 {
		fprintf(buf, "No data collected for RTP benchmark.\n")
		return
	}
	w := func(format string, args ...interface{}) { fprintf(buf, format, args...) }
	elapsed := time.Since(start)
	speed := safeDiv(baseRounds, int64(elapsed.Seconds()))
	w("\n运行局数: %d，用时: %v，速度: %.0f 局/秒\n", baseRounds, elapsed.Round(time.Second), speed)
	if _enablePurchaseModeRtp {
		w("(购买模式: 每基础新局 autoPurchase，注入时计一次投注 %.0f 倍数)\n", float64(_purchaseAmountRtp))
	}

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
	w("基础模式RTP: %.4f%%\n", baseRTP)
	w("基础模式中奖率: %.4f%%\n", baseWinRate)
	w("基础模式中奖局数: %d\n", baseWinTimes)
	w("基础模式触发免费次数: %d\n", freeTriggerCount)
	w("基础模式触发免费比例: %.4f%%\n", freeTriggerRate)
	w("基础模式平均每局免费次数: %.4f\n", avgFreePerRound)

	w("\n[免费模式统计]\n")
	w("免费模式总游戏局数: %d\n", freeRounds)
	w("免费模式总奖金: %.2f\n", freeWin)
	w("免费模式RTP: %.4f%%\n", freeRTP)
	w("免费模式中奖率: %.4f%%\n", freeWinRate)
	w("免费模式中奖局数: %d\n", freeWinTimes)
	w("\n[免费触发效率]\n")
	w("  总免费游戏次数: %d | 总触发次数: %d\n", freeRounds, freeTriggerCount)
	w("  平均每次触发获得免费次数: %.4f\n", avgFreePerTrigger)

	w("\n[总计]\n")
	w("总回报率(RTP): %.4f%%\n", totalRTP)
	w("总投注金额: %.2f\n", totalBet)
	w("总奖金金额: %.2f\n\n", totalWin)
}

func newRtpBetService() *betOrderService {
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

func resetRtpBetCounters(s *betOrderService) {
	s.stepMultiplier = 0
	s.lineMultiplier = 0
	s.scatterCount = 0
}

func fprintf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func safeDiv[T constraints.Integer | constraints.Float](a, b T) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func (s *betOrderService) autoPurchase(enable bool, purchaseAmount int64) {
	if enable && purchaseAmount > 0 && s.scene.Stage == _spinTypeBase && s.scene.Steps == 0 {
		s.req.Purchase = purchaseAmount
		return
	}
	s.req.Purchase = 0
}
