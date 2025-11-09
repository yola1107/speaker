package xslm2

import (
	"fmt"

	"egame-grpc/global"
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
	// 加载网格和滚轴
	s.loadStepData(isFreeRound, isFirst, nextGrid, rollers)

	// 检查全屏消除（仅免费模式）
	s.checkFullElimination(isFreeRound)

	// 查找中奖
	s.findWinInfos()

	// 基础/免费模式流程
	if !isFreeRound {
		s.processStepForBase()
	} else {
		s.processStepForFree()
	}

	// 回合结束/准备下一步消除
	s.finalizeStep(isFreeRound)

	// 免费次数更新
	s.checkAddFreeCount(isFreeRound)
}

func (s *spin) loadStepData(isFreeRound bool, isFirst bool, nextGrid *int64Grid, rollers *[_colCount]SymbolRoller) {
	if isFirst {
		s.initSpinSymbol(isFreeRound)
		s.roundStartTreasure = getTreasureCount(s.symbolGrid)
		return
	}

	if nextGrid == nil || rollers == nil {
		panic(fmt.Errorf("=====================\n loadStepData: nil data in non-first step, s=%v", s.desc()))
	}

	s.symbolGrid = nextGrid
	s.rollers = *rollers
}

// initSpinSymbol 重新初始化当前step的网格与滚轴
func (s *spin) initSpinSymbol(isFreeRound bool) {
	symbolGrid, newRollers, key := _cnf.initSpinSymbol(isFreeRound, s.femaleCountsForFree)
	clearBlockedCells(&symbolGrid)
	s.symbolGrid = &symbolGrid
	s.rollers = newRollers
	s.rollerKey = key
}

// checkFullElimination 检查并设置全屏消除标志
func (s *spin) checkFullElimination(isFreeRound bool) {
	if !isFreeRound {
		return
	}
	s.enableFullElimination = s.femaleCountsForFree[0] >= _femaleSymbolCountForFullElimination &&
		s.femaleCountsForFree[1] >= _femaleSymbolCountForFullElimination &&
		s.femaleCountsForFree[2] >= _femaleSymbolCountForFullElimination
}

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

/*
	普通模式
		消除条件 盘面有百搭&&有女性符号中奖（7/8/9）
		消除规则 1.有夺宝不消除百搭只消中奖的女性符号，2.无夺宝中奖的女性符号和百搭都消除

	免费模式
		消除条件 有女性符号中奖 （7/8/9  10/11/12）
		消除规则 1.只消中奖的女性符号及中奖的女性百搭（有百搭不消除百搭符号）
		女性符号共3种，中奖后，如果任一种女性符号中奖个数大于或等于10个，则该女性符号转变为对应的女性百搭符号， 有该女性百搭参与到中奖符号都会消出
*
*/
// processStepForBase 基础模式：仅当“女性中奖且盘面存在 Wild”时才继续连消
// - 命中条件：女性中奖 + 盘面含 Wild → 按部分结算（只清女性符号），未命中则直接结算结束本回合
func (s *spin) processStepForBase() {
	if s.hasFemaleWin && hasWildSymbol(s.symbolGrid) {
		s.updateStepResults(true)
		if len(s.winResults) == 0 {
			s.finishRound(false)
			panic("processStepForBase: expected cascade result")
		}
		s.isRoundOver = false
		return
	}
	s.updateStepResults(false)
	s.finishRound(false)
}

// processStepForFree 免费模式：两种触发方式
// - 全屏消除模式：无论女性符号与否，直接走完全结算并统计女性收集
// - 普通免费模式：女性中奖才进入部分结算；否则结束本回合
func (s *spin) processStepForFree() {
	if s.enableFullElimination {
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
		s.updateStepResults(true)
		if len(s.winResults) == 0 {
			s.finishRound(true)
			return
		}
		s.isRoundOver = false
		s.collectFemaleSymbols()
		return
	}

	s.updateStepResults(false)
	s.finishRound(true)
}

// computeCascade 计算当前步的中奖结果，返回是否继续连消
// - partial 控制是否只计算女性符号中奖
// - 注意：返回 false 时并不会自动结束回合，调用方需自行调用 finishRound
// finishRound 将回合状态重置为“已结束”
// - 清空下一步网格，统计夺宝和新增免费次数
// - finalizeStep 会在 isRoundOver=true 时跳过连消处理
func (s *spin) finishRound(isFreeRound bool) {
	s.isRoundOver = true
	s.nextSymbolGrid = nil
	/*
		s.treasureCount = getTreasureCount(s.symbolGrid)
		if isFreeRound {
			s.newFreeRoundCount = s.treasureCount
		} else {
			s.newFreeRoundCount = _cnf.getFreeRoundCount(s.treasureCount)
		}
		if isFreeRound {
			if s.treasureCount > 0 {
				s.newFreeRoundCount = s.treasureCount
			} else {
				s.newFreeRoundCount = 0
			}
		} else {
			s.newFreeRoundCount = _cnf.getFreeRoundCount(s.treasureCount)
		}
	*/
}

// collectFemaleSymbols 收集中奖的女性符号
func (s *spin) collectFemaleSymbols() {
	if s.winGrid == nil {
		return
	}
	for _, row := range s.winGrid {
		for _, symbol := range row {
			// 跳过墙格标记
			if symbol == _blocked {
				continue
			}
			// 只统计普通女性符号（7,8,9），不统计女性百搭符号（10,11,12）
			if symbol >= _femaleA && symbol <= _femaleC {
				idx := symbol - _femaleA
				if s.nextFemaleCountsForFree[idx] < _femaleSymbolCountForFullElimination {
					s.nextFemaleCountsForFree[idx]++
				}
			}
		}
	}
}

// updateStepResults 计算中奖结果
// partialElimination 为 true 表示“只结算女性符号的中奖”（用于基础/免费模式连消中，保留其它符号供后续消除使用）
func (s *spin) updateStepResults(partialElimination bool) {
	var winResults []*winResult
	var winGrid int64Grid
	lineMultiplier := int64(0)
	for _, info := range s.winInfos {
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

		/*
			effectiveLineCount := info.LineCount
			if partialElimination && (info.Symbol >= _femaleA && info.Symbol <= _femaleC || info.Symbol >= _wildFemaleA && info.Symbol <= _wildFemaleC) {
				effectiveLineCount = computeFemalePartialLineCount(info)
				if effectiveLineCount == 0 {
					continue
				}
			}
			baseLineMultiplier := _cnf.getSymbolMultiplier(info.Symbol, info.SymbolCount)
			totalMultiplier := baseLineMultiplier * effectiveLineCount
			if partialElimination && (info.Symbol >= _femaleA && info.Symbol <= _femaleC || info.Symbol >= _wildFemaleA && info.Symbol <= _wildFemaleC) {
				fmt.Printf("[RTP-TRACE] partial=%v symbol=%d count=%d lines=%d base=%d total=%d\n",
					partialElimination, info.Symbol, info.SymbolCount, effectiveLineCount, baseLineMultiplier, totalMultiplier)
			}
			result := winResult{
				Symbol:             info.Symbol,
				SymbolCount:        info.SymbolCount,
				LineCount:          effectiveLineCount,
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
		*/

	}
	s.stepMultiplier = lineMultiplier
	s.winResults = winResults
	s.winGrid = &winGrid
}

// checkAddFreeCount 新增免费次数
func (s *spin) checkAddFreeCount(isFreeRound bool) {
	if s.isRoundOver {
		s.treasureCount = getTreasureCount(s.symbolGrid)
		if isFreeRound {

			newTreasure := s.treasureCount - s.roundStartTreasure
			if newTreasure < 0 {
				newTreasure = 0
			}
			s.newFreeRoundCount = newTreasure

			// s.newFreeRoundCount = s.treasureCount
		} else {
			s.newFreeRoundCount = _cnf.getFreeRoundCount(s.treasureCount)
		}
	} else {
		s.treasureCount = 0
		s.newFreeRoundCount = 0
	}
}

// finalizeStep 根据连消结果准备下一步或结束本回合
func (s *spin) finalizeStep(isFreeRound bool) {
	if s.isRoundOver {
		s.nextSymbolGrid = nil
		return
	}

	// 如果没有中奖网格，不应该继续连消
	if s.winGrid == nil {
		global.GVA_LOG.Error("finalizeStep: winGrid is nil but isRoundOver is false")
		s.finishRound(isFreeRound)
		return
	}

	// if isFreeRound {
	// 	s.resetFemaleCollectionIfEliminated()
	// }

	// 消除 → 下落 → 填充
	newGrid := *s.symbolGrid
	clearBlockedCells(&newGrid)
	clearBlockedCells(s.winGrid)

	hasTreasure := getTreasureCount(&newGrid) > 0
	eliminatedCount := 0

	// 消除中奖符号（保护夺宝和百搭）
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if isBlockedCell(r, c) || s.winGrid[r][c] == _blank {
				continue
			}
			symbol := newGrid[r][c]
			if isBlockedCell(r, c) {
				continue
			}
			if symbol == _treasure || (symbol == _wild && (isFreeRound || hasTreasure)) {
				continue
			}
			newGrid[r][c] = _eliminated
			eliminatedCount++
		}
	}

	if eliminatedCount == 0 {
		global.GVA_LOG.Error("no symbols eliminated, forcing round end")
		s.finishRound(isFreeRound)
		return
	}

	// 符号下落
	for c := int64(0); c < _colCount; c++ {
		writePos := _rowCount - 1
		if c == 0 || c == _colCount-1 {
			writePos = _rowCount - 2
		}
		for r := _rowCount - 1; r >= 0; r-- {
			if r == _rowCount-1 && (c == 0 || c == _colCount-1) {
				continue
			}
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
			if r != writePos {
				newGrid[writePos][c] = newGrid[r][c]
				newGrid[r][c] = _blank
			}
			writePos--
		}
	}

	// 填充新符号
	for c := int64(0); c < _colCount; c++ {
		for r := int64(_rowCount - 1); r >= 0; r-- {
			if r == _rowCount-1 && (c == 0 || c == _colCount-1) {
				continue
			}
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
}

// clearBlockedCells 将网格中的墙格（最后一行左右角）置为 _blocked
func clearBlockedCells(grid *int64Grid) {
	if grid == nil {
		return
	}
	(*grid)[_rowCount-1][0] = _blocked
	(*grid)[_rowCount-1][_colCount-1] = _blocked
}

func isBlockedCell(row, col int64) bool {
	return row == _rowCount-1 && (col == 0 || col == _colCount-1)
}

func isMatchingFemaleWild(target, candidate int64) bool {
	if candidate < _wildFemaleA || candidate > _wildFemaleC {
		return false
	}
	if target < _femaleA || target > _femaleC {
		return false
	}
	return (candidate - _wildFemaleA) == (target - _femaleA)
}

/*func (s *spin) resetFemaleCollectionIfEliminated() {
	if s.winGrid == nil || s.symbolGrid == nil {
		return
	}
	var resetFlags [3]bool
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.winGrid[r][c] == _blank {
				continue
			}
			symbol := s.symbolGrid[r][c]
			switch {
			case symbol >= _femaleA && symbol <= _femaleC:
				resetFlags[symbol-_femaleA] = true
			case symbol >= _wildFemaleA && symbol <= _wildFemaleC:
				resetFlags[symbol-_wildFemaleA] = true
			}
		}
	}
	for i, reset := range resetFlags {
		if reset {
			s.femaleCountsForFree[i] = 0
			s.nextFemaleCountsForFree[i] = 0
		}
	}
}*/

/*
func isSubstituteForSymbol(target, candidate int64) bool {
	if candidate == _wild {
		return true
	}
	if candidate < _wildFemaleA || candidate > _wildFemaleC {
		return false
	}
	if target < _femaleA || target > _femaleC {
		return false
	}
	return (candidate - _wildFemaleA) == (target - _femaleA)
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func computeFemalePartialLineCount(info *winInfo) int64 {
	var baseSymbol int64
	var femaleWildSymbol int64
	switch {
	case info.Symbol >= _femaleA && info.Symbol <= _femaleC:
		baseSymbol = info.Symbol
		femaleWildSymbol = _wildFemaleA + (info.Symbol - _femaleA)
	case info.Symbol >= _wildFemaleA && info.Symbol <= _wildFemaleC:
		baseSymbol = _femaleA + (info.Symbol - _wildFemaleA)
		femaleWildSymbol = info.Symbol
	default:
		return info.LineCount
	}

	total := int64(1)
	for c := int64(0); c < info.SymbolCount; c++ {
		var primary, femaleWild, wild int64
		for r := int64(0); r < _rowCount; r++ {
			val := info.WinGrid[r][c]
			if val == _blank || val == _blocked {
				continue
			}
			if val == baseSymbol {
				primary++
				continue
			}
			if val == femaleWildSymbol {
				femaleWild++
				continue
			}
			if val == _wild {
				wild++
			}
		}

		var columnCount int64
		if info.Symbol >= _wildFemaleA && info.Symbol <= _wildFemaleC {
			if femaleWild > 0 {
				columnCount = femaleWild
			} else if primary > 0 {
				columnCount = primary
			} else if wild > 0 {
				columnCount = 1
			} else {
				return 0
			}
		} else {
			if primary > 0 {
				columnCount = primary
			} else if femaleWild > 0 {
				columnCount = femaleWild
			} else if wild > 0 {
				columnCount = 1
			} else {
				return 0
			}
		}
		if columnCount == 0 {
			return 0
		}
		total *= columnCount
	}
	return total
}
*/
