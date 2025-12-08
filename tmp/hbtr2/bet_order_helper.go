package hbtr2

import (
	"fmt"
	"strconv"
	"strings"

	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

func (s *betOrderService) getRequestContext() bool {
	switch {
	case !s.mdbGetMerchant():
		return false
	case !s.mdbGetMember():
		return false
	case !s.mdbGetGame():
		return false
	default:
		return true
	}
}

func (s *betOrderService) selectGameRedis() {
	index := _gameID % int64(len(global.GVA_GAME_REDIS))
	s.gameRedis = global.GVA_GAME_REDIS[index]
}

func (s *betOrderService) updateBetAmount() bool {
	s.betAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_baseMultiplier))

	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
		return false
	}
	return true
}

func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

// symbolGridToString 将网格转换为字符串格式
func symbolGridToString(symbolGrid int64Grid) string {
	var b strings.Builder
	b.Grow(512)
	cellIndex := 0
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			b.WriteString(strconv.Itoa(cellIndex + 1))
			b.WriteString(":")
			b.WriteString(strconv.FormatInt(symbolGrid[r][c], 10))
			b.WriteString("; ")
			cellIndex++
		}
	}
	return b.String()
}

func (s *betOrderService) updateBonusAmount(stepMultiplier int64) {
	// RTP测试模式或无倍数时直接返回
	if s.debug.open || stepMultiplier == 0 {
		s.bonusAmount = decimal.Zero
		return
	}

	s.bonusAmount = s.betAmount.
		Mul(decimal.NewFromInt(stepMultiplier)).
		Div(decimal.NewFromInt(_baseMultiplier))

	if s.bonusAmount.GreaterThan(decimal.Zero) {
		rounded := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(rounded)
		s.client.ClientOfFreeGame.IncRoundBonus(rounded)
		if s.isFreeRound {
			s.client.ClientOfFreeGame.IncrFreeTotalMoney(rounded)
		}
	}
}

func (s *betOrderService) getWinDetail() string {
	var returnRouteDetail []CardType
	if s.stepMultiplier > 0 {
		returnRouteDetail = append(returnRouteDetail, s.getCardTypes()...)
	} else if s.addFreeTime > 0 {
		returnRouteDetail = append(returnRouteDetail, CardType{
			Type:     int(_scatter),
			Route:    int(s.scatterCount),
			Multiple: 0,
			Way:      int(s.addFreeTime),
		})
	}
	if len(returnRouteDetail) == 0 {
		return ""
	}
	winDetailsBytes, _ := json.CJSON.Marshal(returnRouteDetail)
	return string(winDetailsBytes)
}

// buildWinInfoDetail 对齐 hbtr 的 winInfo 结构，返回 map 而非字符串
func (s *betOrderService) buildWinInfoDetail() map[string]any {
	winArr := s.getCardTypes()
	// hbtr 的 winInfo.Detail 是 map[string]any，字段命名保持一致
	detail := map[string]any{
		"winArr":    winArr,              // 连线信息
		"ctSumCv":   s.lineMultiplier,    // 当前连线倍数
		"sumCv":     s.stepMultiplier,    // 累积倍数（含连消倍数）
		"addNum":    s.addFreeTime,       // 新增免费次数
		"type":      btoi(s.isFreeRound), // 0 普通，1 免费
		"freeNum":   int(s.client.ClientOfFreeGame.GetFreeTimes()),
		"remainNum": s.scene.FreeNum, // 剩余免费次数
		"roundNum":  s.scene.Steps,   // 当前回合序号
		"next":      !s.isRoundOver,  // 是否还有下步
		"over":      s.isRoundOver && s.scene.FreeNum == 0,
	}
	return detail
}

// btoi bool转int
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *betOrderService) getScatterCount() int64 {
	var count int64
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			//if s.symbolGrid[r][c] == _scatter {
			if isScatter(s.symbolGrid[r][c]) {
				count++
			}
		}
	}
	return count
}

func (s *betOrderService) handleSymbolGrid() {
	var symbolGrid int64Grid
	for c := int64(1); c < _colCount-1; c++ { // 填充第0行（中间4列）注意：[0][0] [0][5]是墙格符号为0
		symbolGrid[0][c] = s.scene.SymbolRoller[_colCount].BoardSymbol[_rowCount-1-c]
	}
	for r := int64(1); r < _rowCount; r++ { // 填充第1-4行（所有列）
		for c := int64(0); c < _colCount; c++ {
			symbolGrid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r-1]
		}
	}
	s.symbolGrid = symbolGrid
}

// findWinInfos 查找中奖信息（Ways玩法：从左到右连续匹配）
func (s *betOrderService) findWinInfos() {
	winInfos := make([]WinInfo, 0, _wild-_blank-1)
	var totalWinGrid int64Grid

	for symbol := _blank + 1; symbol < _wild; symbol++ {
		info, ok := s.findSymbolWinInfo(symbol)
		if !ok {
			continue
		}

		winInfos = append(winInfos, *info)

		// 合并中奖位置到总网格（用于消除）
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < info.SymbolCount; c++ {
				if info.WinGrid[r][c] != 0 {
					totalWinGrid[r][c] = info.WinGrid[r][c]
				}
			}
		}
	}

	s.winInfos = winInfos
	s.winGrid = totalWinGrid
}

// findSymbolWinInfo 查找符号中奖（Ways玩法：从左到右连续，至少3列，Wild可替代）
func (s *betOrderService) findSymbolWinInfo(symbol int64) (*WinInfo, bool) {
	hasRealSymbol := false
	lineCount := int64(1)
	var winGrid int64Grid

	// 逐列扫描，统计匹配的符号
	for c := int64(0); c < _colCount; c++ {
		matchCount := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || isWild(currSymbol) {
				if currSymbol == symbol {
					hasRealSymbol = true
				}
				matchCount++
				winGrid[r][c] = currSymbol // 存储实际符号值
			}
		}

		// 当前列没有匹配
		if matchCount == 0 {
			if c >= _minMatchCount && hasRealSymbol {
				if odds := s.getSymbolBaseMultiplier(symbol, int(c)); odds > 0 {
					return &WinInfo{Symbol: symbol, SymbolCount: c, LineCount: lineCount, Odds: odds, Multiplier: odds * lineCount, WinGrid: winGrid}, true
				}
			}
			return nil, false
		}

		// 计算路数：每列匹配数相乘
		lineCount *= matchCount

		// 如果到了最后一列且有真实符号，返回中奖信息
		if c == _colCount-1 && hasRealSymbol {
			odds := s.getSymbolBaseMultiplier(symbol, int(_colCount))
			if odds > 0 {
				return &WinInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, Odds: odds, Multiplier: odds * lineCount, WinGrid: winGrid}, true
			}
		}
	}

	return nil, false
}

// eliminateWinSymbols 消除中奖位置的符号；但 wild 保留（即便参与了中奖）
func (s *betOrderService) eliminateWinSymbols() *int64Grid {
	nextGrid := s.symbolGrid
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.winGrid[r][c] > 0 && !isWild(nextGrid[r][c]) {
				nextGrid[r][c] = 0
			}
		}
	}
	return &nextGrid
}

// moveWildSymbols wild符号向左下移动，遇到scatter跳过，目标位置有符号则转成wild
// 返回：所有wild的移动记录（Bat数组）
func (s *betOrderService) moveWildSymbols(nextGrid *int64Grid) []Bat {
	wildPositions := make([]position, 0, _totalCells/2)

	// 先收集所有wild的位置 从下到上
	for c := int64(_colCount - 1); c >= 0; c-- {
		for r := int64(_rowCount - 1); r >= 0; r-- {
			if isWild(nextGrid[r][c]) {
				wildPositions = append(wildPositions, position{Row: r, Col: c})
			}
		}
	}

	// 移动每个wild
	bats := make([]Bat, 0, len(wildPositions))
	for _, pos := range wildPositions {
		if bat := s.moveSingleWild(nextGrid, pos.Row, pos.Col); bat != nil {
			bats = append(bats, *bat)
		}
	}

	return bats
}

// moveSingleWild 移动单个wild符号向左下方向，只尝试一格
// 返回移动记录，如果无法移动则返回nil
func (s *betOrderService) moveSingleWild(nextGrid *int64Grid, startRow, startCol int64) *Bat {
	// 起点清理：14/15 还原为 12/13，其余清0
	switch nextGrid[startRow][startCol] {
	case _scaWild:
		nextGrid[startRow][startCol] = _scatter
	case _freeWild:
		nextGrid[startRow][startCol] = _freePlus
	default:
		nextGrid[startRow][startCol] = 0
	}

	// 只尝试左下一格
	row, col := startRow+1, startCol-1
	if row >= _rowCount || col < 0 || isBlockedCell(row, col) {
		return nil
	}

	// 目标为 12/13 时生成 14/15，否则生成 11
	var target int64
	original := nextGrid[row][col]
	switch original {
	case _scatter:
		target = _scaWild
	case _freePlus:
		target = _freeWild
	default:
		target = _wild
	}
	nextGrid[row][col] = target

	// 调试标记：基础模式0-99，免费模式100-199，低位表示状态
	if s.debug.open {
		s.debug.mark = 0
		if s.hasWild() {
			s.debug.mark |= 1 // bit0: 有wild在盘面上
		}
		if len(s.bats) > 0 {
			s.debug.mark |= 2 // bit1: 有wild移动
		}
		if target == _scaWild || target == _freeWild {
			s.debug.mark |= 4 // bit2: 发生wild->scatter转换
		}
		if s.getScatterCount() > 0 {
			s.debug.mark |= 8 // bit3: 有scatter在盘面上
		}
		// 免费模式标记偏移100 - 使用scene.Stage而不是isFreeRound，确保状态同步
		if s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli {
			s.debug.mark += 100
		}
	}

	return &Bat{X: startRow, Y: startCol, TransX: row, TransY: col, Syb: original, Sybn: target}
}

// moveSymbols 处理符号下落和左移动
// 第0行：从左到右移动符号（跳过wild位置），对应roller下标[6]
// 第1-4行：按列处理，从下往上下落符号（跳过wild位置，墙格位置不参与下落），对应roller下标[0-5]
func (s *betOrderService) moveSymbols(grid *int64Grid) *int64Grid {
	/*
		处理第0行：水平左移动（对应roller下标[6]）
		逻辑：从左到右扫描，如果当前位置是空位，从右侧找到第一个非空非wild符号向左移动
		注意：[0][0] [0][5]是墙格符号为0，只处理中间4列（列1-4）
		示例：[0, 4, 0, 8] -> [4, 8, 0, 0]
	*/
	for c := int64(1); c < _colCount-1; c++ {
		if grid[0][c] != 0 {
			continue
		}
		// 从右找第一个 非0 非 wild
		for k := c + 1; k < _colCount-1; k++ {
			if val := grid[0][k]; val != 0 && !isWild(val) {
				grid[0][c] = val
				grid[0][k] = 0
				break
			}
		}
	}

	/*
			处理第1-4行：垂直下落（对应roller下标[0-5]）
			逻辑：从下往上扫描每列，将非wild非0符号向下压缩到底部，wild位置保持不变，允许符号穿过wild下落
			优化：单遍扫描，同时处理收集和重新放置
			示例：初始 [5, 0, 7, 0, 9] → 结果 [0, 0, 5, 7, 9]
		    示例：初始 [5, 0, 12, 0, 9] → 结果 [0, 0, 12, 5, 9]
	*/
	/*
		for col := int64(0); col < _colCount; col++ {
				// 第一步：收集所有非wild非空符号，并清空这些位置
				var symbols []int64
				for row := int64(_rowCount - 1); row >= 1; row-- {
					if isBlockedCell(row, col) {
						continue
					}
					val := grid[row][col]
					if val != 0 && !isWild(val) {
						symbols = append(symbols, val)
						grid[row][col] = 0
					}
				}

				// 第二步：从底部开始重新放置符号，跳过wild位置
				writePos := int64(_rowCount - 1)
				for _, symbol := range symbols {
					// 找到下一个可写位置（不是wild的空位）
					for writePos >= 1 && (isWild(grid[writePos][col]) || isBlockedCell(writePos, col)) {
						writePos--
					}
					if writePos >= 1 {
						grid[writePos][col] = symbol
						writePos--
					}
					// 如果没地方放，符号被丢弃
				}
			}
	*/
	for col := int64(0); col < _colCount; col++ {
		writePos := int64(_rowCount - 1)

		for readPos := int64(_rowCount - 1); readPos >= 1; readPos-- {
			if isBlockedCell(readPos, col) {
				continue
			}

			val := grid[readPos][col]
			if val != 0 && !isWild(val) {
				// 找到可写位置（跳过wild和墙格）
				for writePos >= 1 && (isWild(grid[writePos][col]) || isBlockedCell(writePos, col)) {
					writePos--
				}
				if writePos >= 1 {
					if writePos != readPos {
						grid[writePos][col] = val
						grid[readPos][col] = 0
					}
					writePos--
				} else {
					// 没地方放，清空
					grid[readPos][col] = 0
				}
			}
		}
	}
	return grid
}

func (s *betOrderService) fallingWinSymbols(nextSymbolGrid *int64Grid) *int64Grid {
	metrix := *nextSymbolGrid

	// 填充第0行空位（中间4列），补充新符号并同步到roller[6]
	for c := int64(1); c < _colCount-1; c++ {
		if metrix[0][c] == 0 {
			metrix[0][c] = s.scene.SymbolRoller[_colCount].getNextSymbol(s.gameConfig)
		}
		s.scene.SymbolRoller[_colCount].BoardSymbol[_boardSize-c] = metrix[0][c]
	}

	// 填充第1~4行空位（所有列），补充新符号并同步到对应roller
	for c := int64(0); c < _colCount; c++ {
		for r := int64(_rowCount - 1); r >= 1; r-- {
			if metrix[r][c] == 0 {
				metrix[r][c] = s.scene.SymbolRoller[c].getNextSymbol(s.gameConfig)
			}
			s.scene.SymbolRoller[c].BoardSymbol[r-1] = metrix[r][c]
		}
	}

	/*
		// 将更新后的metrix写回nextSymbolGrid （暂时不回写）
		*nextSymbolGrid = metrix
	*/
	return nextSymbolGrid
}

func GridToString(grid *int64Grid, winGrid *int64Grid) string {
	if grid == nil {
		return "(空)\n"
	}
	var buf strings.Builder
	buf.Grow(512)
	writeGridToBuilder(&buf, grid, winGrid)
	return buf.String()
}

func writeGridToBuilder(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
	if grid == nil {
		buf.WriteString("(空)\n")
		return
	}
	rGrid := grid
	rWinGrid := winGrid

	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {

			symbol := rGrid[r][c]                           // 使用 grid 的符号值（显示盘面所有符号）
			isWin := rWinGrid != nil && rWinGrid[r][c] != 0 // 检查 winGrid 是否有标记（中奖位置）
			if symbol == 0 {
				if isWin {
					buf.WriteString("   *|")
				} else {
					buf.WriteString("    |")
				}
			} else {
				if isWin {
					_, _ = fmt.Fprintf(buf, " %2d*|", symbol)
				} else {
					_, _ = fmt.Fprintf(buf, " %2d |", symbol)
				}
			}
			if c < _colCount-1 {
				buf.WriteString(" ")
			}
		}
		buf.WriteString("\n")
	}
}

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

func (s *betOrderService) getCardTypes() []CardType {
	if len(s.winInfos) == 0 {
		return nil
	}
	cardTypes := make([]CardType, len(s.winInfos))
	for i, elem := range s.winInfos {
		cardTypes[i] = CardType{
			Type:     int(elem.Symbol),
			Way:      int(elem.LineCount),
			Multiple: int(elem.Odds),
			Route:    int(elem.SymbolCount),
		}
	}
	return cardTypes
}

// isWild 检查符号是否为wild符号（可替代、不可消除、下落时占位）
func isWild(symbol int64) bool {
	return symbol == _wild || symbol == _scaWild || symbol == _freeWild
}

// isScatter 检查符号是否为scatter符号（夺宝/免费触发符号）
func isScatter(symbol int64) bool {
	return symbol == _scatter || symbol == _freePlus || symbol == _scaWild || symbol == _freeWild
}

func isBlockedCell(r, c int64) bool {
	return r == 0 && (c == 0 || c == _colCount-1)
}

// hasWild 检查当前符号盘面是否存在任何 wild（11/14/15）
func (s *betOrderService) hasWild() bool {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if isWild(s.symbolGrid[r][c]) {
				return true
			}
		}
	}
	return false
}
