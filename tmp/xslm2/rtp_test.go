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
	testRounds       = 1e7 // 测试局数
	progressInterval = 1e5 // 进度输出间隔（调试用，每1000局输出一次）
	debugFileOpen    = 0   // 调试文件开关（0=关闭，非0=开启详细日志文件）
	freeModeLogOnly  = 0   // 免费模式日志开关（0=全部输出，非0=仅输出免费模式）
)

func init() {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	cfg.DisableStacktrace = true                               // 禁用堆栈跟踪，减少输出信息
	cfg.EncoderConfig.EncodeCaller = zapcore.FullCallerEncoder // 使用完整调用者信息
	logger, _ := cfg.Build()
	global.GVA_LOG = logger
}

// getFemaleStateKey 根据女性符号收集数量计算状态key（000-111）
// 返回 0-7 的索引，对应状态：000,001,010,011,100,101,110,111
// 规则：A>=10为1，B>=10为1，C>=10为1，状态 = A*4 + B*2 + C*1
func getFemaleStateKey(counts [3]int64) int {
	state := 0
	if counts[0] >= _femaleSymbolCountForFullElimination {
		state += 4 // A位
	}
	if counts[1] >= _femaleSymbolCountForFullElimination {
		state += 2 // B位
	}
	if counts[2] >= _femaleSymbolCountForFullElimination {
		state += 1 // C位
	}
	return state
}

type rtpStats struct {
	rounds               int64    // 游戏局数
	totalWin             float64  // 总奖金
	winRounds            int64    // 中奖局数
	femaleSymbolWin      float64  // 女性符号中奖贡献
	femaleWildWin        float64  // 女性百搭中奖贡献
	cascadeSteps         int64    // 总连消步数
	maxCascadeSteps      int      // 单局最大连消步数
	freeTriggered        int64    // 基础模式触发免费次数
	fullElimination      int64    // 免费模式全屏消除次数
	treasureInFree       int64    // 免费模式中出现夺宝的次数
	extraFreeRounds      int64    // 免费模式新增的额外次数
	maxFreeStreak        int64    // 免费模式单次触发的最长连续局数
	freeFemaleStateCount [8]int64 // 免费模式女性符号状态统计 [000,001,010,011,100,101,110,111]
}

func TestRtp(t *testing.T) {
	base, free := &rtpStats{}, &rtpStats{}
	totalBet := 0.0
	start := time.Now()
	buf := &strings.Builder{}

	var fileBuf *strings.Builder
	if debugFileOpen > 0 {
		fileBuf = &strings.Builder{}
	}

	svc := newRtpBetService()
	sharedClient, sharedScene := svc.client, svc.scene
	baseGameCount, freeRoundIdx := 0, 0

	for base.rounds < testRounds {
		if base.rounds == 0 {
			svc.resetForNextRound(false)
		}
		svc.client, svc.scene = sharedClient, sharedScene

		isFree := svc.client.ClientOfFreeGame.GetFreeNum() > 0
		svc.isFreeRound = isFree

		var (
			cascadeCount            int
			roundWin                float64
			roundStartFemaleCounts  [3]int64
			roundHasFullElimination bool
			nextGrid                *int64Grid
			rollers                 *[_colCount]SymbolRoller
			gameNum                 int
		)

		for {
			isFirst := cascadeCount == 0
			if isFirst {
				roundStartFemaleCounts = svc.scene.FemaleCountsForFree
				svc.spin.femaleCountsForFree = roundStartFemaleCounts
				svc.spin.nextFemaleCountsForFree = roundStartFemaleCounts
				// isFirst代表新的一局，新的一局里面有很多个step（连续消除有多个，没有消除就只有1个step）
				// prevStepTreasureCount 设置为 0，因为新的一局开始时没有上一 step 的夺宝数量
				// 注意：免费模式下，由于夺宝符号不会被消除，order_step.go 中直接使用 stepTreasureCount（最终盘面夺宝数量）作为新增免费次数
				svc.spin.prevStepTreasureCount = 0

				if isFree {
					// 免费模式开始时，确保 betAmount 正确设置（与正常游戏流程一致）
					// betAmount 应该等于基础模式的 betAmount，即 BaseBat
					svc.betAmount = decimal.NewFromInt(_cnf.BaseBat)
					svc.client.ClientOfFreeGame.SetBetAmount(svc.betAmount.Round(2).InexactFloat64())
					freeRoundIdx++
					gameNum = freeRoundIdx
					free.freeFemaleStateCount[getFemaleStateKey(roundStartFemaleCounts)]++
				} else {
					// 基础模式开始时，确保 betAmount 正确设置
					svc.betAmount = decimal.NewFromInt(_cnf.BaseBat)
					svc.client.ClientOfFreeGame.SetBetAmount(svc.betAmount.Round(2).InexactFloat64())
					baseGameCount++
					gameNum = baseGameCount
				}
			} else {
				svc.spin.femaleCountsForFree = svc.spin.nextFemaleCountsForFree
				// 连消步骤中，确保 betAmount 正确设置（从 ClientOfFreeGame 获取，与正常游戏流程一致）
				svc.betAmount = decimal.NewFromFloat(svc.client.ClientOfFreeGame.GetBetAmount())
				// 连消步骤中，从 scene 恢复上一 step 的夺宝数量
				// 注意：免费模式下，由于夺宝符号不会被消除，order_step.go 中直接使用 stepTreasureCount 作为新增免费次数，不再使用 prevStepTreasureCount
				// 但保留此设置不影响逻辑（可能用于其他场景或调试）
				svc.spin.prevStepTreasureCount = svc.scene.TreasureNum
			}

			stepStartFemaleCounts := svc.spin.femaleCountsForFree
			svc.spin.baseSpin(isFree, isFirst, nextGrid, rollers)
			svc.updateStepResult()
			svc.updateScene(isFree)

			cascadeCount++
			// 使用 bonusAmount 作为实际奖金（与正常游戏流程一致）
			// bonusAmount = betAmount / BaseBat * stepMultiplier
			stepWin := svc.bonusAmount.InexactFloat64()
			roundWin += stepWin

			if isFree {
				if remainingFree := int64(svc.client.ClientOfFreeGame.GetFreeNum()); remainingFree > free.maxFreeStreak {
					free.maxFreeStreak = remainingFree
				}
				if svc.spin.enableFullElimination && svc.spin.hasFemaleWildWin {
					roundHasFullElimination = true
				}
			}

			if debugFileOpen > 0 && fileBuf != nil && (freeModeLogOnly == 0 || isFree) {
				triggerRound := 0
				if isFree {
					triggerRound = baseGameCount
				}
				writeSpinDetail(fileBuf, svc, gameNum, cascadeCount, isFree, triggerRound, stepStartFemaleCounts, roundStartFemaleCounts, stepWin, roundWin)
			}

			if isFree {
				free.totalWin += stepWin
				for _, wr := range svc.spin.winResults {
					gain := float64(wr.TotalMultiplier)
					if wr.Symbol >= _femaleA && wr.Symbol <= _femaleC {
						free.femaleSymbolWin += gain
					} else if wr.Symbol >= _wildFemaleA && wr.Symbol <= _wildFemaleC {
						free.femaleWildWin += gain
					}
				}
			} else {
				base.totalWin += stepWin
				for _, wr := range svc.spin.winResults {
					gain := float64(wr.TotalMultiplier)
					if wr.Symbol >= _femaleA && wr.Symbol <= _femaleC {
						base.femaleSymbolWin += gain
					} else if wr.Symbol >= _wildFemaleA && wr.Symbol <= _wildFemaleC {
						base.femaleWildWin += gain
					}
				}
			}

			if svc.spin.isRoundOver {
				if isFree {
					free.cascadeSteps += int64(cascadeCount)
					if cascadeCount > free.maxCascadeSteps {
						free.maxCascadeSteps = cascadeCount
					}
					if svc.spin.stepTreasureCount > 0 {
						free.treasureInFree++
					}
					if svc.spin.newFreeRoundCount > 0 {
						free.extraFreeRounds += svc.spin.newFreeRoundCount
					}
					if roundHasFullElimination {
						free.fullElimination++
					}
				} else {
					base.cascadeSteps += int64(cascadeCount)
					if cascadeCount > base.maxCascadeSteps {
						base.maxCascadeSteps = cascadeCount
					}
				}
				break
			}

			nextGrid, rollers = svc.scene.NextSymbolGrid, svc.scene.SymbolRollers
		}

		if isFree {
			free.rounds++
			if roundWin > 0 {
				free.winRounds++
			}

			if svc.client.ClientOfFreeGame.GetFreeNum() == 0 {
				svc.scene.FemaleCountsForFree = [3]int64{}
				svc.scene.NextSymbolGrid = nil
				svc.scene.SymbolRollers = nil
				svc.scene.TreasureNum = 0
				svc.scene.RollerKey = ""
				freeRoundIdx = 0
			}
		} else {
			base.rounds++
			if roundWin > 0 {
				base.winRounds++
			}
			totalBet += float64(_cnf.BaseBat)

			if svc.spin.newFreeRoundCount > 0 {
				base.freeTriggered++
			}
		}

		if base.rounds%int64(min(progressInterval, testRounds)) == 0 {
			printProgress(buf, base.rounds, totalBet, base.totalWin, free.totalWin, time.Since(start))
			fmt.Print(buf.String())
		}

		svc.resetForNextRound(isFree)
		sharedClient, sharedScene = svc.client, svc.scene
	}

	// 输出最终统计
	printFinalStats(buf, base, free, totalBet, start)
	result := buf.String()
	fmt.Print(result)

	// 保存调试文件
	if debugFileOpen > 0 && fileBuf != nil {
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
	totalRTP := (baseWin + freeWin) * 100 / totalBet
	fmt.Fprintf(buf, "\r进度: %d局 | 用时: %v | 速度: %.0f局/秒 | 基础RTP: %.2f%% | 免费RTP: %.2f%% | 总RTP: %.2f%%",
		rounds, elapsed.Round(time.Second), speed, baseWin*100/totalBet, freeWin*100/totalBet, totalRTP)
}

func printWinContribution(w func(string, ...interface{}), femaleSymbolWin, femaleWildWin, totalWin float64) {
	if totalWin > 0 {
		w("  女性符号中奖贡献: %.2f (%.2f%%)\n", femaleSymbolWin, femaleSymbolWin*100/totalWin)
		w("  女性百搭中奖贡献: %.2f (%.2f%%)\n", femaleWildWin, femaleWildWin*100/totalWin)
	} else {
		w("  女性符号中奖贡献: %.2f\n", femaleSymbolWin)
		w("  女性百搭中奖贡献: %.2f\n", femaleWildWin)
	}
}

func printFinalStats(buf *strings.Builder, base, free *rtpStats, totalBet float64, start time.Time) {
	w := func(s string, args ...interface{}) { buf.WriteString(fmt.Sprintf(s, args...)) }

	w("\n===== 详细统计汇总 =====\n")
	w("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	w("\n[基础模式统计]\n")
	w("基础模式总游戏局数: %d\n", base.rounds)
	w("基础模式总投注(倍数): %.2f\n", totalBet)
	w("基础模式总奖金: %.2f\n", base.totalWin)
	if totalBet > 0 {
		w("基础模式RTP: %.2f%% (基础模式奖金/基础模式投注)\n", base.totalWin*100/totalBet)
	}
	w("基础模式免费局触发次数: %d\n", base.freeTriggered)
	if base.rounds > 0 {
		w("基础模式触发免费局比例: %.2f%%\n", float64(base.freeTriggered)*100/float64(base.rounds))
		w("基础模式平均每局免费次数: %.2f\n", float64(free.rounds)/float64(base.rounds))
		w("基础模式中奖率: %.2f%%\n", float64(base.winRounds)*100/float64(base.rounds))
	}
	w("基础模式中奖局数: %d\n", base.winRounds)
	w("\n[基础模式中奖贡献分析]\n")
	printWinContribution(w, base.femaleSymbolWin, base.femaleWildWin, base.totalWin)

	w("\n[免费模式统计]\n")
	w("免费模式总游戏局数: %d\n", free.rounds)
	w("免费模式总奖金: %.2f\n", free.totalWin)
	if totalBet > 0 {
		w("免费模式RTP: %.2f%% (免费模式奖金/基础模式投注，因为免费模式不投注)\n", free.totalWin*100/totalBet)
	}
	w("免费模式额外增加局数: %d\n", free.extraFreeRounds)
	w("免费模式最大连续局数: %d\n", free.maxFreeStreak)
	w("免费模式中奖局数: %d\n", free.winRounds)
	if free.rounds > 0 {
		w("免费模式中奖率: %.2f%%\n", float64(free.winRounds)*100/float64(free.rounds))
		w("免费模式全屏消除次数: %d (%.2f%%)\n", free.fullElimination, float64(free.fullElimination)*100/float64(free.rounds))
		w("免费模式出现夺宝的次数: %d (%.2f%%)\n", free.treasureInFree, float64(free.treasureInFree)*100/float64(free.rounds))
		w("\n[免费模式女性符号状态统计]\n")
		stateNames := []string{"000", "001", "010", "011", "100", "101", "110", "111"}
		totalStateCount := int64(0)
		for i := 0; i < 8; i++ {
			totalStateCount += free.freeFemaleStateCount[i]
		}
		w("  总统计次数: %d (应该等于免费模式总游戏局数: %d)\n", totalStateCount, free.rounds)
		for i := 0; i < 8; i++ {
			count := free.freeFemaleStateCount[i]
			w("  状态 %s: %.4f%% (%d次)\n", stateNames[i], float64(count)*100/float64(free.rounds), count)
		}
	}
	w("\n[免费模式中奖贡献分析]\n")
	printWinContribution(w, free.femaleSymbolWin, free.femaleWildWin, free.totalWin)

	totalWin := base.totalWin + free.totalWin
	w("\n[免费触发效率]\n")
	w("  总免费游戏次数: %d (真实的游戏局数，包含中途增加的免费次数)\n", free.rounds)
	w("  总触发次数: %d (基础模式触发免费游戏的次数)\n", base.freeTriggered)
	if base.freeTriggered > 0 {
		w("  平均1次触发获得免费游戏: %.2f次 (总免费游戏次数 / 总触发次数)\n", float64(free.rounds)/float64(base.freeTriggered))
	} else {
		w("  平均1次触发获得免费游戏: 0 (未触发)\n")
	}
	w("\n[总计]\n")
	w("  总投注(倍数): %.2f (仅基础模式投注，免费模式不投注)\n", totalBet)
	w("  总奖金: %.2f (基础模式奖金 + 免费模式奖金)\n", totalWin)
	if totalBet > 0 {
		w("  总回报率(RTP): %.2f%% (总奖金/总投注 = %.2f/%.2f)\n", totalWin*100/totalBet, totalWin, totalBet)
	}
	if totalWin > 0 {
		w("  基础贡献: %.2f%% | 免费贡献: %.2f%%\n", base.totalWin*100/totalWin, free.totalWin*100/totalWin)
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
	// 基础模式的新一局应该重置 TreasureNum 为 0，只有免费模式才保留上一轮的 TreasureNum
	var treasureNum int64
	if wasFree {
		treasureNum = s.scene.TreasureNum
	} else {
		treasureNum = 0
	}
	s.bonusAmount = decimal.Zero
	s.amount = decimal.Zero
	s.spin = spin{
		femaleCountsForFree:     femaleCounts,
		nextFemaleCountsForFree: femaleCounts,
		rollerKey:               "",
		rollers:                 [_colCount]SymbolRoller{},
		nextSymbolGrid:          nil,
		prevStepTreasureCount:   0,     // 重置为0，会在主循环中根据 isFree 正确设置
		stepTreasureCount:       0,     // 重置为0，会在 finalizeRound 中通过 getTreasureCount 统计
		isRoundOver:             false, // 显式初始化为 false，确保内层循环能正常进行
	}
	s.scene = &SpinSceneData{
		FemaleCountsForFree: femaleCounts,
		NextSymbolGrid:      nil, // 确保清空，避免残留数据导致问题
		SymbolRollers:       nil, // 确保清空，避免残留数据导致问题
		RollerKey:           "",
		TreasureNum:         treasureNum,
	}
	s.isFreeRound = false
	s.isFirst = false
	s.client.IsRoundOver = false

	// 重置 client.ClientOfFreeGame 的统计字段，使其与 newRtpBetService() 初始化时一致
	// 注意：不重置免费次数（FreeNum），因为它会被游戏逻辑管理（updateStepResult 会调用 SetFreeNum/Incr/Decr）
	// 注意：不调用 Reset()，因为它会清空免费次数（FreeNum）
	if !wasFree {
		// 基础模式结束后：重置所有统计字段（与 newRtpBetService 初始化时一致）
		// newRtpBetService 中 ClientOfFreeGame 的所有字段都是零值
		s.client.ClientOfFreeGame.ResetGeneralWinTotal()   // GeneralWinTotal -> 0
		s.client.ClientOfFreeGame.ResetRoundBonus()        // RoundBonus -> 0
		s.client.ClientOfFreeGame.ResetRoundBonusStaging() // StagingRoundBonus -> 0
		// FreeTotalMoney, BetAmount, FreeTimes, BonusTimes 等字段在 newRtpBetService 时也是 0
		// 但这些字段在 RTP 测试中可能不会被使用，如果被使用，需要通过其他方式重置
		// 注意：不调用 SetLastMaxFreeNum(0)，因为它会重置最大免费次数记录
	} else {
		// 免费模式结束后：只重置当前回合的奖金统计
		// FreeTotalMoney 在免费模式中会累计，不应该重置
		s.client.ClientOfFreeGame.ResetRoundBonus()        // RoundBonus -> 0
		s.client.ClientOfFreeGame.ResetRoundBonusStaging() // StagingRoundBonus -> 0
	}
}

func (s *betOrderService) updateScene(bool) {
	s.syncSceneFromSpin()
}

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, gameNum, step int, isFree bool, triggeringBaseRound int, stepStartFemaleCounts [3]int64, roundStartFemaleCounts [3]int64, stepWin float64, roundWin float64) {
	if step == 1 {
		writeRoundHeader(buf, svc, gameNum, isFree, triggeringBaseRound, stepStartFemaleCounts, 0)
	}
	buf.WriteString(fmt.Sprintf("Step%d 初始盘面:\n", step))
	printGrid(buf, svc.spin.symbolGrid, nil)
	if len(svc.spin.winResults) > 0 {
		buf.WriteString(fmt.Sprintf("Step%d 中奖标记:\n", step))
		printGrid(buf, svc.spin.symbolGrid, svc.spin.winGrid)
	}
	if !svc.spin.isRoundOver && svc.spin.nextSymbolGrid != nil {
		buf.WriteString(fmt.Sprintf("Step%d 下一盘面预览（实际消除+下落+填充结果）:\n", step))
		printGrid(buf, svc.spin.nextSymbolGrid, nil)
	}
	writeStepSummary(buf, svc, step, isFree, stepStartFemaleCounts, roundStartFemaleCounts, stepWin, roundWin)
	buf.WriteString("\n")
}

func writeRoundHeader(buf *strings.Builder, svc *betOrderService, gameNum int, isFree bool, triggeringBaseRound int, femaleStart [3]int64, _ int64) {
	if isFree {
		buf.WriteString(fmt.Sprintf("\n=============[基础模式] 第%d局 - 免费第%d局 =============\n", triggeringBaseRound, gameNum))
	} else {
		buf.WriteString(fmt.Sprintf("\n=============[基础模式] 第%d局 =============\n", gameNum))
	}
	buf.WriteString("────────────────────────────────────────────────────────\n")
	buf.WriteString("[转轮坐标信息]\n")
	buf.WriteString(fmt.Sprintf("滚轴配置Key: %s\n", svc.spin.rollerKey))
	buf.WriteString("转轮信息长度/起始：")
	for c := int64(0); c < _colCount; c++ {
		if c > 0 {
			buf.WriteString("， ")
		}
		length := GetReelLength(svc.spin.rollers[c].Real, int(c))
		start := svc.spin.rollers[c].Start
		if c == 0 || c == _colCount-1 {
			start = (start + 1) % length
		}
		buf.WriteString(fmt.Sprintf("%d[%d]", length, start))
	}
	buf.WriteString("\n")
	if isFree {
		buf.WriteString(fmt.Sprintf("女性收集状态: 上一步=%v\n", femaleStart))
		if svc.spin.enableFullElimination {
			buf.WriteString("🎯 全屏消除模式已激活（三种女性符号均>=10）\n")
		}
	}
}

func writeStepSummary(buf *strings.Builder, svc *betOrderService, step int, isFree bool, stepStartFemaleCounts [3]int64, roundStartFemaleCounts [3]int64, stepWin float64, roundWin float64) {
	buf.WriteString(fmt.Sprintf("Step%d 中奖详情:\n", step))
	if len(svc.spin.winResults) == 0 {
		buf.WriteString("\t未中奖\n")
		if svc.spin.isRoundOver {
			if svc.spin.stepTreasureCount > 0 {
				buf.WriteString(fmt.Sprintf("\t💎 当前轮累计夺宝数量: %d \n", svc.spin.stepTreasureCount))
			}
			if !isFree && svc.spin.newFreeRoundCount > 0 {
				buf.WriteString(fmt.Sprintf("\t基础模式。 夺宝=%d 免费次数=%d\n", svc.spin.stepTreasureCount, svc.spin.newFreeRoundCount))
			}
		}
		return
	}

	actualTreasureCount := getTreasureCount(svc.spin.symbolGrid)
	buf.WriteString(fmt.Sprintf("\t基础=%v, 触发: 女性中奖=%v, 女性百搭参与=%v, 有百搭=%v, 全屏=%v, 有夺宝=%v, (%d)\n",
		!isFree, svc.spin.hasFemaleWin, svc.spin.hasFemaleWildWin,
		hasWildSymbol(svc.spin.symbolGrid), svc.spin.enableFullElimination,
		actualTreasureCount > 0, actualTreasureCount))

	final := svc.spin.nextFemaleCountsForFree
	stepDelta := [3]int64{final[0] - stepStartFemaleCounts[0], final[1] - stepStartFemaleCounts[1], final[2] - stepStartFemaleCounts[2]}
	roundDelta := [3]int64{final[0] - roundStartFemaleCounts[0], final[1] - roundStartFemaleCounts[1], final[2] - roundStartFemaleCounts[2]}
	buf.WriteString(fmt.Sprintf("\t女性收集: 上一步=%v → 当前=%v (本步=%v, 回合累计=%v | 回合起点=%v)\n",
		stepStartFemaleCounts, final, stepDelta, roundDelta, roundStartFemaleCounts))

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

	if !svc.spin.isRoundOver {
		buf.WriteString(fmt.Sprintf("\t🔁 连消继续 → Step%d (%s)\n\n", step+1, reason))
	} else {
		stopReason := "无后续可消除"
		if svc.spin.hasFemaleWin {
			if svc.spin.enableFullElimination {
				stopReason = "全屏消除已完成"
			} else {
				stopReason = "女性连消在本步结束"
			}
		}
		extra := ""
		//if svc.spin.stepTreasureCount > 0 {
		//	extra = fmt.Sprintf(" | 💎💎💎 当前轮累计夺宝数量=%d 💎💎💎", svc.spin.stepTreasureCount)
		//}
		//if isFree && svc.spin.newFreeRoundCount > 0 {
		//	if extra != "" {
		//		extra += fmt.Sprintf(" | 新增免费次数=%d ⭐", svc.spin.newFreeRoundCount)
		//	} else {
		//		extra = fmt.Sprintf(" | 新增免费次数=%d ⭐", svc.spin.newFreeRoundCount)
		//	}
		//}
		buf.WriteString(fmt.Sprintf("\t🛑 连消结束（%s）%s\n\n", stopReason, extra))
	}

	lineBet := svc.betAmount.Div(decimal.NewFromInt(_cnf.BaseBat))
	for _, wr := range svc.spin.winResults {
		amount := lineBet.Mul(decimal.NewFromInt(wr.TotalMultiplier)).Round(2).InexactFloat64()
		buf.WriteString(fmt.Sprintf("\t符号: %d(%d), 连线: %d, 乘积: %d, 赔率: %.2f, 下注: %g×%d, 奖金: %g\n",
			wr.Symbol, wr.Symbol, wr.SymbolCount, wr.LineCount, float64(wr.BaseLineMultiplier),
			svc.req.BaseMoney, svc.req.Multiple, amount))
	}
	buf.WriteString(fmt.Sprintf("\t累计中奖: %.2f\n", roundWin))

	if svc.spin.isRoundOver && svc.spin.stepTreasureCount > 0 {
		buf.WriteString(fmt.Sprintf("\t💎 当前轮累计夺宝数量: %d \n", svc.spin.stepTreasureCount))
	}
	if !isFree && svc.spin.isRoundOver && svc.spin.newFreeRoundCount > 0 {
		buf.WriteString(fmt.Sprintf("\t基础模式。 夺宝=%d 免费次数=%d\n", svc.spin.stepTreasureCount, svc.spin.newFreeRoundCount))
	}
}

func printGrid(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
	if grid == nil {
		buf.WriteString("(空)\n")
		return
	}
	rGrid := reverseGridRows(grid)
	rWinGrid := reverseGridRows(winGrid)
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbol := rGrid[r][c]
			isWin := rWinGrid[r][c] != _blank && rWinGrid[r][c] != _blocked
			if isWin {
				if symbol == _blank {
					buf.WriteString("   *|")
				} else {
					fmt.Fprintf(buf, " %2d*|", symbol)
				}
			} else {
				if symbol == _blank {
					buf.WriteString("    |")
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

// reverseGridRows 网格行序反转
func reverseGridRows(grid *int64Grid) int64Grid {
	if grid == nil {
		return int64Grid{}
	}
	var reversed int64Grid
	for i := int64(0); i < _rowCount; i++ {
		reversed[i] = grid[_rowCount-1-i]
	}
	return reversed
}
