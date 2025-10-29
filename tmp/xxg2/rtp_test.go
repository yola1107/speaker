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

const totalRuntime = 1000000 //1000000

// init 初始化测试环境
func init() {
	// 创建一个简单的测试日志器
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel) // 只输出警告和错误
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	logger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("Failed to create logger: %v", err))
	}

	global.GVA_LOG = logger
}

func TestRtp(t *testing.T) {
	betService := newBetService()

	// 统计变量
	var (
		// 基础模式统计
		baseRounds        int64 // 基础模式总游戏局数
		baseTotalBet      int64 // 基础模式总投注
		baseTotalWin      int64 // 基础模式总奖金
		baseWinRounds     int64 // 基础模式中奖局数
		baseFreeTriggered int64 // 基础模式免费局触发次数

		// 免费模式统计
		freeRounds      int64 // 免费模式总游戏局数
		freeTotalWin    int64 // 免费模式总奖金
		freeWinRounds   int64 // 免费模式中奖局数
		freeExtraRounds int64 // 免费模式额外增加局数

		// 当前回合统计
		currentRoundBaseWin int64 // 当前回合基础游戏总奖金
		currentRoundFreeWin int64 // 当前回合免费游戏总奖金
	)

	stepCount := int64(0)
	for {
		stepCount++

		// 初始化游戏配置
		betService.initGameConfigs()
		betService.isRoundFirstStep = true

		// 执行baseSpin
		res, err := betService.baseSpin()
		if err != nil {
			panic(err)
		}

		// 统计数据
		currentFreeNum := betService.client.ClientOfFreeGame.GetFreeNum()

		// 根据是否为免费游戏进行统计
		if betService.isFree {
			// 免费模式
			freeRounds++
			if res.stepMultiplier > 0 {
				freeTotalWin += res.stepMultiplier
				currentRoundFreeWin += res.stepMultiplier
			}

			// 统计免费模式额外增加局数
			if betService.newFreeCount > 0 {
				freeExtraRounds += betService.newFreeCount
			}
		} else {
			// 基础模式
			if res.stepMultiplier > 0 {
				baseTotalWin += res.stepMultiplier
				currentRoundBaseWin += res.stepMultiplier
			}

			// 检查是否触发免费游戏
			if betService.newFreeCount > 0 {
				baseFreeTriggered++
			}
		}

		// 判断spin是否结束：免费次数为0表示当前round结束
		spinOver := currentFreeNum == 0

		if spinOver {
			// 统计基础模式局数和投注
			baseRounds++
			baseTotalBet += _baseMultiplier

			// 统计基础模式中奖局数
			if currentRoundBaseWin > 0 {
				baseWinRounds++
			}

			// 统计免费模式中奖局数
			if currentRoundFreeWin > 0 {
				freeWinRounds++
			}

			// 重置当前回合统计
			currentRoundBaseWin = 0
			currentRoundFreeWin = 0

			// 创建新的游戏服务
			betService = newBetService()

			// 定期输出进度
			if baseRounds%100000 == 0 {
				totalWin := baseTotalWin + freeTotalWin
				totalRtp := decimal.NewFromInt(totalWin).Div(decimal.NewFromInt(baseTotalBet)).Mul(decimal.NewFromInt(100)).Round(2).InexactFloat64()
				baseRtp := decimal.NewFromInt(baseTotalWin).Div(decimal.NewFromInt(baseTotalBet)).Mul(decimal.NewFromInt(100)).Round(2).InexactFloat64()
				freeRtp := decimal.NewFromInt(freeTotalWin).Div(decimal.NewFromInt(baseTotalBet)).Mul(decimal.NewFromInt(100)).Round(2).InexactFloat64()

				fmt.Printf("\r进度: %d局 | 基础RTP: %.2f%% | 免费RTP: %.2f%% | 总RTP: %.2f%%",
					baseRounds, baseRtp, freeRtp, totalRtp)
			}
		}

		if baseRounds >= totalRuntime {
			break
		}
	}

	// 计算统计结果
	totalWin := baseTotalWin + freeTotalWin
	baseTotalBetFloat := decimal.NewFromInt(baseTotalBet)

	// 输出最终统计结果
	fmt.Println()
	fmt.Println("==========================================")
	fmt.Println("===== 详细统计汇总 =====")
	fmt.Printf("基础模式总游戏局数: %d\n", baseRounds)
	fmt.Printf("基础模式总投注: %.2f\n", decimal.NewFromInt(baseTotalBet).InexactFloat64())
	fmt.Printf("基础模式总奖金: %.2f\n", decimal.NewFromInt(baseTotalWin).InexactFloat64())
	fmt.Printf("基础模式RTP: %.2f%%\n",
		decimal.NewFromInt(baseTotalWin).Div(baseTotalBetFloat).Mul(decimal.NewFromInt(100)).Round(2).InexactFloat64())
	fmt.Printf("基础模式免费局触发次数: %d\n", baseFreeTriggered)
	fmt.Printf("基础模式触发免费局比例: %.2f%%\n",
		decimal.NewFromInt(baseFreeTriggered).Div(decimal.NewFromInt(baseRounds)).Mul(decimal.NewFromInt(100)).Round(2).InexactFloat64())
	fmt.Printf("基础模式平均每局免费次数: %.2f\n",
		decimal.NewFromInt(freeRounds).Div(decimal.NewFromInt(baseRounds)).InexactFloat64())
	fmt.Printf("基础模式中奖局数: %d\n", baseWinRounds)
	fmt.Printf("基础模式中奖率: %.2f%%\n",
		decimal.NewFromInt(baseWinRounds).Div(decimal.NewFromInt(baseRounds)).Mul(decimal.NewFromInt(100)).Round(2).InexactFloat64())

	fmt.Println()
	fmt.Printf("免费模式总游戏局数: %d\n", freeRounds)
	fmt.Printf("免费模式总奖金: %.2f\n", decimal.NewFromInt(freeTotalWin).InexactFloat64())
	fmt.Printf("免费模式RTP: %.2f%%\n",
		decimal.NewFromInt(freeTotalWin).Div(baseTotalBetFloat).Mul(decimal.NewFromInt(100)).Round(2).InexactFloat64())
	fmt.Printf("免费模式额外增加局数: %d\n", freeExtraRounds)
	fmt.Printf("免费模式中奖局数: %d\n", freeWinRounds)
	if freeRounds > 0 {
		fmt.Printf("免费模式中奖率: %.2f%%\n",
			decimal.NewFromInt(freeWinRounds).Div(decimal.NewFromInt(freeRounds)).Mul(decimal.NewFromInt(100)).Round(2).InexactFloat64())
	} else {
		fmt.Printf("免费模式中奖率: 0.00%%（未触发免费游戏）\n")
	}

	fmt.Println()
	fmt.Printf("总投注金额: %.2f\n", decimal.NewFromInt(baseTotalBet).InexactFloat64())
	fmt.Printf("总奖金金额: %.2f\n", decimal.NewFromInt(totalWin).InexactFloat64())
	fmt.Printf("总回报率 (RTP): %.2f%%\n",
		decimal.NewFromInt(totalWin).Div(baseTotalBetFloat).Mul(decimal.NewFromInt(100)).Round(2).InexactFloat64())
	fmt.Println("==========================================")
}

func newBetService() *betOrderService {
	return &betOrderService{
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
			ID:       GameID,
			GameName: "XXG2",
		},
		client: &client.Client{
			ClientOfFreeGame: &client.ClientOfFreeGame{},
			ClientGameCache:  &client.ClientGameCache{},
		},
		lastOrder:        nil,
		gameRedis:        nil,
		scene:            scene{},
		gameOrder:        nil,
		bonusAmount:      decimal.Decimal{},
		betAmount:        decimal.Decimal{},
		amount:           decimal.Decimal{},
		isRoundFirstStep: true,
		forRtpBench:      true,
	}
}
