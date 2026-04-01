package ajtm

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.debug.mark = 0
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}

	if s.isFreeRound && s.scene.FreeNum > 0 && s.scene.Stage == _spinTypeFree {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
	}

	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.scene.SymbolRoller = s.initSpinSymbol()
	}

	s.handleSymbolGrid()
	s.findWinInfos()
	s.processWinInfos()
	return nil
}

func (s *betOrderService) processWinInfos() {
	s.addFreeTime = 0
	s.debug.mark = 0
	s.scatterCount = s.getScatterCount()

	if len(s.winInfos) > 0 {
		s.processWin()
	} else {
		s.processNoWin()
	}
	s.updateBonusAmount(s.stepMultiplier)
}

func (s *betOrderService) processWin() {
	var totalMul int64
	for _, info := range s.winInfos {
		totalMul += info.Multiplier
	}
	s.lineMultiplier = totalMul

	mysMul := s.scene.MysMultiplierTotal
	if mysMul <= 0 {
		mysMul = 1
	}
	s.stepMultiplier = s.lineMultiplier * mysMul
	s.isRoundOver = false

	s.scene.Steps++
	s.scene.RoundMultiplier += s.stepMultiplier

	s.transformWinningLongSymbols()
	s.nextSymbolGrid = s.moveSymbols()
	s.fallingWinSymbols(s.nextSymbolGrid)

	if s.isFreeRound {
		s.scene.NextStage = _spinTypeFreeEli
	} else {
		s.scene.NextStage = _spinTypeBaseEli
	}
}

func (s *betOrderService) processNoWin() {
	s.addFreeTime = 0
	s.lineMultiplier = 0
	s.stepMultiplier = 0
	s.isRoundOver = true
	s.scene.Steps = 0
	s.longEvents = s.longEvents[:0]

	if s.isFreeRound {
		s.refreshLongCountFromRoller()
		if s.scene.FreeNum <= 0 {
			s.scene.FreeNum = 0
			s.scene.MysMultiplierTotal = 0
			s.scene.NextStage = _spinTypeBase
		} else {
			s.scene.NextStage = _spinTypeFree
		}

	} else {
		s.scene.MysMultiplierTotal = 0
		if newFree, _ := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
			s.client.ClientOfFreeGame.Incr(uint64(newFree))
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
		}

		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
		} else {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
		}
	}
}
