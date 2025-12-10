package sgz

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage() // RTP 测试模式：直接调用时需要手动进行状态转换
	}
	if err := s.initialize(); err != nil {
		return err
	}
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.scene.SymbolRoller = s.initSpinSymbol()
	}
	s.handleSymbolGrid()
	s.checkSymbolGridWin()
	s.processWinInfos()
	return nil
}

func (s *betOrderService) processWinInfos() {
	s.addFreeTime = 0 // 重置增加的免费次数
	if len(s.winInfos) > 0 {
		s.processWin()
	} else {
		s.processNoWin()
	}
}

func (s *betOrderService) processWin() {
	s.gameMultiple = s.getStreakMultiplier()
	s.lineMultiplier = s.handleWinElemsMultiplier(s.winInfos)
	s.stepMultiplier = s.lineMultiplier * s.gameMultiple
	s.isRoundOver = false

	s.scene.Steps++
	s.scene.ContinueNum++
	s.scene.RoundMultiplier += s.stepMultiplier
	s.scene.CityValue += s.stepMultiplier // 战斗力++

	s.nextSymbolGrid = s.moveSymbols()
	s.fallingWinSymbols(s.nextSymbolGrid)

	if s.isFreeRound {
		s.scene.NextStage = _spinTypeFreeEli
	} else {
		s.scene.NextStage = _spinTypeBaseEli
	}
	s.updateBonusAmount(s.stepMultiplier)
}

func (s *betOrderService) processNoWin() {
	s.gameMultiple = 0
	s.lineMultiplier = 0
	s.stepMultiplier = 0
	s.isRoundOver = true
	s.scatterCount = s.getScatterCount()

	s.scene.Steps = 0
	s.scene.ContinueNum = 0

	s.updateBonusAmount(0)
	s.client.ClientOfFreeGame.SetLastWinId(0)

	if s.isFreeRound {
		if newFree := s.scatterCount; newFree > 0 {
			s.client.ClientOfFreeGame.Incr(uint64(newFree))
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
		}

		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--

		if s.scene.FreeNum <= 0 {
			//s.scene.HeroID = 0
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
		} else {
			s.scene.NextStage = _spinTypeFree
		}

	} else {
		if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
			s.client.ClientOfFreeGame.Incr(uint64(newFree))
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
		}

		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
		} else {
			//s.scene.HeroID = 0
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
		}

	}
}

func (s *betOrderService) handleWinElemsMultiplier(elems []WinInfo) int64 {
	var stepMultiplier int64
	for _, elem := range elems {
		stepMultiplier += elem.Multiplier
	}
	return stepMultiplier
}
