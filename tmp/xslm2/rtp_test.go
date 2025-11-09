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
	testRounds       = 10000000 // 测试局数
	progressInterval = 1000000  // 进度输出间隔（调试用，每1000局输出一次）
	debugFileOpen    = false    // 调试文件开关（true=输出详细信息到文件）
)

func init() {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	logger, _ := cfg.Build()
	global.GVA_LOG = logger
}

type rtpStats struct {
	rounds          int64     // 游戏局数
	totalWin        float64   // 总奖金
	winRounds       int64     // 中奖局数
	femaleSymbolWin float64   // 女性符号中奖贡献
	femaleWildWin   float64   // 女性百搭中奖贡献
	cascadeSteps    int64     // 总连消步数
	maxCascadeSteps int       // 单局最大连消步数
	cascadeDistrib  [20]int64 // 连消步数分布
	treasureCount   [6]int64  // 夺宝统计 [1..5]
	freeTriggered   int64     // 基础模式触发免费次数
	totalFreeGiven  int64     // 基础模式获得的免费总次数
	fullElimination int64     // 免费模式全屏消除次数
	femaleCollect   [3]int64  // 免费模式女性收集总量
	treasureInFree  int64     // 免费模式中出现夺宝的次数
	extraFreeRounds int64     // 免费模式新增的额外次数
	freeWithCascade int64     // 免费模式有连消的局数
	freeNoCascade   int64     // 免费模式无连消的局数
	maxFreeStreak   int64     // 免费模式单次触发的最长连续局数
}

func TestRtp(t *testing.T) {
	base, free := &rtpStats{}, &rtpStats{}
	totalBet := 0.0
	start := time.Now()
	buf := &strings.Builder{}

	var fileBuf *strings.Builder
	if debugFileOpen {
		fileBuf = &strings.Builder{}
	}

	svc := newRtpBetService()
	sharedClient := svc.client
	sharedScene := svc.scene
	tmpInterval := int64(min(progressInterval, testRounds))
	baseGameCount, freeGameCount := 0, 0
	triggeringBaseRound := 0
	inFreeSession := false
	currentFreePeak := int64(0)

	initRound := func(isNewRound bool) {
		if isNewRound {
			svc.resetForNextRound(false)
		}
		svc.client = sharedClient
		svc.scene = sharedScene
	}

	for base.rounds < testRounds {
		initRound(base.rounds == 0)

		isFree := svc.client.ClientOfFreeGame.GetFreeNum() > 0
		svc.isFreeRound = isFree
		stats := base
		if isFree {
			stats = free
			if !inFreeSession {
				inFreeSession = true
				currentFreePeak = int64(svc.client.ClientOfFreeGame.GetFreeNum())
				if currentFreePeak > free.maxFreeStreak {
					free.maxFreeStreak = currentFreePeak
				}
			}
		}

		cascadeCount := 0
		roundWin := 0.0

		var nextGrid *int64Grid
		var rollers *[_colCount]SymbolRoller

		for {
			isFirst := cascadeCount == 0
			if isFirst {
				if isFree {
					svc.spin.roundStartFemaleCounts = svc.scene.FemaleCountsForFree
				} else {
					svc.spin.roundStartFemaleCounts = [3]int64{}
				}
				// 首次step：从scene恢复女性符号计数
				svc.spin.femaleCountsForFree = svc.scene.FemaleCountsForFree
				svc.spin.nextFemaleCountsForFree = svc.scene.FemaleCountsForFree
			} else {
				// 后续step：使用上次更新后的女性符号计数
				svc.spin.femaleCountsForFree = svc.spin.nextFemaleCountsForFree
			}

			svc.spin.baseSpin(isFree, isFirst, nextGrid, rollers)
			svc.updateStepResult()
			svc.updateScene(isFree)

			nextGrid = svc.scene.NextSymbolGrid
			rollers = svc.scene.SymbolRollers

			cascadeCount++
			stepWin := float64(svc.spin.stepMultiplier)
			roundWin += stepWin

			if isFree {
				remainingFree := int64(svc.client.ClientOfFreeGame.GetFreeNum())
				if remainingFree > currentFreePeak {
					currentFreePeak = remainingFree
					if currentFreePeak > free.maxFreeStreak {
						free.maxFreeStreak = currentFreePeak
					}
				}
			}

			if debugFileOpen && fileBuf != nil {
				if cascadeCount == 1 {
					if isFree {
						freeGameCount++
					} else {
						baseGameCount++
					}
				}
				gameNum := baseGameCount
				triggerRound := 0
				if isFree {
					gameNum = freeGameCount
					triggerRound = triggeringBaseRound
				}
				writeSpinDetail(fileBuf, svc, gameNum, cascadeCount, isFree, triggerRound)
			}

			stats.totalWin += stepWin
			if isFree && svc.spin.treasureCount > 0 {
				free.extraFreeRounds += svc.spin.newFreeRoundCount
			}

			for _, wr := range svc.spin.winResults {
				gain := float64(wr.TotalMultiplier)
				switch {
				case wr.Symbol >= _femaleA && wr.Symbol <= _femaleC:
					stats.femaleSymbolWin += gain
				case wr.Symbol >= _wildFemaleA && wr.Symbol <= _wildFemaleC:
					stats.femaleWildWin += gain
				}
			}

			// 检查是否回合结束
			if svc.spin.isRoundOver {
				break
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
			if svc.client.ClientOfFreeGame.GetFreeNum() == 0 {
				// 清空场景：女性符号计数 + 网格数据 + 滚轴数据
				svc.scene.FemaleCountsForFree = [3]int64{}
				svc.scene.NextSymbolGrid = nil
				svc.scene.SymbolRollers = nil
				triggeringBaseRound = 0
				inFreeSession = false
				currentFreePeak = 0
			}
		} else {
			base.rounds++
			if roundWin > 0 {
				base.winRounds++
			}
			totalBet += float64(_cnf.BaseBat)

			// 触发免费游戏
			if svc.spin.newFreeRoundCount > 0 {
				base.freeTriggered++
				base.totalFreeGiven += svc.spin.newFreeRoundCount
				triggeringBaseRound = baseGameCount
			}
		}

		// 进度输出
		if base.rounds%tmpInterval == 0 {
			printProgress(buf, base.rounds, totalBet, base.totalWin, free.totalWin, time.Since(start))
			fmt.Print(buf.String())
		}

		// 重置回合状态
		svc.resetForNextRound(isFree)
		sharedScene = svc.scene
		sharedClient = svc.client
	}

	// 输出最终统计
	printFinalStats(buf, base, free, totalBet, start)
	result := buf.String()
	fmt.Print(result)

	// 保存调试文件
	if debugFileOpen && fileBuf != nil {
		saveDebugFile(result, fileBuf.String(), start)
	}
}

func printProgress(buf *strings.Builder, rounds int64, totalBet, baseWin, freeWin float64, elapsed time.Duration) {
	if totalBet <= 0 {
		return
	}
	buf.Reset()
	speed := float64(rounds)
	if elapsed > 0 {
		speed = float64(rounds) / elapsed.Seconds()
	}
	fmt.Fprintf(buf, "\r进度: %d局 | 用时: %v | 速度: %.0f局/秒 | 基础RTP: %.2f%% | 免费RTP: %.2f%% | 总RTP: %.2f%%",
		rounds,
		elapsed.Round(time.Second),
		speed,
		baseWin*100/totalBet,
		freeWin*100/totalBet,
		(baseWin+freeWin)*100/totalBet,
	)
}

func printFinalStats(buf *strings.Builder, base, free *rtpStats, totalBet float64, start time.Time) {
	w := func(s string, args ...interface{}) { buf.WriteString(fmt.Sprintf(s, args...)) }

	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	w("===== 详细统计汇总 =====\n")
	w("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	w("\n【基础模式统计】\n")
	w("基础模式总游戏局数: %d\n", base.rounds)
	w("基础模式总投注(倍数): %.2f\n", totalBet)
	w("基础模式总奖金: %.2f\n", base.totalWin)
	if totalBet > 0 {
		w("基础模式RTP: %.2f%%\n", base.totalWin*100/totalBet)
	}
	w("基础模式免费局触发次数: %d\n", base.freeTriggered)
	if base.rounds > 0 {
		w("基础模式触发免费局比例: %.2f%%\n", float64(base.freeTriggered)*100/float64(base.rounds))
		w("基础模式平均每局免费次数: %.2f\n", float64(free.rounds)/float64(base.rounds))
		w("基础模式中奖率: %.2f%%\n", float64(base.winRounds)*100/float64(base.rounds))
	}
	w("基础模式中奖局数: %d\n", base.winRounds)

	w("\n【免费模式统计】\n")
	w("免费模式总游戏局数: %d\n", free.rounds)
	w("免费模式总奖金: %.2f\n", free.totalWin)
	if totalBet > 0 {
		w("免费模式RTP: %.2f%%\n", free.totalWin*100/totalBet)
	}
	w("免费模式额外增加局数: %d\n", free.extraFreeRounds)
	w("免费模式最大连续局数: %d\n", free.maxFreeStreak)
	w("免费模式中奖局数: %d\n", free.winRounds)
	if free.rounds > 0 {
		w("免费模式中奖率: %.2f%%\n", float64(free.winRounds)*100/float64(free.rounds))
	}

	totalWin := base.totalWin + free.totalWin
	w("\n【总计】\n")
	w("  总投注(倍数): %.2f\n", totalBet)
	w("  总奖金: %.2f\n", totalWin)
	if totalBet > 0 {
		w("  总回报率(RTP): %.2f%%\n", (base.totalWin+free.totalWin)*100/totalBet)
	}
	if totalWin > 0 {
		w("  基础贡献: %.2f%% | 免费贡献: %.2f%%\n",
			base.totalWin*100/totalWin,
			free.totalWin*100/totalWin,
		)
	}
	w("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
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
	femaleCounts := [3]int64{}
	if wasFree {
		femaleCounts = s.scene.FemaleCountsForFree
	}

	s.bonusAmount = decimal.Zero
	s.spin = spin{
		femaleCountsForFree:     femaleCounts,
		nextFemaleCountsForFree: femaleCounts,
		rollerKey:               "",
		roundStartTreasure:      0,
	}
	s.scene = &SpinSceneData{
		FemaleCountsForFree: femaleCounts,
	}
}

// updateScene 模拟 saveScene 的核心逻辑，保持测试流程与正式流程一致
func (s *betOrderService) updateScene(isFree bool) {
	s.syncSceneFromSpin()
}

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, gameNum, step int, isFree bool, triggeringBaseRound int) {
	if step == 1 {
		writeRoundHeader(buf, svc, gameNum, isFree, triggeringBaseRound)
	}

	buf.WriteString(fmt.Sprintf("Step%d 初始盘面:\n", step))
	printGrid(buf, svc.spin.symbolGrid, nil)

	if len(svc.spin.winResults) > 0 {
		buf.WriteString(fmt.Sprintf("Step%d 中奖标记:\n", step))
		printGrid(buf, svc.spin.symbolGrid, svc.spin.winGrid)
	}

	if !svc.spin.isRoundOver && svc.spin.nextSymbolGrid != nil {
		//下一步初始网格（实际消除+下落+填充结果）
		buf.WriteString(fmt.Sprintf("Step%d 下一盘面预览（实际消除+下落+填充结果）:\n", step))
		printGrid(buf, svc.spin.nextSymbolGrid, nil)
	}

	writeStepSummary(buf, svc, step, isFree)

	buf.WriteString("\n")
}

func writeRoundHeader(buf *strings.Builder, svc *betOrderService, gameNum int, isFree bool, triggeringBaseRound int) {
	if isFree {
		buf.WriteString(fmt.Sprintf("\n=============[基础模式] 第%d局 - 免费第%d局 =============\n", triggeringBaseRound, gameNum))
	} else {
		buf.WriteString(fmt.Sprintf("\n=============[基础模式] 第%d局 =============\n", gameNum))
	}
	buf.WriteString("────────────────────────────────────────────────────────\n")
	buf.WriteString("【转轮坐标信息】\n")
	buf.WriteString(fmt.Sprintf("滚轴配置Key: %s\n", svc.spin.rollerKey))
	buf.WriteString("转轮信息长度/起始：")
	for c := int64(0); c < _colCount; c++ {
		if c > 0 {
			buf.WriteString("， ")
		}
		length := GetReelLength(svc.spin.rollers[c].Real, int(c))
		buf.WriteString(fmt.Sprintf("%d[%d]", length, svc.spin.rollers[c].Start))
	}
	buf.WriteString("\n")
	if isFree {
		buf.WriteString(fmt.Sprintf("女性收集状态: A=%d | B=%d | C=%d\n",
			svc.spin.femaleCountsForFree[0], svc.spin.femaleCountsForFree[1], svc.spin.femaleCountsForFree[2]))
		if svc.spin.enableFullElimination {
			buf.WriteString("🎯 全屏消除模式已激活（三种女性符号均>=10）\n")
		}
		buf.WriteString(fmt.Sprintf("剩余免费次数: %d\n", svc.client.ClientOfFreeGame.GetFreeNum()))
	}
}

func writeStepSummary(buf *strings.Builder, svc *betOrderService, step int, isFree bool) {
	buf.WriteString(fmt.Sprintf("Step%d 中奖详情:\n", step))
	if len(svc.spin.winResults) == 0 {
		buf.WriteString("\t未中奖\n")
		return
	}

	buf.WriteString(fmt.Sprintf("\t触发: 女性中奖=%v, 有百搭=%v, 全屏=%v, 夺宝=%d\n",
		svc.spin.hasFemaleWin,
		hasWildSymbol(svc.spin.symbolGrid),
		svc.spin.enableFullElimination,
		svc.spin.treasureCount,
	))

	startRound := svc.spin.roundStartFemaleCounts
	stepStart := svc.spin.stepStartFemaleCounts
	final := svc.spin.nextFemaleCountsForFree
	stepDelta := [3]int64{
		final[0] - stepStart[0],
		final[1] - stepStart[1],
		final[2] - stepStart[2],
	}
	roundDelta := [3]int64{
		final[0] - startRound[0],
		final[1] - startRound[1],
		final[2] - startRound[2],
	}

	buf.WriteString(fmt.Sprintf("\t当前免费次数=%d | 新增免费=%d | 女性收集: 起始=%v → 结束=%v (本步=%v, 回合累计=%v)\n",
		svc.client.ClientOfFreeGame.GetFreeNum(),
		svc.spin.newFreeRoundCount,
		startRound,
		final,
		stepDelta,
		roundDelta,
	))

	if !svc.spin.isRoundOver {
		reason := "无女性中奖"
		if svc.spin.hasFemaleWin {
			if isFree && svc.spin.enableFullElimination {
				reason = "女性中奖且全屏消除启动"
			} else if isFree {
				reason = "女性中奖触发部分消除"
			} else {
				reason = "女性中奖与百搭触发"
			}
		}
		buf.WriteString(fmt.Sprintf("\t🔁 连消继续 → Step%d (%s，夺宝=%d)\n\n",
			step+1,
			reason,
			svc.spin.treasureCount,
		))
	} else {
		stopReason := "无后续可消除"
		if svc.spin.hasFemaleWin && svc.spin.enableFullElimination {
			stopReason = "全屏消除已完成"
		} else if svc.spin.hasFemaleWin {
			stopReason = "女性连消在本步结束"
		}
		buf.WriteString(fmt.Sprintf("\t🛑 连消结束（%s）\n\n", stopReason))
	}

	lineBet := svc.betAmount.Div(decimal.NewFromInt(_cnf.BaseBat))

	for _, wr := range svc.spin.winResults {
		amount := lineBet.Mul(decimal.NewFromInt(wr.TotalMultiplier)).Round(2).InexactFloat64()
		buf.WriteString(fmt.Sprintf("\t符号: %d(%d), 连线: %d, 乘积: %d, 赔率: %.2f, 下注: %g×%d, 奖金: %g\n",
			wr.Symbol,
			wr.Symbol,
			wr.SymbolCount,
			wr.LineCount,
			float64(wr.BaseLineMultiplier),
			svc.req.BaseMoney,
			svc.req.Multiple,
			amount,
		))
	}
	buf.WriteString(fmt.Sprintf("\t累计中奖: %.2f\n", svc.bonusAmount.Round(2).InexactFloat64()))
}

func printGrid(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
	if grid == nil {
		buf.WriteString("(空)\n")
		return
	}
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbol := grid[r][c]
			// 判断是否中奖：winGrid中非0且非_blocked（_blocked是墙格标记）
			isWin := winGrid != nil && winGrid[r][c] != _blank && winGrid[r][c] != _blocked
			if isWin {
				if symbol == _blank {
					fmt.Fprintf(buf, "   *|")
				} else {
					fmt.Fprintf(buf, " %2d*|", symbol)
				}
			} else {
				if symbol == _blank {
					fmt.Fprintf(buf, "    |")
				} else {
					fmt.Fprintf(buf, " %2d |", symbol)
				}
			}
			if c < _colCount-1 {
				buf.WriteString(" ")
			}
		}
		buf.WriteString("\n")
	}
}

func saveDebugFile(statsResult, detailResult string, start time.Time) {
	_ = os.MkdirAll("logs", 0755)
	filename := fmt.Sprintf("logs/%s.txt", time.Now().Format("20060102_150405"))
	_ = os.WriteFile(filename, []byte(statsResult+detailResult), 0644)
	fmt.Printf("\n📄 调试信息已保存到: %s\n", filename)
}
