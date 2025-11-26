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
	cfg.DisableStacktrace = true                               // 禁用堆栈跟踪，减少输出信息
	cfg.EncoderConfig.EncodeCaller = zapcore.FullCallerEncoder // 使用完整调用者信息
	logger, _ := cfg.Build()
	global.GVA_LOG = logger
}

const (
	benchTestRounds       = int64(1e8) // 测试局数
	benchProgressInterval = int64(1e7) // 进度输出间隔
)

// 索引对应关系：0=base, 1=000, 2=001, 3=010, 4=100, 5=011, 6=101, 7=110, 8=111, 9=008
var stateNames = []string{"base", "000", "001", "010", "100", "011", "101", "110", "111", "008"}

func TestRtp2(t *testing.T) {
	betService := newBerService()
	var totalWin, baseWin, freeWin, baseWinTime, freeWinTime, freeTime int64
	var baseRounds, freeRounds int64   // 基础模式和免费模式的局数（完整回合数）
	var freeWinRounds int64            // 免费模式中奖局数（每一局免费游戏结束时，如果中奖就统计一次）
	var totalFreeGameCount int64       // 实际执行的免费游戏步数（包括中途追加的免费次数）
	var freeFemaleStateCount [10]int64 // 免费模式女性符号状态统计 [000,001,010,011,100,101,110,111,008,009] 对应滚轴下标Index
	var freeRoundWin float64           // 记录当前免费游戏一局的累计奖金
	var roundWin float64               //
	start := time.Now()
	runtime := int64(0) // 已完成的整局数（基础+对应的免费全部结束算 1）

	fmt.Println()

	// 平均每一轮，
	var femaleKeyWins [10]float64 // 免费模式女性符号状态统计 [000,001,010,011,100,101,110,111,008,009] 对应滚轴下标Index

	// key 触发率对的上
	for runtime < benchTestRounds {
		// 根据 Steps 判断是否为第一步骤：Steps == 0 表示新的一轮开始（第一步骤）
		betService.isFirst = (betService.scene.Steps == 0) // 新的一局
		isFirst := betService.isFirst

		// 执行一步 spin（可能是基础模式的一步，或免费模式的一步）
		err := betService.baseSpin() // step 一步
		if err != nil {
			panic(err)
		}

		// baseSpin() 内部会通过 handleStageTransition() 设置 isFreeRound
		isFree := betService.isFreeRound

		// 在新的一局开始时统计女性状态（baseSpin 后 SymbolRoller 已生成）
		// 使用 roller[0].Real 对应的状态索引（getSceneSymbol 中已根据女性状态选择）
		// Real 索引直接对应数组索引：0-9
		if isFirst && isFree {
			freeFemaleStateCount[betService.scene.SymbolRoller[0].Real]++
		}

		// 当前这一步的奖金（stepMultiplier 在 baseSpin 中已被正确更新）
		stepWin := betService.stepMultiplier
		totalWin += stepWin

		// 这一“步”是基础模式还是免费模式，直接看 isFreeRound
		// 注意：isFreeRound 在进入 baseSpin 前，由 handleStageTransition() 根据 Stage/NextStage 决定
		if betService.isFreeRound {
			// 免费模式：累加免费模式奖金
			freeWin += stepWin
			freeRoundWin += float64(stepWin) // 累计当前免费游戏一局的奖金

			// 统计真实执行的免费游戏步数：每次在免费模式下执行 baseSpin() 就是一步免费游戏
			// 注意：不需要依赖 FreeNum 的变化，因为：
			// 1. 有消除时 FreeNum 不变，但这一步仍然是免费游戏的一步
			// 2. 收集夺宝时 FreeNum 可能增加，但当前这一步仍然是免费游戏的一步
			totalFreeGameCount++

			roundWin += float64(stepWin)

			// 免费模式中奖步数统计：每一步如果中奖就累加
			if stepWin > 0 {
				freeWinTime++
			}

			// 免费游戏一局结束：当前是免费模式 && 本步结束（每一局免费游戏结束时都统计）
			if betService.isRoundOver {
				freeRounds++
				// 如果这一局有中奖，统计中奖局数
				if freeRoundWin > 0 {
					freeWinRounds++
				}
				freeRoundWin = 0 // 重置当前免费游戏一局的累计奖金  +++

				// 统计 000 001 010... 对应的每局赢分
				femaleKeyWins[betService.scene.SymbolRoller[0].Real] += roundWin

				roundWin = 0 // 清理每局的累积赢分
			}

		} else {

			// 基础模式：累加基础模式奖金
			baseWin += stepWin

			roundWin += float64(stepWin)

			// 基础模式这一步结束
			if betService.isRoundOver {
				// 基础模式一局结束
				baseRounds++
				if roundWin > 0 {
					baseWinTime++ // 一局赢了
				}
				// 在基础模式结束时，如果 newFreeRoundCount > 0，说明这一发基础局触发了免费游戏
				if betService.newFreeRoundCount > 0 {
					freeTime++
				}

				roundWin = 0 // 清理每局的累积赢分
			}
		}

		// 判断一整局（基础 + 可能存在的一串免费局）是否结束：
		// 条件：当前这一步结束 && 没有剩余免费次数
		// 无论当前这一步是 base 还是 free，只要 FreeNum == 0，说明这一整轮已经跑完
		if betService.isRoundOver && betService.scene.FreeNum <= 0 {
			runtime++
			// 复用同一个 betService，不再每轮重新 new，减少大量分配开销
			// 重置状态，确保新的一轮开始时状态正确
			betService.scene = &SpinSceneData{} // 重置场景状态（Steps 默认为 0，Stage 默认为 0）
			betService.stepMultiplier = 0
			betService.lineMultiplier = 0
			betService.treasureCount = 0
			betService.newFreeRoundCount = 0
			betService.isRoundOver = false
			betService.client.IsRoundOver = false
			betService.client.ClientOfFreeGame.Reset()
			betService.client.ClientOfFreeGame.ResetGeneralWinTotal()
			betService.client.ClientOfFreeGame.ResetRoundBonus()
			betService.client.SetLastMaxFreeNum(0)
			freeRoundWin = 0 // 重置当前免费游戏一局的累计奖金
			// 注意：isFirst 会在循环开始时根据 Steps 自动设置

			// 到达打印间隔时输出统计
			if runtime%benchProgressInterval == 0 {
				// 平均每次触发免费获得多少次免费局（包括中途追加）
				var avgFreeGamePerTrigger float64
				if freeTime > 0 {
					avgFreeGamePerTrigger = float64(freeRounds) / float64(freeTime)
				}

				// freeWinRate 的分母：使用总免费游戏局数，避免除 0（与 xslm2 对齐，按局计算）
				freeWinRateDenominator := freeRounds
				if freeWinRateDenominator == 0 {
					freeWinRateDenominator = 1
				}

				fmt.Printf(
					"\rRuntime=%d baseRtp=%.4f%%,baseWinRate=%.4f%% freeRtp=%.4f%% freeWinRate=%.4f%%, freeTriggerRate=%.4f%% Rtp=%.4f%%\n",
					runtime,
					calculateRtp(baseWin, runtime, _baseMultiplier),        // baseRtp 。 ==> base / runtime*20 === 1*1**20
					calculateRtp(baseWinTime, runtime, 1),                  // baseWinRate
					calculateRtp(freeWin, runtime, _baseMultiplier),        // freeRtp 相对总下注
					calculateRtp(freeWinRounds, freeWinRateDenominator, 1), // freeWinRate = 免费中奖局数 / 免费总局数
					calculateRtp(freeTime, runtime, 1),                     // freeTriggerRate
					calculateRtp(totalWin, runtime, _baseMultiplier),       // 总 RTP
				)
				fmt.Printf(
					"\rtotalWin=%d freeWin=%d,baseWin=%d ,baseWinTime=%d ,freeTime=%d, freeRounds=%d ,freeWinRounds=%d ,freeWinTime=%d\n",
					totalWin, freeWin, baseWin, baseWinTime, freeTime, freeRounds, freeWinRounds, freeWinTime,
				)
				fmt.Printf(
					"\rtotalFreeGameCount=%d, freeTime=%d, avgFreeGamePerTrigger=%.4f\n",
					totalFreeGameCount, freeTime, avgFreeGamePerTrigger,
				)

				// 输出一次简要进度后换行，避免覆盖
				fmt.Println()
			}
		}
	}

	// =================== 详细统计汇总 ===================
	totalBet := float64(runtime * _baseMultiplier) // 仅基础模式下注，免费模式不扣钱
	elapsed := time.Since(start)

	fmt.Println("\n===== 详细统计汇总 =====")
	fmt.Printf("运行局数: %d，用时: %v，速度: %.0f 局/秒\n\n",
		runtime, elapsed.Round(time.Second), float64(runtime)/elapsed.Seconds())

	// 基础模式统计
	fmt.Println("[基础模式统计]")
	fmt.Printf("基础模式总游戏局数: %d\n", baseRounds)
	fmt.Printf("基础模式总投注(倍数): %.2f\n", totalBet)
	fmt.Printf("基础模式总奖金: %.2f\n", float64(baseWin))
	if totalBet > 0 {
		fmt.Printf("基础模式RTP: %.2f%% (基础模式奖金/基础模式投注)\n", float64(baseWin)*100/totalBet)
	}
	fmt.Printf("基础模式免费局触发次数: %d\n", freeTime)
	if baseRounds > 0 {
		fmt.Printf("基础模式触发免费局比例: %.2f%%\n", float64(freeTime)*100/float64(baseRounds))
		fmt.Printf("基础模式平均每局免费次数: %.2f\n", float64(freeRounds)/float64(baseRounds))
		fmt.Printf("基础模式中奖率: %.2f%%\n", float64(baseWinTime)*100/float64(baseRounds))
	}
	fmt.Printf("基础模式中奖局数: %d\n\n", baseWinTime)

	// 免费模式统计
	fmt.Println("[免费模式统计]")
	fmt.Printf("免费模式总游戏局数: %d\n", freeRounds)
	fmt.Printf("免费模式总游戏步数: %d\n", totalFreeGameCount)
	fmt.Printf("免费模式总奖金: %.2f\n", float64(freeWin))
	if totalBet > 0 {
		fmt.Printf("免费模式RTP: %.2f%% (免费模式奖金/基础模式投注，因为免费模式不投注)\n", float64(freeWin)*100/totalBet)
	}
	fmt.Printf("免费模式中奖局数: %d\n", freeWinRounds)
	fmt.Printf("免费模式中奖步数: %d\n", freeWinTime)
	if freeRounds > 0 {
		fmt.Printf("免费模式中奖率(按局): %.2f%%\n", float64(freeWinRounds)*100/float64(freeRounds))
	}
	if totalFreeGameCount > 0 {
		fmt.Printf("免费模式中奖率(按步): %.2f%%\n", float64(freeWinTime)*100/float64(totalFreeGameCount))
	}
	if freeRounds > 0 {
		fmt.Printf("\n[免费模式女性符号状态统计]\n")
		totalStateCount := int64(0)
		for i := 0; i < 10; i++ {
			totalStateCount += freeFemaleStateCount[i]
		}
		fmt.Printf("  总统计次数: %d (应该等于免费模式总游戏局数: %d)\n", totalStateCount, freeRounds)
		for i := 1; i < 9; i++ {
			count := freeFemaleStateCount[i]
			if freeRounds > 0 {
				fmt.Printf("  状态 %s: %.2f%% (%d次)\n", stateNames[i], float64(count)*100/float64(freeRounds), count)
			} else {
				fmt.Printf("  状态 %s: 0.00%% (%d次)\n", stateNames[i], count)
			}
		}
		fmt.Println("\n[免费模式女性 key 赢分统计]")
		for i := 0; i < len(femaleKeyWins); i++ {
			winSum := femaleKeyWins[i]
			count := freeFemaleStateCount[i]
			avg := 0.0
			if count > 0 {
				avg = winSum / float64(count)
			}
			avgBet := avg / float64(_baseMultiplier)
			fmt.Printf("  key=%s | 总赢分=%.2f | 次数=%d | 平均倍数=%.4f\n",
				stateNames[i], winSum, count, avgBet)
		}
	}
	fmt.Println()

	// 免费触发效率
	fmt.Println("[免费触发效率]")
	fmt.Printf("总免费游戏次数(真实局数): %d\n", freeRounds)
	fmt.Printf("总免费游戏步数(真实步数): %d\n", totalFreeGameCount)
	fmt.Printf("总触发次数: %d (基础模式触发免费游戏的次数)\n", freeTime)
	if freeTime > 0 {
		fmt.Printf("平均1次触发获得免费游戏: %.2f次 (总免费游戏局数 / 总触发次数)\n\n",
			float64(freeRounds)/float64(freeTime))
	} else {
		fmt.Println("平均1次触发获得免费游戏: 0 (未触发)")
	}

	// 总计
	fmt.Println("[总计]")
	fmt.Printf("总投注(倍数): %.2f \n", totalBet)
	fmt.Printf("总奖金: %.2f \n", float64(totalWin))
	fmt.Printf("总回报率(RTP): %.2f%% (总奖金/总投注)\n",
		calculateRtp(totalWin, runtime, _baseMultiplier))
}

// calculateRtp 计算RTP百分比（用 float64，避免 decimal 开销）
func calculateRtp(win, rounds, multiplier int64) float64 {
	if rounds == 0 || multiplier == 0 {
		return 0
	}
	return float64(win) / float64(rounds*multiplier) * 100.0
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
