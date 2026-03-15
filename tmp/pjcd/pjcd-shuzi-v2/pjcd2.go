package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// ==================== 游戏配置参数 ====================
const (
	ROWS             = 3
	COLS             = 5
	REEL_LENGTH      = 100
	NUM_SYMBOLS      = 7
	WILD_SYMBOL      = 8
	SC_SYMBOL        = 9
	NUM_LINES        = 20
	BASE_BET         = 20
	BET_SIZE         = 0.1
	BET_LEVEL        = 5
	TOTAL_SPINS      = int(1e8) // 3000000
	PRINT_SPINS      = 20
	PRINT_FREE_SPINS = 0
	REEL_REGEN_N     = 10
	WILD_MAX_ROUNDS  = 3
)

// 基础模式判奖轮数倍数
var BASE_ROUND_MULTIPLIERS = []float64{1.0, 2.0, 3.0, 5.0}

// 免费模式判奖轮数倍数
var FREE_ROUND_MULTIPLIERS = []float64{3.0, 6.0, 9.0, 15.0}

const FOURTH_ROUND_BASE_MULTIPLIER = 5.0
const FREE_FOURTH_ROUND_BASE_MULTIPLIER = 15.0

// 基础模式符号权重
var BASE_SYMBOL_WEIGHTS = []int{1900, 1900, 1500, 1500, 1200, 1050, 950}
var FREE_SYMBOL_WEIGHTS = []int{1600, 1600, 1500, 1500, 1350, 1250, 1200}
var SYMBOL_PERMUTATION_WEIGHTS = []int{8100, 1700, 200}

const BASE_SCATTER_SYMBOL_PROB = 183

var BASE_WILD_SYMBOL_PROBS = []int{275, 275, 275}

const FREE_SCATTER_SYMBOL_PROB = 180

var FREE_WILD_SYMBOL_PROBS = []int{585, 585, 585}

// 免费模式参数
const (
	FREE_GAME_TIMES                      = 8
	FREE_GAME_SCATTER_MIN                = 3
	FREE_GAME_ADD_TIMES_PER_SCATTER      = 2
	FREE_GAME_TWO_SCATTER_ADD_TIMES      = 3
	FREE_GAME_ADD_TIMES_SCATTER_MIN      = 2
	FREE_GAME_ADD_MORE_TIMES_PER_SCATTER = 2
)

// 赔付表
var PAY_TABLE = [7][5]int{
	{0, 0, 2, 5, 10},
	{0, 0, 2, 5, 10},
	{0, 0, 5, 8, 20},
	{0, 0, 5, 8, 20},
	{0, 0, 8, 10, 50},
	{0, 0, 10, 20, 100},
	{0, 0, 20, 50, 200},
}

// 20条中奖线定义（与 pjcd/game_json.go 保持一致，0-based）
var LINES = [20][5]int{
	{5, 6, 7, 8, 9},
	{10, 11, 12, 13, 14},
	{0, 1, 2, 3, 4},
	{10, 6, 2, 8, 14},
	{0, 6, 12, 8, 4},
	{10, 11, 7, 13, 14},
	{0, 1, 7, 3, 4},
	{5, 1, 2, 3, 9},
	{5, 11, 12, 13, 9},
	{10, 6, 7, 8, 14},
	{0, 6, 7, 8, 4},
	{5, 6, 12, 8, 9},
	{5, 6, 2, 8, 9},
	{5, 11, 7, 13, 9},
	{5, 1, 7, 3, 9},
	{10, 6, 12, 8, 14},
	{0, 6, 2, 8, 4},
	{10, 11, 7, 3, 4},
	{0, 1, 7, 13, 14},
	{10, 1, 12, 3, 14},
}

// ==================== 数据结构定义 ====================
type Reel []int
type Reels []Reel
type GameMatrix [ROWS][COLS]int
type WildRoundsMatrix [ROWS][COLS]int

type WinResult struct {
	lineIndex  int
	symbol     int
	count      int
	multiplier int
	payout     float64
	positions  []Position
	roundNum   int
	roundMulti float64
	hasW2Bonus bool
	w2Bonus    int
}

type Position struct {
	row, col int
}

type GameMode int

const (
	BaseMode GameMode = iota
	FreeMode
)

type GameStats struct {
	baseBet          float64
	basePayout       float64
	baseSpins        int
	baseWinningSpins int
	baseRounds       int

	freePayout       float64
	freeSpins        int
	freeWinningSpins int
	freeRounds       int

	maxW2Bonus   int
	totalW2Bonus int
	w2Triggers   int

	freeGamesTriggered int
	freeGamesExtended  int
}

// ==================== 工具函数 ====================
func weightedRandom(weights []int, excludeSymbol int) int {
	adjustedWeights := make([]int, len(weights))
	copy(adjustedWeights, weights)

	if excludeSymbol >= 1 && excludeSymbol <= len(weights) {
		adjustedWeights[excludeSymbol-1] = 0
	}

	total := 0
	for _, w := range adjustedWeights {
		total += w
	}

	if total == 0 {
		copy(adjustedWeights, weights)
		total = 0
		for _, w := range adjustedWeights {
			total += w
		}
	}

	r := rand.Intn(total)
	sum := 0
	for i, w := range adjustedWeights {
		sum += w
		if r < sum {
			return i
		}
	}
	return len(adjustedWeights) - 1
}

func generateReel(symbolWeights []int, scatterProb int, wildProbs []int) Reel {
	reel := make([]int, 0, REEL_LENGTH)
	lastSymbol := 0

	for len(reel) < REEL_LENGTH {
		symbol := weightedRandom(symbolWeights, lastSymbol) + 1
		permType := weightedRandom(SYMBOL_PERMUTATION_WEIGHTS, -1)
		permLength := permType + 1

		for i := 0; i < permLength && len(reel) < REEL_LENGTH; i++ {
			reel = append(reel, symbol)
		}

		lastSymbol = symbol
	}

	if len(reel) > REEL_LENGTH {
		reel = reel[:REEL_LENGTH]
	}

	return reel
}

func generateReels(mode GameMode) Reels {
	reels := make(Reels, COLS)

	var symbolWeights []int
	var scatterProb int
	var wildProbs []int

	if mode == BaseMode {
		symbolWeights = BASE_SYMBOL_WEIGHTS
		scatterProb = BASE_SCATTER_SYMBOL_PROB
		wildProbs = BASE_WILD_SYMBOL_PROBS
	} else {
		symbolWeights = FREE_SYMBOL_WEIGHTS
		scatterProb = FREE_SCATTER_SYMBOL_PROB
		wildProbs = FREE_WILD_SYMBOL_PROBS
	}

	for c := 0; c < COLS; c++ {
		reels[c] = generateReel(symbolWeights, scatterProb, wildProbs)
	}

	for c := 0; c < COLS; c++ {
		allowWild := c >= 1 && c <= 3
		wildProb := 0
		if allowWild {
			wildProb = wildProbs[c-1]
		}
		for i := 0; i < REEL_LENGTH; i++ {
			// 对齐说明：
			// pjcd 使用“同一次随机抽样”决定 SC/WILD：
			// r < scatterProb => SC
			// scatterProb <= r < scatterProb+wildProb => WILD（仅中间三列）
			// 旧 pjcd2 采用两次独立随机（先 SC 再 WILD），会使实际 WILD 概率变为 (1-SC)*WILD，
			// 进而拉低/扭曲轮轴特征与RTP。这里改为单次抽样，严格对齐 pjcd。
			r := rand.Intn(10000)
			if r < scatterProb {
				reels[c][i] = SC_SYMBOL
				continue
			}
			if allowWild && r < scatterProb+wildProb {
				reels[c][i] = WILD_SYMBOL
			}
		}
	}

	return reels
}

func getSymbolFromReel(reel Reel, pos int) int {
	return reel[pos%REEL_LENGTH]
}

func generateMatrix(reels Reels, startPos [COLS]int) (GameMatrix, WildRoundsMatrix) {
	var matrix GameMatrix
	var wildRounds WildRoundsMatrix

	for col := 0; col < COLS; col++ {
		reel := reels[col]
		start := startPos[col]

		for row := 0; row < ROWS; row++ {
			pos := (start + row) % REEL_LENGTH
			symbol := reel[pos]
			// 对齐说明：
			// pjcd 内部 symbolGrid 的行索引是“0=底部，2=顶部”，并非常见的“0=顶部”。
			// 这里按 row 直接落位，保持 start 对应底部行；否则会导致线位映射上下颠倒，改变连线命中分布。
			matrix[row][col] = symbol

			if symbol == WILD_SYMBOL {
				wildRounds[row][col] = 0
			}
		}
	}

	return matrix, wildRounds
}

func symbolsMatch(sym1, sym2 int) bool {
	if sym1 == WILD_SYMBOL || sym2 == WILD_SYMBOL {
		return true
	}
	return sym1 == sym2
}

func checkLineWin(matrix GameMatrix, wildRounds WildRoundsMatrix, lineIndex int) (WinResult, bool) {
	line := LINES[lineIndex]
	symbols := make([]int, COLS)
	positions := make([]Position, COLS)

	for i, pos := range line {
		row := pos / COLS
		col := pos % COLS
		symbols[i] = matrix[row][col]
		positions[i] = Position{row, col}
	}

	firstSymbol := symbols[0]
	if firstSymbol == SC_SYMBOL {
		return WinResult{}, false
	}

	count := 1
	for i := 1; i < COLS; i++ {
		currentSymbol := symbols[i]

		if currentSymbol == SC_SYMBOL {
			break
		}
		if symbolsMatch(firstSymbol, currentSymbol) {
			count++
		} else {
			break
		}
		if firstSymbol == WILD_SYMBOL && currentSymbol != WILD_SYMBOL {
			firstSymbol = currentSymbol
		}
	}

	if count >= 3 {
		winSymbol := firstSymbol
		for i := 0; i < count; i++ {
			if symbols[i] != WILD_SYMBOL {
				winSymbol = symbols[i]
				break
			}
		}

		multiplier := 0
		if winSymbol >= 1 && winSymbol <= 7 {
			multiplier = PAY_TABLE[winSymbol-1][count-1]
		}

		winPositions := positions[:count]

		return WinResult{
			lineIndex:  lineIndex + 1,
			symbol:     winSymbol,
			count:      count,
			multiplier: multiplier,
			positions:  winPositions,
		}, multiplier > 0
	}

	return WinResult{}, false
}

func checkAllWins(matrix GameMatrix, wildRounds WildRoundsMatrix) []WinResult {
	var wins []WinResult

	for lineIdx := 0; lineIdx < NUM_LINES; lineIdx++ {
		if win, ok := checkLineWin(matrix, wildRounds, lineIdx); ok {
			basePayout := float64(win.multiplier) * BET_SIZE * float64(BET_LEVEL)
			win.payout = basePayout
			wins = append(wins, win)
		}
	}

	return wins
}

func countScatterSymbols(matrix GameMatrix) int {
	count := 0
	for r := 0; r < ROWS; r++ {
		for c := 0; c < COLS; c++ {
			if matrix[r][c] == SC_SYMBOL {
				count++
			}
		}
	}
	return count
}

func markWildSymbolsInWins(matrix GameMatrix, wildRounds *WildRoundsMatrix, wins []WinResult) (int, []Position) {
	wildPositions := make(map[Position]bool)
	w2Positions := make([]Position, 0)

	for _, win := range wins {
		for _, pos := range win.positions {
			if matrix[pos.row][pos.col] == WILD_SYMBOL {
				key := Position{pos.row, pos.col}
				if !wildPositions[key] {
					wildPositions[key] = true

					if (*wildRounds)[pos.row][pos.col] == 2 {
						w2Positions = append(w2Positions, Position{pos.row, pos.col})
					}
				}
			}
		}
	}

	w2Count := 0
	for pos := range wildPositions {
		currentRounds := (*wildRounds)[pos.row][pos.col]
		(*wildRounds)[pos.row][pos.col] = currentRounds + 1

		if currentRounds == 2 {
			w2Count++
		}
	}

	return w2Count, w2Positions
}

func simulateSingleDrop(matrix GameMatrix, wildRounds WildRoundsMatrix, reels Reels, fallPos [COLS]int, w2Positions []Position) (GameMatrix, WildRoundsMatrix, [COLS]int) {
	newMatrix := matrix
	newWildRounds := wildRounds
	newFallPos := fallPos

	removePositions := make([][]bool, ROWS)
	for r := 0; r < ROWS; r++ {
		removePositions[r] = make([]bool, COLS)
	}

	wins := checkAllWins(matrix, wildRounds)

	for _, win := range wins {
		for _, pos := range win.positions {
			if newMatrix[pos.row][pos.col] != WILD_SYMBOL {
				removePositions[pos.row][pos.col] = true
			}
		}
	}

	for r := 0; r < ROWS; r++ {
		for c := 0; c < COLS; c++ {
			if newMatrix[r][c] == WILD_SYMBOL && newWildRounds[r][c] >= WILD_MAX_ROUNDS {
				removePositions[r][c] = true
			}
		}
	}

	for _, pos := range w2Positions {
		removePositions[pos.row][pos.col] = true
	}

	for col := 0; col < COLS; col++ {
		// 对齐说明：
		// pjcd 的 dropSymbols 是“向低索引压缩”(writePos 从 0 往上)，
		// ringSymbol 再从高索引到低索引补新符号；即内部网格重力方向是“高索引 -> 低索引”。
		// 这里改为同样的压缩与补位顺序，避免连消形态与触发率偏移。
		writePos := 0
		for row := 0; row < ROWS; row++ {
			if removePositions[row][col] {
				newMatrix[row][col] = 0
				newWildRounds[row][col] = 0
				continue
			}
			if writePos != row {
				newMatrix[writePos][col] = newMatrix[row][col]
				newWildRounds[writePos][col] = newWildRounds[row][col]
				newMatrix[row][col] = 0
				newWildRounds[row][col] = 0
			}
			writePos++
		}

		for row := ROWS - 1; row >= writePos; row-- {
			// 对齐说明：
			// 先推进 Fall 再取符号，且补位顺序为“从高索引到低索引”，与 pjcd ringSymbol 一致。
			newFallPos[col] = (newFallPos[col] + 1) % REEL_LENGTH
			symbol := getSymbolFromReel(reels[col], newFallPos[col])
			newMatrix[row][col] = symbol
			newWildRounds[row][col] = 0
		}
	}

	return newMatrix, newWildRounds, newFallPos
}

func getRoundMultiplier(round int, w2Bonus int, mode GameMode) float64 {
	if round <= 0 {
		return 1.0
	}

	var baseMultipliers []float64
	var fourthRoundBase float64

	if mode == BaseMode {
		baseMultipliers = BASE_ROUND_MULTIPLIERS
		fourthRoundBase = FOURTH_ROUND_BASE_MULTIPLIER
	} else {
		baseMultipliers = FREE_ROUND_MULTIPLIERS
		fourthRoundBase = FREE_FOURTH_ROUND_BASE_MULTIPLIER
	}

	if round <= 3 {
		return baseMultipliers[round-1]
	}

	return fourthRoundBase + float64(w2Bonus)
}

func simulateDrop(matrix GameMatrix, wildRounds WildRoundsMatrix, reels Reels, startPos [COLS]int, mode GameMode, printDetails bool, stats *GameStats, freeModeFourthRoundBonus *int) (GameMatrix, WildRoundsMatrix, [COLS]int, float64, int) {
	currentMatrix := matrix
	currentWildRounds := wildRounds
	var currentFallPos [COLS]int
	for c := 0; c < COLS; c++ {
		// 对齐说明：
		// pjcd 在建盘后将 Fall 初始化为 end=(start+rowCount-1)%len(reel)，
		// 补位时先 Fall+1 再取符号；这里保持同样初始化，确保连消补位分布一致。
		currentFallPos[c] = (startPos[c] + ROWS - 1) % REEL_LENGTH
	}
	totalPayout := 0.0
	round := 1
	rounds := 0

	// 第4轮倍数加成（W2百搭符号提供）
	// 基础模式：每次spin独立
	// 免费模式：在整个免费模式期间累积
	fourthRoundBonus := 0
	if mode == BaseMode {
		// 基础模式：每次spin从0开始
		fourthRoundBonus = 0
	} else {
		// 免费模式：使用传入的累积值
		fourthRoundBonus = *freeModeFourthRoundBonus
	}

	for {
		wins := checkAllWins(currentMatrix, currentWildRounds)
		if len(wins) == 0 {
			break
		}

		w2Count, w2Positions := markWildSymbolsInWins(currentMatrix, &currentWildRounds, wins)

		if w2Count > 0 {
			// 无论当前是第几轮，W2百搭符号都会增加第4轮倍数
			bonusIncrease := w2Count * 5

			// 更新第4轮判奖倍数加成
			fourthRoundBonus += bonusIncrease

			// 如果是在免费模式下，更新外部累积值
			if mode == FreeMode {
				*freeModeFourthRoundBonus = fourthRoundBonus
			}

			// 统计W2百搭符号
			stats.w2Triggers += w2Count
			stats.totalW2Bonus += bonusIncrease
			if bonusIncrease > stats.maxW2Bonus {
				stats.maxW2Bonus = bonusIncrease
			}
		}

		// 获取当前轮数的倍数
		roundMulti := getRoundMultiplier(round, fourthRoundBonus, mode)

		// 计算本轮赔付（应用轮数倍数）
		roundPayout := 0.0
		for i := range wins {
			wins[i].roundNum = round
			wins[i].roundMulti = roundMulti

			// 标记是否有W2百搭符号加成
			if round >= 4 && fourthRoundBonus > 0 {
				wins[i].hasW2Bonus = true
				wins[i].w2Bonus = fourthRoundBonus
			}

			basePayout := wins[i].payout
			wins[i].payout = basePayout * roundMulti
			roundPayout += wins[i].payout
		}
		totalPayout += roundPayout

		if mode == BaseMode {
			stats.baseRounds++
		} else {
			stats.freeRounds++
		}

		if printDetails {
			fmt.Printf("\n  ── 第%d轮判奖 ──\n", round)
			printMatrix(currentMatrix, currentWildRounds, fmt.Sprintf("  第%d轮盘面:", round))

			var fourthRoundBase float64
			if mode == BaseMode {
				fourthRoundBase = FOURTH_ROUND_BASE_MULTIPLIER
			} else {
				fourthRoundBase = FREE_FOURTH_ROUND_BASE_MULTIPLIER
			}

			if round >= 4 && fourthRoundBonus > 0 {
				fmt.Printf("  轮数倍数: %.0f倍 (基础%.0f + W2加成%d)\n", roundMulti, fourthRoundBase, fourthRoundBonus)
			} else {
				fmt.Printf("  轮数倍数: %.0f倍\n", roundMulti)
			}

			fmt.Printf("  中奖信息:\n")
			for _, win := range wins {
				symbolStr := fmt.Sprintf("%d", win.symbol)
				if win.symbol == WILD_SYMBOL {
					symbolStr = "W"
				}
				bonusText := ""
				if win.hasW2Bonus {
					bonusText = fmt.Sprintf("(W2加成:%d)", win.w2Bonus)
				}
				fmt.Printf("    - 线%d: 符号%s连续%d个, 基础倍数:%d, 轮数倍数:%.0f, 总赔付:%.1f %s\n",
					win.lineIndex, symbolStr, win.count, win.multiplier, win.roundMulti, win.payout, bonusText)
			}

			if w2Count > 0 {
				fmt.Printf("  W2百搭符号: %d个参与中奖\n", w2Count)
				if round >= 4 {
					fmt.Printf("  第4轮倍数增加: +%d (当前第4轮倍数: %.0f)\n", w2Count*5, roundMulti)
				} else {
					fmt.Printf("  第4轮倍数增加: +%d (将在第4轮生效)\n", w2Count*5)
				}
			}

			fmt.Printf("  本轮总赔付: %.1f\n", roundPayout)
		}

		// 执行单轮掉落
		currentMatrix, currentWildRounds, currentFallPos = simulateSingleDrop(
			currentMatrix, currentWildRounds, reels, currentFallPos, w2Positions)

		round++
		rounds++
		if round > 20 {
			fmt.Println("警告：掉落轮数超过20轮，可能有问题")
			break
		}
	}

	return currentMatrix, currentWildRounds, currentFallPos, totalPayout, rounds
}

func printMatrix(matrix GameMatrix, wildRounds WildRoundsMatrix, title string) {
	fmt.Println(title)
	for r := 0; r < ROWS; r++ {
		fmt.Print("  ")
		for c := 0; c < COLS; c++ {
			symbol := matrix[r][c]
			switch symbol {
			case WILD_SYMBOL:
				rounds := wildRounds[r][c]
				if rounds > 0 {
					fmt.Printf("W%d ", rounds)
				} else {
					fmt.Print("W  ")
				}
			case SC_SYMBOL:
				fmt.Print("SC ")
			case 0:
				fmt.Print(".  ")
			default:
				fmt.Printf("%d  ", symbol)
			}
		}
		fmt.Println()
	}
}

func calculateFreeGames(scatterCount int) int {
	if scatterCount < FREE_GAME_SCATTER_MIN {
		return 0
	}
	return FREE_GAME_TIMES + (scatterCount-FREE_GAME_SCATTER_MIN)*FREE_GAME_ADD_TIMES_PER_SCATTER
}

func calculateFreeRetriggerGames(scatterCount int) int {
	if scatterCount < FREE_GAME_ADD_TIMES_SCATTER_MIN {
		return 0
	}
	return FREE_GAME_TWO_SCATTER_ADD_TIMES + (scatterCount-FREE_GAME_ADD_TIMES_SCATTER_MIN)*FREE_GAME_ADD_MORE_TIMES_PER_SCATTER
}

func simulateGame() {
	stats := &GameStats{
		baseSpins: 0,
		freeSpins: 0,
	}

	baseReels := generateReels(BaseMode)
	baseReelSpinCounter := 0

	freeReels := make(Reels, 0)
	var freeStartPos [COLS]int
	freeSpinsRemaining := 0
	inFreeMode := false
	freeSpinCounter := 0

	currentReels := baseReels
	currentMode := BaseMode

	// 免费模式打印计数器
	freeSpinsPrinted := 0

	// 免费模式第四轮倍数加成（在整个免费模式期间累积）
	freeModeFourthRoundBonus := 0

	// 模拟基础模式
	for stats.baseSpins < TOTAL_SPINS {
		printDetails := stats.baseSpins < PRINT_SPINS

		if printDetails {
			fmt.Printf("\n=== 基础模式 第%d次旋转 ===\n", stats.baseSpins+1)
		}

		if stats.baseSpins%REEL_REGEN_N == 0 && stats.baseSpins > 0 {
			baseReels = generateReels(BaseMode)
			baseReelSpinCounter = 0
			currentReels = baseReels
			if printDetails {
				fmt.Println("【新轮轴已生成】")
			}
		}

		var startPos [COLS]int
		for c := 0; c < COLS; c++ {
			startPos[c] = rand.Intn(REEL_LENGTH)
		}

		matrix, wildRounds := generateMatrix(currentReels, startPos)

		spinBet := BET_SIZE * float64(BET_LEVEL) * float64(BASE_BET)
		stats.baseBet += spinBet

		if printDetails {
			fmt.Printf("起点位置: %v\n", startPos)
		}

		// 基础模式下，传入nil表示不使用外部累积
		finalMatrix, finalWildRounds, _, spinPayout, rounds := simulateDrop(
			matrix, wildRounds, currentReels, startPos, currentMode, printDetails, stats, nil)

		stats.basePayout += spinPayout
		stats.baseSpins++
		if rounds > 0 || len(checkAllWins(matrix, wildRounds)) > 0 {
			stats.baseWinningSpins++
		}

		scatterCount := countScatterSymbols(finalMatrix)

		if printDetails {
			hasWinningSpin := rounds > 0 || len(checkAllWins(matrix, wildRounds)) > 0

			if hasWinningSpin {
				fmt.Printf("\n  本次旋转进行了%d轮判奖\n", rounds)
				printMatrix(finalMatrix, finalWildRounds, "  最终盘面:")
				fmt.Printf("  本次旋转总赔付: %.1f\n", spinPayout)
			} else {
				printMatrix(matrix, wildRounds, "初始盘面:")
				fmt.Println("未中奖")
			}

			if scatterCount > 0 {
				fmt.Printf("  最终盘面有%d个夺宝符号\n", scatterCount)
			}
		}

		if scatterCount >= FREE_GAME_SCATTER_MIN {
			freeSpinsRemaining = calculateFreeGames(scatterCount)
			stats.freeGamesTriggered++

			if printDetails {
				fmt.Printf("  → 触发免费模式: %d次免费旋转\n", freeSpinsRemaining)
			}

			freeReels = generateReels(FreeMode)
			currentReels = freeReels
			currentMode = FreeMode
			inFreeMode = true
			freeSpinCounter = 0

			// 重置免费模式第四轮倍数加成
			freeModeFourthRoundBonus = 0

			for c := 0; c < COLS; c++ {
				freeStartPos[c] = rand.Intn(REEL_LENGTH)
			}

			// 进入免费模式循环
			for inFreeMode && freeSpinsRemaining > 0 {
				// 检查是否还需要打印免费模式spin
				printFreeDetails := freeSpinsPrinted < PRINT_FREE_SPINS

				if printFreeDetails {
					fmt.Printf("\n=== 免费模式 第%d次旋转 (剩余%d次) ===\n", freeSpinCounter+1, freeSpinsRemaining)
				}

				var freeSpinStartPos [COLS]int
				if freeSpinCounter == 0 {
					freeSpinStartPos = freeStartPos
				} else {
					for c := 0; c < COLS; c++ {
						freeSpinStartPos[c] = rand.Intn(REEL_LENGTH)
					}
				}

				freeMatrix, freeWildRounds := generateMatrix(freeReels, freeSpinStartPos)

				if printFreeDetails {
					fmt.Printf("起点位置: %v\n", freeSpinStartPos)
				}

				// 免费模式下，传入累积的第四轮倍数加成
				freeFinalMatrix, freeFinalWildRounds, freeFinalStartPos, freeSpinPayout, freeRounds := simulateDrop(
					freeMatrix, freeWildRounds, freeReels, freeSpinStartPos, FreeMode, printFreeDetails, stats, &freeModeFourthRoundBonus)

				stats.freePayout += freeSpinPayout
				stats.freeSpins++
				if freeRounds > 0 || len(checkAllWins(freeMatrix, freeWildRounds)) > 0 {
					stats.freeWinningSpins++
				}

				freeScatterCount := countScatterSymbols(freeFinalMatrix)

				if printFreeDetails {
					hasFreeWinningSpin := freeRounds > 0 || len(checkAllWins(freeMatrix, freeWildRounds)) > 0

					if hasFreeWinningSpin {
						fmt.Printf("\n  本次免费旋转进行了%d轮判奖\n", freeRounds)
						printMatrix(freeFinalMatrix, freeFinalWildRounds, "  最终盘面:")
						fmt.Printf("  本次免费旋转总赔付: %.1f\n", freeSpinPayout)
						fmt.Printf("  当前免费模式第四轮倍数加成累积: +%d\n", freeModeFourthRoundBonus)
					} else {
						printMatrix(freeMatrix, freeWildRounds, "初始盘面:")
						fmt.Println("未中奖")
					}

					if freeScatterCount > 0 {
						fmt.Printf("  最终盘面有%d个夺宝符号\n", freeScatterCount)
					}

					// 增加已打印的免费模式spin计数
					freeSpinsPrinted++
				}

				if freeScatterCount >= FREE_GAME_ADD_TIMES_SCATTER_MIN {
					additionalGames := calculateFreeRetriggerGames(freeScatterCount)
					freeSpinsRemaining += additionalGames
					stats.freeGamesExtended++

					if printFreeDetails {
						fmt.Printf("  → %d个夺宝符号，增加免费次数: +%d次 (当前剩余%d次)\n", freeScatterCount, additionalGames, freeSpinsRemaining)
					}
				}

				freeStartPos = freeFinalStartPos
				freeSpinsRemaining--
				freeSpinCounter++

				// 防止无限循环
				if stats.baseSpins+stats.freeSpins > TOTAL_SPINS*10 {
					fmt.Println("警告：总旋转次数过多，强制结束")
					inFreeMode = false
					break
				}
			}

			// 对齐说明：
			// pjcd 免费结束后直接回到基础模式并复用当前基础轮轴；
			// 旧 pjcd2 在这里会强制重建基础轮轴，导致基础段分布突变、长期RTP偏移。
			// 因此此处仅切回 baseReels，不做重建。
			// 免费模式结束
			if inFreeMode {
				currentReels = baseReels
				currentMode = BaseMode
				inFreeMode = false
				freeSpinsRemaining = 0

				// 重置免费模式第四轮倍数加成
				freeModeFourthRoundBonus = 0

				if stats.baseSpins < PRINT_SPINS {
					fmt.Println("\n【免费模式结束，回到基础模式】")
				}
			}
		}

		baseReelSpinCounter++

		// 显示进度
		if stats.baseSpins%100000 == 0 {
			fmt.Printf("已完成基础模式spin: %d/%d\n", stats.baseSpins, TOTAL_SPINS)
		}
	}

	// 统计结果
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("模拟结果")
	fmt.Println(strings.Repeat("=", 50))

	// 基础模式统计
	//fmt.Println("\n基础模式统计:")
	//fmt.Printf("总旋转次数: %d\n", stats.baseSpins)
	//fmt.Printf("中奖Spin数: %d\n", stats.baseWinningSpins)
	//fmt.Printf("总判奖轮数: %d\n", stats.baseRounds)
	//fmt.Printf("平均每Spin判奖轮数: %.2f\n", float64(stats.baseRounds)/float64(stats.baseSpins))
	//fmt.Printf("总投注: %.1f\n", stats.baseBet)
	//fmt.Printf("总赔付: %.1f\n", stats.basePayout)

	baseRTP := 0.0
	if stats.baseBet > 0 {
		baseRTP = (stats.basePayout / stats.baseBet) * 100
	}

	baseHitRate := 0.0
	if stats.baseSpins > 0 {
		baseHitRate = (float64(stats.baseWinningSpins) / float64(stats.baseSpins)) * 100
	}

	fmt.Printf("基础模式RTP: %.2f%%\n", baseRTP)
	fmt.Printf("基础模式中奖率: %.2f%%\n", baseHitRate)

	// 免费模式统计
	/*fmt.Println("\n免费模式统计:")
	fmt.Printf("总免费旋转次数: %d\n", stats.freeSpins)
	fmt.Printf("中奖免费Spin数: %d\n", stats.freeWinningSpins)
	fmt.Printf("总判奖轮数: %d\n", stats.freeRounds)
	if stats.freeSpins > 0 {
		fmt.Printf("平均每免费Spin判奖轮数: %.2f\n", float64(stats.freeRounds)/float64(stats.freeSpins))
		if stats.freeGamesTriggered > 0 {
			fmt.Printf("平均每次免费模式spin数: %.2f\n", float64(stats.freeSpins)/float64(stats.freeGamesTriggered))
		}
	}*/
	//fmt.Printf("总赔付: %.1f\n", stats.freePayout)
	//fmt.Printf("增加免费次数触发次数: %d\n", stats.freeGamesExtended)

	if stats.freeSpins > 0 {
		freeHitRate := (float64(stats.freeWinningSpins) / float64(stats.freeSpins)) * 100
		fmt.Printf("免费模式中奖率: %.2f%%\n", freeHitRate)
	}
	fmt.Printf("免费模式总游戏局数: %d\n", stats.freeSpins)
	fmt.Printf("免费模式触发次数: %d\n", stats.freeGamesTriggered)
	if stats.freeGamesTriggered > 0 {
		fmt.Printf("平均每次触发获得免费次数: %.2f\n", float64(stats.freeSpins)/float64(stats.freeGamesTriggered))
	}

	// 免费模式触发率
	freeGameTriggerRate := 0.0
	if stats.baseSpins > 0 {
		freeGameTriggerRate = (float64(stats.freeGamesTriggered) / float64(stats.baseSpins)) * 100
	}
	fmt.Printf("免费模式触发率: %.2f%%\n", freeGameTriggerRate)

	// 免费模式RTP
	freeGameRTP := 0.0
	if stats.baseBet > 0 {
		freeGameRTP = (stats.freePayout / stats.baseBet) * 100
	}
	fmt.Printf("免费模式RTP: %.2f%%\n", freeGameRTP)

	// 整体统计
	//fmt.Println("\n整体统计:")
	//fmt.Printf("总投注: %.1f\n", totalBet)
	//fmt.Printf("总赔付: %.1f\n", totalPayout)
	//fmt.Printf("整体RTP: %.2f%%\n", overallRTP)
	//fmt.Printf("总旋转次数: %d (基础: %d, 免费: %d)\n",
	//stats.baseSpins+stats.freeSpins, stats.baseSpins, stats.freeSpins)
	totalBet := stats.baseBet
	totalPayout := stats.basePayout + stats.freePayout
	overallRTP := 0.0
	if totalBet > 0 {
		overallRTP = (totalPayout / totalBet) * 100
	}

	fmt.Printf("整体RTP: %.2f%%\n", overallRTP)

	// W2百搭符号统计
	/*if stats.w2Triggers > 0 {
		fmt.Println("\nW2百搭符号统计:")
		fmt.Printf("W2触发次数: %d\n", stats.w2Triggers)
		fmt.Printf("最大单次W2加成: +%d\n", stats.maxW2Bonus)
		fmt.Printf("平均每次W2加成: +%.1f\n", float64(stats.totalW2Bonus)/float64(stats.w2Triggers))
	}*/
}

func main() {
	rand.Seed(time.Now().UnixNano())
	start := time.Now()
	fmt.Println("开始游戏模拟...")
	simulateGame()
	fmt.Printf("use: %v \n", time.Since(start))
}
