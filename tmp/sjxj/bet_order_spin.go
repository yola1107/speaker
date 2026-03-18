package sjxj

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}
	// 免费模式：每局消耗 1 次免费次数
	if s.isFreeRound && s.scene.FreeNum > 0 {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
	}
	s.scene.SymbolRoller = s.initSpinSymbol()
	s.handleSymbolGrid()
	s.checkSymbolGridWin()
	s.processWinInfos()
	return nil
}

func (s *betOrderService) processWinInfos() {
	s.isRoundOver = true
	s.lineMultiplier = s.handleWinElemsMultiplier(s.winInfos)
	s.scatterCount = s.getScatterCount()

	if len(s.winInfos) == 0 {
		s.client.ClientOfFreeGame.SetLastWinId(0)
	}

	if s.isFreeRound {
		s.tryUnlockNextRow()

		if newRow := s.scene.UnlockedRows - s.scene.PrevUnlockedRows; newRow > 0 {
			newFree := int64(newRow) * s.gameConfig.FreeUnlockAddSpins
			s.client.ClientOfFreeGame.Incr(uint64(newFree))
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
		}

		if s.scene.FreeNum <= 0 {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
			s.clearFreeScatterLock()
		} else {
			s.scene.NextStage = _spinTypeFree
			s.lockScatter()
		}
	} else {
		// 基础模式 触发免费
		if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
			s.client.ClientOfFreeGame.Incr(uint64(newFree))
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
		}
		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
			s.scene.UnlockedRows = _rowCountReward
			s.scene.PrevUnlockedRows = _rowCountReward
			s.lockScatter()

		} else {
			s.scene.NextStage = _spinTypeBase
		}
	}
	s.updateBonusAmount(s.lineMultiplier)
}

func (s *betOrderService) handleWinElemsMultiplier(elems []WinInfo) int64 {
	if len(elems) == 0 {
		return 0
	}
	var mul int64
	for _, elem := range elems {
		mul += elem.Odds
	}
	return mul
}

// 锁定整个盘面的夺宝
func (s *betOrderService) lockScatter() {
	s.scene.ScatterLock = [_rowCount][_colCount]int64{}
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.symbolGrid[r][c] == _treasure {
				s.scene.ScatterLock[r][c] = 1
			}
		}
	}
}

func (s *betOrderService) clearFreeScatterLock() {
	s.scene.ScatterLock = [_rowCount][_colCount]int64{}
	s.scene.UnlockedRows = _rowCountReward
	s.scene.PrevUnlockedRows = _rowCountReward
}

// tryUnlockNextRow 根据已解锁区 Scatter 数，按阈值逐行推进 UnlockedRows++。
func (s *betOrderService) tryUnlockNextRow() {
	if s.scene.UnlockedRows >= _rowCount {
		s.scene.PrevUnlockedRows = s.scene.UnlockedRows
		return
	}

	s.scene.PrevUnlockedRows = s.scene.UnlockedRows
	currScatter := s.scatterCount

	for i := s.scene.UnlockedRows - _rowCountReward; i < len(s.gameConfig.FreeUnlockThresholds); i++ {
		if currScatter >= s.gameConfig.FreeUnlockThresholds[i] {
			s.scene.UnlockedRows++
		} else {
			break
		}
	}
}
