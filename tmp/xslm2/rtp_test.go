package xslm2

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
	testRounds       = 1e5 // 测试局数（10万局，快速验证）
	progressInterval = 1e4 // 进度输出间隔
)

func init() {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	logger, _ := cfg.Build()
	global.GVA_LOG = logger
}

type stats struct {
	rounds     int64 // 游戏局数
	totalWin   int64 // 总奖金
	winRounds  int64 // 中奖局数
	currentWin int64 // 当前回合累计

	// 免费游戏
	freeCount        int64    // 免费触发次数
	freeTotalInitial int64    // 基础触发获得的总免费次数
	treasureCount    [6]int64 // 夺宝统计 [3]=3个,[4]=4个,[5]=5个

	// 女性符号收集统计（xslm2特色）
	femaleAFullElim      int64 // femaleA全屏消除次数
	femaleBFullElim      int64 // femaleB全屏消除次数
	femaleCFullElim      int64 // femaleC全屏消除次数
	totalFemaleCollected int64 // 累计收集女性符号总数
}

func TestRtp(t *testing.T) {
	base, free := &stats{}, &stats{}
	bet := int64(0)
	start := time.Now()
	buf := &strings.Builder{}

	svc := newBetService()

	for base.rounds < testRounds {
		res, err := svc.betOrder(&request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     _gameID,
			BaseMoney:  1,
			Multiple:   1,
		})
		if err != nil {
			t.Fatalf("betOrder error: %v", err)
		}

		// 解析结果
		isFree := res["isFreeRound"].(bool)
		stepMultiplier := res["stepMultiplier"].(int64)
		isRoundOver := res["isRoundOver"].(bool)

		// 统计女性符号收集（仅免费模式）
		if isFree {
			if femaleCountsRaw, ok := res["nextFemaleCountsForFree"]; ok {
				if counts, ok := femaleCountsRaw.([3]int64); ok {
					free.totalFemaleCollected += counts[0] + counts[1] + counts[2]
				}
			}
		}

		// 统计
		if isFree {
			free.rounds++
			free.totalWin += stepMultiplier
			if stepMultiplier > 0 {
				free.winRounds++
			}
			free.currentWin += stepMultiplier
		} else {
			base.currentWin += stepMultiplier
		}

		// 回合结束
		if isRoundOver {
			if !isFree {
				base.rounds++
				base.totalWin += base.currentWin
				if base.currentWin > 0 {
					base.winRounds++
				}
				base.currentWin = 0
				bet += _baseMultiplier

				// 检查免费触发
				if newFree, ok := res["newFreeRoundCount"].(int64); ok && newFree > 0 {
					base.freeCount++
					base.freeTotalInitial += newFree

					// 统计夺宝数量
					if treasure, ok := res["treasureCount"].(int64); ok {
						if treasure >= 3 && treasure <= 5 {
							base.treasureCount[treasure]++
						}
					}
				}
			}
			free.currentWin = 0

			// 进度输出
			if base.rounds%progressInterval == 0 {
				printProgress(buf, base.rounds, bet, base.totalWin, free.totalWin, time.Since(start))
				fmt.Print(buf.String())
			}

			if base.rounds >= testRounds {
				break
			}

			// 重置service
			svc = newBetService()
		}
	}

	printResult(buf, base, free, bet)
	fmt.Print(buf.String())
}

func newBetService() *betOrderService {
	return &betOrderService{
		req: &request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     _gameID,
			BaseMoney:  1,
			Multiple:   1,
		},
		merchant: &merchant.Merchant{ID: 20020, Merchant: "Test"},
		member:   &member.Member{ID: 1, MemberName: "Test", Balance: 10000000, Currency: "USD"},
		game:     &game.Game{ID: _gameID, GameName: "XSLM2"},
		client: &client.Client{
			ClientOfFreeGame: &client.ClientOfFreeGame{},
			ClientGameCache:  &client.ClientGameCache{},
		},
		bonusAmount: decimal.Decimal{},
		betAmount:   decimal.Decimal{},
		amount:      decimal.Decimal{},
	}
}

func printProgress(buf *strings.Builder, r, bet, bw, fw int64, t time.Duration) {
	if bet == 0 {
		return
	}
	b := float64(bet)
	buf.Reset()
	buf.WriteString(fmt.Sprintf("\r进度: %d局 | 用时: %v | 速度: %.0f局/秒 | 基础RTP: %.2f%% | 免费RTP: %.2f%% | 总RTP: %.2f%%",
		r, t.Round(time.Second), float64(r)/t.Seconds(),
		float64(bw)*100/b, float64(fw)*100/b, float64(bw+fw)*100/b))
}

func printResult(buf *strings.Builder, base, free *stats, bet int64) {
	b := float64(bet)
	w := func(s string, args ...interface{}) { buf.WriteString(fmt.Sprintf(s, args...)) }

	buf.Reset()
	w("\n==========================================\n")
	w("===== 详细统计汇总 =====\n\n")

	// 基础模式
	w("基础模式总游戏局数: %d\n", base.rounds)
	w("基础模式总投注: %.2f\n", b)
	w("基础模式总奖金: %.2f\n", float64(base.totalWin))
	w("基础模式RTP: %.2f%%\n", float64(base.totalWin)*100/b)
	w("基础模式免费局触发次数: %d\n", base.freeCount)
	w("基础模式触发免费局比例: %.2f%%\n", float64(base.freeCount)*100/float64(base.rounds))
	w("基础模式平均每局免费次数: %.2f\n", float64(free.rounds)/float64(base.rounds))
	w("基础模式中奖局数: %d\n", base.winRounds)
	w("基础模式中奖率: %.2f%%\n\n", float64(base.winRounds)*100/float64(base.rounds))

	// 免费触发
	w("【免费游戏触发详情】\n")
	w("  - 3个夺宝: %d 次 (预期获得 %d 免费次数)\n", base.treasureCount[3], base.treasureCount[3]*7)
	w("  - 4个夺宝: %d 次 (预期获得 %d 免费次数)\n", base.treasureCount[4], base.treasureCount[4]*10)
	w("  - 5个夺宝: %d 次 (预期获得 %d 免费次数)\n", base.treasureCount[5], base.treasureCount[5]*15)
	w("基础触发获得总免费次数: %d\n\n", base.freeTotalInitial)

	// 免费模式
	w("免费模式总游戏局数: %d\n", free.rounds)
	w("免费模式总奖金: %.2f\n", float64(free.totalWin))
	w("免费模式RTP: %.2f%%\n", float64(free.totalWin)*100/b)
	w("免费模式中奖局数: %d\n", free.winRounds)
	w("免费模式中奖率: %.2f%%\n\n", float64(free.winRounds)*100/float64(free.rounds))

	// 女性符号统计（xslm2特色）
	w("【女性符号收集统计】\n")
	w("累计收集女性符号总数: %d\n", free.totalFemaleCollected)
	w("femaleA全屏消除触发次数: %d\n", free.femaleAFullElim)
	w("femaleB全屏消除触发次数: %d\n", free.femaleBFullElim)
	w("femaleC全屏消除触发次数: %d\n", free.femaleCFullElim)
	if base.freeCount > 0 {
		w("平均每次免费游戏收集符号: %.2f\n", float64(free.totalFemaleCollected)/float64(base.freeCount))
	}

	// 总计
	total := base.totalWin + free.totalWin
	w("\n\n总投注金额: %.2f\n", b)
	w("总奖金金额: %.2f\n", float64(total))
	w("总回报率 (RTP): %.2f%%\n", float64(total)*100/b)
	w("==========================================\n")
}
