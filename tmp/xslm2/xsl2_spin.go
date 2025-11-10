package xslm2

import (
	"egame-grpc/global"

	"go.uber.org/zap"
)

type spin struct {
	// 持久化数据（与 scene 同步）
	femaleCountsForFree     [3]int64                // 当前step的收集进度 [A,B,C]
	nextFemaleCountsForFree [3]int64                // 下一step的收集进度（用于累加）
	rollerKey               string                  // 当前滚轴配置key（基础=base / 免费=收集状态）
	rollers                 [_colCount]SymbolRoller // 滚轴状态（Start递减）
	nextSymbolGrid          *int64Grid              // 下一step的网格（消除下落填充后）

	// 内存数据（仅本次请求/step 使用）
	enableFullElimination  bool         // 全屏消除标志（三种女性>=10触发）
	roundStartFemaleCounts [3]int64     // 本回合开始时的女性收集，用于调试输出
	roundStartTreasure     int64        // 本回合开始时已有的夺宝数量（用于免费模式计算新增免费次数）
	symbolGrid             *int64Grid   // 当前符号网格（4x5）
	winGrid                *int64Grid   // 中奖标记网格
	winInfos               []*winInfo   // 中奖信息（原始）
	winResults             []*winResult // 中奖结果（计算后）
	stepMultiplier         int64        // step总倍数
	hasFemaleWin           bool         // 本step女性符号中奖标志（触发连消）
	// 回合状态
	isRoundOver       bool  // 回合结束标志（false=继续连消）
	treasureCount     int64 // 夺宝符号数量
	newFreeRoundCount int64 // 新增免费次数
}

func (s *spin) desc() string {
	return ToJSON(s)
}

// baseSpin 核心逻辑：生成网格 → 查找中奖 → 判断连消 → 消除下落填充
func (s *spin) baseSpin(isFreeRound bool, isFirst bool, nextGrid *int64Grid, rollers *[_colCount]SymbolRoller) {
	s.loadStepData(isFreeRound, isFirst, nextGrid, rollers)
	s.findWinInfos()
	s.processStep(isFreeRound)
	s.handleCascade(isFreeRound)
	s.finalizeRound(isFreeRound)
}

// loadStepData 加载当前step的网格和滚轴数据
// - isFirst: 是否为回合的第一个step（首次spin）
// - nextGrid: 连消时传入的下一step网格（首次spin时为nil）
// - rollers: 连消时传入的滚轴状态（首次spin时为nil）
func (s *spin) loadStepData(isFreeRound bool, isFirst bool, nextGrid *int64Grid, rollers *[_colCount]SymbolRoller) {
	if isFirst {
		// 首次spin：初始化新网格
		s.initSpinSymbol(isFreeRound)
		s.roundStartTreasure = getTreasureCount(s.symbolGrid)
		s.nextFemaleCountsForFree = s.femaleCountsForFree
	} else {
		// 连消step：使用上一step的结果
		if nextGrid == nil || rollers == nil {
			global.GVA_LOG.Error("loadStepData", zap.String("reason", "missing cascade data, fallback"), zap.Bool("isFreeRound", isFreeRound))
			// 数据缺失时回退到初始化
			s.initSpinSymbol(isFreeRound)
			s.roundStartTreasure = getTreasureCount(s.symbolGrid)
			s.nextFemaleCountsForFree = s.femaleCountsForFree
			s.roundStartFemaleCounts = s.femaleCountsForFree
		} else {
			s.symbolGrid = nextGrid
			s.rollers = *rollers
		}
	}

	s.checkFullElimination(isFreeRound)
}

// initSpinSymbol 初始化当前step的网格与滚轴
// 根据是否为免费模式以及女性符号收集状态选择对应的滚轴配置
func (s *spin) initSpinSymbol(isFreeRound bool) {
	symbolGrid, newRollers, key := _cnf.initSpinSymbol(isFreeRound, s.femaleCountsForFree)
	clearBlockedCells(&symbolGrid)
	s.symbolGrid = &symbolGrid
	s.rollers = newRollers
	s.rollerKey = key
}

// checkFullElimination 检查并设置全屏消除标志
// 规则：免费模式下，三种女性符号（A/B/C）都达到10个时，触发全屏消除模式
// 全屏消除模式下，有女性百搭参与的中奖符号都会消失（女性百搭消失，但百搭不消失）
func (s *spin) checkFullElimination(isFreeRound bool) {
	if !isFreeRound {
		return
	}
	s.enableFullElimination = s.femaleCountsForFree[0] >= _femaleSymbolCountForFullElimination &&
		s.femaleCountsForFree[1] >= _femaleSymbolCountForFullElimination &&
		s.femaleCountsForFree[2] >= _femaleSymbolCountForFullElimination
}

// findWinInfos 查找所有中奖组合
// 返回是否有中奖，并设置 hasFemaleWin 标志（用于判断是否触发连消）
func (s *spin) findWinInfos() bool {
	var winInfos []*winInfo
	s.hasFemaleWin = false
	for symbol := _blank + 1; symbol < _wildFemaleA; symbol++ {
		if info, ok := s.findNormalSymbolWinInfo(symbol); ok {
			if symbol >= _femaleA {
				s.hasFemaleWin = true
			}
			winInfos = append(winInfos, info)
		}
	}
	for symbol := _wildFemaleA; symbol < _wild; symbol++ {
		if info, ok := s.findWildSymbolWinInfo(symbol); ok {
			s.hasFemaleWin = true
			winInfos = append(winInfos, info)
		}
	}
	s.winInfos = winInfos
	return len(winInfos) > 0
}

// findNormalSymbolWinInfo 查找普通符号（1-9）的中奖信息
// 匹配规则：符号本身、百搭（13）、或匹配的女性百搭（10-12）都可以匹配
// 返回中奖信息（符号、数量、连线数、中奖网格）和是否中奖
func (s *spin) findNormalSymbolWinInfo(symbol int64) (*winInfo, bool) {
	exist := false
	lineCount := int64(1)
	var winGrid int64Grid
	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || currSymbol == _wild || isMatchingFemaleWild(symbol, currSymbol) {
				if currSymbol == symbol {
					exist = true
				}
				count++
				winGrid[r][c] = currSymbol
			}
		}
		if count == 0 {
			if c >= _minMatchCount && exist {
				info := winInfo{Symbol: symbol, SymbolCount: c, LineCount: lineCount, WinGrid: winGrid}
				return &info, true
			}
			break
		}
		lineCount *= count
		if c == _colCount-1 && exist {
			info := winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, WinGrid: winGrid}
			return &info, true
		}
	}
	return nil, false
}

// findWildSymbolWinInfo 查找女性百搭符号（10-12）的中奖信息
// 匹配规则：女性百搭本身或普通百搭（13）可以匹配
// 注意：女性百搭不能相互替换，只能被普通百搭替换
func (s *spin) findWildSymbolWinInfo(symbol int64) (*winInfo, bool) {
	lineCount := int64(1)
	var winGrid int64Grid
	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || currSymbol == _wild {
				count++
				winGrid[r][c] = currSymbol
			}
		}
		if count == 0 {
			if c >= _minMatchCount {
				info := winInfo{Symbol: symbol, SymbolCount: c, LineCount: lineCount, WinGrid: winGrid}
				return &info, true
			}
			break
		}
		lineCount *= count
		if c == _colCount-1 {
			info := winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, WinGrid: winGrid}
			return &info, true
		}
	}
	return nil, false
}

func (s *spin) processStep(isFreeRound bool) {
	if isFreeRound {
		s.processStepForFree()
	} else {
		s.processStepForBase()
	}
}

// processStepForBase 处理基础模式的step逻辑
// 规则：
//   - 消除条件：女性中奖（7/8/9）且盘面存在百搭（13）
//   - 消除规则：
//     1. 有夺宝：不消除百搭，只消除中奖的女性符号
//     2. 无夺宝：中奖的女性符号和百搭都消除
//   - 普通符号（1-6）保留，不消除
//
// 命中条件时继续连消，否则结束本回合
func (s *spin) processStepForBase() {
	if s.hasFemaleWin && hasWildSymbol(s.symbolGrid) {
		// 计算所有中奖（包括普通符号），但标记为只消除女性符号
		s.updateStepResults(false) // 计算所有中奖
		if len(s.winResults) == 0 {
			s.finishRound(false)
			global.GVA_LOG.Error("processStepForBase", zap.String("reason", "partial elimination but no winResults"))
			return
		}
		s.isRoundOver = false
		return
	}
	s.updateStepResults(false)
	s.finishRound(false)
}

// processStepForFree 处理免费模式的step逻辑
// 规则：
//   - 全屏消除模式（三种女性都>=10）：
//   - 无论是否有女性符号中奖，都计算所有中奖
//   - 有女性百搭参与的中奖符号都会消失（女性百搭消失，但百搭不消失）
//   - 普通免费模式：
//   - 消除条件：有女性符号中奖（7/8/9 或 10/11/12）
//   - 消除规则：只消除中奖的女性符号和女性百搭，百搭不消失，普通符号（1-6）保留
//
// 命中条件时继续连消，否则结束本回合
func (s *spin) processStepForFree() {
	if s.enableFullElimination {
		// 全屏消除模式：计算所有中奖
		s.updateStepResults(false)
		if len(s.winResults) == 0 {
			s.finishRound(true)
			return
		}
		s.isRoundOver = false
		s.collectFemaleSymbols()
		return
	}

	if s.hasFemaleWin {
		// 普通免费模式：只计算女性符号中奖
		s.updateStepResults(true)
		if len(s.winResults) == 0 {
			s.finishRound(true)
			global.GVA_LOG.Error("processStepForFree", zap.String("reason", "partial elimination but no winResults"))
			return
		}
		s.isRoundOver = false
		s.collectFemaleSymbols()
		return
	}

	// 无女性中奖，结束本回合
	s.updateStepResults(false)
	s.finishRound(true)
}

// finishRound 结束当前回合
// 设置 isRoundOver=true，handleCascade 会跳过连消处理
func (s *spin) finishRound(isFreeRound bool) {
	s.isRoundOver = true
	s.nextSymbolGrid = nil
	s.femaleCountsForFree = s.nextFemaleCountsForFree
}

// collectFemaleSymbols 收集中奖的女性符号
// 规则：只统计普通女性符号（7,8,9），不统计女性百搭符号（10,11,12）
// 每种女性符号达到10个后，不再累加（已触发全屏消除模式）
func (s *spin) collectFemaleSymbols() {
	if s.winGrid == nil {
		return
	}
	for _, row := range s.winGrid {
		for _, symbol := range row {
			if symbol == _blocked {
				continue
			}
			// 只统计普通女性符号（7,8,9）
			if symbol >= _femaleA && symbol <= _femaleC {
				idx := symbol - _femaleA
				if s.nextFemaleCountsForFree[idx] < _femaleSymbolCountForFullElimination {
					s.nextFemaleCountsForFree[idx]++
				}
			}
		}
	}
}

// updateStepResults 计算当前step的中奖结果
// partialElimination: false=计算所有中奖，true=只计算女性符号中奖（7-9和10-12）
// 当 partialElimination=true 时，普通符号（1-6）的中奖会被跳过，用于连消中保留这些符号供后续消除
func (s *spin) updateStepResults(partialElimination bool) {
	var winResults []*winResult
	var winGrid int64Grid
	lineMultiplier := int64(0)

	for _, info := range s.winInfos {
		// partialElimination=true 时，跳过普通符号（1-6）的中奖
		if partialElimination && info.Symbol < _femaleA {
			continue
		}
		baseLineMultiplier := _cnf.getSymbolMultiplier(info.Symbol, info.SymbolCount)
		totalMultiplier := baseLineMultiplier * info.LineCount
		result := winResult{
			Symbol:             info.Symbol,
			SymbolCount:        info.SymbolCount,
			LineCount:          info.LineCount,
			BaseLineMultiplier: baseLineMultiplier,
			TotalMultiplier:    totalMultiplier,
			WinGrid:            info.WinGrid,
		}
		winResults = append(winResults, &result)
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if info.WinGrid[r][c] != _blank {
					winGrid[r][c] = info.WinGrid[r][c]
				}
			}
		}
		lineMultiplier += totalMultiplier

	}
	s.stepMultiplier = lineMultiplier
	s.winResults = winResults
	s.winGrid = &winGrid
}

// finalizeRound 计算新增免费次数等结算信息
// 规则：
//   - 基础模式：3/4/5个夺宝分别获得7/10/15次免费游戏
//   - 免费模式：每收集1个夺宝，免费次数+1
//
// 仅在回合结束时计算（isRoundOver=true）
func (s *spin) finalizeRound(isFreeRound bool) {
	if !s.isRoundOver {
		// 连消阶段不计免费次数
		s.treasureCount = 0
		s.newFreeRoundCount = 0
		return
	}

	s.treasureCount = getTreasureCount(s.symbolGrid)
	if isFreeRound {
		// 免费局：比较首尾夺宝数量
		newTreasure := s.treasureCount - s.roundStartTreasure
		if newTreasure < 0 {
			newTreasure = 0
		}
		s.newFreeRoundCount = newTreasure
		return
	}

	// 基础局：按夺宝总数查配置
	s.newFreeRoundCount = _cnf.getFreeRoundCount(s.treasureCount)
}

// handleCascade 根据连消结果准备下一步或结束本回合
// 主要流程：消除中奖符号 → 符号下落 → 填充新符号
// 消除规则根据模式（基础/免费/全屏消除）和条件（是否有夺宝、是否有百搭）决定
func (s *spin) handleCascade(isFreeRound bool) {
	if s.isRoundOver {
		s.nextSymbolGrid = nil
		return
	}

	// 如果没有中奖网格，不应该继续连消
	if s.winGrid == nil {
		global.GVA_LOG.Error("handleCascade: winGrid is nil but isRoundOver is false")
		s.finishRound(isFreeRound)
		return
	}

	newGrid := s.buildCascadeGrid()
	hasTreasure := getTreasureCount(&newGrid) > 0
	rule := s.selectEliminationRule(isFreeRound, hasTreasure)
	eliminatedCount := s.applyEliminationRule(&newGrid, rule, hasTreasure)
	if eliminatedCount == 0 {
		global.GVA_LOG.Error("no symbols eliminated, forcing round end")
		s.finishRound(isFreeRound)
		return
	}

	// 步骤2：符号下落
	for c := int64(0); c < _colCount; c++ {
		writePos := int64(0)
		// 第一列和最后一列的第一行是墙格，从第二行开始写入
		if c == 0 || c == _colCount-1 {
			writePos = 1
		}

		for r := int64(0); r < _rowCount; r++ {
			if isBlockedCell(r, c) {
				continue
			}
			if newGrid[r][c] == _eliminated {
				newGrid[r][c] = _blank
				continue
			}
			if newGrid[r][c] == _blank {
				continue
			}
			// 将符号移动到最下方的可用位置
			if r != writePos {
				newGrid[writePos][c] = newGrid[r][c]
				newGrid[r][c] = _blank
			}
			writePos++
		}
	}

	// 步骤3：填充新符号（从滚轴获取符号填充空白位置）
	for c := int64(0); c < _colCount; c++ {
		for r := int64(0); r < _rowCount; r++ {
			if isBlockedCell(r, c) {
				continue
			}
			if newGrid[r][c] == _blank {
				newGrid[r][c] = s.rollers[c].getFallSymbol()
			}
		}
	}

	clearBlockedCells(&newGrid)
	s.nextSymbolGrid = &newGrid
	s.femaleCountsForFree = s.nextFemaleCountsForFree
}

func (s *spin) buildCascadeGrid() int64Grid {
	grid := *s.symbolGrid
	clearBlockedCells(&grid)
	clearBlockedCells(s.winGrid)
	return grid
}

func (s *spin) applyEliminationRule(grid *int64Grid, rule eliminationRule, hasTreasure bool) int {
	eliminated := 0
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if isBlockedCell(r, c) {
				continue
			}
			if s.winGrid[r][c] == _blank {
				continue
			}
			symbol := (*grid)[r][c]
			if symbol == _treasure {
				continue
			}
			if rule(symbol, s.winGrid[r][c], hasTreasure) {
				(*grid)[r][c] = _eliminated
				eliminated++
			}
		}
	}
	return eliminated
}

type eliminationRule func(symbol, winSymbol int64, hasTreasure bool) bool

func (s *spin) selectEliminationRule(isFreeRound bool, hasTreasure bool) eliminationRule {
	switch {
	case isFreeRound && s.enableFullElimination:
		return func(symbol, winSymbol int64, _ bool) bool {
			// 全屏模式：女性百搭及其覆盖的中奖符号全部消除，普通百搭保留
			if symbol == _wild || winSymbol == _wild {
				return false
			}
			return true
		}
	case isFreeRound && s.hasFemaleWin:
		return func(symbol, winSymbol int64, _ bool) bool {
			// 免费模式普通连消：仅移除女性符号和女性百搭，百搭保留
			if symbol == _wild || winSymbol == _wild {
				return false
			}
			return (symbol >= _femaleA && symbol <= _femaleC) ||
				(symbol >= _wildFemaleA && symbol <= _wildFemaleC)
		}
	case !isFreeRound && s.hasFemaleWin && hasWildSymbol(s.symbolGrid):
		return func(symbol, winSymbol int64, hasTreasure bool) bool {
			// 基础模式连消：移除女性符号；无夺宝时百搭也会消失
			if symbol >= _femaleA && symbol <= _femaleC {
				return true
			}
			if symbol == _wild || winSymbol == _wild {
				return !hasTreasure
			}
			return false
		}
	default:
		return func(symbol, _ int64, _ bool) bool {
			// 默认：所有中奖符号都消除
			return true
		}
	}
}

// clearBlockedCells 将网格中的墙格（第一行左右角）置为 _blocked
// 墙格用于阻挡符号下落，影响填充逻辑
func clearBlockedCells(grid *int64Grid) {
	if grid == nil {
		return
	}
	(*grid)[0][0] = _blocked
	(*grid)[0][_colCount-1] = _blocked
}

// isBlockedCell 判断指定位置是否为墙格
func isBlockedCell(row, col int64) bool {
	return row == 0 && (col == 0 || col == _colCount-1)
}

// isMatchingFemaleWild 判断女性百搭是否匹配目标女性符号
// 规则：女性百搭（10/11/12）只能匹配对应的女性符号（7/8/9），不能相互替换
func isMatchingFemaleWild(target, candidate int64) bool {
	if candidate < _wildFemaleA || candidate > _wildFemaleC {
		return false
	}
	if target < _femaleA || target > _femaleC {
		return false
	}
	return (candidate - _wildFemaleA) == (target - _femaleA)
}
