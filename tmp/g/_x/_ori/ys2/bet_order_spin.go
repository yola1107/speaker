package ys2

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
		s.scene.SymbolRoller = s.gameConfig.initSpinSymbol(s.isFreeRound, s.scene.BonusNum)
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
	s.scatterCount = s.getScatterCount()

	if len(s.winInfos) > 0 {
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
		if s.isFreeRound {
			s.scene.NextStage = _spinTypeFreeEli
		} else {
			s.scene.NextStage = _spinTypeBaseEli
		}
		s.nextSymbolGrid = s.moveSymbols()
		s.fallingWinSymbols(s.nextSymbolGrid)

	} else {
		s.lineMultiplier = 0
		s.stepMultiplier = 0
		s.scene.Steps = 0
		s.isRoundOver = true
		if s.isFreeRound {
			if s.scatterCount >= s.gameConfig.Free.ScatterMin {
				newFree := s.gameConfig.getFreeCfgByType(s.scene.BonusNum).Times
				s.scene.FreeNum += newFree
				s.addFreeTime = newFree
			}
			if s.scene.FreeNum <= 0 {
				s.scene.FreeNum = 0
				s.scene.NextStage = _spinTypeBase
			} else {
				s.scene.NextStage = _spinTypeFree
			}
		} else {
			if s.scatterCount >= s.gameConfig.Free.ScatterMin {
				s.startBonusSelection()
			} else {
				s.scene.FreeNum = 0
				s.scene.NextStage = _spinTypeBase
			}
		}
	}

	s.applyMaxWinLimit()
	s.updateBonusAmount()
}

func (s *betOrderService) startBonusSelection() {
	s.scene.FreeNum = 0
	s.scene.BonusNum = 0
	s.scene.ScatterNum = s.scatterCount
	s.scene.BonusState = _bonusStatePending
	s.scene.NextStage = _spinTypeBase
}

func (s *betOrderService) ensureBonusSelected() error {
	if s.scene == nil || s.scene.BonusState != _bonusStatePending {
		return nil
	}
	return ErrBonusNumMustSelect
}
