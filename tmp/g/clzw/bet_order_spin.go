package clzw

func (s *betOrderService) baseSpin() error {
	if err := s.initialize(); err != nil {
		return err
	}
	if s.isFreeRound && s.scene.FreeNum > 0 && s.scene.Stage == _spinTypeFree {
		s.scene.FreeTimes++
		s.scene.FreeNum--
	}
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.scene.SymbolRoller = s.gameConfig.initSpinSymbol(s.isFreeRound, s.scene.PurchaseAmount > 0)
	}
	s.handleSymbolGrid()
	s.findWinInfos()
	s.processWinInfos()
	return nil
}
func (s *betOrderService) processWinInfos() {
	s.limit = false
	s.addFreeTime = 0
	s.roundStep = s.scene.Steps
	s.isPurchase = s.scene.PurchaseAmount > 0
	s.scatterCount = s.getScatterCount()
	if len(s.winInfos) > 0 {
		s.processWin()
	} else {
		s.processNoWin()
	}
	s.applyLionMultiplier()
	s.applyMaxWinLimit()
	s.updateBonusAmount()
}

func (s *betOrderService) processWin() {
	var totalMul int64
	for _, info := range s.winInfos {
		totalMul += info.Multiplier
	}
	s.lineMultiplier = totalMul
	s.stepMultiplier = s.lineMultiplier
	if mul := s.getStepMultiplier(); mul > 1 {
		s.stepMultiplier *= mul
	}
	s.scene.Steps++
	s.isRoundOver = false

	s.nextSymbolGrid = s.moveSymbols()
	s.fallingWinSymbols(s.nextSymbolGrid)

	if s.isFreeRound {
		s.scene.NextStage = _spinTypeFreeEli
	} else {
		s.scene.NextStage = _spinTypeBaseEli
	}
}

func (s *betOrderService) processNoWin() {
	s.lineMultiplier = 0
	s.stepMultiplier = 0
	s.scene.Steps = 0
	s.isRoundOver = true

	if newFree := s.gameConfig.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
		s.scene.FreeNum += newFree
		s.addFreeTime = newFree
	}

	if s.scene.FreeNum > 0 {
		s.scene.NextStage = _spinTypeFree
	} else {
		s.scene.FreeNum = 0
		s.scene.PurchaseAmount = 0
		s.scene.NextStage = _spinTypeBase
	}
}
