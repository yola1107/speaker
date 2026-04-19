package yhwy

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}

	if s.isFreeRound {
		s.scene.FreeTimes++
		s.scene.FreeNum--
	}

	s.initSpinSymbol()
	s.handleSymbolGrid()
	s.checkSymbolGridWin()
	s.processWinInfos()
	return nil
}

func (s *betOrderService) processWinInfos() {
	s.addFreeTime = 0
	s.isRoundOver = true
	s.scatterCount = s.getScatterCount()

	s.lineMultiplier = 0
	for _, elem := range s.winInfos {
		s.lineMultiplier += elem.Odds
	}
	s.stepMultiplier = s.lineMultiplier

	if s.isFreeRound {
		if s.scene.FreeNum <= 0 {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
			s.scene.Lock = int64Grid{} // 清理
		} else {
			s.scene.NextStage = _spinTypeFree
		}

	} else {
		if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
		}
		if s.addFreeTime > 0 {
			s.scene.NextStage = _spinTypeFree
		} else {
			s.scene.NextStage = _spinTypeBase
		}
		s.scene.Lock = int64Grid{} // 清理
	}

	s.updateBonusAmount(s.stepMultiplier)
}
