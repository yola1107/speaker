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
	totalRuntime     = 1000000
	progressInterval = 100000
)

func init() {
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	logger, _ := config.Build()
	global.GVA_LOG = logger
}

type modeStats struct {
	rounds     int64 // 游戏局数
	totalWin   int64 // 总奖金
	winRounds  int64 // 中奖局数
	currentWin int64 // 当前回合累计奖金（临时）

	// 基础模式
	freeCount        int64 // 免费游戏触发次数
	freeTotalInitial int64 // 基础触发获得的总免费次数
	freeTreasure3    int64 // 3个夺宝触发次数
	freeTreasure4    int64 // 4个夺宝触发次数
	freeTreasure5    int64 // 5个夺宝触发次数
	baseTreasure1    int64 // 基础模式1个夺宝次数
	baseTreasure2    int64 // 基础模式2个夺宝次数
	baseWindConv     int64 // 基础模式Wind转换总次数

	// 免费模式
	extraRounds     int64     // 免费中额外增加的局数
	treasureInFree  int64     // 免费中出现夺宝次数
	totalNewBat     int64     // 累计新增蝙蝠总数
	maxBatInFree    int64     // 单次免费最大蝙蝠数量
	batDistribution [10]int64 // 免费结束时蝙蝠数量分布
	freeWithBonus   int64     // 有奖金的spin次数
	freeNoBonus     int64     // 无奖金的spin次数
}

// TestRtp RTP测试
//
// 游戏规则：
//   - 4行5列Ways玩法，中奖不消除
//   - 3+个夺宝触发免费游戏（10次起，每多1个+2次）
//   - 基础模式：1-2个夺宝随机转换Wind符号
//   - 免费模式：蝙蝠8方向持续移动直到免费结束
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
		// 1. 初始化配置并更新场景状态
		betService.initGameConfigs()
		updateSceneStage(betService)

		// 2. 执行一次Spin
		res, err := betService.baseSpin()
		if err != nil {
			panic(err)
		}

		// 检测免费异常（防止无限循环）
		if betService.isFreeRound() {
			if freeNum := betService.client.ClientOfFreeGame.GetFreeNum(); freeNum > 10000 {
				panic(fmt.Sprintf("免费次数异常：%d", freeNum))
			}
		}

		// 3. 统计Spin结果
		if betService.isFreeRound() {
			// 免费模式：每个spin = 1局游戏
			freeStats.rounds++
			freeStats.totalWin += res.stepMultiplier
			freeStats.currentWin += res.stepMultiplier

			// 中奖判断
			if res.stepMultiplier > 0 {
				freeStats.winRounds++
				freeStats.freeWithBonus++
			} else {
				freeStats.freeNoBonus++
			}

			// 统计新增免费次数（免费中出现夺宝）
			if betService.newFreeCount > 0 {
				freeStats.extraRounds += betService.newFreeCount
				freeStats.treasureInFree++
			}

			// 统计免费游戏中累计新增的蝙蝠数量
			totalBat := betService.scene.InitialBatCount + betService.scene.AccumulatedNewBat
			if totalBat > freeStats.maxBatInFree {
				freeStats.maxBatInFree = totalBat
			}
		} else {
			// 基础模式：累计当前回合奖金
			baseStats.totalWin += res.stepMultiplier
			baseStats.currentWin += res.stepMultiplier

			// 统计Wind转换（1-2个夺宝）
			if res.treasureCount == 1 {
				baseStats.baseTreasure1++
				baseStats.baseWindConv++
			} else if res.treasureCount == 2 {
				baseStats.baseTreasure2++
				baseStats.baseWindConv += 2
			}

			// 统计免费游戏触发（3+个夺宝）
			if betService.newFreeCount > 0 {
				baseStats.freeCount++
				baseStats.freeTotalInitial += betService.newFreeCount
				switch res.treasureCount {
				case 3:
					baseStats.freeTreasure3++
				case 4:
					baseStats.freeTreasure4++
				case 5:
					baseStats.freeTreasure5++
				}
			}
		}

		// 4. Round结束处理
		if res.SpinOver {
			// 免费游戏结束：统计蝙蝠数据
			if res.IsFreeGameEnding {
				totalBats := res.InitialBatCount + res.AccumulatedNewBat
				if totalBats < 10 {
					freeStats.batDistribution[totalBats]++
				}
				freeStats.totalNewBat += res.AccumulatedNewBat
			}

			// 基础模式局数统计（1 round = 1局游戏）
			baseStats.rounds++
			baseTotalBet += _baseMultiplier

			// 基础模式中奖判断（整个round有奖金即算中奖）
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
	if s.scene.IsFreeRound {
		s.scene.Stage = _spinTypeFree
	} else {
		s.scene.Stage = _spinTypeBase
	}

	// 处理阶段切换（base ↔ free）
	if s.scene.NextStage > 0 && s.scene.NextStage != s.scene.Stage {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}

	// 同步免费回合标志
	s.scene.IsFreeRound = (s.scene.Stage == _spinTypeFree)
}

// printProgress 实时输出RTP进度
func printProgress(rounds, totalBet, baseWin, freeWin int64) {
	baseRTP := float64(baseWin) * 100.0 / float64(totalBet)
	freeRTP := float64(freeWin) * 100.0 / float64(totalBet)
	totalRTP := float64(baseWin+freeWin) * 100.0 / float64(totalBet)
	fmt.Printf("\r进度: %d局 | 基础RTP: %.2f%% | 免费RTP: %.2f%% | 总RTP: %.2f%%",
		rounds, baseRTP, freeRTP, totalRTP)
}

func printFinalStats(base, free modeStats, totalBet int64) {
	fmt.Println()
	fmt.Println("==========================================")
	fmt.Println("===== 详细统计汇总 =====")

	// 基础模式统计
	printModeStats("基础模式", base, totalBet, true, free.rounds)

	// 基础模式Wind转换统计
	fmt.Println("【基础模式Wind转换详情】")
	fmt.Printf("  - 1个夺宝: %d 次\n", base.baseTreasure1)
	fmt.Printf("  - 2个夺宝: %d 次\n", base.baseTreasure2)
	fmt.Printf("  - Wind转换总次数: %d\n", base.baseWindConv)
	if base.rounds > 0 {
		fmt.Printf("  - Wind转换触发率: %.2f%%\n", float64(base.baseTreasure1+base.baseTreasure2)*100.0/float64(base.rounds))
	}
	fmt.Println()

	// 免费游戏触发详细统计
	fmt.Println("【免费游戏触发详情】")
	fmt.Printf("  - 3个夺宝: %d 次 (预期获得 %d 免费次数)\n", base.freeTreasure3, base.freeTreasure3*10)
	fmt.Printf("  - 4个夺宝: %d 次 (预期获得 %d 免费次数)\n", base.freeTreasure4, base.freeTreasure4*12)
	fmt.Printf("  - 5个夺宝: %d 次 (预期获得 %d 免费次数)\n", base.freeTreasure5, base.freeTreasure5*14)
	fmt.Printf("基础触发获得总免费次数: %d\n", base.freeTotalInitial)
	fmt.Println()

	// 免费模式统计
	printModeStats("免费模式", free, totalBet, false, 0)

	// 免费模式详细统计
	fmt.Println("【免费模式详细信息】")
	fmt.Printf("有奖金spin次数: %d (%.2f%%)\n", free.freeWithBonus, float64(free.freeWithBonus)*100.0/float64(free.rounds))
	fmt.Printf("没有奖金spin次数: %d (%.2f%%)\n", free.freeNoBonus, float64(free.freeNoBonus)*100.0/float64(free.rounds))
	fmt.Println()

	// 免费次数核算
	fmt.Println("【免费次数核算】")
	theoretical := base.freeTotalInitial + free.extraRounds
	diff := theoretical - free.rounds
	diffPercent := 0.0
	if theoretical > 0 {
		diffPercent = float64(diff) * 100.0 / float64(theoretical)
	}
	fmt.Printf("理论总免费次数 = 基础触发(%d) + 免费增加(%d) = %d\n", base.freeTotalInitial, free.extraRounds, theoretical)
	fmt.Printf("实际玩的免费次数 = %d\n", free.rounds)
	fmt.Printf("差异: %d (%.2f%%)\n", diff, diffPercent)
	fmt.Println()

	// 蝙蝠统计
	fmt.Println("【蝙蝠数量统计】")
	fmt.Printf("免费游戏中累计新增蝙蝠总数: %d\n", free.totalNewBat)
	fmt.Printf("单次免费游戏最大蝙蝠数量: %d\n", free.maxBatInFree)
	if base.freeCount > 0 {
		avgNewBat := float64(free.totalNewBat) / float64(base.freeCount)
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
//   - isBase: 是否为基础模式（决定显示哪些统计项）
//   - freeRounds: 免费游戏总局数（仅基础模式需要）
func printModeStats(name string, stats modeStats, totalBet int64, isBase bool, freeRounds int64) {
	fmt.Printf("%s总游戏局数: %d\n", name, stats.rounds)

	if isBase {
		fmt.Printf("%s总投注: %.2f\n", name, float64(totalBet))
	}

	fmt.Printf("%s总奖金: %.2f\n", name, float64(stats.totalWin))
	fmt.Printf("%sRTP: %.2f%%\n", name, float64(stats.totalWin)*100.0/float64(totalBet))

	if isBase {
		// 基础模式：免费触发率
		fmt.Printf("%s免费局触发次数: %d\n", name, stats.freeCount)
		fmt.Printf("%s触发免费局比例: %.2f%%\n", name, float64(stats.freeCount)*100.0/float64(stats.rounds))
		fmt.Printf("%s平均每局免费次数: %.2f\n", name, float64(freeRounds)/float64(stats.rounds))
	} else {
		// 免费模式：额外增加统计
		fmt.Printf("%s中出现夺宝次数: %d\n", name, stats.treasureInFree)
		fmt.Printf("%s额外增加局数: %d\n", name, stats.extraRounds)
	}

	// 中奖率统计
	fmt.Printf("%s中奖局数: %d\n", name, stats.winRounds)
	if stats.rounds > 0 {
		fmt.Printf("%s中奖率: %.2f%%\n", name, float64(stats.winRounds)*100.0/float64(stats.rounds))
	}
	fmt.Println()
}

// newBetService 创建测试用的游戏服务实例
//
// 说明：
//   - 模拟真实玩家的游戏环境
//   - 余额设置足够大，确保测试不会因余额不足而中断
//   - forRtpBench=true 标记为RTP测试模式
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
		scene:            &SpinSceneData{},
		bonusAmount:      decimal.Decimal{},
		betAmount:        decimal.Decimal{},
		amount:           decimal.Decimal{},
		isRoundFirstStep: true,
		isSpinFirstRound: true,
		forRtpBench:      true, // RTP测试模式
	}
}
