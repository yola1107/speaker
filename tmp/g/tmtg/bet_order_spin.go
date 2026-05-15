package tmtg

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
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
		s.initSpinSymbol()
	}
	s.handleSymbolGrid()
	s.findWinInfos()
	s.processWinInfos()
	return nil
}

func (s *betOrderService) processWinInfos() {
	s.addFreeTime = 0
	s.scatterCount = s.counter[_treasure]

	var totalOdds int64
	for _, w := range s.winInfos {
		totalOdds += w.Odds
	}
	s.lineMultiplier = totalOdds
	s.stepMultiplier = s.lineMultiplier
	if s.bombMulSum > 0 && s.stepMultiplier > 0 {
		s.stepMultiplier *= s.bombMulSum
	}

	var haveEli, wildKeep bool
	for _, w := range s.winInfos {
		if w.Symbol < _treasure {
			haveEli = true
			if w.Count < _minMatchCount {
				wildKeep = true // wild 不消除
			}
		}
	}

	if !s.isFreeRound {
		s.processBase(haveEli, wildKeep)
	} else {
		s.processFree(haveEli, wildKeep)
	}

	s.updateBonusAmount()
	s.applyMaxWinLimit()
}

func (s *betOrderService) processBase(haveEli, wildKeep bool) {
	if haveEli {
		s.scene.Steps++
		s.isRoundOver = false
		s.scene.NextStage = _spinTypeBaseEli
		s.nextSymbolGrid = s.moveSymbols(wildKeep)
		s.fallingWinSymbols()

	} else {
		s.scene.Steps = 0
		s.isRoundOver = true
		if newFree := s.gameConfig.calcNewFreeGameNum(s.isFreeRound, s.scatterCount); newFree > 0 {
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
		}
		if s.addFreeTime > 0 {
			s.scene.NextStage = _spinTypeFree
		} else {
			s.scene.NextStage = _spinTypeBase
		}
	}
}

func (s *betOrderService) processFree(haveEli, wildKeep bool) {
	var haveWild = s.counter[_wild] > 0

	switch {
	case haveWild && !haveEli:
		s.scene.Steps++
		s.isRoundOver = false
		s.scene.NextStage = _spinTypeFreeBombEli
		s.nextSymbolGrid = s.eliBombSymbols()
		s.fallingWinSymbols()

	case haveEli:
		s.scene.Steps++
		s.isRoundOver = false
		s.scene.NextStage = _spinTypeFreeEli
		s.nextSymbolGrid = s.moveSymbols(wildKeep)
		s.fallingWinSymbols()

	default:
		s.scene.Steps = 0
		s.isRoundOver = true
		if newFree := s.gameConfig.calcNewFreeGameNum(s.isFreeRound, s.scatterCount); newFree > 0 {
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
		}
		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
		} else {
			s.scene.NextStage = _spinTypeBase
		}
	}
}
