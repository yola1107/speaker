package xslm2

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	mathRand "math/rand"
	"strconv"
	"strings"
	"sync"

	"egame-grpc/global"
)

// 随机数生成器
var randPool = &sync.Pool{
	New: func() any {
		var seed int64
		_ = binary.Read(rand.Reader, binary.LittleEndian, &seed)
		return mathRand.New(mathRand.NewSource(seed))
	},
}

// contains 检查值是否在切片中
func contains[T comparable](slice []T, val T) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// ToJSON json string
func ToJSON(v any) string {
	j, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(j)
}

func WGrid(grid *int64Grid) string {
	if grid == nil {
		return ""
	}
	b := strings.Builder{}
	b.WriteString("\n")
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			sym := grid[r][c]
			b.WriteString(fmt.Sprintf("%3d", sym))
			if c < _colCount-1 {
				b.WriteString("| ")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// gridToString 网格转字符串（通用函数）
func gridToString(grid *int64Grid) string {
	if grid == nil {
		return ""
	}

	var b strings.Builder
	for r := int64(0); r < _rowCount; r++ {
		b.WriteString("[")
		for c := int64(0); c < _colCount; c++ {
			b.WriteString(strconv.FormatInt(grid[r][c], 10))
			if c < _colCount-1 {
				b.WriteString(",")
			}
		}
		b.WriteString("]")
		if r < _rowCount-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// hasWildSymbol 判断符号网格中是否有Wild符号
func hasWildSymbol(grid *int64Grid) bool {
	if grid == nil {
		return false
	}
	for _, row := range grid {
		for _, symbol := range row {
			if symbol == _wild {
				return true
			}
		}
	}
	return false
}

// getTreasureCount 获取符号网格中的夺宝符号数量
func getTreasureCount(grid *int64Grid) int64 {
	return countSymbol(grid, _treasure)
}

// countSymbol 统计符号网格中指定符号的数量（通用函数）
func countSymbol(grid *int64Grid, targetSymbol int64) int64 {
	if grid == nil {
		return 0
	}
	count := int64(0)
	for _, row := range grid {
		for _, symbol := range row {
			// 跳过墙格标记
			if symbol == _blocked {
				continue
			}
			if symbol == targetSymbol {
				count++
			}
		}
	}
	return count
}

// 调试用

// handleSymbolGrid 从 SymbolRoller.BoardSymbol 恢复网格
// BoardSymbol 从下往上存储，需要转换为标准的 symbolGrid 坐标系统
func handleSymbolGrid(rollers [_colCount]SymbolRoller) int64Grid {
	var symbolGrid int64Grid
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			// BoardSymbol 从下往上存储，所以需要反转索引
			// symbolGrid[0][col] 对应 BoardSymbol[3]，symbolGrid[3][col] 对应 BoardSymbol[0]
			symbolGrid[_rowCount-1-r][c] = rollers[c].BoardSymbol[r]
		}
	}
	return symbolGrid
}

// fallingWinSymbols 将处理后的网格写回 SymbolRoller.BoardSymbol
// 注意：fillBlanks 已经填充了所有空白，所以这里只需要将网格写入 BoardSymbol
func fallingWinSymbols(rollers *[_colCount]SymbolRoller, nextSymbolGrid int64Grid) {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			// BoardSymbol 从下往上存储，所以需要反转索引
			rollers[c].BoardSymbol[r] = nextSymbolGrid[_rowCount-1-r][c]
		}
	}
	// ringSymbol 用于填充 BoardSymbol 中的 0（空白），但 fillBlanks 已经填充了所有空白
	// 所以这里调用 ringSymbol 是为了确保 BoardSymbol 中没有 0（防御性编程）
	for i := range rollers {
		rollers[i].ringSymbol()
	}
}

// verifyGridConsistencyWithLog 验证并记录不一致的详细信息
func verifyGridConsistencyWithLog(rollers [_colCount]SymbolRoller, nextSymbolGrid *int64Grid) bool {
	if nextSymbolGrid == nil {
		return false
	}
	restoredGrid := handleSymbolGrid(rollers)
	allMatch := true
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if restoredGrid[r][c] != (*nextSymbolGrid)[r][c] {
				allMatch = false
				global.GVA_LOG.Sugar().Warnf("网格不一致: row=%d col=%d, BoardSymbol恢复=%d, nextGrid=%d",
					r, c, restoredGrid[r][c], (*nextSymbolGrid)[r][c])
			}
		}
	}
	return allMatch
}

/*
算分：女性百搭（10，11，12）可替换为基础符号（1，2，3，4，5，6，7，8，9），但连线上必须要有基础符号

消除：
	基础模式：消除中奖的女性符号（7，8，9）及百搭，如果盘面有夺宝则百搭不消除
	免费模式：
		1> 全屏情况：当触发111时（ABC都大于10）， 女性百搭参与的中奖符号（1-12）全部消除，（普通百搭 13 保留、夺宝 14 保留）
		2> 非全屏情况：中奖的女性符号会消除（7，8，9，10，11，12），夺宝符号和百搭不消除
*/

//
// v2
/*
算分：女性百搭（10，11，12）可替换为基础符号（1，2，3，4，5，6，7，8，9），但连线上必须要有基础符号

消除：
	基础模式：消除中奖的女性符号（7，8，9）及百搭，如果盘面有夺宝则百搭不消除
	免费模式：
		1> 全屏情况：每个中奖Way找女性百搭，找到则改way除百搭13之外的符号都全部消除
		2> 非全屏情况：每个中奖way找女性，找到该way女性及女性百搭都消除
*/

/*




package xslm2

import "egame-grpc/global"

type spin struct {
	// 持久化
	femaleCountsForFree     [3]int64
	nextFemaleCountsForFree [3]int64
	rollerKey               string
	rollers                 [_colCount]SymbolRoller
	nextSymbolGrid          *int64Grid

	// 本 step
	symbolGrid            *int64Grid
	winGrid               *int64Grid
	winInfos              []*winInfo
	winResults            []*winResult
	stepMultiplier        int64
	hasFemaleWin          bool
	hasFemaleWildWin      bool
	enableFullElimination bool
	isRoundOver           bool
	treasureCount         int64
	newFreeRoundCount     int64

	// 免费局夺宝累计（按步增量累加，避免净差值漏计）
	treasureGainedThisRound int64
	lastTreasureCount       int64

	// 本回合开始时的女性收集，用于调试输出
	roundStartFemaleCounts [3]int64
	roundStartTreasure     int64
}

type cascadeMode int

const (
	cascadeModeNone cascadeMode = iota
	cascadeModeBase
	cascadeModeFreePartial
	cascadeModeFreeFull
)

func (s *spin) desc() string { return ToJSON(s) }

func (s *spin) baseSpin(isFree, isFirst bool, nextGrid *int64Grid, rollers *[_colCount]SymbolRoller) {
	s.loadStepData(isFree, isFirst, nextGrid, rollers)
	s.findWinInfos()
	s.processStep(isFree)
	s.finalizeRound(isFree)
}

func (s *spin) loadStepData(isFree, isFirst bool, nextGrid *int64Grid, rollers *[_colCount]SymbolRoller) {
	if isFirst {
		s.initSpin(isFree)
		s.roundStartTreasure = getTreasureCount(s.symbolGrid)
		s.nextFemaleCountsForFree = s.femaleCountsForFree
	} else if nextGrid == nil || rollers == nil {
		global.GVA_LOG.Sugar().Errorf("免费模式下，空nextGrid/rollers：%s", s.desc())
		s.initSpin(isFree)
		s.roundStartTreasure = getTreasureCount(s.symbolGrid)
		s.nextFemaleCountsForFree = s.femaleCountsForFree
		s.roundStartFemaleCounts = s.femaleCountsForFree
	} else {
		s.symbolGrid = nextGrid
		s.rollers = *rollers
	}

	if isFree {
		convertFemaleToWild(s.symbolGrid, s.femaleCountsForFree)
		currTreasure := getTreasureCount(s.symbolGrid)
		if isFirst {
			s.treasureGainedThisRound = currTreasure
		} else if diff := currTreasure - s.lastTreasureCount; diff > 0 {
			s.treasureGainedThisRound += diff
		}
		s.lastTreasureCount = currTreasure
	} else {
		s.treasureGainedThisRound, s.lastTreasureCount = 0, 0
	}

	s.enableFullElimination = isFree &&
		s.femaleCountsForFree[0] >= _femaleSymbolCountForFullElimination &&
		s.femaleCountsForFree[1] >= _femaleSymbolCountForFullElimination &&
		s.femaleCountsForFree[2] >= _femaleSymbolCountForFullElimination
}

func (s *spin) initSpin(isFree bool) {
	grid, rls, key := _cnf.initSpinSymbol(isFree, s.femaleCountsForFree)
	clearBlockedCells(&grid)
	s.symbolGrid, s.rollers, s.rollerKey = &grid, rls, key
}

func (s *spin) findWinInfos() bool {
	s.hasFemaleWin = false
	s.hasFemaleWildWin = false
	var wins []*winInfo
	for sym := _blank + 1; sym < _wildFemaleA; sym++ {
		if info, ok := s.findNormalWin(sym); ok {
			if sym >= _femaleA {
				s.hasFemaleWin = true
			}
			if infoHasFemaleWild(info.WinGrid) {
				s.hasFemaleWildWin = true
			}
			wins = append(wins, info)
		}
	}
	for sym := _wildFemaleA; sym < _wild; sym++ {
		if info, ok := s.findWildWin(sym); ok {
			s.hasFemaleWin, s.hasFemaleWildWin = true, true
			wins = append(wins, info)
		}
	}
	s.winInfos = wins
	return len(wins) > 0
}

func (s *spin) findNormalWin(sym int64) (*winInfo, bool) {
	lineCnt := int64(1)
	var wg int64Grid
	exist := false
	for c := int64(0); c < _colCount; c++ {
		cnt := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			curr := s.symbolGrid[r][c]
			if curr == sym || curr == _wild || isMatchingFemaleWild(sym, curr) {
				if curr == sym {
					exist = true
				}
				cnt++
				wg[r][c] = curr
			}
		}
		if cnt == 0 {
			if c >= _minMatchCount && exist {
				return &winInfo{Symbol: sym, SymbolCount: c, LineCount: lineCnt, WinGrid: wg}, true
			}
			break
		}
		lineCnt *= cnt
		if c == _colCount-1 && exist {
			return &winInfo{Symbol: sym, SymbolCount: _colCount, LineCount: lineCnt, WinGrid: wg}, true
		}
	}
	return nil, false
}

func (s *spin) findWildWin(sym int64) (*winInfo, bool) {
	lineCnt := int64(1)
	var wg int64Grid
	for c := int64(0); c < _colCount; c++ {
		cnt := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			curr := s.symbolGrid[r][c]
			if curr == sym || curr == _wild {
				cnt++
				wg[r][c] = curr
			}
		}
		if cnt == 0 {
			if c >= _minMatchCount {
				return &winInfo{Symbol: sym, SymbolCount: c, LineCount: lineCnt, WinGrid: wg}, true
			}
			break
		}
		lineCnt *= cnt
		if c == _colCount-1 {
			return &winInfo{Symbol: sym, SymbolCount: _colCount, LineCount: lineCnt, WinGrid: wg}, true
		}
	}
	return nil, false
}

func (s *spin) processStep(isFree bool) {
	s.updateStepResults()
	if len(s.winResults) == 0 || s.stepMultiplier == 0 || s.winGrid == nil {
		s.finishRound(isFree)
		return
	}

	mode, collect := s.determineCascadeMode(isFree)
	if mode == cascadeModeNone {
		s.finishRound(isFree)
		return
	}
	if collect {
		s.collectFemaleSymbols()
	}

	elimGrid := s.buildEliminationGrid(mode)
	if elimGrid == nil {
		s.finishRound(isFree)
		return
	}

	nextGrid := s.executeCascade(elimGrid, mode)
	if nextGrid == nil {
		s.finishRound(isFree)
		return
	}
	s.nextSymbolGrid = nextGrid
	s.femaleCountsForFree = s.nextFemaleCountsForFree
	s.isRoundOver = false
}

func (s *spin) updateStepResults() {
	var winResults []*winResult
	var winGrid int64Grid
	var lineMultiplier int64

	for _, info := range s.winInfos {
		base := _cnf.getSymbolMultiplier(info.Symbol, info.SymbolCount)
		if base == 0 {
			continue
		}
		total := base * info.LineCount
		winResults = append(winResults, &winResult{
			Symbol:             info.Symbol,
			SymbolCount:        info.SymbolCount,
			LineCount:          info.LineCount,
			BaseLineMultiplier: base,
			TotalMultiplier:    total,
			WinGrid:            info.WinGrid,
		})
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if info.WinGrid[r][c] != _blank {
					winGrid[r][c] = info.WinGrid[r][c]
				}
			}
		}
		lineMultiplier += total
	}
	s.stepMultiplier, s.winResults = lineMultiplier, winResults
	if lineMultiplier > 0 {
		s.winGrid = &winGrid
	} else {
		s.winGrid = nil
	}
}

func (s *spin) finishRound(isFree bool) {
	s.isRoundOver = true
	s.nextSymbolGrid = nil
	s.femaleCountsForFree = s.nextFemaleCountsForFree
}

func (s *spin) determineCascadeMode(isFree bool) (cascadeMode, bool) {
	if !isFree {
		if s.hasFemaleWin && hasWildSymbol(s.symbolGrid) {
			return cascadeModeBase, false
		}
		return cascadeModeNone, false
	}

	if s.enableFullElimination && s.hasFemaleWildWin {
		return cascadeModeFreeFull, true
	}

	if s.hasFemaleWin {
		return cascadeModeFreePartial, true
	}

	return cascadeModeNone, false
}

func (s *spin) collectFemaleSymbols() {
	if s.winGrid == nil {
		return
	}
	for _, row := range s.winGrid {
		for _, symbol := range row {
			if symbol < _femaleA || symbol > _femaleC {
				continue
			}
			if idx := symbol - _femaleA; idx >= 0 && s.nextFemaleCountsForFree[idx] < _femaleSymbolCountForFullElimination {
				s.nextFemaleCountsForFree[idx]++
			}
		}
	}
}

func (s *spin) buildEliminationGrid(mode cascadeMode) *int64Grid {
	switch mode {
	case cascadeModeBase:
		return s.winGrid
	case cascadeModeFreePartial:
		return s.buildFreePartialElimGrid()
	case cascadeModeFreeFull:
		return s.buildFullScreenElimGrid()
	default:
		return nil
	}
}

func (s *spin) buildFreePartialElimGrid() *int64Grid {
	var mask int64Grid
	has := false
	for _, res := range s.winResults {
		if res == nil {
			continue
		}
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				sym := res.WinGrid[r][c]
				if sym == _blocked {
					continue
				}
				if sym == _blank {
					continue
				}
				if sym >= _femaleA && sym <= _wildFemaleC {
					mask[r][c] = sym
					has = true
				}
			}
		}
	}
	if !has {
		return nil
	}
	return &mask
}

func (s *spin) buildFullScreenElimGrid() *int64Grid {
	if !s.enableFullElimination || !s.hasFemaleWildWin {
		return nil
	}
	var mask int64Grid
	has := false
	for _, res := range s.winResults {
		if res == nil || !infoHasFemaleWild(res.WinGrid) {
			continue
		}
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				sym := res.WinGrid[r][c]
				if sym == _blocked {
					continue
				}
				if sym == _blank || sym == _wild {
					continue
				}
				mask[r][c] = sym
				has = true
			}
		}
	}
	if !has {
		return nil
	}
	return &mask
}

func (s *spin) finalizeRound(isFree bool) {
	if !s.isRoundOver {
		s.treasureCount = 0
		s.newFreeRoundCount = 0
		return
	}

	s.treasureCount = getTreasureCount(s.symbolGrid)
	if isFree {
		if s.treasureGainedThisRound < 0 {
			s.treasureGainedThisRound = 0
		}
		s.newFreeRoundCount = s.treasureGainedThisRound
		s.treasureGainedThisRound = 0
		s.lastTreasureCount = 0
	} else {
		s.newFreeRoundCount = _cnf.getFreeRoundCount(s.treasureCount)
	}
}

func (s *spin) executeCascade(elimGrid *int64Grid, mode cascadeMode) *int64Grid {
	grid := *s.symbolGrid
	clearBlockedCells(&grid)
	hasTreasure := getTreasureCount(&grid) > 0

	eliminated := 0
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if isBlockedCell(r, c) || elimGrid[r][c] == _blank {
				continue
			}
			symbol := grid[r][c]
			if symbol == _treasure || !shouldEliminateSymbol(symbol, mode, hasTreasure) {
				continue
			}
			grid[r][c] = _eliminated
			eliminated++
		}
	}
	if eliminated == 0 {
		global.GVA_LOG.Error("no symbols eliminated, forcing round end")
		return nil
	}

	s.dropSymbols(&grid)
	s.fillBlanks(&grid)
	clearBlockedCells(&grid)
	if mode == cascadeModeFreePartial || mode == cascadeModeFreeFull {
		convertFemaleToWild(&grid, s.femaleCountsForFree)
	}
	return &grid
}

func shouldEliminateSymbol(sym int64, mode cascadeMode, hasTreasure bool) bool {
	switch mode {
	case cascadeModeBase:
		if sym >= _femaleA && sym <= _femaleC {
			return true
		}
		return sym == _wild && !hasTreasure
	case cascadeModeFreePartial:
		return sym >= _femaleA && sym <= _wildFemaleC
	case cascadeModeFreeFull:
		return sym != _wild
	default:
		return false
	}
}

func (s *spin) dropSymbols(grid *int64Grid) {
	for c := int64(0); c < _colCount; c++ {
		writePos := int64(0)
		if c == 0 || c == _colCount-1 {
			writePos = 1
		}

		for r := int64(0); r < _rowCount; r++ {
			if isBlockedCell(r, c) {
				continue
			}
			switch val := (*grid)[r][c]; val {
			case _eliminated:
				(*grid)[r][c] = _blank
			case _blank:
				continue
			default:
				if r != writePos {
					(*grid)[writePos][c] = val
					(*grid)[r][c] = _blank
				}
				writePos++
			}
		}
	}
}

func (s *spin) fillBlanks(grid *int64Grid) {
	for c := int64(0); c < _colCount; c++ {
		for r := int64(0); r < _rowCount; r++ {
			if !isBlockedCell(r, c) && (*grid)[r][c] == _blank {
				(*grid)[r][c] = s.rollers[c].getFallSymbol()
			}
		}
	}
}

func convertFemaleToWild(grid *int64Grid, counts [3]int64) {
	if grid == nil {
		return
	}
	for idx := int64(0); idx < 3; idx++ {
		if counts[idx] < _femaleSymbolCountForFullElimination {
			continue
		}
		normal, wild := _femaleA+idx, _wildFemaleA+idx
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if !isBlockedCell(r, c) && (*grid)[r][c] == normal {
					(*grid)[r][c] = wild
				}
			}
		}
	}
}

func clearBlockedCells(grid *int64Grid) {
	if grid != nil {
		(*grid)[0][0], (*grid)[0][_colCount-1] = _blocked, _blocked
	}
}

func isBlockedCell(r, c int64) bool { return r == 0 && (c == 0 || c == _colCount-1) }

func isMatchingFemaleWild(target, curr int64) bool {
	if curr < _wildFemaleA || curr > _wildFemaleC {
		return false
	}
	return target >= (_blank+1) && target <= _femaleC
}

func infoHasFemaleWild(grid int64Grid) bool {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if grid[r][c] >= _wildFemaleA && grid[r][c] <= _wildFemaleC {
				return true
			}
		}
	}
	return false
}


*/

/*



────────────────────────────────────────────────────────
【转轮坐标信息】
滚轴配置Key: free-000
转轮信息长度/起始：37[23]， 59[21]， 59[32]， 59[14]， 37[21]
女性收集状态: [1 4 10]
初始盘面夺宝数量: 0
本轮免费总次数: 8
Step1 初始盘面:
  9 |   9 |   7 |   9 |   6 |
  6 |   9 |   7 |   9 |   6 |
  6 |   1 |   9 |   8 |   7 |
 99 |   2 |   8 |   7 |  99 |
Step1 中奖标记:
  9*|   9*|   7 |   9*|   6 |
  6 |   9*|   7 |   9*|   6 |
  6 |   1 |   9*|   8 |   7 |
 99 |   2 |   8 |   7 |  99 |
Step1 下一盘面预览（实际消除+下落+填充结果）:
  9 |   9 |  13 |   4 |   6 |
  6 |   9 |   7 |  13 |   6 |
  6 |   1 |   7 |   8 |   7 |
 99 |   2 |   8 |   7 |  99 |
Step1 中奖详情:
	触发: 女性中奖=true, 女性百搭参与=false, 有百搭=false, 全屏=false, 有夺宝=false
	女性收集: 起始=[1 4 6] → 结束=[1 4 10] (本步=[0 0 4], 回合累计=[0 0 4])
	🔁 连消继续 → Step2 (女性中奖触发部分消除)

	符号: 9(9), 连线: 4, 乘积: 4, 赔率: 12.00, 下注: 1×1, 奖金: 48
	累计中奖: 48.00

Step2 初始盘面:
 12 |  12 |  13 |   4 |   6 |
  6 |  12 |   7 |  13 |   6 |
  6 |   1 |   7 |   8 |   7 |
 99 |   2 |   8 |   7 |  99 |
Step2 中奖标记:
 12*|  12*|  13*|   4*|   6*|
  6*|  12*|   7*|  13*|   6*|
  6*|   1*|   7*|   8*|   7*|
 99 |   2*|   8*|   7*|  99 |
Step2 下一盘面预览（实际消除+下落+填充结果）:
 12 |   4 |   5 |   2 |   6 |
  6 |  13 |   5 |   3 |   6 |
  6 |   1 |   5 |   4 |   6 |
 99 |   2 |  13 |  13 |  99 |
Step2 中奖详情:
	触发: 女性中奖=true, 女性百搭参与=true, 有百搭=true, 全屏=false, 有夺宝=false
	女性收集: 起始=[1 4 6] → 结束=[5 6 10] (本步=[4 2 0], 回合累计=[4 2 4])
	🔁 连消继续 → Step3 (女性中奖触发部分消除)

	符号: 1(1), 连线: 4, 乘积: 3, 赔率: 3.00, 下注: 1×1, 奖金: 9
	符号: 2(2), 连线: 4, 乘积: 3, 赔率: 3.00, 下注: 1×1, 奖金: 9
	符号: 4(4), 连线: 4, 乘积: 4, 赔率: 3.00, 下注: 1×1, 奖金: 12
	符号: 6(6), 连线: 5, 乘积: 12, 赔率: 15.00, 下注: 1×1, 奖金: 180
	符号: 7(7), 连线: 5, 乘积: 12, 赔率: 20.00, 下注: 1×1, 奖金: 240
	符号: 8(8), 连线: 4, 乘积: 8, 赔率: 12.00, 下注: 1×1, 奖金: 96
	符号: 12(12), 连线: 4, 乘积: 2, 赔率: 15.00, 下注: 1×1, 奖金: 30
	累计中奖: 624.00

Step3 初始盘面:
 12 |   4 |   5 |   2 |   6 |
  6 |  13 |   5 |   3 |   6 |
  6 |   1 |   5 |   4 |   6 |
 99 |   2 |  13 |  13 |  99 |
Step3 中奖标记:
 12*|   4*|   5*|   2*|   6*|
  6*|  13*|   5*|   3*|   6*|
  6*|   1*|   5*|   4*|   6*|
 99 |   2*|  13*|  13*|  99 |
Step3 下一盘面预览（实际消除+下落+填充结果）:
  3 |   4 |   5 |   2 |   6 |
  6 |  13 |   5 |   3 |   6 |
  6 |   1 |   5 |   4 |   6 |
 99 |   2 |  13 |  13 |  99 |
Step3 中奖详情:
	触发: 女性中奖=true, 女性百搭参与=true, 有百搭=true, 全屏=false, 有夺宝=false
	女性收集: 起始=[1 4 6] → 结束=[5 6 10] (本步=[0 0 0], 回合累计=[4 2 4])
	🔁 连消继续 → Step4 (女性中奖触发部分消除)

	符号: 1(1), 连线: 4, 乘积: 2, 赔率: 3.00, 下注: 1×1, 奖金: 6
	符号: 2(2), 连线: 4, 乘积: 4, 赔率: 3.00, 下注: 1×1, 奖金: 12
	符号: 3(3), 连线: 4, 乘积: 2, 赔率: 3.00, 下注: 1×1, 奖金: 6
	符号: 4(4), 连线: 4, 乘积: 4, 赔率: 3.00, 下注: 1×1, 奖金: 12
	符号: 5(5), 连线: 4, 乘积: 4, 赔率: 6.00, 下注: 1×1, 奖金: 24
	符号: 6(6), 连线: 5, 乘积: 9, 赔率: 15.00, 下注: 1×1, 奖金: 135
	符号: 12(12), 连线: 4, 乘积: 1, 赔率: 15.00, 下注: 1×1, 奖金: 15
	累计中奖: 834.00

*/
