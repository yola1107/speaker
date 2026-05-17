package tmtg

func (s *betOrderService) baseSpin() error {
	if err := s.initialize(); err != nil {
		return err
	}
	if s.isFreeRound && s.scene.FreeNum > 0 && s.scene.Stage == _spinTypeFree {
		s.scene.FreeTimes++
		s.scene.FreeNum--
	}
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.initSpinSymbol()
	}
	s.handleSymbolGrid()
	s.findWinInfos()
	s.processWinInfos()
	return nil
}

func (s *betOrderService) processWinInfos() {
	s.addFreeTime = 0
	s.scatterCount = s.counter[_treasure]
	s.isPurchase = s.scene.PurchaseAmount > 0
	s.lineMultiplier = 0
	for _, w := range s.winInfos {
		s.lineMultiplier += w.Odds
	}
	s.scene.RoundMulAcc += s.lineMultiplier
	s.stepMultiplier = 0

	var haveEli = len(s.winInfos) > 0
	var wildKeep bool
	//for _, w := range s.winInfos {
	//	if w.Symbol < _treasure {
	//
	//		if w.Count < _minMatchCount {
	//			wildKeep = true // wild 不消除
	//			break
	//		}
	//	}
	//}

	if !s.isFreeRound {
		s.processBase(haveEli, wildKeep)
	} else {
		s.processFree(haveEli, wildKeep)
	}
	s.calcWin()
	s.applyMaxWinLimit()
	s.updateBonusAmount()
}

func (s *betOrderService) processBase(haveEli, wildKeep bool) {
	if haveEli {
		s.scene.Steps++
		s.isRoundOver = false
		s.scene.NextStage = _spinTypeBaseEli
		s.nextSymbolGrid = s.moveSymbols(wildKeep)
		s.fallingWinSymbols()

	} else {
		s.scene.Steps = 0
		s.isRoundOver = true
		if newFree := s.gameConfig.calcNewFreeGameNum(s.isFreeRound, s.scatterCount); newFree > 0 {
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
		}
		if s.addFreeTime > 0 {
			s.scene.NextStage = _spinTypeFree
		} else {
			s.scene.NextStage = _spinTypeBase
		}
	}
}

func (s *betOrderService) processFree(haveEli, wildKeep bool) {
	var haveWild = s.counter[_wild] > 0

	switch {
	case haveWild && !haveEli:
		s.scene.Steps++
		s.isRoundOver = false
		s.scene.NextStage = _spinTypeFreeBombEli
		s.nextSymbolGrid = s.eliBombSymbols()
		s.fallingWinSymbols()

	case haveEli:
		s.scene.Steps++
		s.isRoundOver = false
		s.scene.NextStage = _spinTypeFreeEli
		s.nextSymbolGrid = s.moveSymbols(wildKeep)
		s.fallingWinSymbols()

	default:
		s.scene.Steps = 0
		s.isRoundOver = true
		if newFree := s.gameConfig.calcNewFreeGameNum(s.isFreeRound, s.scatterCount); newFree > 0 {
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
		}
		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
		} else {
			s.scene.NextStage = _spinTypeBase
		}
	}
}

// calcWin 整转 tumble 内只累计线赔率，局末一次结算；免费局且盘面有 bomb 时 (spin_p+spin_w)*sum(m)。
func (s *betOrderService) calcWin() {
	if !s.isRoundOver {
		return
	}
	mul := s.scene.RoundMulAcc
	s.scene.RoundMulAcc = 0

	if s.isFreeRound && mul > 0 {
		// 统计盘面 bomb 格上的倍率
		var sum int64
		for r := 0; r < _rowCount; r++ {
			for c := 0; c < _colCount; c++ {
				if s.symbolGrid[r][c] == _bomb {
					sum += s.scene.BombMulGrid[r][c]
				}
			}
		}
		if sum > 0 {
			mul *= sum
		}
	} else if !s.isFreeRound && s.scatterCount >= _scatterEntryMin {
		mul += s.gameConfig.scatterPayMultiplier(s.scatterCount)
	}
	s.stepMultiplier = mul
}
