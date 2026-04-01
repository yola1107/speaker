package yhwy

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}

	if s.isFreeRound {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
	}

	s.initSpinSymbol()
	s.handleSymbolGrid()
	s.processGame()
	return nil
}

func (s *betOrderService) processGame() {
	s.addFreeTime = 0
	s.lineMultiplier = 0
	s.scatterCount = 0
	s.revealSymbol = _blank
	s.spreadToReel = 1
	s.isSakuraReset = false
	s.resetDirection = _resetDirectionNone
	s.mysteryGrid = cloneGrid(s.originGrid)
	s.finalGrid = cloneGrid(s.originGrid)
	s.winGrid = int64Grid{}
	s.winInfos = nil

	if s.isFreeRound {
		for r := 0; r < _rowCount; r++ {
			s.mysteryGrid[r][0] = _Mystery
		}
	}

	s.trySakuraReset()
	s.trySakuraSpread()
	s.applyMysteryReveal()
	s.checkSymbolGridWin()
	s.scatterCount = s.getScatterCount()
	s.processWinInfos()
}

func (s *betOrderService) trySakuraReset() {
	if s.isFreeRound {
		return
	}

	topThree := s.mysteryGrid[0][0] == _Mystery && s.mysteryGrid[1][0] == _Mystery && s.mysteryGrid[2][0] == _Mystery
	bottomThree := s.mysteryGrid[1][0] == _Mystery && s.mysteryGrid[2][0] == _Mystery && s.mysteryGrid[3][0] == _Mystery
	if !topThree && !bottomThree {
		return
	}

	s.isSakuraReset = true
	if topThree && !bottomThree {
		s.resetDirection = _resetDirectionDown
	} else {
		s.resetDirection = _resetDirectionUp
	}
	for r := 0; r < _rowCount; r++ {
		s.mysteryGrid[r][0] = _Mystery
	}
}

func (s *betOrderService) trySakuraSpread() {
	for r := 0; r < _rowCount; r++ {
		if s.mysteryGrid[r][0] != _Mystery {
			return
		}
	}

	s.spreadToReel = s.pickSpreadToReel()
	for c := 1; c < int(s.spreadToReel); c++ {
		for r := 0; r < _rowCount; r++ {
			s.mysteryGrid[r][c] = _Mystery
		}
	}
}

func (s *betOrderService) applyMysteryReveal() {
	s.finalGrid = cloneGrid(s.mysteryGrid)
	if !s.hasMysteryOnBoard() {
		return
	}

	s.revealSymbol = s.pickRevealSymbol()
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.finalGrid[r][c] == _Mystery {
				s.finalGrid[r][c] = s.revealSymbol
			}
		}
	}
}

func (s *betOrderService) hasMysteryOnBoard() bool {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.mysteryGrid[r][c] == _Mystery {
				return true
			}
		}
	}
	return false
}

func (s *betOrderService) processWinInfos() {
	s.stepMultiplier = s.lineMultiplier
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
		s.isRoundOver = true
	} else {
		if s.addFreeTime > 0 {
			s.scene.NextStage = _spinTypeFree
			s.isRoundOver = false
		} else {
			s.scene.NextStage = _spinTypeBase
			s.isRoundOver = true
		}
	}

	s.updateBonusAmount(s.stepMultiplier)
}
