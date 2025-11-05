package xslm2

import (
	"fmt"
	"os"
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
	testRounds       = 1e7   // 测试局数 (1000万局)
	progressInterval = 1e5   // 进度输出间隔
	debugFileOpen    = false // 调试文件开关（true=输出详细信息到文件）
)

func init() {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	logger, _ := cfg.Build()
	global.GVA_LOG = logger
}

type rtpStats struct {
	// 基础统计
	rounds    int64 // 游戏局数
	totalWin  int64 // 总奖金
	winRounds int64 // 中奖局数

	// 连消统计
	cascadeSteps    int64     // 总连消步数
	maxCascadeSteps int       // 单局最大连消步数
	cascadeDistrib  [20]int64 // 连消步数分布 [0]=无连消,[1]=1步,[2]=2步...

	// 基础模式特有
	baseWildTrigger int64    // Wild触发连消次数
	baseFemaleWild  int64    // 女性+Wild组合次数
	treasureCount   [6]int64 // 夺宝统计 [3]=3个,[4]=4个,[5]=5个
	freeTriggered   int64    // 免费游戏触发次数
	totalFreeGiven  int64    // 基础触发给予的总免费次数

	// 免费模式特有
	fullElimination  int64    // 全屏消除触发次数
	femaleCollect    [3]int64 // 女性符号收集统计 [0]=A,[1]=B,[2]=C
	avgFemalePerFree float64  // 平均每次免费收集的女性符号数
	treasureInFree   int64    // 免费中夺宝符号出现次数
	extraFreeRounds  int64    // 免费中新增的额外次数
	freeWithCascade  int64    // 有连消的免费局数
	freeNoCascade    int64    // 无连消的免费局数
}

func TestRtp(t *testing.T) {
	base, free := &rtpStats{}, &rtpStats{}
	bet := int64(0)
	start := time.Now()
	buf := &strings.Builder{}

	var fileBuf *strings.Builder
	if debugFileOpen {
		fileBuf = &strings.Builder{}
	}

	svc := newRtpBetService()
	tmpInterval := int64(min(progressInterval, testRounds))
	baseGameCount, freeGameCount := 0, 0

	for base.rounds < testRounds {
		isFree := svc.client.ClientOfFreeGame.GetFreeNum() > 0
		cascadeCount := 0
		roundWin := int64(0)
		hadWildInPrevStep := false

		// 一个完整回合（包含所有连消step）
		for {
			svc.spin.baseSpin(isFree)
			svc.updateStepResult()

			cascadeCount++
			stepWin := svc.spin.stepMultiplier
			roundWin += stepWin

			// 调试输出
			if debugFileOpen && fileBuf != nil {
				if !isFree {
					baseGameCount++
					writeSpinDetail(fileBuf, svc, baseGameCount, cascadeCount, isFree)
				} else {
					freeGameCount++
					writeSpinDetail(fileBuf, svc, freeGameCount, cascadeCount, isFree)
				}
			}

			// 统计
			if isFree {
				free.cascadeSteps++
				free.totalWin += stepWin

				// 全屏消除
				if svc.spin.enableFullElimination {
					free.fullElimination++
				}

				// 女性符号收集
				for i, count := range svc.spin.nextFemaleCountsForFree {
					free.femaleCollect[i] = count
				}

				// 夺宝统计
				if svc.spin.treasureCount > 0 {
					free.treasureInFree++
					free.extraFreeRounds += svc.spin.treasureCount
				}

			} else {
				base.cascadeSteps++
				base.totalWin += stepWin

				// Wild触发连消（从第二步开始）
				if cascadeCount > 1 {
					if svc.spin.hasFemaleWin && hadWildInPrevStep {
						base.baseFemaleWild++
					}
					if hadWildInPrevStep {
						base.baseWildTrigger++
					}
				}

				// 夺宝统计
				tc := svc.spin.treasureCount
				if tc >= 3 && tc <= 5 {
					base.treasureCount[tc]++
				}
			}

			// 保存Wild状态供下一step使用
			hadWildInPrevStep = hasWildSymbol(svc.spin.symbolGrid)

			// 检查是否回合结束
			if svc.spin.isRoundOver {
				break
			}

			// 更新场景数据继续连消
			svc.scene.FemaleCountsForFree = svc.spin.nextFemaleCountsForFree
			svc.spin.femaleCountsForFree = svc.spin.nextFemaleCountsForFree
		}

		// 回合统计
		if cascadeCount > base.maxCascadeSteps {
			base.maxCascadeSteps = cascadeCount
		}
		if cascadeCount < 20 {
			if isFree {
				free.cascadeDistrib[cascadeCount]++
			} else {
				base.cascadeDistrib[cascadeCount]++
			}
		}

		if isFree {
			free.rounds++
			if roundWin > 0 {
				free.winRounds++
				free.freeWithCascade++
			} else {
				free.freeNoCascade++
			}

			// 免费游戏结束
			svc.client.ClientOfFreeGame.IncrFreeTimes()
			svc.client.ClientOfFreeGame.Decr()
			if svc.client.ClientOfFreeGame.GetFreeNum() == 0 {
				// 清空场景
				svc.scene.FemaleCountsForFree = [3]int64{}
			}
		} else {
			base.rounds++
			if roundWin > 0 {
				base.winRounds++
			}
			bet += _cnf.BaseBat

			// 触发免费游戏
			if svc.spin.newFreeRoundCount > 0 {
				base.freeTriggered++
				base.totalFreeGiven += svc.spin.newFreeRoundCount
				svc.client.ClientOfFreeGame.SetFreeNum(uint64(svc.spin.newFreeRoundCount))
			}
		}

		// 进度输出
		if base.rounds%tmpInterval == 0 {
			printProgress(buf, base.rounds, bet, base.totalWin, free.totalWin, time.Since(start))
			fmt.Print(buf.String())
		}

		// 重置回合状态
		svc.resetForNextRound(isFree)
	}

	// 输出最终统计
	printFinalStats(buf, base, free, bet)
	result := buf.String()
	fmt.Print(result)

	// 保存调试文件
	if debugFileOpen && fileBuf != nil {
		saveDebugFile(result, fileBuf.String())
	}
}

func printProgress(buf *strings.Builder, rounds, bet, baseWin, freeWin int64, elapsed time.Duration) {
	if bet == 0 {
		return
	}
	b := float64(bet)
	buf.Reset()
	fmt.Fprintf(buf, "\r进度: %d局 | 用时: %v | 速度: %.0f局/秒 | 基础RTP: %.2f%% | 免费RTP: %.2f%% | 总RTP: %.2f%%",
		rounds, elapsed.Round(time.Second), float64(rounds)/elapsed.Seconds(),
		float64(baseWin)*100/b, float64(freeWin)*100/b, float64(baseWin+freeWin)*100/b)
}

func printFinalStats(buf *strings.Builder, base, free *rtpStats, bet int64) {
	b := float64(bet)
	w := func(s string, args ...interface{}) { buf.WriteString(fmt.Sprintf(s, args...)) }

	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	w("                   XSLM2 RTP测试报告\n")
	w("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// ========== 基础模式 ==========
	w("【基础模式统计】\n")
	w("  总局数: %d\n", base.rounds)
	w("  总投注: %.2f\n", b)
	w("  总奖金: %.2f\n", float64(base.totalWin))
	w("  RTP: %.2f%%\n", float64(base.totalWin)*100/b)
	w("  中奖局数: %d (%.2f%%)\n", base.winRounds, float64(base.winRounds)*100/float64(base.rounds))
	w("  平均连消步数: %.2f\n", float64(base.cascadeSteps)/float64(base.rounds))
	w("  最大连消步数: %d\n\n", base.maxCascadeSteps)

	// 连消触发详情
	w("【连消机制统计】\n")
	w("  Wild触发连消: %d次 (%.2f%%)\n", base.baseWildTrigger,
		float64(base.baseWildTrigger)*100/float64(base.rounds))
	w("  女性+Wild组合: %d次 (%.2f%%)\n\n", base.baseFemaleWild,
		float64(base.baseFemaleWild)*100/float64(base.rounds))

	// 夺宝统计
	w("【夺宝符号统计】\n")
	for i := 3; i <= 5; i++ {
		if base.treasureCount[i] > 0 {
			expectedFree := int64(0)
			switch i {
			case 3:
				expectedFree = base.treasureCount[i] * 7
			case 4:
				expectedFree = base.treasureCount[i] * 10
			case 5:
				expectedFree = base.treasureCount[i] * 15
			}
			w("  %d个夺宝: %d次 (%.2f%%) → 预期%d次免费\n", i, base.treasureCount[i],
				float64(base.treasureCount[i])*100/float64(base.rounds), expectedFree)
		}
	}
	w("  免费触发次数: %d (%.2f%%)\n", base.freeTriggered,
		float64(base.freeTriggered)*100/float64(base.rounds))
	w("  基础给予总免费次数: %d\n\n", base.totalFreeGiven)

	// 连消步数分布
	w("【连消步数分布】\n")
	for i := 1; i < 10; i++ {
		if base.cascadeDistrib[i] > 0 {
			w("  %d步: %d次 (%.2f%%)\n", i, base.cascadeDistrib[i],
				float64(base.cascadeDistrib[i])*100/float64(base.rounds))
		}
	}
	w("\n")

	// ========== 免费模式 ==========
	w("【免费模式统计】\n")
	w("  总局数: %d\n", free.rounds)
	w("  总奖金: %.2f\n", float64(free.totalWin))
	w("  RTP: %.2f%%\n", float64(free.totalWin)*100/b)
	w("  中奖局数: %d (%.2f%%)\n", free.winRounds,
		float64(free.winRounds)*100/float64(free.rounds))
	w("  有连消局数: %d (%.2f%%)\n", free.freeWithCascade,
		float64(free.freeWithCascade)*100/float64(free.rounds))
	w("  无连消局数: %d (%.2f%%)\n", free.freeNoCascade,
		float64(free.freeNoCascade)*100/float64(free.rounds))
	w("  平均连消步数: %.2f\n\n", float64(free.cascadeSteps)/float64(free.rounds))

	// 全屏消除
	w("【全屏消除统计】\n")
	w("  触发次数: %d\n", free.fullElimination)
	if free.rounds > 0 {
		w("  触发率: %.2f%%\n\n", float64(free.fullElimination)*100/float64(free.rounds))
	}

	// 女性符号收集
	w("【女性符号收集】\n")
	w("  女性A收集: %d\n", free.femaleCollect[0])
	w("  女性B收集: %d\n", free.femaleCollect[1])
	w("  女性C收集: %d\n", free.femaleCollect[2])
	w("  总收集数: %d\n\n", free.femaleCollect[0]+free.femaleCollect[1]+free.femaleCollect[2])

	// 免费中夺宝
	w("【免费模式夺宝】\n")
	w("  夺宝出现次数: %d\n", free.treasureInFree)
	w("  新增免费次数: %d\n\n", free.extraFreeRounds)

	// 免费次数核算
	w("【免费次数核算】\n")
	theoretical := base.totalFreeGiven + free.extraFreeRounds
	diff := theoretical - free.rounds
	w("  理论总免费次数: %d (基础%d + 额外%d)\n", theoretical, base.totalFreeGiven, free.extraFreeRounds)
	w("  实际玩的免费次数: %d\n", free.rounds)
	w("  差异: %d (%.2f%%)\n\n", diff, float64(diff)*100/float64(theoretical))

	// ========== 总计 ==========
	total := base.totalWin + free.totalWin
	w("【总计】\n")
	w("  总投注金额: %.2f\n", b)
	w("  总奖金金额: %.2f\n", float64(total))
	w("  总回报率(RTP): %.2f%%\n", float64(total)*100/b)
	w("  基础贡献: %.2f%% | 免费贡献: %.2f%%\n",
		float64(base.totalWin)*100/float64(total),
		float64(free.totalWin)*100/float64(total))

	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
}

// ========== 辅助函数 ==========

func newRtpBetService() *betOrderService {
	return &betOrderService{
		req: &request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     _gameID,
			BaseMoney:  1,
			Multiple:   1,
		},
		merchant: &merchant.Merchant{ID: 20020, Merchant: "TestMerchant"},
		member:   &member.Member{ID: 1, MemberName: "TestUser", Balance: 10000000, Currency: "USD"},
		game:     &game.Game{ID: _gameID, GameName: "XSLM2"},
		client: &client.Client{
			ClientOfFreeGame: &client.ClientOfFreeGame{},
			ClientGameCache:  &client.ClientGameCache{},
		},
		scene:       &SpinSceneData{},
		bonusAmount: decimal.Decimal{},
		betAmount:   decimal.NewFromInt(_cnf.BaseBat),
		amount:      decimal.Decimal{},
		debug:       rtpDebugData{open: true},
	}
}

func (s *betOrderService) resetForNextRound(wasFree bool) {
	// 保留场景数据（女性符号计数）
	sceneBackup := s.scene

	// 重置其他数据
	s.bonusAmount = decimal.Zero
	s.spin = spin{}

	// 恢复场景
	s.scene = sceneBackup
	s.spin.femaleCountsForFree = sceneBackup.FemaleCountsForFree
	s.spin.nextFemaleCountsForFree = sceneBackup.FemaleCountsForFree
}

// ========== 调试输出函数 ==========

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, gameNum, step int, isFree bool) {
	mode := "基础模式"
	if isFree {
		mode = "免费模式"
	}
	w := func(s string, args ...interface{}) { buf.WriteString(fmt.Sprintf(s, args...)) }

	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	w("【%s - 第%d局 - Step%d】\n", mode, gameNum, step)
	w("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 符号网格
	w("\n【符号网格】\n")
	printGrid(buf, svc.spin.symbolGrid)

	// 场景状态（免费模式）
	if isFree {
		w("\n【女性符号收集】\n")
		w("  女性A: %d", svc.spin.femaleCountsForFree[0])
		w(" | 女性B: %d", svc.spin.femaleCountsForFree[1])
		w(" | 女性C: %d\n", svc.spin.femaleCountsForFree[2])
		if svc.spin.enableFullElimination {
			w("  🎯 全屏消除已触发！\n")
		}
	}

	// 中奖信息
	w("\n【中奖信息】\n")
	if len(svc.spin.winResults) == 0 {
		w("  未中奖\n")
	} else {
		for i, wr := range svc.spin.winResults {
			w("  [%d] 符号:%d | 连列:%d | Ways:%d | 基础倍率:%d | 总倍率:%d\n",
				i+1, wr.Symbol, wr.SymbolCount, wr.LineCount,
				wr.BaseLineMultiplier, wr.TotalMultiplier)
		}
	}
	w("  总倍数: %d\n", svc.spin.stepMultiplier)

	// 中奖网格
	if len(svc.spin.winResults) > 0 {
		w("\n【中奖网格】\n")
		printGrid(buf, svc.spin.winGrid)
	}

	// 回合状态
	w("\n【回合状态】\n")
	if svc.spin.isRoundOver {
		w("  ✓ 回合结束\n")
		if svc.spin.treasureCount > 0 {
			w("  夺宝数量: %d", svc.spin.treasureCount)
			if svc.spin.newFreeRoundCount > 0 {
				w(" → 触发 %d 次免费游戏", svc.spin.newFreeRoundCount)
			}
			w("\n")
		}
	} else {
		w("  → 继续连消\n")
		if svc.spin.hasFemaleWin {
			w("  有女性中奖\n")
		}
	}
}

func printGrid(buf *strings.Builder, grid *int64Grid) {
	if grid == nil {
		buf.WriteString("  (空)\n")
		return
	}
	for r := int64(0); r < _rowCount; r++ {
		buf.WriteString("  ")
		for c := int64(0); c < _colCount; c++ {
			fmt.Fprintf(buf, "%3d", grid[r][c])
			if c < _colCount-1 {
				buf.WriteString(" | ")
			}
		}
		buf.WriteString("\n")
	}
}

func saveDebugFile(statsResult, detailResult string) {
	header := fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"+
		"               XSLM2 RTP测试调试日志\n"+
		"               生成时间: %s\n"+
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n",
		time.Now().Format("2006-01-02 15:04:05"))

	content := header + statsResult + "\n" + detailResult

	_ = os.MkdirAll("logs", 0755)
	filename := fmt.Sprintf("logs/xslm2_rtp_%s.txt", time.Now().Format("20060102_150405"))
	_ = os.WriteFile(filename, []byte(content), 0644)
	fmt.Printf("\n📄 调试信息已保存到: %s\n", filename)
}
