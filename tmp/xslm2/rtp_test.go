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
	testRounds       = 10000   // 测试局数
	progressInterval = 1000000 // 进度输出间隔（调试用，每1000局输出一次）
	debugFileOpen    = true    // 调试文件开关（true=输出详细信息到文件）
	freeModeLogOnly  = true    // 只打印免费模式日志开关（true=只打印免费模式日志，false=打印所有日志）
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
	rounds               int64     // 游戏局数
	totalWin             float64   // 总奖金
	winRounds            int64     // 中奖局数
	femaleSymbolWin      float64   // 女性符号中奖贡献
	femaleWildWin        float64   // 女性百搭中奖贡献
	cascadeSteps         int64     // 总连消步数
	maxCascadeSteps      int       // 单局最大连消步数
	cascadeDistrib       [20]int64 // 连消步数分布
	treasureCount        [6]int64  // 夺宝统计 [1..5]
	freeTriggered        int64     // 基础模式触发免费次数
	totalFreeGiven       int64     // 基础模式获得的免费总次数
	fullElimination      int64     // 免费模式全屏消除次数
	femaleCollect        [3]int64  // 免费模式女性收集总量
	treasureInFree       int64     // 免费模式中出现夺宝的次数
	extraFreeRounds      int64     // 免费模式新增的额外次数
	freeWithCascade      int64     // 免费模式有连消的局数
	freeNoCascade        int64     // 免费模式无连消的局数
	maxFreeStreak        int64     // 免费模式单次触发的最长连续局数
	freeFemaleStateCount [8]int64  // 免费模式女性符号状态统计 [000,001,010,011,100,101,110,111]
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
	baseGameCount := 0
	triggeringBaseRound := 0
	inFreeSession := false
	currentFreePeak := int64(0)
	currentFreeRoundIdx := 0

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
				currentFreeRoundIdx = 0
			}
		}

		cascadeCount := 0
		roundWin := 0.0

		var nextGrid *int64Grid
		var rollers *[_colCount]SymbolRoller

		roundDisplayIdx := 0
		for {
			isFirst := cascadeCount == 0
			if isFirst {
				if isFree {
					svc.spin.roundStartFemaleCounts = svc.scene.FemaleCountsForFree
					// 首次step：从scene恢复女性符号计数
					svc.spin.femaleCountsForFree = svc.scene.FemaleCountsForFree
					svc.spin.nextFemaleCountsForFree = svc.scene.FemaleCountsForFree
					// 统计免费模式女性符号状态（000-111）- 使用本局开始时的状态（使用spin的数据，因为已经恢复了）
					state := getFemaleStateKey(svc.spin.femaleCountsForFree)
					free.freeFemaleStateCount[state]++
				} else {
					svc.spin.roundStartFemaleCounts = [3]int64{}
					// 首次step：从scene恢复女性符号计数
					svc.spin.femaleCountsForFree = svc.scene.FemaleCountsForFree
					svc.spin.nextFemaleCountsForFree = svc.scene.FemaleCountsForFree
				}
			} else {
				// 后续step：使用上次更新后的女性符号计数
				svc.spin.femaleCountsForFree = svc.spin.nextFemaleCountsForFree
			}

			// 保存 step 开始时的女性收集数量（用于日志记录）
			stepStartFemaleCounts := svc.spin.femaleCountsForFree

			svc.spin.baseSpin(isFree, isFirst, nextGrid, rollers)
			svc.updateStepResult()
			svc.updateScene(isFree)

			nextGrid = svc.scene.NextSymbolGrid
			rollers = svc.scene.SymbolRollers

			cascadeCount++
			if cascadeCount == 1 {
				if isFree {
					currentFreeRoundIdx++
					roundDisplayIdx = currentFreeRoundIdx
				} else {
					baseGameCount++
					roundDisplayIdx = baseGameCount
				}
			}

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
				// 如果开启了只打印免费模式日志，且当前不是免费模式，则跳过
				if freeModeLogOnly && !isFree {
					// 跳过基础模式的日志
				} else {
					if roundDisplayIdx == 0 {
						if isFree {
							roundDisplayIdx = currentFreeRoundIdx
						} else {
							roundDisplayIdx = baseGameCount
						}
					}
					triggerRound := 0
					if isFree {
						triggerRound = triggeringBaseRound
					}
					writeSpinDetail(fileBuf, svc, roundDisplayIdx, cascadeCount, isFree, triggerRound, stepStartFemaleCounts, stepWin, roundWin)
				}
			}

			stats.totalWin += stepWin

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
				stats.cascadeSteps += int64(cascadeCount)
				if cascadeCount > stats.maxCascadeSteps {
					stats.maxCascadeSteps = cascadeCount
				}
				if cascadeCount < 20 {
					stats.cascadeDistrib[cascadeCount]++
				} else {
					stats.cascadeDistrib[19]++
				}

				// 免费模式下，回合结束时统计新增的免费次数
				if isFree && svc.spin.newFreeRoundCount > 0 {
					free.extraFreeRounds += svc.spin.newFreeRoundCount
				}

				// 注意：winRounds 在回合结束后的 if isFree/else 块中统计，这里不重复统计
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
				currentFreeRoundIdx = 0
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
	fmt.Fprintf(buf, "\r进度: %d局 | 用时: %v | 速度: %.0f局/秒 | 基础RTP: %.2f%% | 免费RTP: %.2f%% | 总RTP: %.2f%% (基础+免费)",
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
		w("基础模式RTP: %.2f%% (基础模式奖金/基础模式投注)\n", base.totalWin*100/totalBet)
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
		w("免费模式RTP: %.2f%% (免费模式奖金/基础模式投注，因为免费模式不投注)\n", free.totalWin*100/totalBet)
	}
	w("免费模式额外增加局数: %d\n", free.extraFreeRounds)
	w("免费模式最大连续局数: %d\n", free.maxFreeStreak)
	w("免费模式中奖局数: %d\n", free.winRounds)
	if free.rounds > 0 {
		w("免费模式中奖率: %.2f%%\n", float64(free.winRounds)*100/float64(free.rounds))
		w("\n【免费模式女性符号状态统计】\n")
		stateNames := []string{"000", "001", "010", "011", "100", "101", "110", "111"}
		totalStateCount := int64(0)
		for i := 0; i < 8; i++ {
			totalStateCount += free.freeFemaleStateCount[i]
		}
		w("  总统计次数: %d (应该等于免费模式总游戏局数: %d)\n", totalStateCount, free.rounds)
		for i := 0; i < 8; i++ {
			count := free.freeFemaleStateCount[i]
			percentage := float64(count) * 100 / float64(free.rounds)
			w("  状态 %s: %.4f%% (%d次)\n", stateNames[i], percentage, count)
		}
	}

	totalWin := base.totalWin + free.totalWin
	w("\n【免费触发效率】\n")
	w("  实际免费总局数: %d | 触发次数: %d\n", free.rounds, base.freeTriggered)
	if base.freeTriggered > 0 {
		w("  平均每次触发获得免费次数: %.2f\n", float64(free.rounds)/float64(base.freeTriggered))
	} else {
		w("  平均每次触发获得免费次数: 0 (未触发)\n")
	}
	w("\n【总计】\n")
	w("  总投注(倍数): %.2f (仅基础模式投注，免费模式不投注)\n", totalBet)
	w("  总奖金: %.2f (基础模式奖金 + 免费模式奖金)\n", totalWin)
	if totalBet > 0 {
		w("  总回报率(RTP): %.2f%% (总奖金/总投注 = %.2f/%.2f)\n", (base.totalWin+free.totalWin)*100/totalBet, totalWin, totalBet)
		w("  说明: 总RTP = 基础RTP + 免费RTP，因为免费模式的奖金来自基础模式的投注\n")
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

// 保存每个回合的初始滚轴状态（用于显示）
var roundInitialRollers = make(map[int][_colCount]SymbolRoller)

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, gameNum, step int, isFree bool, triggeringBaseRound int, stepStartFemaleCounts [3]int64, stepWin float64, roundWin float64) {
	if step == 1 {
		initialTreasure := getTreasureCount(svc.spin.symbolGrid)
		writeRoundHeader(buf, svc, gameNum, isFree, triggeringBaseRound, initialTreasure)
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

	writeStepSummary(buf, svc, step, isFree, stepStartFemaleCounts, stepWin, roundWin)

	buf.WriteString("\n")
}

// getInitialStart 根据当前网格反推初始的起始位置
// 注意：这个方法假设 symbolGrid 是初始网格（未被修改），且是从 start 位置连续取4个符号生成的
func getInitialStart(symbolGrid *int64Grid, roller SymbolRoller, col int) int {
	if symbolGrid == nil || _cnf == nil {
		return roller.Start
	}
	data := _cnf.RealData[roller.Real][col]
	if len(data) == 0 {
		return roller.Start
	}
	// 根据网格的 row 0 反推初始 start
	// 网格的 row 0 对应 data[(start+0)%len(data)] = data[start]
	// 网格的 row 1 对应 data[(start+1)%len(data)]
	// 网格的 row 2 对应 data[(start+2)%len(data)]
	// 网格的 row 3 对应 data[(start+3)%len(data)]

	// 在 data 中查找所有可能的起始位置
	var candidates []int
	for i := 0; i < len(data); i++ {
		if data[i] == (*symbolGrid)[0][col] {
			candidates = append(candidates, i)
		}
	}

	// 如果有多个候选位置，检查后续3个位置是否匹配
	for _, i := range candidates {
		match := true
		for row := int64(1); row < _rowCount; row++ {
			expectedSymbol := (*symbolGrid)[row][col]
			actualSymbol := data[(i+int(row))%len(data)]
			if expectedSymbol != actualSymbol {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}

	// 如果无法反推（网格可能被修改过），返回当前的 Start（可能已经被修改）
	// 这种情况下，显示的起始位置可能不准确，但至少不会崩溃
	return roller.Start
}

func writeRoundHeader(buf *strings.Builder, svc *betOrderService, gameNum int, isFree bool, triggeringBaseRound int, initialTreasure int64) {
	if isFree {
		buf.WriteString(fmt.Sprintf("\n=============[基础模式] 第%d局 - 免费第%d局 =============\n", triggeringBaseRound, gameNum))
	} else {
		buf.WriteString(fmt.Sprintf("\n=============[基础模式] 第%d局 =============\n", gameNum))
	}
	buf.WriteString("────────────────────────────────────────────────────────\n")
	buf.WriteString("【转轮坐标信息】\n")
	buf.WriteString(fmt.Sprintf("滚轴配置Key: %s\n", svc.spin.rollerKey))
	buf.WriteString("转轮信息长度/起始：")
	// Start 字段在初始化后不会被修改（getFallSymbol 只修改 End），所以可以直接使用
	for c := int64(0); c < _colCount; c++ {
		if c > 0 {
			buf.WriteString("， ")
		}
		length := GetReelLength(svc.spin.rollers[c].Real, int(c))
		start := svc.spin.rollers[c].Start
		// 第一列和最后一列的第一行是墙格，所以第一个有效符号是 data[start+1]
		// 为了对应第一个有效符号的位置，第一列和最后一列显示 (start+1) % len
		if c == 0 || c == _colCount-1 {
			displayStart := (start + 1) % length
			buf.WriteString(fmt.Sprintf("%d[%d]", length, displayStart))
		} else {
			buf.WriteString(fmt.Sprintf("%d[%d]", length, start))
		}
	}
	buf.WriteString("\n")
	if isFree {
		buf.WriteString(fmt.Sprintf("女性收集状态: %v\n", svc.spin.femaleCountsForFree))
		if svc.spin.enableFullElimination {
			buf.WriteString("🎯 全屏消除模式已激活（三种女性符号均>=10）\n")
		}
		// 先打印初始盘面夺宝数量，再打印免费次数信息（本轮总次数=已玩+剩余）
		buf.WriteString(fmt.Sprintf("初始盘面夺宝数量: %d\n", initialTreasure))
		remain := sameAsZeroIfNeg(int64(svc.client.ClientOfFreeGame.GetFreeNum()))
		totalThisRound := sameAsZeroIfNeg(int64(svc.client.ClientOfFreeGame.GetFreeTimes())) + remain
		if totalThisRound == 0 && remain > 0 {
			totalThisRound = remain
		}
		buf.WriteString(fmt.Sprintf("本轮免费总次数: %d\n", totalThisRound))
	}
	// 基础模式：在回合头打印初始盘面夺宝数量
	if !isFree {
		buf.WriteString(fmt.Sprintf("初始盘面夺宝数量: %d\n", initialTreasure))
	}
}

// sameAsZeroIfNeg ensures non-negative display values
func sameAsZeroIfNeg(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

func writeStepSummary(buf *strings.Builder, svc *betOrderService, step int, isFree bool, stepStartFemaleCounts [3]int64, stepWin float64, roundWin float64) {
	buf.WriteString(fmt.Sprintf("Step%d 中奖详情:\n", step))
	if len(svc.spin.winResults) == 0 {
		buf.WriteString("\t未中奖\n")
		return
	}

	// 获取盘面上实际的夺宝符号数量（而不是treasureCount，因为连消过程中treasureCount可能为0）
	actualTreasureCount := getTreasureCount(svc.spin.symbolGrid)

	// 格式化输出，便于搜索：添加特殊标记用于grep搜索
	triggerInfo := fmt.Sprintf("\t触发: 女性中奖=%v, 有百搭=%v, 全屏=%v, 有夺宝=%v",
		svc.spin.hasFemaleWin,
		hasWildSymbol(svc.spin.symbolGrid),
		svc.spin.enableFullElimination,
		actualTreasureCount > 0,
	)

	// 如果是目标组合（女性中奖=true, 有百搭=true, 夺宝>0），添加特殊标记
	if svc.spin.hasFemaleWin && hasWildSymbol(svc.spin.symbolGrid) && actualTreasureCount > 0 {
		triggerInfo += " ⭐【目标组合】"
	}

	buf.WriteString(triggerInfo + "\n")

	startRound := svc.spin.roundStartFemaleCounts
	stepStart := stepStartFemaleCounts
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

	buf.WriteString(fmt.Sprintf("\t女性收集: 起始=%v → 结束=%v (本步=%v, 回合累计=%v)\n",
		startRound,
		final,
		stepDelta,
		roundDelta,
	))
	//// 免费模式：打印本回合截至当前step累计新增的夺宝数量
	//if isFree {
	//	buf.WriteString(fmt.Sprintf("\t本回合新增夺宝累计=%d\n", svc.spin.treasureGainedThisRound))
	//}

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
		// 使用实际的夺宝数量
		extra := ""
		if isFree && svc.spin.treasureGainedThisRound > 0 {
			extra = fmt.Sprintf(" | 新增夺宝=%d ⭐", svc.spin.treasureGainedThisRound)
		}
		buf.WriteString(fmt.Sprintf("\t🔁 连消继续 → Step%d (%s)%s\n\n", step+1, reason, extra))
	} else {
		stopReason := "无后续可消除"
		if svc.spin.hasFemaleWin && svc.spin.enableFullElimination {
			stopReason = "全屏消除已完成"
		} else if svc.spin.hasFemaleWin {
			stopReason = "女性连消在本步结束"
		}
		newTreasure := int64(0)
		if isFree {
			newTreasure = svc.spin.newFreeRoundCount
		}
		extra := ""
		if newTreasure > 0 {
			extra = fmt.Sprintf(" | 新增夺宝=%d ⭐", newTreasure)
		}
		buf.WriteString(fmt.Sprintf("\t🛑 连消结束（%s）%s\n\n", stopReason, extra))
		// 仅在回合结束时打印免费次数信息
		if isFree {
			buf.WriteString(fmt.Sprintf("\t剩余免费次数=%d | 本回合新增=%d\n",
				svc.client.ClientOfFreeGame.GetFreeNum(),
				svc.spin.newFreeRoundCount,
			))
		}
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
	// 使用真实的累计中奖值（roundWin），而不是bonusAmount（bonusAmount只是当前step的奖金）
	buf.WriteString(fmt.Sprintf("\t累计中奖: %.2f\n", roundWin))
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
			// 判断是否中奖：winGrid中非0且非_blocked（_blocked是墙格标记）
			isWin := rWinGrid[r][c] != _blank && rWinGrid[r][c] != _blocked
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
