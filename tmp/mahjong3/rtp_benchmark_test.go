package mahjong

import (
	"fmt"
	"testing"
	"time"

	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"

	"github.com/shopspring/decimal"
)

const (
	_benchmarkRounds           int64 = 1e8
	_benchmarkProgressInterval int64 = 1e7
)

func TestRtpBenchmark(t *testing.T) {
	svc := newRtpBetService()
	svc.initGameConfigs()
	start := time.Now()
	progressStep := int64(min(_benchmarkProgressInterval, _benchmarkRounds))

	var (
		baseRounds, freeRounds                                 int64
		baseWin, freeWin, totalBet, totalWin                   float64
		baseWinTimes, freeWinTimes, freeTriggerCount, freeTime int64
	)

	for baseRounds < _benchmarkRounds {
		// 每次循环开始时初始化配置
		svc.initGameConfigs()

		// 判断是否为免费游戏
		isFree := svc.client.ClientOfFreeGame.GetFreeNum() > 0

		// 设置场景状态
		svc.scene.Stage = _spinTypeBase
		if svc.isFreeRound {
			svc.scene.Stage = _spinTypeFree
		}

		// 新免费回合开始
		if svc.scene.NextStage > 0 && svc.scene.NextStage != svc.scene.Stage {
			svc.scene.Stage = svc.scene.NextStage
			svc.scene.NextStage = 0
		}

		// 更新 isFreeRound 状态
		svc.isFreeRound = svc.scene.Stage == _spinTypeFree || svc.scene.Stage == _spinTypeFreeEli

		var baseRoundWin, freeRoundWin float64
		var baseHasWin, freeHasWin bool
		var roundStartedAsFree bool

		// 循环处理连消和免费游戏，直到 SpinOver
		for {
			// 处理场景状态转换（在 baseSpin 之前）
			if svc.scene.NextStage > 0 && svc.scene.NextStage != svc.scene.Stage {
				svc.scene.Stage = svc.scene.NextStage
				svc.scene.NextStage = 0
			}

			// 更新 isFreeRound 状态（在 baseSpin 之前，用于设置状态）
			svc.isFreeRound = svc.scene.Stage == _spinTypeFree || svc.scene.Stage == _spinTypeFreeEli
			isFree = svc.client.ClientOfFreeGame.GetFreeNum() > 0
			if svc.isFreeRound {
				isFree = true
			}

			// 记录 round 开始时的状态
			if svc.scene.Steps == 0 {
				roundStartedAsFree = isFree
			}

			// 执行 baseSpin（处理一个 step）
			result, err := svc.baseSpin()
			if err != nil {
				panic(err)
			}

			// 在 baseSpin 之后判断状态（关键：此时状态已经更新，包括免费游戏触发）
			currentIsFree := svc.isFreeRound

			// 计算当前 step 的奖金：BaseMoney * Multiple * stepMultiplier
			bonusAmount := decimal.NewFromFloat(svc.req.BaseMoney).
				Mul(decimal.NewFromInt(svc.req.Multiple)).
				Mul(decimal.NewFromInt(result.stepMultiplier))
			stepWin := bonusAmount.InexactFloat64()

			// 根据当前模式统计奖金（使用 baseSpin 之后的状态，与 rtp_test.go 保持一致）
			if currentIsFree {
				freeRoundWin += stepWin
				if svc.isRoundOver {
					// 免费游戏的 round 结束时统计
					freeRounds++ // 在 RoundOver 时增加免费游戏 round 数
					if svc.scene.RoundMultiplier > 0 {
						freeHasWin = true
					}
				}
			} else {
				baseRoundWin += stepWin
				if svc.scene.RoundMultiplier > 0 && svc.isRoundOver {
					baseHasWin = true
				}
			}

			// 检查是否触发免费游戏（基础模式且未中奖时，scatterCount >= FreeGameMin）
			// 注意：当免费游戏触发时，SpinOver = false，winInfo.Next = true
			wasFree := roundStartedAsFree
			isFree = svc.client.ClientOfFreeGame.GetFreeNum() > 0
			if !wasFree && isFree && result.scatterCount >= svc.gameConfig.FreeGameMin {
				freeTriggerCount++
			}

			// 如果 SpinOver，结束当前 round
			if result.SpinOver {
				// 统计免费游戏触发次数（与 rtp_test.go 保持一致）
				if result.winInfo.State == 2 {
					freeTime++
				}

				// 处理免费游戏结束
				if isFree && svc.client.ClientOfFreeGame.GetFreeNum() == 0 {
					svc.scene.Stage = _spinTypeBase
					svc.scene.NextStage = 0
					// 更新 isFreeRound 状态
					svc.isFreeRound = svc.scene.Stage == _spinTypeFree || svc.scene.Stage == _spinTypeFreeEli
				}

				// 如果完全结束，重置为新的 spin
				if !result.winInfo.Next {
					svc.scene.Stage = _spinTypeBase
					svc.scene.NextStage = 0
					// 更新 isFreeRound 状态
					svc.isFreeRound = svc.scene.Stage == _spinTypeFree || svc.scene.Stage == _spinTypeFreeEli
					svc.isRoundFirstStep = true
					svc.isSpinFirstRound = true
				}

				break
			}

			// 如果 winInfo.Next = true 但 SpinOver = false，继续执行下一轮（连消或免费游戏）
			if result.winInfo.Next && !result.SpinOver {
				// 更新场景状态（必须在 baseSpin 之前更新）
				if svc.scene.NextStage > 0 {
					svc.scene.Stage = svc.scene.NextStage
					svc.scene.NextStage = 0
					// 更新 isFreeRound 状态
					// 不需要更新 isFreeRound，使用 isFreeRound() 方法判断
				}
				// 继续循环（不重置 isRoundFirstStep 和 Steps，让连消过程自然累积）
				continue
			}
		}

		// 统计：根据 round 结束时的状态统计
		// 注意：rtp_test.go 在每次 baseSpin() 后根据 IsFreeRound 和 RoundOver 统计
		// 这里我们在整个 round 结束后统计
		if roundStartedAsFree {
			// 免费游戏模式：免费游戏的 round 数已在 RoundOver 时统计
			freeWin += freeRoundWin
			if freeHasWin {
				freeWinTimes++
			}
			// 免费游戏时，继续下一轮（不增加 baseRounds）
			continue
		} else {
			// 基础模式
			baseRounds++
			baseWin += baseRoundWin
			totalBet += float64(_baseMultiplier) // 投注倍数 = 20
			totalWin = baseWin + freeWin
			if baseHasWin {
				baseWinTimes++
			}

			if progressStep > 0 && baseRounds%progressStep == 0 {
				printBenchmarkProgress(baseRounds, totalBet, baseWin, freeWin, totalWin, baseWinTimes, freeWinTimes, freeRounds, freeTriggerCount, freeTime, start)
			}
		}
	}

	fmt.Println()
	totalWin = baseWin + freeWin
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
	fmt.Printf("\rtotalWin-%.0f freeWin=%.0f,baseWin-%.0f ,baseWinTime-%d ,freeTime-%d, freeRound-%d ,freeWinTime-%d\n",
		totalWin, freeWin, baseWin, baseWinTimes, freeTime, freeRounds, freeWinTimes)
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

func newRtpBetService() *betOrderService {
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
		scene:            &SpinSceneData{},
		bonusAmount:      decimal.Decimal{},
		betAmount:        decimal.Decimal{},
		amount:           decimal.Decimal{},
		isRoundFirstStep: true,
		isSpinFirstRound: true,
		debug:            rtpDebugData{open: true},
	}
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
