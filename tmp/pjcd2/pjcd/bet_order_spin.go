package pjcd

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage() // RTP 测试模式：手动进行状态转换
	}
	if err := s.initialize(); err != nil {
		return err
	}
	// 在 Round 首 Step 时扣减免费次数（参考 mahjong 实现）
	if s.isFreeRound && s.scene.IsRoundFirstStep {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
		s.scene.IsRoundFirstStep = false // 标记已处理，避免重复执行
	}
	// 新回合初始化
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.scene.SymbolRoller = s.initSpinSymbol()
		//s.scene.WildStateGrid = int64Grid{} // 新回合重置百搭状态
		// 基础模式：重置蝴蝶百搭个数
		if !s.isFreeRound {
			s.scene.ButterflyCount = 0
		}
		// 免费模式：ButterflyCount 保持累计（不重置）
	}
	s.handleSymbolGrid()
	//s.initWildStateGrid() // 初始化新百搭状态
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
	s.gameMultiple, s.gameMultiples, s.gameMultipleIndex, s.butterflyMultiplier = s.getStreakMultiplier()
	s.lineMultiplier = s.handleWinElemsMultiplier(s.winInfos)
	s.stepMultiplier = s.lineMultiplier * s.gameMultiple
	s.isRoundOver = false

	s.scene.Steps++
	s.scene.ContinueNum++
	s.scene.RoundMultiplier += s.stepMultiplier

	// 执行消除和移动流程：消除 -> wild移动 -> 下落+左移动 -> 同步roller
	//s.evolveWilds() // 进化百搭形态

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

	// 基础模式结束，重置蝴蝶百搭个数
	if !s.isFreeRound {
		s.scene.ButterflyCount = 0
	}

	s.updateBonusAmount(0)
	s.client.ClientOfFreeGame.SetLastWinId(0)

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
			s.scene.IsRoundFirstStep = false // 免费模式结束
			s.scene.ButterflyCount = 0       // 免费结束重置
		} else {
			s.scene.NextStage = _spinTypeFree
			s.scene.IsRoundFirstStep = true // 下一轮免费回合的首 Step
		}
	} else {
		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
			s.scene.IsRoundFirstStep = true // 新进入免费模式，标记为首 Step
			s.scene.ButterflyCount = 0      // 进入免费时重置
		} else {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
			s.scene.IsRoundFirstStep = false // 普通模式不需要此标志
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
