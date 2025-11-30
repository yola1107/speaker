package mahjong

import (
	"fmt"
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
	progressStep := int64(min(_benchmarkProgressInterval, _benchmarkRounds))

	var (
		baseRounds, freeRounds                                 int64
		baseWin, freeWin, totalBet, totalWin                   float64
		baseWinTimes, freeWinTimes, freeTriggerCount, freeTime int64
		freeRoundWin, roundWin                                 float64
	)

	for baseRounds < _benchmarkRounds {
		wasFreeBeforeSpin := svc.isFreeRound

		ret, err := svc.baseSpin()
		if err != nil {
			panic(err)
		}

		stepWin := ret.stepWin
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
				if !wasFreeBeforeSpin && ret.winInfo.State == runStateFreeGame {
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
				printBenchmarkProgress(baseRounds, totalBet, baseWin, freeWin, totalWin, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, freeTime, start)
			}
		}
	}

	fmt.Println()
	printBenchmarkSummary(baseRounds, totalBet, baseWin, freeWin, totalWin, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, freeTime, start)
}

func printBenchmarkProgress(baseRounds int64, totalBet, baseWin, freeWin, totalWin float64, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, freeTime int64, start time.Time) {
	if baseRounds == 0 || totalBet == 0 {
		return
	}

	freeRoundsSafe := max(freeRounds, 1)
	avgFreePerTrigger := float64(0)
	if freeTriggerCount > 0 {
		avgFreePerTrigger = float64(freeRounds) / float64(freeTriggerCount)
	}

	// 第一行：与 rtp_test.go 格式保持一致，并添加 avgFree
	fmt.Printf("Runtime-%d baseRtp=%.4f%%,baseWinRate-%.4f%% freeRtp-%.4f%% freeWinRate-%.4f%%, freeTriggerRate-%.4f%% avgFree=%.4f Rtp-%.4f%%\n",
		baseRounds,
		baseWin*100/totalBet,
		float64(baseWinTimes)*100/float64(baseRounds),
		freeWin*100/totalBet,
		float64(freeWinTimes)*100/float64(freeRoundsSafe),
		float64(freeTriggerCount)*100/float64(baseRounds),
		avgFreePerTrigger,
		(baseWin+freeWin)*100/totalBet,
	)
	// 第二行：详细数据（与 rtp_test.go 保持一致）
	fmt.Printf("\rtotalWin-%.0f freeWin=%.0f,baseWin-%.0f ,baseWinTime-%d ,freeTime-%d, freeRound-%d ,freeWinTime-%d, elapsed=%v\n",
		totalWin, freeWin, baseWin, baseWinTimes, freeTime, freeRounds, freeWinTimes, time.Since(start).Round(time.Second))
}

func printBenchmarkSummary(baseRounds int64, totalBet, baseWin, freeWin, totalWin float64, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, freeTime int64, start time.Time) {
	if baseRounds == 0 || totalBet == 0 {
		fmt.Println("No data collected for RTP benchmark.")
		return
	}

	baseRTP := baseWin * 100 / totalBet
	freeRTP := freeWin * 100 / totalBet
	totalRTP := (baseWin + freeWin) * 100 / totalBet
	baseWinRate := float64(baseWinTimes) * 100 / float64(baseRounds)
	freeWinRate := float64(0)
	if freeRounds > 0 {
		freeWinRate = float64(freeWinTimes) * 100 / float64(freeRounds)
	}
	freeTriggerRate := float64(freeTriggerCount) * 100 / float64(baseRounds)
	avgFreePerRound := float64(0)
	if baseRounds > 0 {
		avgFreePerRound = float64(freeRounds) / float64(baseRounds)
	}
	avgFreePerTrigger := float64(0)
	if freeTriggerCount > 0 {
		avgFreePerTrigger = float64(freeRounds) / float64(freeTriggerCount)
	}

	fmt.Println("[基础模式统计]")
	fmt.Printf("基础模式总游戏局数: %d\n", baseRounds)
	fmt.Printf("基础模式总投注(倍数): %.2f\n", totalBet)
	fmt.Printf("基础模式总奖金: %.2f\n", baseWin)
	fmt.Printf("基础模式RTP: %.2f%%\n", baseRTP)
	fmt.Printf("基础模式免费局触发次数: %d\n", freeTriggerCount)
	fmt.Printf("基础模式触发免费局比例: %.2f%%\n", freeTriggerRate)
	fmt.Printf("基础模式平均每局免费次数: %.2f\n", avgFreePerRound)
	fmt.Printf("基础模式中奖率: %.2f%%\n", baseWinRate)
	fmt.Printf("基础模式中奖局数: %d\n", baseWinTimes)

	fmt.Println()
	fmt.Println("[免费模式统计]")
	fmt.Printf("免费模式总游戏局数: %d\n", freeRounds)
	fmt.Printf("免费模式总奖金: %.2f\n", freeWin)
	fmt.Printf("免费模式RTP: %.2f%%\n", freeRTP)
	fmt.Printf("免费模式中奖率: %.2f%%\n", freeWinRate)
	fmt.Printf("免费模式中奖局数: %d\n", freeWinTimes)

	fmt.Println()
	fmt.Println("[免费触发效率]")
	fmt.Printf("  总免费游戏次数: %d | 总触发次数: %d\n", freeRounds, freeTriggerCount)
	if freeTriggerCount > 0 {
		fmt.Printf("  平均每次触发获得免费次数: %.2f\n", avgFreePerTrigger)
	} else {
		fmt.Printf("  平均每次触发获得免费次数: 0\n")
	}

	fmt.Println()
	fmt.Println("[总计]")
	fmt.Printf("总回报率(RTP): %.2f%%\n", totalRTP)
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
	// 复用 scene 而不是重新分配（减少内存分配，提升性能）
	s.scene.Steps = 0
	s.scene.Stage = _spinTypeBase // 直接设置为基础模式，避免 handleStageTransition 每次检查
	s.scene.NextStage = 0
	s.scene.FreeNum = 0
	s.scene.BonusNum = 0
	s.scene.ScatterNum = 0
	s.scene.BonusState = 0
	s.scene.ContinueNum = 0
	s.scene.RoundMultiplier = 0
	// SymbolRoller 会在下次使用时重新初始化，无需清零

	if s.client.ClientOfFreeGame != nil {
		s.client.ClientOfFreeGame.FreeNum = 0
		s.client.ClientOfFreeGame.FreeTotalMoney = 0
		s.client.ClientOfFreeGame.BonusTimes = 0
		s.client.ClientOfFreeGame.BetAmount = 0
		s.client.ClientOfFreeGame.FreeTimes = 0
		s.client.ClientOfFreeGame.BonusState = 0
		s.client.ClientOfFreeGame.FreeType = 0
		s.client.ClientOfFreeGame.LastWinId = 0
		s.client.ClientOfFreeGame.LastMapId = 0
		s.client.ClientOfFreeGame.FreeMultiple = 0
		s.client.ClientOfFreeGame.GeneralWinTotal = 0
		s.client.ClientOfFreeGame.FreeClean = 0
	}
	s.lastOrder = nil
	s.gameOrder = nil
	s.bonusAmount = decimal.Decimal{}
	s.betAmount = decimal.Decimal{}
	s.amount = decimal.Decimal{}
	s.orderSN = ""
	s.parentOrderSN = ""
	s.freeOrderSN = ""
	s.stepMultiplier = 0
	s.isRoundOver = false
	s.isFreeRound = false
	s.gameMultiple = 1 // 重置为 1 而不是 0
	s.winInfos = nil
	s.nextSymbolGrid = int64Grid{}
	s.symbolGrid = int64Grid{}
	s.winGrid = int64Grid{}
}
