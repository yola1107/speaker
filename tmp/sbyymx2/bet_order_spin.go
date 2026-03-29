package sbyymx2

func (s *betOrderService) baseSpin() error {
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
	s.wildMultiplier = 1
	s.lineMultiplier = 0
	s.stepMultiplier = 0
	s.next = false
	s.isRoundOver = true
	s.isInstrumentWin = false
	s.isWildExpandCol = false

	if s.stepIsRespinMode {
		s.processRespinUntilWin()
	} else {
		s.processBaseWin()
	}
}

func (s *betOrderService) processRespinUntilWin() {
	s.checkSymbolGridWin()
	s.wildMultiplier = s.weightWildMultiplier() // 随机wild倍数

	if len(s.winInfos) > 0 {
		// 中奖则结束重转模式
		s.scene.IsRespinMode = false
		s.next = false

		// 百搭+乐器符号中奖，两边乐器符号也变大
		s.expandInstrumentToWild()
	} else {
		// 未中奖继续重转
		s.scene.IsRespinMode = true
		s.next = true
		s.isRoundOver = false
	}

	s.stepMultiplier = s.lineMultiplier * s.wildMultiplier
	s.updateBonusAmount(s.stepMultiplier)
}

func (s *betOrderService) processBaseWin() {
	// 先判定中奖（策划文档要求先判定中奖后决定变大）
	s.checkSymbolGridWin()

	// 百搭中奖触发符号变大
	if len(s.winInfos) > 0 && s.symbolGrid[1][1] == _wild && s.isHitWildExpandProb() {
		s.isWildExpandCol = true                    // 标记百搭扩展状态
		s.wildMultiplier = s.weightWildMultiplier() // 随机wild倍数
		// 中间列百搭变大
		for r := 0; r < _rowCount; r++ {
			s.symbolGrid[r][1] = _wild
		}
		// 百搭+乐器符号中奖，两边乐器符号也变大
		s.expandInstrumentToWild()
	}
	s.stepMultiplier = s.lineMultiplier * s.wildMultiplier
	s.updateBonusAmount(s.stepMultiplier)
}

// expandInstrumentToWild 百搭+乐器符号中奖，两边乐器符号也变大
func (s *betOrderService) expandInstrumentToWild() {
	if symbol := s.winInfos[0].Symbol; symbol >= _tambourine && symbol <= _drum {
		s.isInstrumentWin = true
		for r := 0; r < _rowCount; r++ {
			s.symbolGrid[r][0] = symbol
			s.symbolGrid[r][2] = symbol
		}
	}
}
