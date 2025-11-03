package xslm2

import (
	"egame-grpc/model/slot"
)

// spin spin数据结构
type spin struct {
	preset                  *slot.XSLM                     // 预设数据
	stepMap                 *stepMap                       // 当前step数据
	femaleCountsForFree     [_femaleC - _femaleA + 1]int64 // 女性符号计数（当前）
	enableFullElimination   bool                           // 全屏消除标志
	isRoundOver             bool                           // 回合结束标志
	symbolGrid              *int64Grid                     // 符号网格
	winInfos                []*winInfo                     // 中奖信息
	winResults              []*winResult                   // 中奖结果
	winGrid                 *int64Grid                     // 中奖网格
	hasFemaleWin            bool                           // 有女性中奖标志
	lineMultiplier          int64                          // 线倍数
	stepMultiplier          int64                          // 步骤倍数
	nextFemaleCountsForFree [_femaleC - _femaleA + 1]int64 // 女性符号计数（下一局）
	treasureCount           int64                          // 夺宝数量
	newFreeRoundCount       int64                          // 新增免费次数
}

// updateStepResult 更新步骤结果（主入口）
func (s *spin) updateStepResult(isFreeRound bool) bool {
	// 1. 加载预设数据
	if !s.loadStepData(isFreeRound) {
		return false
	}

	// 2. 检查全屏消除
	s.updateStepData()

	// 3. 查找中奖
	s.findWinInfos()

	// 4. 处理步骤（基础或免费）
	switch {
	case !isFreeRound:
		s.processStepForBase()
	default:
		s.processStepForFree()
	}

	return true
}

// ========== 基础模式和免费模式逻辑 ==========

// processStepForBase 处理基础模式步骤
// 基础模式只有一种特殊情况：有女性中奖 + 有Wild符号
// 这种情况下继续下一step（预设数据会继续播放）
func (s *spin) processStepForBase() {
	switch {
	case s.hasFemaleWin && s.hasWildSymbol():
		// 有女性中奖且有Wild → 继续下一step（部分消除）
		s.updateStepResults(true) // 只计算女性符号
		// isRoundOver保持false，会继续播放下一个预设step

	default:
		// 其他情况 → 回合结束
		s.updateStepResults(false) // 计算所有符号
		s.isRoundOver = true

		// 检查是否触发免费游戏
		s.treasureCount = s.getTreasureCount()
		if s.treasureCount >= _triggerTreasureCount {
			s.newFreeRoundCount = _freeRounds[s.treasureCount-_triggerTreasureCount]
		}
	}
}

// processStepForFree 处理免费模式步骤
// 免费模式有三种情况：
// 1. enableFullElimination=true: 女性符号达到10个，触发全屏消除
// 2. hasFemaleWin=true: 有女性中奖，继续下一step
// 3. default: 无女性中奖，回合结束
func (s *spin) processStepForFree() {
	switch {
	case s.enableFullElimination:
		// 情况1：全屏消除（女性符号>=10）
		s.updateStepResults(false) // 计算所有符号中奖

		// 统计中奖的女性符号数量
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				symbol := s.winGrid[r][c]
				if symbol >= _femaleA && symbol <= _femaleC {
					s.updateFemaleCountForFree(symbol)
				}
			}
		}

		// 检查是否还有中奖（全屏转Wild后可能继续中奖）
		s.isRoundOver = len(s.winResults) == 0

	case s.hasFemaleWin:
		// 情况2：有女性中奖，继续下一step
		s.updateStepResults(true) // 只计算女性符号

		// 统计中奖的女性符号数量
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				symbol := s.winGrid[r][c]
				if symbol >= _femaleA && symbol <= _femaleC {
					s.updateFemaleCountForFree(symbol)
				}
			}
		}

	default:
		// 情况3：无女性中奖，回合结束
		s.updateStepResults(false)
		s.isRoundOver = true
	}

	// 回合结束时，检查夺宝符号追加免费次数
	if s.isRoundOver {
		s.newFreeRoundCount = s.getTreasureCount()
	}
}

// updateFemaleCountForFree 更新女性符号收集计数
// 免费游戏中，每次中奖的女性符号都会被收集
// 三种女性符号（A/B/C）独立计数，达到10个触发全屏消除
// 参数：
//
//	symbol - 女性符号ID（7=femaleA, 8=femaleB, 9=femaleC）
func (s *spin) updateFemaleCountForFree(symbol int64) {
	switch symbol {
	case _femaleA:
		// 计数已超过10，不再累加
		if s.nextFemaleCountsForFree[_femaleA-_femaleA] > _femaleSymbolCountForFullElimination {
			return
		}
		s.nextFemaleCountsForFree[_femaleA-_femaleA]++

	case _femaleB:
		if s.nextFemaleCountsForFree[_femaleB-_femaleA] > _femaleSymbolCountForFullElimination {
			return
		}
		s.nextFemaleCountsForFree[_femaleB-_femaleA]++

	case _femaleC:
		if s.nextFemaleCountsForFree[_femaleC-_femaleA] > _femaleSymbolCountForFullElimination {
			return
		}
		s.nextFemaleCountsForFree[_femaleC-_femaleA]++
	}
}
