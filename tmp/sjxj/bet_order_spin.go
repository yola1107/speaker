package sjxj

import "math/rand/v2"

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}
	if s.isFreeRound && s.scene.FreeNum > 0 {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
	}
	s.scene.SymbolRoller = s.initSpinSymbol()
	s.handleSymbolGrid()
	s.processWinInfos()
	return nil
}

func (s *betOrderService) processWinInfos() {
	s.addFreeTime = 0
	s.isRoundOver = true
	s.scatterCount = s.getScatterCount()

	if s.isFreeRound {
		s.winGrid = int64Grid{}
		s.winInfos = []WinInfo{}

		s.tryUnlockNextRow()
		isFullScatter, freeGameMul, newScatterCount := s.calcCurrentFreeGameMul()
		s.scatterCount = newScatterCount

		if !isFullScatter && s.scene.UnlockedRows > s.scene.PrevUnlockedRows {
			if resetNum := s.gameConfig.FreeUnlockResetSpins; s.scene.FreeNum < resetNum {
				add := resetNum - s.scene.FreeNum
				s.scene.FreeNum = resetNum
				s.client.ClientOfFreeGame.Incr(uint64(add))
				s.addFreeTime = add
			}
		}
		if s.scene.FreeNum <= 0 || isFullScatter {
			s.stepMultiplier = freeGameMul
			s.scene.FreeNum = 0
			s.client.ClientOfFreeGame.SetFreeNum(0)
			s.scene.NextStage = _spinTypeBase
		} else {
			s.stepMultiplier = 0
			s.scene.NextStage = _spinTypeFree
		}

	} else {
		s.checkSymbolGridWin()
		s.stepMultiplier = 0
		for _, elem := range s.winInfos {
			s.stepMultiplier += elem.Odds
		}

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

	s.updateBonusAmount(s.stepMultiplier)
}

func (s *betOrderService) lockScatter() {
	nextLock := [_rowCount][_colCount]int64{}
	cfg := s.gameConfig.FreeScatterMulByRow
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.symbolGrid[r][c] == _treasure {
				mul := s.scene.ScatterLock[r][c]
				if mul == 0 {
					mul = cfg[r][rand.IntN(len(cfg[r]))]
				}
				nextLock[r][c] = mul
			}
		}
	}
	s.scene.ScatterLock = nextLock
}

func (s *betOrderService) tryUnlockNextRow() {
	s.scene.PrevUnlockedRows = s.scene.UnlockedRows
	if s.scene.UnlockedRows >= _rowCount {
		return
	}
	for s.scene.UnlockedRows < _rowCount {
		if s.scatterCount < s.gameConfig.FreeUnlockThresholds[s.scene.UnlockedRows] {
			break
		}
		s.scene.UnlockedRows++
	}
}
