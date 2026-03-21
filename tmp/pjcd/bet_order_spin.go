package pjcd

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}
	if s.isFreeRound && s.scene.IsRoundFirstStep {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
		s.scene.IsRoundFirstStep = false
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
	s.addFreeTime = 0
	wildForm := s.calcWildForm()
	s.updateGameMultiple()
	if len(s.winInfos) > 0 {
		s.processWin(wildForm)
	} else {
		s.processNoWin()
	}
}

func (s *betOrderService) updateGameMultiple() {
	s.gameMultiple, s.gameMultipleIndex = s.getStreakMultiplier()
	if s.gameMultipleIndex >= 3 && s.scene.TotalWildEliCount > 0 {
		s.wildMultiplier = s.gameConfig.WildAddFourthMultiple * s.scene.TotalWildEliCount
	}
	s.gameMultiple += s.wildMultiplier
}

// getStreakMultiplier 获取轮次倍数
// 返回值：当前倍数、当前索引
func (s *betOrderService) getStreakMultiplier() (int64, int64) {
	var multipliers []int64
	if s.isFreeRound {
		multipliers = s.gameConfig.FreeRoundMultipliers
	} else {
		multipliers = s.gameConfig.BaseRoundMultipliers
	}
	if len(multipliers) == 0 {
		return 0, 0
	}

	index := s.scene.ContinueNum
	if index < 0 {
		index = 0
	}
	if index >= int64(len(multipliers)) {
		index = int64(len(multipliers)) - 1
	}
	return multipliers[index], index
}

func (s *betOrderService) processWin(wildForm int64Grid) {
	s.lineMultiplier = s.handleWinElemsMultiplier(s.winInfos)
	s.stepMultiplier = s.lineMultiplier * s.gameMultiple
	s.isRoundOver = false

	s.scene.Steps++
	s.scene.ContinueNum++
	s.scene.RoundMultiplier += s.stepMultiplier

	s.nextSymbolGrid = s.moveSymbols(wildForm)
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

	// 基础模式结束，重置蝴蝶百搭个数
	if !s.isFreeRound {
		s.scene.TotalWildEliCount = 0
	}

	s.updateBonusAmount(0)
	s.client.ClientOfFreeGame.SetLastWinId(0)

	if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
		s.client.ClientOfFreeGame.Incr(uint64(newFree))
		s.scene.FreeNum += newFree
		s.addFreeTime = newFree
	}

	if s.isFreeRound {
		if s.scene.FreeNum <= 0 {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
			s.scene.IsRoundFirstStep = false
			s.scene.TotalWildEliCount = 0
			// 同一次免费触发期间复用 FreeReelData；免费结束回到基础模式后清理，确保下次触发重新生成
			if len(s.scene.FreeReelData) > 0 {
				s.scene.FreeReelData = nil
			}
		} else {
			s.scene.NextStage = _spinTypeFree
			s.scene.IsRoundFirstStep = true
		}
	} else {
		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
			s.scene.IsRoundFirstStep = true
			s.scene.TotalWildEliCount = 0
		} else {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
			s.scene.IsRoundFirstStep = false
		}
	}
}

func (s *betOrderService) handleWinElemsMultiplier(elems []WinInfo) int64 {
	var mul int64
	for _, elem := range elems {
		mul += elem.Odds
	}
	return mul
}
