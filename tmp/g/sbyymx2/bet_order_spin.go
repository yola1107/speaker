package sbyymx2

func (s *betOrderService) baseSpin() error {
	if err := s.initialize(); err != nil {
		return err
	}
	s.stepIsRespinMode = s.scene.IsRespinMode || s.isHitRespinProb()
	s.initSpinSymbol()
	s.handleSymbolGrid()
	s.processGame()
	return nil
}

func (s *betOrderService) processGame() {
	s.wildMultiplier = 1
	s.lineMultiplier = 0
	s.stepMultiplier = 0
	s.isRoundOver = true
	s.isInstrumentWin = false
	s.isWildExpandCol = false

	s.checkSymbolGridWin()

	if s.stepIsRespinMode {
		// 重转到赢
		if len(s.winInfos) == 0 {
			s.scene.IsRespinMode = true
			s.isRoundOver = false
		} else {
			s.wildMultiplier = s.weightWildMultiplier()
			s.scene.IsRespinMode = false
			s.expandInstrumentToWild()
		}
	} else {
		// 基础模式
		midR, midC := _rowCount/2, _colCount/2
		if len(s.winInfos) > 0 && s.symbolGrid[midR][midC] == _wild && s.isHitWildExpandProb() {
			s.isWildExpandCol = true
			s.wildMultiplier = s.weightWildMultiplier()
			for r := 0; r < _rowCount; r++ {
				s.symbolGrid[r][midC] = _wild
			}
			s.expandInstrumentToWild()
		}
	}

	s.stepMultiplier = s.lineMultiplier * s.wildMultiplier
	s.updateBonusAmount(s.stepMultiplier)
}

func (s *betOrderService) expandInstrumentToWild() {
	if symbol := s.winInfos[0].Symbol; symbol >= _tambourine && symbol <= _drum {
		s.isInstrumentWin = true
		left, right := 0, _colCount-1
		for r := 0; r < _rowCount; r++ {
			s.symbolGrid[r][left] = symbol
			s.symbolGrid[r][right] = symbol
		}
	}
}
