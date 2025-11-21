package xslm3

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"

	"github.com/shopspring/decimal"
)

func init() {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	cfg.DisableStacktrace = true                               // 禁用堆栈跟踪，减少输出信息
	cfg.EncoderConfig.EncodeCaller = zapcore.FullCallerEncoder // 使用完整调用者信息
	logger, _ := cfg.Build()
	global.GVA_LOG = logger
}

const (
	testRounds       = int64(1e6) // 测试局数
	progressInterval = int64(1e5) // 进度输出间隔
	betMultiplier    = int64(20)  // 下注倍数
)

func TestRtp(t *testing.T) {
	betService := newBerService()
	var totalWin, baseWin, freeWin, baseWinTime, freeWinTime, freeTime, freeRound int64
	runtime := int64(0)

	fmt.Println()

	for runtime < testRounds {
		betService.initGameConfigs()
		// 注意：状态跳转和 isFreeRound 的设置都由 baseSpin2 的 handleStageTransition 处理
		// scene.FreeNum 和 client.FreeNum 在 eliminateResultForBase2/Free2 中已经同步更新

		// 执行spin
		err := betService.baseSpin()
		if err != nil {
			panic(err)
		}

		stepWin := betService.stepMultiplier
		totalWin += stepWin

		// 调试：检查免费模式状态（仅在发现问题时启用）
		// 如果当前 Stage 是免费模式（_spinTypeFree 或 _spinTypeFreeEli），但 isFreeRound 是 false，那才是问题
		if (betService.scene.Stage == _spinTypeFree || betService.scene.Stage == _spinTypeFreeEli) && !betService.isFreeRound {
			fmt.Printf("ERROR: Stage=%d (free mode) but isFreeRound=false, FreeNum=%d, NextStage=%d, stepWin=%d\n",
				betService.scene.Stage, betService.scene.FreeNum, betService.scene.NextStage, stepWin)
		}
		// 如果当前 Stage 是基础模式（_spinTypeBase 或 _spinTypeBaseEli），但 isFreeRound 是 true，那也是问题
		if (betService.scene.Stage == _spinTypeBase || betService.scene.Stage == _spinTypeBaseEli) && betService.isFreeRound {
			fmt.Printf("ERROR: Stage=%d (base mode) but isFreeRound=true, FreeNum=%d, NextStage=%d, stepWin=%d\n",
				betService.scene.Stage, betService.scene.FreeNum, betService.scene.NextStage, stepWin)
		}

		if betService.isFreeRound {
			freeWin += stepWin
			// 免费游戏结束：isRoundOver 且 FreeNum == 0（免费次数用完）
			if betService.isRoundOver && betService.scene.FreeNum == 0 {
				freeRound++
				if stepWin > 0 {
					freeWinTime++
				}
			}
		} else {
			baseWin += stepWin
			// 基础模式结束：isRoundOver 且不是免费模式
			if betService.isRoundOver {
				if stepWin > 0 {
					baseWinTime++
				}
				// 检查是否触发免费游戏（在基础模式结束时）
				if betService.newFreeRoundCount > 0 {
					freeTime++
				}
			}
		}

		// 处理spin结束（基础模式结束或免费游戏完全结束）
		if betService.isRoundOver && (!betService.isFreeRound || betService.scene.FreeNum == 0) {
			runtime++
			betService = newBerService()

			// 输出进度
			if runtime%progressInterval == 0 {
				freeRoundForCalc := freeRound
				if freeRoundForCalc == 0 {
					freeRoundForCalc = 1
				}
				fmt.Printf("\rRuntime=%d baseRtp=%.4f%%,baseWinRate=%.4f%% freeRtp=%.4f%% freeWinRate=%.4f%%, freeTriggerRate=%.4f%% Rtp=%.4f%%\n",
					runtime,
					calculateRtp(baseWin, runtime, betMultiplier),
					calculateRtp(baseWinTime, runtime, 1),
					calculateRtp(freeWin, runtime, betMultiplier),
					calculateRtp(freeWinTime, freeRoundForCalc, 1),
					calculateRtp(freeTime, runtime, 1),
					calculateRtp(totalWin, runtime, betMultiplier),
				)
				fmt.Printf("\rtotalWin=%d freeWin=%d,baseWin=%d ,baseWinTime=%d ,freeTime=%d, freeRound=%d ,freeWinTime=%d\n",
					totalWin, freeWin, baseWin, baseWinTime, freeTime, freeRound, freeWinTime)
			}
		}
	}
}

// calculateRtp 计算RTP百分比
func calculateRtp(win, rounds, multiplier int64) float64 {
	if rounds == 0 {
		return 0
	}
	return decimal.NewFromInt(win).
		Div(decimal.NewFromInt(rounds * multiplier)).
		Mul(decimal.NewFromInt(100)).
		Round(4).
		InexactFloat64()
}

// newRtpClient 创建用于RTP测试的Client，不访问Redis
func newRtpClient() *client.Client {
	return &client.Client{
		MemberId:       1,
		Member:         "Jack23",
		NickName:       "Jack23",
		Merchant:       "Jack23",
		GameId:         18900,
		Timestamp:      time.Now().Unix(),
		ActivityId:     0,
		Lock:           sync.Mutex{},
		BetLock:        sync.Mutex{},
		SyncLock:       sync.Mutex{},
		MaxFreeNum:     0,
		LastMaxFreeNum: 0,
		TreasureNum:    0,
		IsRoundOver:    false,
		ClientOfGame: &client.ClientOfGame{
			BonusTimes:        0,
			HeadMultipleTimes: []uint64{},
			EnNum:             0,
			TimeList:          []int{},
			Lock:              sync.Mutex{},
		},
		SliceSlow: []int64{},
		ClientOfFreeGame: &client.ClientOfFreeGame{
			FreeNum:         0,
			FreeTotalMoney:  0,
			BonusTimes:      0,
			BetAmount:       0,
			FreeTimes:       0,
			BonusState:      0,
			FreeType:        0,
			LastWinId:       0,
			LastMapId:       0,
			FreeMultiple:    0,
			GeneralWinTotal: 0,
			FreeClean:       0,
			RoundBonus:      0,
			Lock:            sync.Mutex{},
		},
		ClientGameCache: &client.ClientGameCache{
			ExpiredTime: 90 * 24 * 3600 * time.Second,
		},
	}
}

func newBerService() *betOrderService {
	return &betOrderService{
		req: &request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     18900,
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
			ID: 18900,
		},
		client:         newRtpClient(),
		lastOrder:      nil,
		gameRedis:      nil,
		scene:          &scene{},
		gameOrder:      nil,
		bonusAmount:    decimal.Decimal{},
		betAmount:      decimal.Decimal{},
		amount:         decimal.Decimal{},
		orderSN:        "",
		parentOrderSN:  "",
		freeOrderSN:    "",
		gameType:       0,
		stepMultiplier: 0,
	}
}
