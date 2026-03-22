package hcsqy

import "math/rand/v2"

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}
	if s.isFreeRound && s.scene.FreeNum > 0 && !s.scene.IsMustWin {
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
	s.mustWinCol = -1
	s.wildExpandCol = -1
	s.wildMultiplier = 1
	s.lineMultiplier = 0
	s.scatterCount = s.getScatterCount()

	// 必赢模式：正在重转中 或 概率触发新必赢
	if s.scene.IsMustWin || isHitFloat(s.gameConfig.MustWinProb) {
		s.processMustWin()
		return
	}

	// 百搭变大：先找候选列，再5%判定
	s.processWildExpand()
}

func (s *betOrderService) processMustWin() {
	s.next = true
	s.scene.IsMustWin = true
	s.wildMultiplier = s.pickWildMultiplier()
	col := rand.IntN(_colCount)
	for row := 0; row < _rowCount; row++ {
		s.symbolGrid[row][col] = _wild
	}
	s.mustWinCol = int8(col)
	s.scatterCount = s.getScatterCount()

	s.checkSymbolGridWin()
	if len(s.winInfos) > 0 {
		s.scene.IsMustWin = false
		s.next = false
	}
	s.processWinInfos()
}

func (s *betOrderService) processWildExpand() {
	var candidates []int
	for col := 0; col < _colCount; col++ {
		wildCount, hasTreasure := 0, false
		for row := 0; row < _rowCount; row++ {
			switch s.symbolGrid[row][col] {
			case _wild:
				wildCount++
			case _treasure:
				hasTreasure = true
			}
		}
		if wildCount >= 1 && wildCount <= 2 && !hasTreasure {
			candidates = append(candidates, col)
		}
	}

	if len(candidates) > 0 && isHitFloat(s.gameConfig.WildExpandProb) {
		col := candidates[rand.IntN(len(candidates))]
		s.wildMultiplier = s.pickWildMultiplier()
		s.wildExpandCol = int8(col)
		for row := 0; row < _rowCount; row++ {
			s.symbolGrid[row][col] = _wild
		}
	}

	s.checkSymbolGridWin()
	s.processWinInfos()
}

func (s *betOrderService) processWinInfos() {
	s.stepMultiplier = s.lineMultiplier * s.wildMultiplier

	if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
		s.client.ClientOfFreeGame.Incr(uint64(newFree))
		s.scene.FreeNum += newFree
		s.addFreeTime = newFree
	}

	// 设置下一阶段：有免费次数则免费模式，否则基础模式
	if s.scene.FreeNum > 0 {
		if s.scene.IsPurchase || s.client.ClientOfFreeGame.GetPurchaseAmount() > 0 {
			s.scene.IsPurchase = true
			s.scene.NextStage = _spinTypeBuyFree
		} else {
			s.scene.NextStage = _spinTypeFree
		}
	} else {
		s.scene.FreeNum = 0
		if s.scene.IsPurchase || s.client.ClientOfFreeGame.GetPurchaseAmount() > 0 {
			s.scene.IsPurchase = false
			s.client.ClientOfFreeGame.SetPurchaseAmount(0)
			s.scene.NextStage = _spinTypeBase
		} else {
			s.scene.NextStage = _spinTypeBase
		}
	}

	// 必赢模式中或触发免费时，回合未结束
	if s.scene.IsMustWin || s.addFreeTime > 0 {
		s.isRoundOver = false
	} else {
		s.isRoundOver = true
	}

	s.updateBonusAmount(s.stepMultiplier)
}
