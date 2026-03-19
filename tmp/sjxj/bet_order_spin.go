package sjxj

import "math/rand/v2"

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
	s.processWinInfos()
	return nil
}

func (s *betOrderService) processWinInfos() {
	// 每局重置：避免上一局 addFreeTime 残留影响统计/日志
	s.addFreeTime = 0
	s.isRoundOver = true
	s.scatterCount = s.getScatterCount()

	if s.isFreeRound {
		s.winGrid = int64Grid{}
		s.winInfos = []WinInfo{}

		s.tryUnlockNextRow()
		isFullScatter, freeGameMul, newScatterCount := s.calcCurrentFreeGameMul()
		s.scatterCount = newScatterCount // 更新最新解锁后的 Scatter 数

		if !isFullScatter && s.scene.UnlockedRows > s.scene.PrevUnlockedRows {
			// 解锁后将“剩余免费次数”补到至少 free_game_times（默认3），而非叠加。
			if resetNum := s.gameConfig.FreeUnlockResetSpins; s.scene.FreeNum < resetNum {
				add := resetNum - s.scene.FreeNum
				s.scene.FreeNum = resetNum
				s.client.ClientOfFreeGame.Incr(uint64(add))
				s.addFreeTime = add
			}
		}
		if s.scene.FreeNum <= 0 || isFullScatter {
			s.stepMultiplier = freeGameMul // 设置倍数结算
			s.scene.FreeNum = 0
			s.client.ClientOfFreeGame.SetFreeNum(0)
			s.scene.NextStage = _spinTypeBase
		} else {
			s.stepMultiplier = 0
			s.scene.NextStage = _spinTypeFree
		}

	} else {
		s.checkSymbolGridWin()
		s.stepMultiplier = s.handleWinElemsMultiplier(s.winInfos)

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

	s.updateBonusAmount(s.stepMultiplier)
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

// lockScatter 锁定整个盘面的夺宝并分配固定倍数
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

// tryUnlockNextRow 根据已解锁区 Scatter 数，按阈值逐行推进 UnlockedRows++。
func (s *betOrderService) tryUnlockNextRow() {
	if s.scene.UnlockedRows >= _rowCount {
		s.scene.PrevUnlockedRows = s.scene.UnlockedRows
		return
	}

	s.scene.PrevUnlockedRows = s.scene.UnlockedRows
	currScatter := s.scatterCount

	// free_unlock_thresholds 按 UnlockedRows 索引（推荐长度8）。
	for s.scene.UnlockedRows < _rowCount {
		threshold := s.gameConfig.FreeUnlockThresholds[s.scene.UnlockedRows]
		if currScatter < threshold {
			break
		}
		s.scene.UnlockedRows++
	}
}
