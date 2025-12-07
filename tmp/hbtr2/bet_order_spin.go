package hbtr2

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage() // RTP测试模式需要手动状态转换
	}
	if err := s.initialize(); err != nil {
		return err
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
	s.addFreeTime = 0 // 重置免费次数
	s.nextSymbolGrid = nil
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
	s.scene.RoundMultiplier += s.stepMultiplier

	// 执行消除和移动流程：消除 -> wild移动 -> 下落+左移动 -> 同步roller
	nextGrid := s.eliminateWinSymbols()
	s.bats = s.moveWildSymbols(nextGrid)
	nextGrid = s.moveSymbols(nextGrid)
	s.nextSymbolGrid = s.fallingWinSymbols(nextGrid)

	if s.isFreeRound {
		s.scene.NextStage = _spinTypeFreeEli
	} else {
		s.scene.NextStage = _spinTypeBaseEli
	}
	s.updateBonusAmount(s.stepMultiplier)
}

// processNoWin 处理未中奖情况
func (s *betOrderService) processNoWin() {
	// 重置游戏状态
	s.gameMultiple = 1
	s.lineMultiplier = 0
	s.stepMultiplier = 0
	s.isRoundOver = true
	s.scatterCount = s.getScatterCount()
	s.scene.Steps = 0

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
			s.scene.FreeNum = 0
			s.scene.ScatterNum = 0
			s.scene.NextStage = _spinTypeBase
		} else {
			s.scene.NextStage = _spinTypeFree
		}

	} else {
		if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
			s.client.ClientOfFreeGame.Incr(uint64(newFree))
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
			s.scene.NextStage = _spinTypeFree
		} else {
			s.scene.NextStage = _spinTypeBase
		}
		s.scene.ScatterNum = s.scatterCount
	}
}

func (s *betOrderService) handleWinElemsMultiplier(elems []WinInfo) int64 {
	var total int64
	for _, elem := range elems {
		total += elem.Multiplier
	}
	return total
}

// getStreakMultiplier 获取连击倍数（当前固定为1）
func (s *betOrderService) getStreakMultiplier() int64 {
	return 1
}
