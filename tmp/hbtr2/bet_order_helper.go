package hbtr2

import (
	"fmt"
	"strconv"
	"strings"

	"egame-grpc/gamelogic"
	"egame-grpc/global"

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

func (s *betOrderService) buildWinInfoDetail() map[string]any {
	// hbtr 对齐：roundNum 从 0 开始，sumCv 取累计倍数
	roundNum := int64(0)
	if s.scene.Steps > 0 {
		roundNum = int64(s.scene.Steps - 1)
	}

	// hbtr 的 sumCv 是累计连线倍数总和，使用 RoundMultiplier 近似
	sumCv := int64(0)
	if len(s.winInfos) > 0 {
		sumCv = s.scene.RoundMultiplier // 当前请求内的累计倍数
	}

	detail := map[string]any{
		"addNum":   s.addFreeTime,    // 新增免费次数
		"ctSumCv":  s.lineMultiplier, // 当前连线倍数
		"next":     !s.isRoundOver,   // 是否还有下步
		"over":     s.isRoundOver && s.scene.FreeNum == 0,
		"roundNum": roundNum,            // 当前回合序号（从0开始）
		"sumCv":    sumCv,               // 累计倍数（含连消倍数）
		"type":     btoi(s.isFreeRound), // 0 普通，1 免费
		"winArr":   s.buildWinArr(),     // 连线信息（与 hbtr 对齐）
	}

	if s.isFreeRound {
		detail["freeNum"] = int(s.client.ClientOfFreeGame.GetFreeTimes()) // 第几次免费
		detail["remainNum"] = s.scene.FreeNum                             // 剩余免费次数
	}

	return detail
}

// buildWinArr 构建中奖数组
func (s *betOrderService) buildWinArr() []map[string]any {
	if len(s.winInfos) == 0 {
		return []map[string]any{}
	}

	winArr := make([]map[string]any, 0, len(s.winInfos))
	for _, w := range s.winInfos {
		var loc int64Grid
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if w.WinGrid[r][c] == 0 {
					continue
				}
				if isWild(w.WinGrid[r][c]) {
					loc[r][c] = 2 // 百搭
				} else {
					loc[r][c] = 1 // 命中
				}
			}
		}
		winArr = append(winArr, map[string]any{
			"val":     w.Symbol,
			"roadNum": w.LineCount,
			"starNum": w.SymbolCount,
			"odds":    w.Odds,
			"loc":     reverseGridRows(&loc), // 前端坐标与服务端相反，保持与 hbtr 相同的上下翻转
		})
	}
	return winArr
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
	for c := int64(1); c < _colCount-1; c++ { // 填充第0行中间4列
		symbolGrid[0][c] = s.scene.SymbolRoller[_colCount].BoardSymbol[_rowCount-1-c]
	}
	for r := int64(1); r < _rowCount; r++ { // 填充第1-4行
		for c := int64(0); c < _colCount; c++ {
			symbolGrid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r-1]
		}
	}
	s.symbolGrid = symbolGrid
}

// findWinInfos 查找中奖信息
func (s *betOrderService) findWinInfos() {
	winInfos := make([]WinInfo, 0, _wild-_blank-1)
	var totalWinGrid int64Grid

	// 第一列有wild时，检查盘面所有符号；否则使用默认符号列表
	checkSymbols := _checkList
	hasWild, symbols := s.checkWildInFirstCol()
	if hasWild {
		checkSymbols = symbols
	}

	for _, symbol := range checkSymbols {
		info, ok := s.findSymbolWinInfo(symbol, hasWild)
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

// checkWildInFirstCol 检查第一列是否有wild，若有则返回盘面所有不重复符号
func (s *betOrderService) checkWildInFirstCol() (bool, []int64) {
	// 检查第一列是否有wild
	for row := int64(0); row < _rowCount; row++ {
		if isWild(s.symbolGrid[row][0]) {
			// 收集盘面不重复符号
			seen := make(map[int64]bool, 10)
			symbols := make([]int64, 0, 10)
			for r := int64(0); r < _rowCount; r++ {
				for c := int64(0); c < _colCount; c++ {
					sym := s.symbolGrid[r][c]
					if sym > 0 && sym <= _gandalf && !seen[sym] {
						seen[sym] = true
						symbols = append(symbols, sym)
					}
				}
			}
			return true, symbols
		}
	}
	return false, nil
}

// findSymbolWinInfo 查找符号中奖
func (s *betOrderService) findSymbolWinInfo(symbol int64, hasWild bool) (*WinInfo, bool) {
	hasRealSymbol := false
	lineCount := int64(1)
	var winGrid int64Grid

	// 特殊处理 前三列都有wild的特殊情况
	if hasWild {
		hasRealSymbol = true
	}

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

// eliminateWinSymbols 消除中奖符号
func (s *betOrderService) eliminateWinSymbols(nextGrid *int64Grid) {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.winGrid[r][c] > 0 && !isWild(nextGrid[r][c]) {
				nextGrid[r][c] = 0
			}
		}
	}
}

// moveWildSymbols 移动wild符号
func (s *betOrderService) moveWildSymbols(nextGrid *int64Grid) {
	for c := int64(0); c <= int64(_colCount-1); c++ {
		for r := int64(_rowCount - 1); r >= 0; r-- {
			if !isWild(nextGrid[r][c]) {
				continue
			}
			s.moveSingleWild(nextGrid, r, c)
		}
	}
}

// moveSingleWild 移动单个wild
func (s *betOrderService) moveSingleWild(nextGrid *int64Grid, startRow, startCol int64) {
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
		return
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
}

// moveSymbols 处理符号移动和下落（wild位置固定）
func (s *betOrderService) moveSymbols(nextGrid *int64Grid) *int64Grid {
	// 处理第0行：水平左移动（对应roller下标[6]）
	for c := int64(1); c < _colCount-1; c++ {
		if nextGrid[0][c] != 0 {
			continue
		}
		// 从右找第一个 非0 非 wild
		for k := c + 1; k < _colCount-1; k++ {
			if val := nextGrid[0][k]; val != 0 && !isWild(val) {
				nextGrid[0][c] = val
				nextGrid[0][k] = 0
				break
			}
		}
	}

	// 处理第1-4行：垂直下落（对应roller下标[0-5]）
	for col := int64(0); col < _colCount; col++ {
		writePos := int64(_rowCount - 1)

		for readPos := int64(_rowCount - 1); readPos >= 1; readPos-- {
			if isBlockedCell(readPos, col) {
				continue
			}

			val := nextGrid[readPos][col]
			if val != 0 && !isWild(val) {
				// 找到可写位置（跳过wild和墙格）
				for writePos >= 1 && (isWild(nextGrid[writePos][col]) || isBlockedCell(writePos, col)) {
					writePos--
				}
				if writePos >= 1 {
					if writePos != readPos {
						nextGrid[writePos][col] = val
						nextGrid[readPos][col] = 0
					}
					writePos--
				} else {
					// 没地方放，清空
					nextGrid[readPos][col] = 0
				}
			}
		}
	}
	return nextGrid
}

func (s *betOrderService) fallingWinSymbols(nextGrid *int64Grid) *int64Grid {
	metrix := *nextGrid

	// 填充第0行空位
	for c := int64(1); c < _colCount-1; c++ {
		if metrix[0][c] == 0 {
			metrix[0][c] = s.scene.SymbolRoller[_colCount].getNextSymbol(s.gameConfig)
		}
		s.scene.SymbolRoller[_colCount].BoardSymbol[_boardSize-c] = metrix[0][c]
	}
	// 填充第1-4行空位
	for c := int64(0); c < _colCount; c++ {
		for r := int64(_rowCount - 1); r >= 1; r-- {
			if metrix[r][c] == 0 {
				metrix[r][c] = s.scene.SymbolRoller[c].getNextSymbol(s.gameConfig)
			}
			s.scene.SymbolRoller[c].BoardSymbol[r-1] = metrix[r][c]
		}
	}
	// *nextGrid = metrix // 暂时不回写到nextSymbolGrid 保留调试用
	return nextGrid
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

			symbol := rGrid[r][c]
			isWin := rWinGrid != nil && rWinGrid[r][c] != 0
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
