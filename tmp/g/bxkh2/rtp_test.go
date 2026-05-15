package bxkh2

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"
)

const (
	_benchmarkRounds           int64 = 10000000
	_benchmarkProgressInterval int64 = 1000000
)

func TestRtp(t *testing.T) {
	svc := newBerService()
	svc.initGameConfigs()
	buf := &strings.Builder{}

	progressStep := int64(max(min(_benchmarkProgressInterval, _benchmarkRounds), 1))
	var (
		err                                                    error
		baseRounds, freeRounds                                 int64
		baseWin, freeWin, totalBet, totalWin                   float64
		baseWinTimes, freeWinTimes, freeTriggerCount, freeTime int64
		freeRoundWin, roundWin                                 float64
	)
	start := time.Now()

	for baseRounds < _benchmarkRounds {
		if err = spinOneStep(svc); err != nil {
			panic(err)
		}

		stepWin := float64(svc.stepMultiplier)
		totalWin += stepWin
		roundWin += stepWin
		if svc.isFreeRound {
			freeWin += stepWin
			freeRoundWin += stepWin
		} else {
			baseWin += stepWin
		}

		if !svc.isRoundOver {
			continue
		}

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
			if svc.addFreeTime > 0 {
				freeTriggerCount++
				freeTime++
			}
			totalBet += float64(_baseMultiplier)
		}

		roundWin = 0
		if svc.scene.FreeNum <= 0 {
			resetBetServiceForNextRound(svc)
		}

		if baseRounds%progressStep == 0 {
			printBenchmarkProgress(buf, baseRounds, totalBet, baseWin, freeWin, totalWin, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, freeTime, start)
			fmt.Print(buf.String())
		}
	}

	printBenchmarkSummary(buf, baseRounds, totalBet, baseWin, freeWin, totalWin, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, freeTime, start)
	fmt.Print(buf.String())
}

func spinOneStep(s *betOrderService) error {
	s.gameConfig = _gameJsonConfig
	s.scene.Stage = _spinTypeBase
	if s.isFreeRound {
		s.scene.Stage = _spinTypeFree
	}

	if s.scene.NextStage > 0 && s.scene.NextStage != s.scene.Stage {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
		s.isFreeRound = s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli
	}

	if err := s.baseSpin(); err != nil {
		return err
	}
	return nil
}

func resetBetServiceForNextRound(s *betOrderService) {
	s.stepMultiplier = 0
	s.lineMultiplier = 0
	s.scatterCount = 0
	s.addFreeTime = 0
	s.isFreeRound = false
	s.isRoundOver = false
	s.winInfos = nil
	s.winGrid = int64Grid{}
	s.longWinGrid = int64Grid{}
	s.nextSymbolGrid = int64Grid{}
	//s.debug.originSymbolGrid = int64Grid{}
	if s.scene != nil {
		s.scene.Reset()
	}
}

func printBenchmarkProgress(buf *strings.Builder, baseRounds int64, totalBet, baseWin, freeWin, totalWin float64, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, freeTime int64, start time.Time) {
	if baseRounds == 0 || totalBet <= 0 {
		return
	}
	buf.Reset()
	fprintf(buf, "\rRuntime=%d baseRtp=%.4f%%,baseWinRate=%.4f%% freeRtp=%.4f%% freeWinRate=%.4f%%, freeTriggerRate=%.4f%% Rtp=%.4f%%\n",
		baseRounds,
		safeDivFloat(baseWin*100, totalBet),
		safeDiv(baseWinTimes*100, baseRounds),
		safeDivFloat(freeWin*100, totalBet),
		safeDiv(freeWinTimes*100, max(freeRounds, 1)),
		safeDiv(freeTriggerCount*100, baseRounds),
		safeDivFloat(totalWin*100, totalBet),
	)
	fprintf(buf, "\rtotalWin-%.0f freeWin=%.0f,baseWin-%.0f ,baseWinTime-%d ,freeTime-%d, freeRound-%d ,freeWinTime-%d, elapsed=%v\n",
		totalWin, freeWin, baseWin, baseWinTimes, freeTime, freeRounds, freeWinTimes, time.Since(start).Round(time.Second))
}

func printBenchmarkSummary(buf *strings.Builder, baseRounds int64, totalBet, baseWin, freeWin, totalWin float64, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, freeTime int64, start time.Time) {
	buf.Reset()
	if baseRounds == 0 || totalBet <= 0 {
		fprintf(buf, "No data collected for RTP benchmark.\n")
		return
	}
	elapsed := time.Since(start)
	fprintf(buf, "\n运行局数: %d，用时: %v，速度: %.0f 局/秒\n\n", baseRounds, elapsed.Round(time.Second), safeDiv(baseRounds, int64(elapsed.Seconds())))
	baseRTP := safeDivFloat(baseWin*100, totalBet)
	freeRTP := safeDivFloat(freeWin*100, totalBet)
	totalRTP := safeDivFloat(totalWin*100, totalBet)
	baseWinRate := safeDiv(baseWinTimes*100, baseRounds)
	freeWinRate := safeDiv(freeWinTimes*100, freeRounds)
	freeTriggerRate := safeDiv(freeTriggerCount*100, baseRounds)
	avgFreePerTrigger := safeDiv(freeRounds, freeTriggerCount)

	fprintf(buf, "[基础模式统计]\n")
	fprintf(buf, "基础模式总游戏局数: %d\n", baseRounds)
	fprintf(buf, "基础模式总投注(倍数): %.2f\n", totalBet)
	fprintf(buf, "基础模式总奖金: %.2f\n", baseWin)
	fprintf(buf, "基础模式RTP: %.2f%%\n", baseRTP)
	fprintf(buf, "基础模式免费局触发次数: %d\n", freeTime)
	fprintf(buf, "基础模式触发免费局比例: %.2f%%\n", freeTriggerRate)
	fprintf(buf, "基础模式中奖率: %.2f%%\n", baseWinRate)
	fprintf(buf, "基础模式中奖局数: %d\n", baseWinTimes)

	fprintf(buf, "\n[免费模式统计]\n")
	fprintf(buf, "免费模式总游戏局数: %d\n", freeRounds)
	fprintf(buf, "免费模式总奖金: %.2f\n", freeWin)
	fprintf(buf, "免费模式RTP: %.2f%%\n", freeRTP)
	fprintf(buf, "免费模式中奖率: %.2f%%\n", freeWinRate)
	fprintf(buf, "免费模式中奖局数: %d\n", freeWinTimes)
	fprintf(buf, "\n[免费触发效率]\n")
	fprintf(buf, "  总免费游戏次数: %d | 总触发次数: %d\n", freeRounds, freeTriggerCount)
	fprintf(buf, "  平均每次触发获得免费次数: %.2f\n", avgFreePerTrigger)

	fprintf(buf, "\n[总计]\n")
	fprintf(buf, "总回报率(RTP): %.2f%%\n", totalRTP)
	fprintf(buf, "总投注金额: %.2f\n", totalBet)
	fprintf(buf, "总奖金金额: %.2f\n", totalWin)
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

func newBerService() *betOrderService {
	s := &betOrderService{
		req: &request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     GameID,
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
			ID: GameID,
		},
		client: &client.Client{
			ClientOfFreeGame: &client.ClientOfFreeGame{},
			ClientGameCache:  &client.ClientGameCache{},
		},
		lastOrder: nil,
		scene:     &SpinSceneData{},
		gameOrder: nil,
		debug:     rtpDebugData{open: true},
	}
	s.initGameConfigs()
	return s
}
