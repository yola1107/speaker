package qnjx

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.debug.mark = 0
		s.debug.added = [3]int64{}
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}
	if s.isFreeRound && s.scene.FreeNum > 0 && s.scene.Stage == _spinTypeFree {
		s.scene.FreeTimes++
		s.scene.FreeNum--
	}
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		baseMul := int64(1)
		if s.isFreeRound {
			baseMul = 2
		}
		s.scene.ColorMul = [3]int64{baseMul, baseMul, baseMul}
		s.scene.ColorCount = [3]int64{}
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
	s.limit = false
	s.scatterCount = s.getScatterCount()
	if len(s.winInfos) > 0 {
		s.processWin()
	} else {
		s.processNoWin()
	}
	s.applyMaxWinMultiplierLimit()
	s.updateBonusAmount(s.stepMultiplier)
}

func (s *betOrderService) processWin() {
	var totalMul int64
	for _, info := range s.winInfos {
		totalMul += info.Multiplier
	}
	s.lineMultiplier = totalMul
	s.mysMul = s.scene.ColorMul[0] * s.scene.ColorMul[1] * s.scene.ColorMul[2]
	s.stepMultiplier = s.lineMultiplier * s.mysMul
	s.isRoundOver = false
	s.scene.Steps++

	s.collectWinningSymbols()
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
	s.mysMul = 0

	if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
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
