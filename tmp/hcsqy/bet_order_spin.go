package hcsqy

import "math/rand/v2"

//import "egame-grpc/game/common/rand"

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}
	if s.isFreeRound && s.scene.FreeNum > 0 && !s.scene.IsRespinMode {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
	}
	s.scene.SymbolRoller = s.initSpinSymbol()
	s.handleSymbolGrid()
	s.processGame()
	return nil
}

func (s *betOrderService) processGame() {
	s.addFreeTime = 0
	s.respinWildCol = -1
	s.wildExpandCol = -1
	s.wildMultiplier = 1
	s.lineMultiplier = 0
	// 夺宝数见 processWinInfos：在重转盖列、百搭变大之后按最终盘面统计

	// 重转至赢模式：正在重转中 或 概率触发新重转
	if s.scene.IsRespinMode || s.isHitRespinProb() {
		s.processRespinUntilWin()
		return
	}

	// 百搭变大：先找候选列，再概率判定
	s.processWildExpand()
}

func (s *betOrderService) processRespinUntilWin() {
	s.next = true
	s.scene.IsRespinMode = true
	s.wildMultiplier = s.weightWildMultiplier()
	c := rand.IntN(_colCount)
	for r := 0; r < _rowCount; r++ {
		s.symbolGrid[r][c] = _wild
	}
	s.respinWildCol = int32(c)

	s.checkSymbolGridWin()
	if len(s.winInfos) > 0 {
		s.scene.IsRespinMode = false
		s.next = false
	}
	s.processWinInfos()
}

func (s *betOrderService) processWildExpand() {
	var candidates []int
	for c := 0; c < _colCount; c++ {
		wildCount, hasTreasure := 0, false
		for r := 0; r < _rowCount; r++ {
			switch s.symbolGrid[r][c] {
			case _wild:
				wildCount++
			case _treasure:
				hasTreasure = true
			}
		}
		if wildCount >= 1 && wildCount <= 2 && !hasTreasure {
			candidates = append(candidates, c)
		}
	}

	if len(candidates) > 0 && s.isHitWildExpandProb() {
		s.wildMultiplier = s.weightWildMultiplier()
		c := candidates[rand.IntN(len(candidates))]
		for r := 0; r < _rowCount; r++ {
			s.symbolGrid[r][c] = _wild
		}
		s.wildExpandCol = int32(c)
	}

	s.checkSymbolGridWin()
	s.processWinInfos()
}

func (s *betOrderService) processWinInfos() {
	// 本步 symbolGrid 已定型（自然停轮 + 重转盖列 / 百搭变大），与结算、回包、订单夺宝数同一口径
	s.scatterCount = s.getScatterCount()
	s.stepMultiplier = s.lineMultiplier * s.wildMultiplier

	if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
		s.client.ClientOfFreeGame.Incr(uint64(newFree))
		s.scene.FreeNum += newFree
		s.addFreeTime = newFree
	}

	isPurchaseMode := s.scene.IsPurchase || s.client.ClientOfFreeGame.GetPurchaseAmount() > 0
	if s.scene.FreeNum <= 0 {
		s.scene.FreeNum = 0
		s.scene.NextStage = _spinTypeBase
		if isPurchaseMode {
			s.scene.IsPurchase = false
			s.client.ClientOfFreeGame.SetPurchaseAmount(0)
		}
	} else if isPurchaseMode {
		s.scene.IsPurchase = true
		s.scene.NextStage = _spinTypeBuyFree
	} else {
		s.scene.NextStage = _spinTypeFree
	}

	// 重转至赢模式中或触发免费时，回合未结束
	s.isRoundOver = !(s.scene.IsRespinMode || s.addFreeTime > 0)

	s.updateBonusAmount(s.stepMultiplier)
}
