package pjcd

// initWildStatesForNewSymbols 初始化新百搭的状态
// 规则：百搭初始形态均为毛虫百搭
func (s *betOrderService) initWildStatesForNewSymbols() {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.symbolGrid[r][c] == _wild && s.wildStates[r][c] == _wildStateNone {
				s.wildStates[r][c] = _wildStateCaterpillar
			}
		}
	}
}

// advanceWildState 推进百搭状态
// 规则：毛虫→蝶茧→蝴蝶，蝴蝶参与中奖后消除
func (s *betOrderService) advanceWildState(row, col int) {
	switch s.wildStates[row][col] {
	case _wildStateCaterpillar:
		s.wildStates[row][col] = _wildStateCocoon
	case _wildStateCocoon:
		s.wildStates[row][col] = _wildStateButterfly
	}
}

// processButterflyBonus 处理蝴蝶百搭加成
// 规则：每只蝴蝶百搭参与中奖时，增加 wild_add_fourth_multiplier 到蝴蝶累加倍数
func (s *betOrderService) processButterflyBonus() {
	if count := s.countWinningButterflies(); count > 0 {
		s.butterflyBonus += count * s.gameConfig.WildAddFourthMultiplier
	}
}

// countWinningButterflies 统计参与中奖的蝴蝶百搭数量
func (s *betOrderService) countWinningButterflies() int64 {
	var count int64
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.winGrid[r][c] != 0 && s.wildStates[r][c] == _wildStateButterfly {
				count++
			}
		}
	}
	return count
}

// clearWildStatesAfterElimination 消除后清理百搭状态
// 规则：只有蝴蝶百搭参与中奖后才会被消除
func (s *betOrderService) clearWildStatesAfterElimination() {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.winGrid[r][c] != 0 && s.wildStates[r][c] == _wildStateButterfly {
				s.wildStates[r][c] = _wildStateNone
			}
		}
	}
}

// processWildStatesOnWin 处理中奖时的百搭形态升级
// 规则：参与中奖的百搭形态升级：毛虫→蝶茧→蝴蝶
func (s *betOrderService) processWildStatesOnWin() {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.winGrid[r][c] != 0 && s.symbolGrid[r][c] == _wild {
				s.advanceWildState(r, c)
			}
		}
	}
}
