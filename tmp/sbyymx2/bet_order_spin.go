package sbyymx2

import "math/rand/v2"

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		// debug 模式下不做任何阶段同步
	}
	if err := s.initialize(); err != nil {
		return err
	}

	// 先确定当前spin是否是重转至赢模式
	s.stepIsRespinMode = s.scene.IsRespinMode || s.isHitRespinProb()

	s.initSpinSymbol()
	s.handleSymbolGrid()
	s.processGame()
	return nil
}

func (s *betOrderService) processGame() {
	// 初始化字段
	s.respinWildCol = -1
	s.wildExpandCol = -1
	s.wildMultiplier = 1
	s.lineMultiplier = 0
	s.next = false
	s.stepMultiplier = 0
	s.isRoundOver = true

	if s.stepIsRespinMode {
		s.processRespinUntilWin()
	} else {
		s.processWildExpand()
	}
}

func (s *betOrderService) processRespinUntilWin() {
	// 随机选择一列填满百搭
	c := rand.IntN(_colCount)
	for r := 0; r < _rowCount; r++ {
		s.symbolGrid[r][c] = _wild
	}
	s.respinWildCol = int32(c)
	s.wildMultiplier = s.weightWildMultiplier()

	s.checkSymbolGridWin()

	// 中奖则结束重转模式
	if len(s.winInfos) > 0 {
		s.scene.IsRespinMode = false
		s.next = false
	} else {
		s.scene.IsRespinMode = true
		s.next = true
		s.isRoundOver = false
	}

	s.stepMultiplier = s.lineMultiplier * s.wildMultiplier
	s.updateBonusAmount(s.stepMultiplier)
}

func (s *betOrderService) processWildExpand() {
	// 找候选列（1-2个百搭）
	var candidates []int
	for c := 0; c < _colCount; c++ {
		wildCount := 0
		for r := 0; r < _rowCount; r++ {
			if s.symbolGrid[r][c] == _wild {
				wildCount++
			}
		}
		if wildCount >= 1 && wildCount <= 2 {
			candidates = append(candidates, c)
		}
	}

	// 概率触发百搭变大
	if len(candidates) > 0 && s.isHitWildExpandProb() {
		c := candidates[rand.IntN(len(candidates))]
		for r := 0; r < _rowCount; r++ {
			s.symbolGrid[r][c] = _wild
		}
		s.wildExpandCol = int32(c)
		s.wildMultiplier = s.weightWildMultiplier()
	}

	s.checkSymbolGridWin()
	s.stepMultiplier = s.lineMultiplier * s.wildMultiplier
	s.updateBonusAmount(s.stepMultiplier)
}
