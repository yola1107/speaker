package xxg2

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
	testRounds       = 1e7   // 测试局数
	progressInterval = 1e5   // 进度输出间隔
	debugFileOpen    = false // 调试模式（true=输出详细信息到文件）
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
	currentWin int64 // 当前回合累计奖金（临时）

	// 基础模式
	freeCount        int64    // 免费游戏触发次数
	freeTotalInitial int64    // 基础触发获得的总免费次数
	treasureCount    [6]int64 // 夺宝统计 [1]=1个,[2]=2个,[3]=3个,[4]=4个,[5]=5个

	// 免费模式
	extraRounds     int64     // 免费中额外增加的局数
	treasureInFree  int64     // 免费中出现夺宝次数
	totalNewBat     int64     // 累计新增蝙蝠总数
	maxBatInFree    int64     // 单次免费最大蝙蝠数量
	batDistribution [10]int64 // 免费结束时蝙蝠数量分布
	freeWithBonus   int64     // 有奖金的spin次数
	freeNoBonus     int64     // 无奖金的spin次数
}

func TestRtp(t *testing.T) {
	base, free := &stats{}, &stats{}
	bet := int64(0)
	start := time.Now()
	buf := &strings.Builder{}

	var fileBuf *strings.Builder
	if debugFileOpen {
		fileBuf = &strings.Builder{}
	}

	svc := newBetService()

	tmpInterval := int64(min(progressInterval, testRounds))
	baseGameCount, freeGameCount, triggeringBaseRound := 0, 0, 0

	for base.rounds < testRounds {
		updateScene(svc)

		res, err := svc.baseSpin()
		if err != nil {
			panic(err)
		}

		isFree := svc.isFreeRound()

		// 调试输出
		if debugFileOpen {
			if !isFree {
				baseGameCount++
				writeSpinDetail(fileBuf, svc, res, baseGameCount, 0, false)
			} else {
				freeGameCount++
				writeSpinDetail(fileBuf, svc, res, freeGameCount, triggeringBaseRound, true)
			}
		}

		// 统计
		stepWin := res.StepMultiplier
		treasureCnt := res.TreasureCount
		if isFree {
			free.rounds++
			free.totalWin += stepWin
			free.currentWin += stepWin
			if stepWin > 0 {
				free.winRounds++
				free.freeWithBonus++
			} else {
				free.freeNoBonus++
			}
			if treasureCnt > 0 {
				free.extraRounds += svc.newFreeCount
				free.treasureInFree += treasureCnt
			}
			tb := svc.scene.InitialBatCount + svc.scene.AccumulatedNewBat
			if tb > free.maxBatInFree {
				free.maxBatInFree = tb
			}
		} else {
			base.totalWin += stepWin
			base.currentWin += stepWin
			if treasureCnt >= 1 && treasureCnt <= 5 {
				base.treasureCount[treasureCnt]++
			}
			if treasureCnt >= 3 && svc.newFreeCount > 0 {
				base.freeCount++
				base.freeTotalInitial += svc.newFreeCount
				triggeringBaseRound = baseGameCount
				freeGameCount = 0
			}
		}

		// 回合结束
		if res.SpinOver {
			if svc.debug.isFreeGameEnding {
				t := svc.debug.initialBatCount + svc.debug.accumulatedNewBat
				if t < 10 {
					free.batDistribution[t]++
				}
				free.totalNewBat += svc.debug.accumulatedNewBat
			}

			base.rounds++
			if base.currentWin > 0 {
				base.winRounds++
			}
			base.currentWin, free.currentWin = 0, 0
			bet += _cnf.BaseBat

			if base.rounds%tmpInterval == 0 {
				printProgress(buf, base.rounds, bet, base.totalWin, free.totalWin, time.Since(start))
				fmt.Print(buf.String())
			}

			if base.rounds >= testRounds {
				break
			}

			resetBetService(svc) // 复用内存
		}
	}

	printResult(buf, base, free, bet)
	result := buf.String()
	fmt.Print(result)

	// 保存调试文件
	if debugFileOpen && fileBuf != nil {
		header := fmt.Sprintf("===== RTP测试调试日志 =====\n    生成时间: %s", time.Now().Format("2006-01-02 15:04:05"))
		content := header + result + "\n" + fileBuf.String()
		_ = os.MkdirAll("logs", 0755)
		filename := fmt.Sprintf("logs/%s.txt", time.Now().Format("20060102_150405"))
		_ = os.WriteFile(filename, []byte(content), 0644)
		fmt.Printf("\n调试信息已保存到: %s\n", filename)
	}
}

func updateScene(s *betOrderService) {
	if s.client.ClientOfFreeGame.GetFreeNum() > 0 {
		s.scene.Stage = _spinTypeFree
	} else {
		s.scene.Stage = _spinTypeBase
	}

	if s.scene.NextStage > 0 && s.scene.NextStage != s.scene.Stage {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
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

func printResult(buf *strings.Builder, base, free *stats, bet int64) {
	b := float64(bet)
	w := func(s string, args ...interface{}) { buf.WriteString(fmt.Sprintf(s, args...)) }

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

	// Wind转换
	if base.treasureCount[1] > 0 || base.treasureCount[2] > 0 {
		w("【基础模式Wind转换详情】\n")
		w("  - 1个夺宝: %d 次\n", base.treasureCount[1])
		w("  - 2个夺宝: %d 次\n", base.treasureCount[2])
		w("  - Wind转换总次数: %d\n", base.treasureCount[1]+base.treasureCount[2]*2)
		w("  - Wind转换触发率: %.2f%%\n\n", float64(base.treasureCount[1]+base.treasureCount[2])*100/float64(base.rounds))
	}

	// 免费触发
	w("【免费游戏触发详情】\n")
	w("  - 3个夺宝: %d 次 (预期获得 %d 免费次数)\n", base.treasureCount[3], base.treasureCount[3]*10)
	w("  - 4个夺宝: %d 次 (预期获得 %d 免费次数)\n", base.treasureCount[4], base.treasureCount[4]*12)
	w("  - 5个夺宝: %d 次 (预期获得 %d 免费次数)\n", base.treasureCount[5], base.treasureCount[5]*14)
	w("基础触发获得总免费次数: %d\n\n", base.freeTotalInitial)

	// 免费模式
	w("免费模式总游戏局数: %d\n", free.rounds)
	w("免费模式总奖金: %.2f\n", float64(free.totalWin))
	w("免费模式RTP: %.2f%%\n", float64(free.totalWin)*100/b)
	w("免费模式中出现夺宝次数: %d\n", free.treasureInFree)
	w("免费模式额外增加局数: %d\n", free.extraRounds)
	w("免费模式中奖局数: %d\n", free.winRounds)
	w("免费模式中奖率: %.2f%%\n\n", float64(free.winRounds)*100/float64(free.rounds))

	w("【免费模式详细信息】\n")
	w("有奖金spin次数: %d (%.2f%%)\n", free.freeWithBonus, float64(free.freeWithBonus)*100/float64(free.rounds))
	w("没有奖金spin次数: %d (%.2f%%)\n\n", free.freeNoBonus, float64(free.freeNoBonus)*100/float64(free.rounds))

	// 核算
	theoretical := base.freeTotalInitial + free.extraRounds
	diff := theoretical - free.rounds
	diffPct := 0.0
	if theoretical > 0 {
		diffPct = float64(diff) * 100 / float64(theoretical)
	}
	w("【免费次数核算】\n")
	w("理论总免费次数 = 基础触发(%d) + 免费增加(%d) = %d\n", base.freeTotalInitial, free.extraRounds, theoretical)
	w("实际玩的免费次数 = %d\n", free.rounds)
	w("差异: %d (%.2f%%)\n\n", diff, diffPct)

	// 蝙蝠
	w("【蝙蝠数量统计】\n")
	w("免费游戏中累计新增蝙蝠总数: %d\n", free.totalNewBat)
	w("单次免费游戏最大蝙蝠数量: %d\n", free.maxBatInFree)
	if base.freeCount > 0 {
		w("平均每次免费游戏新增蝙蝠数量: %.2f\n", float64(free.totalNewBat)/float64(base.freeCount))
	}
	w("免费游戏结束时蝙蝠数量分布:\n")
	for i := 3; i <= 5; i++ {
		if free.batDistribution[i] > 0 {
			w("  - %d个蝙蝠: %d 次 (%.2f%%)\n", i, free.batDistribution[i],
				float64(free.batDistribution[i])*100/float64(base.freeCount))
		}
	}

	// 总计
	total := base.totalWin + free.totalWin
	w("\n\n总投注金额: %.2f\n", b)
	w("总奖金金额: %.2f\n", float64(total))
	w("总回报率 (RTP): %.2f%%\n", float64(total)*100/b)
	w("==========================================\n")
}

// ======== 调试输出辅助函数 ========

func writeSpinDetail(buf *strings.Builder, svc *betOrderService, res *BaseSpinResult, gameNum, triggeringBaseRound int, isFree bool) {
	mode := "基础模式"
	if isFree {
		mode = fmt.Sprintf("免费模式(由基础第%d局触发)", triggeringBaseRound)
	}
	w := func(s string, args ...interface{}) { buf.WriteString(fmt.Sprintf(s, args...)) }

	w("\n===== %s-第%d局 =====\n", mode, gameNum)

	// 转轮坐标
	printReelPositions(buf, svc)

	// 初始符号
	w("【初始符号图案】\n")
	printSymbolGrid(buf, svc, nil)

	// 框符号图案（免费模式）
	if isFree && len(svc.stepMap.Bat) > 0 {
		w("------------------------------------------------------\n")
		w("【框符号图案】当前%d个蝙蝠\n", len(svc.stepMap.Bat))
		printSymbolGrid(buf, svc, svc.stepMap.Bat)
	}

	// 变换后符号
	printTransformedGrid(buf, svc)

	// 蝙蝠移动详情
	if isFree && len(svc.stepMap.Bat) > 0 {
		printBatMovement(buf, svc)
	}

	// 中奖信息
	printWinInfo(buf, svc, res, isFree)
}

func printReelPositions(buf *strings.Builder, svc *betOrderService) {
	if svc.debug.reelPositions[0].length > 0 {
		buf.WriteString("【转轮坐标信息】\n")
		for k, v := range svc.debug.reelPositions {
			fmt.Fprintf(buf, "转轮%d: 长度=%d, 起始位置=%d\n", k+1, v.length, v.startIdx)
		}
	}
}

func printSymbolGrid(buf *strings.Builder, svc *betOrderService, bats []*Bat) {
	// 构建蝙蝠和夺宝位置集合
	batPos := make(map[string]bool)
	treasurePos := make(map[string]bool)

	if bats != nil {
		for _, bat := range bats {
			key := posKey(bat.X, bat.Y)
			batPos[key] = true
		}
	}

	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			sym := getSymbol(svc, r, c)
			if sym == _treasure {
				treasurePos[posKey(r, c)] = true
			}

			key := posKey(r, c)
			if batPos[key] {
				fmt.Fprintf(buf, " ^%2d", sym)
			} else if treasurePos[key] {
				fmt.Fprintf(buf, " *%2d", sym)
			} else {
				fmt.Fprintf(buf, "%3d ", sym)
			}

			if c < _colCount-1 {
				buf.WriteString("| ")
			}
		}
		buf.WriteString("\n")
	}
}

func printTransformedGrid(buf *strings.Builder, svc *betOrderService) {
	if svc.stepMap == nil || len(svc.stepMap.Bat) == 0 {
		return
	}

	// 统计转换信息
	converted := make(map[int64]int)
	transformedPos := make(map[string]bool)
	for _, bat := range svc.stepMap.Bat {
		if bat.Syb != _wild && bat.Sybn == _wild {
			converted[bat.Syb]++
			transformedPos[posKey(bat.TransX, bat.TransY)] = true
		}
	}

	if len(converted) == 0 {
		return
	}

	buf.WriteString("------------------------------------------------------\n【变换后符号图案")
	first := true
	for sym, cnt := range converted {
		if first {
			fmt.Fprintf(buf, "-%d→10", sym)
			first = false
		} else {
			fmt.Fprintf(buf, ", %d→10", sym)
		}
		if cnt > 1 {
			fmt.Fprintf(buf, "(×%d)", cnt)
		}
	}
	buf.WriteString("】\n")

	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			s := svc.symbolGrid[r][c]
			mark := " "
			if s == _wild && transformedPos[posKey(r, c)] {
				if svc.winGrid != nil && svc.winGrid[r][c] > 0 {
					mark = "*"
				} else {
					mark = "+"
				}
			}
			fmt.Fprintf(buf, "%3d%s", s, mark)
			if c < _colCount-1 {
				buf.WriteString("| ")
			}
		}
		buf.WriteString("\n")
	}
}

func printBatMovement(buf *strings.Builder, svc *betOrderService) {
	buf.WriteString("------------------------------------------------------\n")

	// 起飞位置
	buf.WriteString("【蝙蝠起飞位置】\n")
	printBatGrid(buf, svc.stepMap.Bat, true)

	// 降落位置
	buf.WriteString("【蝙蝠降落位置】\n")
	printBatGrid(buf, svc.stepMap.Bat, false)

	// 移动详情
	wildTransformed := 0
	for i, bat := range svc.stepMap.Bat {
		moved := bat.X != bat.TransX || bat.Y != bat.TransY
		transformed := bat.Syb != _wild && bat.Sybn == _wild
		if transformed {
			wildTransformed++
			fmt.Fprintf(buf, "蝙蝠%d: [%d,%d]→[%d,%d] 符号%d→Wild ✓转换\n",
				i+1, bat.X, bat.Y, bat.TransX, bat.TransY, bat.Syb)
		} else if moved {
			fmt.Fprintf(buf, "蝙蝠%d: [%d,%d]→[%d,%d] 符号%d→%d 移动但未转换\n",
				i+1, bat.X, bat.Y, bat.TransX, bat.TransY, bat.Syb, bat.Sybn)
		} else {
			fmt.Fprintf(buf, "蝙蝠%d: [%d,%d] 原地不动 符号%d\n", i+1, bat.X, bat.Y, bat.Syb)
		}
	}
	fmt.Fprintf(buf, "本次转换成Wild的数量: %d/%d\n", wildTransformed, len(svc.stepMap.Bat))
	buf.WriteString("------------------------------------------------------\n")
}

func printBatGrid(buf *strings.Builder, bats []*Bat, isStart bool) {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			batNums := ""
			for i, bat := range bats {
				var rowMatch, colMatch bool
				if isStart {
					rowMatch, colMatch = bat.X == r, bat.Y == c
				} else {
					rowMatch, colMatch = bat.TransX == r, bat.TransY == c
				}
				if rowMatch && colMatch {
					if batNums != "" {
						batNums += ","
					}
					batNums += fmt.Sprintf("%d", i+1)
				}
			}

			if batNums != "" {
				fmt.Fprintf(buf, "%3s ", batNums)
			} else {
				buf.WriteString("    ")
			}

			if c < _colCount-1 {
				buf.WriteString("| ")
			}
		}
		buf.WriteString("\n")
	}
}

func printWinInfo(buf *strings.Builder, svc *betOrderService, res *BaseSpinResult, isFree bool) {
	buf.WriteString("【中奖信息】\n")
	if len(svc.winResults) == 0 {
		buf.WriteString("未中奖\n")
	} else {
		for _, wr := range svc.winResults {
			fmt.Fprintf(buf, "符号: %d(%d), 连线: %d, 乘积: %d, 赔率: %.2f, 下注: 1×1, 奖金: %.2f\n",
				wr.Symbol, wr.Symbol, wr.SymbolCount, wr.LineCount,
				float64(wr.BaseLineMultiplier), float64(wr.TotalMultiplier))
		}
	}
	fmt.Fprintf(buf, "总中奖金额: %.2f\n", svc.bonusAmount.InexactFloat64())

	if isFree && res.TreasureCount > 0 {
		fmt.Fprintf(buf, "【新增免费次数】本局出现%d个夺宝，新增%d次免费游戏（每个夺宝+1次）\n",
			res.TreasureCount, res.TreasureCount)
	}
}

// ======== 工具函数 ========

func getSymbol(svc *betOrderService, row, col int64) int64 {
	if svc.debug.originalGrid != nil {
		return svc.debug.originalGrid[row][col]
	}
	return svc.stepMap.Map[row*_colCount+col]
}

func posKey(row, col int64) string {
	return fmt.Sprintf("%d_%d", row, col)
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
		merchant: &merchant.Merchant{ID: 20020, Merchant: "Jack23"},
		member:   &member.Member{ID: 1, MemberName: "Jack23", Balance: 10000000, Currency: "USD"},
		game:     &game.Game{ID: _gameID, GameName: "XXG2"},
		client: &client.Client{
			ClientOfFreeGame: &client.ClientOfFreeGame{},
			ClientGameCache:  &client.ClientGameCache{},
		},
		scene:       &SpinSceneData{},
		bonusAmount: decimal.Decimal{},
		betAmount:   decimal.Decimal{},
		amount:      decimal.Decimal{},
		debug:       rtpDebugData{open: true},
	}
}

func resetBetService(svc *betOrderService) {
	svc.client.ClientOfFreeGame = &client.ClientOfFreeGame{}
	svc.scene = &SpinSceneData{}
	svc.bonusAmount = decimal.Decimal{}
	svc.betAmount = decimal.Decimal{}
	svc.amount = decimal.Decimal{}
	svc.lineMultiplier = 0
	svc.stepMultiplier = 0
	svc.newFreeCount = 0
	svc.winInfos = nil
	svc.winResults = nil
	svc.symbolGrid = nil
	svc.winGrid = nil
	svc.stepMap = nil
	svc.debug = rtpDebugData{open: true}
}
