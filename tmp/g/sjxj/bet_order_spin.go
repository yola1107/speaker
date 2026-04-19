package sjxj

import "egame-grpc/game/common/rand"

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}
	if s.isFreeRound {
		if s.scene.FreeNum > 0 {
			s.scene.FreeTimes++
			s.scene.FreeNum--
		}
		// 基础进入免费首局：填符号前按已锁定夺宝推进 UnlockedRows（阶段1）
		if s.scene.BaseEnterFreeFirstStep {
			s.unlockByLockedScatter()
			s.scene.BaseEnterFreeFirstStep = false
		}
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
			resetNum := s.gameConfig.FreeUnlockResetSpins
			if cur := s.scene.FreeNum; cur < resetNum {
				s.scene.FreeNum = resetNum
				s.addFreeTime = resetNum - cur
			}
		}
		if s.scene.FreeNum <= 0 || isFullScatter {
			s.stepMultiplier = freeGameMul
			s.scene.FreeNum = 0
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
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
		}
		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
			s.scene.UnlockedRows = _rowCountReward
			s.scene.PrevUnlockedRows = _rowCountReward
			s.scene.BaseEnterFreeFirstStep = true
			s.lockScatter()
		} else {
			s.scene.NextStage = _spinTypeBase
		}
	}

	s.updateBonusAmount(s.stepMultiplier)
}

func (s *betOrderService) lockScatter() {
	cfg := s.gameConfig.FreeScatterMulByRow
	s.scene.ScatterLock = int64Grid{}
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.symbolGrid[r][c] == _treasure {
				s.scene.ScatterLock[r][c] = cfg[r][rand.IntN(len(cfg[r]))]
			}
		}
	}
}

// unlockByLockedScatter 基础进入免费首局：按 ScatterLock 中已锁定夺宝数量推进 UnlockedRows。
// 不修改 PrevUnlockedRows，后续 tryUnlockNextRow 会以阶段1结果为 Prev 基准。
func (s *betOrderService) unlockByLockedScatter() {
	for s.scene.UnlockedRows < _rowCount {
		startRow := _rowCount - s.scene.UnlockedRows
		var count int64
		for r := startRow; r < _rowCount; r++ {
			for c := 0; c < _colCount; c++ {
				if s.scene.ScatterLock[r][c] != 0 {
					count++
				}
			}
		}
		if count < s.gameConfig.FreeUnlockThresholds[s.scene.UnlockedRows] {
			break
		}
		s.scene.UnlockedRows++
	}
}

func (s *betOrderService) tryUnlockNextRow() {
	s.scene.PrevUnlockedRows = s.scene.UnlockedRows
	for s.scene.UnlockedRows < _rowCount {
		s.scatterCount = s.getScatterCount()
		if s.scatterCount < s.gameConfig.FreeUnlockThresholds[s.scene.UnlockedRows] {
			break
		}
		s.scene.UnlockedRows++
	}
}
