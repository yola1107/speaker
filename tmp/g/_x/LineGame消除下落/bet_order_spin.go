package hcsqy

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
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
	s.checkSymbolGridWin()
	s.processWinInfos()
	return nil
}
func (s *betOrderService) processWinInfos() {
	s.addFreeTime = 0 // 重置增加的免费次数
	s.scatterCount = s.getScatterCount()
	if len(s.winInfos) > 0 {
		s.processWin()
	} else {
		s.processNoWin()
	}
	s.updateBonusAmount(s.stepMultiplier)
}

// processWinInfos：中奖后执行“符号消除→下落→填充”，并设置 next=true 让前端继续请求下一步消除。
func (s *betOrderService) processWin() {
	var totalOdds int64
	for _, w := range s.winInfos {
		totalOdds += w.Odds
	}
	s.lineMultiplier = totalOdds
	s.stepMultiplier = s.lineMultiplier
	s.isRoundOver = false

	s.scene.Steps++
	s.scene.RoundMultiplier += s.stepMultiplier

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

	// 免费次数新增
	if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
		s.client.ClientOfFreeGame.Incr(uint64(newFree))
		s.scene.FreeNum += newFree
		s.addFreeTime = newFree
	}

	if s.isFreeRound {
		if s.scene.FreeNum <= 0 {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase

		} else {
			s.scene.NextStage = _spinTypeFree
		}
	} else {
		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree

		} else {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
		}
	}
}
