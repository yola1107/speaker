package bxkh2

func (s *betOrderService) baseSpin() error {
	if err := s.initialize(); err != nil {
		return err
	}
	if s.isFreeRound && s.scene.FreeNum > 0 && s.scene.Stage == _spinTypeFree {
		s.scene.FreeTimes++
		s.scene.FreeNum--
	}

	s.prepareSymbolGrid()

	s.scene.Steps++
	winInfos := s.checkSymbolGridWin()
	s.processWinInfos(winInfos)
	return nil
}

func (s *betOrderService) prepareSymbolGrid() {
	if s.scene.Steps == 0 {
		s.createMatrix(s.scene.Stage)
		if s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeBaseEli {
			s.scene.FreeWinMultiple = 1
		}
		return
	}
	s.symbolGrid = s.buildSymbolGrid()
	//s.buildTailArrays()

	//// 固定当步开算前盘面，供 rtpx 初始盘面/中奖标记稳定输出（仅 debug）。
	//if s.debug.open {
	//	s.debug.originSymbolGrid = s.symbolGrid
	//}
}

func (s *betOrderService) processWinInfos(winInfos []*winInfo) {
	s.addFreeTime = 0
	if len(winInfos) > 0 {
		s.processWin(winInfos)
	} else {
		s.processNoWin()
	}
}

func (s *betOrderService) processWin(winInfos []*winInfo) {
	s.isRoundOver = false
	s.lineMultiplier = s.handleWinInfosMultiplier(winInfos)
	s.stepMultiplier = s.lineMultiplier * s.scene.FreeWinMultiple
	s.updateBonusAmount(s.stepMultiplier)
	s.nextSymbolGrid = s.moveSymbols()
	s.fallingSymbols(s.nextSymbolGrid)

	if s.isFreeRound {
		s.scene.FreeWinMultiple++
		s.scene.NextStage = _spinTypeFreeEli
		return
	}
	s.scene.NextStage = _spinTypeBaseEli
}

func (s *betOrderService) processNoWin() {
	s.stepMultiplier = 0
	s.lineMultiplier = 0
	s.scene.Steps = 0
	s.isRoundOver = true
	s.scatterCount = s.getScatterCount(s.symbolGrid)

	if s.isFreeRound {
		s.scene.NextStage = _spinTypeFree
		if s.scatterCount >= s.gameConfig.FreeGameScatter {
			s.addFreeTime = (s.scatterCount-s.gameConfig.FreeGameScatter)*s.gameConfig.AddFreeTimes + s.gameConfig.FreeTimes
			s.scene.FreeNum += uint64(s.addFreeTime)
		}
		if s.scene.FreeNum < 1 {
			s.scene.NextStage = _spinTypeBase
			s.scene.FreeWinMultiple = 1
		}
		return
	}

	s.scene.NextStage = _spinTypeBase
	s.scene.FreeWinMultiple = 1
	if s.scatterCount >= s.gameConfig.FreeGameScatter {
		s.scene.NextStage = _spinTypeFree
		addFreeTimes := uint64((s.scatterCount-s.gameConfig.FreeGameScatter)*s.gameConfig.AddFreeTimes + s.gameConfig.FreeTimes)
		s.client.SetMaxFreeNum(addFreeTimes)
		s.scene.FreeNum = addFreeTimes
		s.addFreeTime = int64(addFreeTimes)
	}
}
