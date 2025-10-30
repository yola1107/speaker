package xxg2

import (
	"fmt"
	"testing"

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
	totalRuntime     = 1000000 // RTP 测试总局数
	progressInterval = 100000  // 进度输出间隔
)

// init 初始化测试日志环境
func init() {
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	logger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("Failed to create logger: %v", err))
	}
	global.GVA_LOG = logger
}

// modeStats 模式统计数据（基础或免费）
type modeStats struct {
	rounds      int64 // 游戏局数
	totalWin    int64 // 总奖金
	winRounds   int64 // 中奖局数
	currentWin  int64 // 当前回合累计奖金（临时）
	freeCount   int64 // 免费游戏触发次数（基础模式统计）
	extraRounds int64 // 免费游戏中额外增加的局数（免费模式统计）
	// 新增：详细统计字段
	freeTotalInitial   int64 // 基础触发时获得的总免费次数
	freeTreasure3Count int64 // 3个夺宝触发次数
	freeTreasure4Count int64 // 4个夺宝触发次数
	freeTreasure5Count int64 // 5个夺宝触发次数
	freeTreasureInFree int64 // 免费游戏中出现夺宝次数
	// 免费游戏中蝙蝠统计
	totalAccumulatedNewBat int64     // 所有免费游戏累计新增的蝙蝠总数
	maxBatInSingleFree     int64     // 单次免费游戏中的最大蝙蝠数量
	batDistribution        [10]int64 // 免费游戏结束时蝙蝠数量分布（索引0-9代表0-9个蝙蝠）
	// 基础模式统计
	baseTreasure1Count int64 // 基础模式1个夺宝次数（转Wind）
	baseTreasure2Count int64 // 基础模式2个夺宝次数（转Wind）
	baseWindConvCount  int64 // 基础模式Wind转换总次数
	// 免费模式详细统计
	freeWithBonusCount int64 // 免费游戏中有奖金的spin次数
	freeNoBonusCount   int64 // 免费游戏中没有奖金的spin次数
}

// TestRtp RTP 测试入口
//
// 游戏规则：
//   - 5轴4行 way game，中奖不消除
//   - 收集 3+ 个夺宝符号触发免费游戏（10次起，每多1个+2次）
//   - 基础模式：蝙蝠咬人后消失
//   - 免费模式：蝙蝠持续移动直到免费游戏结束
func TestRtp(t *testing.T) {
	// 统计数据初始化
	var (
		baseStats    modeStats // 基础模式统计
		freeStats    modeStats // 免费模式统计
		baseTotalBet int64     // 基础模式总投注
	)

	betService := newBetService()

	// ===== 主循环：模拟玩家持续游戏 =====
	for {
		// 1. 初始化游戏配置并更新场景状态（参考 mahjong）
		betService.initGameConfigs()
		updateSceneStage(betService)

		// 2. 执行一次 Spin（旋转）
		res, err := betService.baseSpin()
		if err != nil {
			panic(err)
		}

		// 调试：检测免费游戏是否异常长
		if betService.isFreeRound() {
			freeNum := betService.client.ClientOfFreeGame.GetFreeNum()
			if freeNum > 10000 { // 提高阈值，因为所有夺宝都增加免费次数
				fmt.Printf("\n警告：免费次数异常！当前剩余：%d\n", freeNum)
				fmt.Printf("蝙蝠数量：%d\n", len(betService.scene.BatPositions))
				fmt.Printf("夺宝数量：%d\n", betService.treasureCount)
				panic("免费游戏次数异常")
			}
		}

		// 3. 统计本次 Spin 结果
		isFree := betService.isFreeRound()

		if isFree {
			// 免费模式：每个 spin = 1 局游戏
			freeStats.rounds++
			freeStats.totalWin += res.stepMultiplier
			freeStats.currentWin += res.stepMultiplier

			// 中奖判断（立即统计）
			if res.stepMultiplier > 0 {
				freeStats.winRounds++
				freeStats.freeWithBonusCount++ // 有奖金
			} else {
				freeStats.freeNoBonusCount++ // 没有奖金
			}

			// 统计免费游戏中新增的局数
			if betService.newFreeCount > 0 {
				freeStats.extraRounds += betService.newFreeCount
				freeStats.freeTreasureInFree++ // 免费中出现夺宝的次数
			}

			// 统计免费游戏中累计新增的蝙蝠数量
			currentTotalBat := betService.scene.InitialBatCount + betService.scene.AccumulatedNewBat
			if currentTotalBat > freeStats.maxBatInSingleFree {
				freeStats.maxBatInSingleFree = currentTotalBat
			}
		} else {
			// 基础模式：累计当前回合奖金
			baseStats.totalWin += res.stepMultiplier
			baseStats.currentWin += res.stepMultiplier

			// 统计基础模式Wind转换（1-2个夺宝触发）
			if res.treasureCount == 1 {
				baseStats.baseTreasure1Count++
			} else if res.treasureCount == 2 {
				baseStats.baseTreasure2Count++
			}
			if res.treasureCount >= 1 && res.treasureCount <= 2 {
				baseStats.baseWindConvCount += res.treasureCount
			}

			// 统计免费游戏触发次数（3+ 个夺宝符号）
			if betService.newFreeCount > 0 {
				baseStats.freeCount++
				baseStats.freeTotalInitial += betService.newFreeCount // 累计触发时获得的免费次数

				// 统计不同夺宝数量的触发次数
				switch res.treasureCount {
				case 3:
					baseStats.freeTreasure3Count++
				case 4:
					baseStats.freeTreasure4Count++
				case 5:
					baseStats.freeTreasure5Count++
				}
			}
		}

		// 4. Round 结束处理（使用 SpinOver 标志，参考 mahjong）
		if res.SpinOver {
			// 如果是免费游戏结束，统计蝙蝠数据（使用返回结果中保存的数据）
			if res.IsFreeGameEnding {
				totalBats := res.InitialBatCount + res.AccumulatedNewBat
				if totalBats >= 0 && totalBats < 10 {
					freeStats.batDistribution[totalBats]++
				}
				freeStats.totalAccumulatedNewBat += res.AccumulatedNewBat
			}

			// 基础模式局数统计（1 round = 1 局游戏）
			baseStats.rounds++
			baseTotalBet += _baseMultiplier

			// 基础模式中奖判断（整个 round 有奖金即算中奖）
			if baseStats.currentWin > 0 {
				baseStats.winRounds++
			}

			// 重置临时变量
			baseStats.currentWin = 0
			freeStats.currentWin = 0

			// 定期输出进度
			if baseStats.rounds%progressInterval == 0 {
				printProgress(baseStats.rounds, baseTotalBet, baseStats.totalWin, freeStats.totalWin)
			}

			// 检查是否完成测试
			if baseStats.rounds >= totalRuntime {
				break
			}

			// 开启新一轮游戏
			betService = newBetService()
		}
	}

	// ===== 输出最终统计结果 =====
	printFinalStats(baseStats, freeStats, baseTotalBet)
}

// updateSceneStage 更新游戏场景状态
//
// 逻辑说明：
//   - Stage 表示当前阶段（1=base, 2=free）
//   - NextStage 表示下一阶段（用于状态切换）
//   - IsFreeRound 与 Stage 保持同步
func updateSceneStage(s *betOrderService) {
	// 默认为基础模式
	s.scene.Stage = _spinTypeBase
	if s.scene.IsFreeRound {
		s.scene.Stage = _spinTypeFree
	}

	// 处理阶段切换（base ↔ free）
	if s.scene.NextStage > 0 && s.scene.NextStage != s.scene.Stage {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}

	// 同步免费回合标志
	s.scene.IsFreeRound = (s.scene.Stage == _spinTypeFree)
}

// printProgress 实时输出 RTP 进度
func printProgress(rounds, totalBet, baseWin, freeWin int64) {
	baseRTP := float64(baseWin) * 100.0 / float64(totalBet)
	freeRTP := float64(freeWin) * 100.0 / float64(totalBet)
	totalRTP := float64(baseWin+freeWin) * 100.0 / float64(totalBet)

	fmt.Printf("\r进度: %d局 | 基础RTP: %.2f%% | 免费RTP: %.2f%% | 总RTP: %.2f%%",
		rounds, baseRTP, freeRTP, totalRTP)
}

// printFinalStats 打印最终统计汇总
func printFinalStats(base, free modeStats, totalBet int64) {
	fmt.Println()
	fmt.Println("==========================================")
	fmt.Println("===== 详细统计汇总 =====")

	// 基础模式统计
	printModeStats("基础模式", base, totalBet, true, free.rounds)

	// 基础模式Wind转换统计
	fmt.Println("【基础模式Wind转换详情】")
	fmt.Printf("  - 1个夺宝: %d 次\n", base.baseTreasure1Count)
	fmt.Printf("  - 2个夺宝: %d 次\n", base.baseTreasure2Count)
	fmt.Printf("  - Wind转换总次数: %d\n", base.baseWindConvCount)
	if base.rounds > 0 {
		fmt.Printf("  - Wind转换触发率: %.2f%%\n", float64(base.baseTreasure1Count+base.baseTreasure2Count)*100.0/float64(base.rounds))
	}
	fmt.Println()

	// 免费游戏触发详细统计
	fmt.Println("【免费游戏触发详情】")
	fmt.Printf("  - 3个夺宝: %d 次 (预期获得 %d 免费次数)\n", base.freeTreasure3Count, base.freeTreasure3Count*10)
	fmt.Printf("  - 4个夺宝: %d 次 (预期获得 %d 免费次数)\n", base.freeTreasure4Count, base.freeTreasure4Count*12)
	fmt.Printf("  - 5个夺宝: %d 次 (预期获得 %d 免费次数)\n", base.freeTreasure5Count, base.freeTreasure5Count*14)
	fmt.Printf("基础触发获得总免费次数: %d\n", base.freeTotalInitial)
	fmt.Println()

	// 免费模式统计
	printModeStats("免费模式", free, totalBet, false, 0)

	// 免费模式详细统计
	fmt.Println("【免费模式详细信息】")
	fmt.Printf("有奖金spin次数: %d (%.2f%%)\n", free.freeWithBonusCount, float64(free.freeWithBonusCount)*100.0/float64(free.rounds))
	fmt.Printf("没有奖金spin次数: %d (%.2f%%)\n", free.freeNoBonusCount, float64(free.freeNoBonusCount)*100.0/float64(free.rounds))
	fmt.Println()

	// 免费次数核算
	fmt.Println("【免费次数核算】")
	theoreticalTotal := base.freeTotalInitial + free.extraRounds
	fmt.Printf("理论总免费次数 = 基础触发(%d) + 免费增加(%d) = %d\n",
		base.freeTotalInitial, free.extraRounds, theoreticalTotal)
	fmt.Printf("实际玩的免费次数 = %d\n", free.rounds)
	diff := theoreticalTotal - free.rounds
	diffPercent := 0.0
	if theoreticalTotal > 0 {
		diffPercent = float64(diff) * 100.0 / float64(theoreticalTotal)
	}
	fmt.Printf("差异: %d (%.2f%%)\n", diff, diffPercent)
	fmt.Println()

	// 蝙蝠统计
	fmt.Println("【蝙蝠数量统计】")
	fmt.Printf("免费游戏中累计新增蝙蝠总数: %d\n", free.totalAccumulatedNewBat)
	fmt.Printf("单次免费游戏最大蝙蝠数量: %d\n", free.maxBatInSingleFree)
	if base.freeCount > 0 {
		avgNewBat := float64(free.totalAccumulatedNewBat) / float64(base.freeCount)
		fmt.Printf("平均每次免费游戏新增蝙蝠数量: %.2f\n", avgNewBat)
	}
	fmt.Println("免费游戏结束时蝙蝠数量分布:")
	for i := 3; i <= 5; i++ {
		if free.batDistribution[i] > 0 {
			percentage := float64(free.batDistribution[i]) * 100.0 / float64(base.freeCount)
			fmt.Printf("  - %d个蝙蝠: %d 次 (%.2f%%)\n", i, free.batDistribution[i], percentage)
		}
	}
	fmt.Println()

	// 总体统计
	totalWin := base.totalWin + free.totalWin
	totalRTP := float64(totalWin) * 100.0 / float64(totalBet)
	fmt.Println()
	fmt.Printf("总投注金额: %.2f\n", float64(totalBet))
	fmt.Printf("总奖金金额: %.2f\n", float64(totalWin))
	fmt.Printf("总回报率 (RTP): %.2f%%\n", totalRTP)
	fmt.Println("==========================================")
}

// printModeStats 打印单个模式的详细统计
//
// 参数说明：
//   - name: 模式名称（基础/免费）
//   - stats: 统计数据
//   - totalBet: 总投注（用于计算 RTP）
//   - isBase: 是否为基础模式（决定显示哪些统计项）
//   - freeRounds: 免费游戏总局数（仅基础模式需要）
func printModeStats(name string, stats modeStats, totalBet int64, isBase bool, freeRounds int64) {
	fmt.Printf("%s总游戏局数: %d\n", name, stats.rounds)

	// 投注统计（仅基础模式）
	if isBase {
		fmt.Printf("%s总投注: %.2f\n", name, float64(totalBet))
	}

	fmt.Printf("%s总奖金: %.2f\n", name, float64(stats.totalWin))

	// RTP 计算（免费模式的 RTP 也相对于基础投注）
	rtp := 0.0
	if totalBet > 0 {
		rtp = float64(stats.totalWin) * 100.0 / float64(totalBet)
	}
	fmt.Printf("%sRTP: %.2f%%\n", name, rtp)

	// 基础模式特有统计
	if isBase {
		// 免费游戏触发率（3+ 个夺宝符号）
		triggerRate := float64(stats.freeCount) * 100.0 / float64(stats.rounds)
		fmt.Printf("%s免费局触发次数: %d\n", name, stats.freeCount)
		fmt.Printf("%s触发免费局比例: %.2f%%\n", name, triggerRate)

		// 平均每局免费次数（免费游戏总局数 / 基础游戏局数）
		avgFree := float64(freeRounds) / float64(stats.rounds)
		fmt.Printf("%s平均每局免费次数: %.2f\n", name, avgFree)
	} else {
		// 免费模式特有统计
		// 额外增加的局数（免费游戏中出现夺宝符号 +2 次）
		fmt.Printf("%s中出现夺宝次数: %d\n", name, stats.freeTreasureInFree)
		fmt.Printf("%s额外增加局数: %d\n", name, stats.extraRounds)
	}

	// 中奖率统计
	fmt.Printf("%s中奖局数: %d\n", name, stats.winRounds)
	if stats.rounds > 0 {
		winRate := float64(stats.winRounds) * 100.0 / float64(stats.rounds)
		fmt.Printf("%s中奖率: %.2f%%\n", name, winRate)
	} else {
		fmt.Printf("%s中奖率: 0.00%%（未触发免费游戏）\n", name)
	}
	fmt.Println()
}

// newBetService 创建测试用的游戏服务实例
//
// 说明：
//   - 模拟真实玩家的游戏环境
//   - 余额设置为足够大，确保测试不会因余额不足而中断
//   - forRtpBench=true 标记为 RTP 测试模式
func newBetService() *betOrderService {
	return &betOrderService{
		req: &request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     GameID,
			BaseMoney:  1, // 投注大小
			Multiple:   1, // 投注倍数
			Purchase:   0, // 不购买
			Review:     0, // 非回放
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
			Balance:    10000000, // 足够大的余额
			Currency:   "USD",
		},
		game: &game.Game{
			ID:       GameID,
			GameName: "XXG2",
		},
		client: &client.Client{
			ClientOfFreeGame: &client.ClientOfFreeGame{},
			ClientGameCache:  &client.ClientGameCache{},
		},
		lastOrder:        nil,
		gameRedis:        nil,
		scene:            &SpinSceneData{},
		gameOrder:        nil,
		bonusAmount:      decimal.Decimal{},
		betAmount:        decimal.Decimal{},
		amount:           decimal.Decimal{},
		isRoundFirstStep: true,
		isSpinFirstRound: true,
		forRtpBench:      true, // RTP 测试模式
	}
}
