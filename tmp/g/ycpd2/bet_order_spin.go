package ycpd

import "github.com/shopspring/decimal"

func (s *betOrderService) baseSpin() error {
	if err := s.initialize(); err != nil {
		return err
	}

	s.scatterCount = 0
	s.addFreeTime = 0
	s.isSpinOver = false
	s.isRoundOver = false
	s.stepMultiplier = 0
	s.winInfos = nil

	if s.scene.Steps == 0 {
		s.scene.SymbolRoller = s.initSpinSymbol(s.scene.Stage)
	}

	s.scene.Steps++
	s.handleSymbolGrid()
	winInfos := s.checkSymbolGridWin()

	if len(winInfos) > 0 {
		spinLimit := false
		s.isRoundOver = false
		stepMultiplierMulCombo := s.lineMultiplier * s.gameMultiple
		s.stepMultiplier = stepMultiplierMulCombo

		maxWin := s.betAmount.Mul(decimal.NewFromInt(s.gameConfig.MaxWinMultiplier))
		currWin := s.calcBonusAmount(stepMultiplierMulCombo)
		lastSpinWin := s.scene.TotalWin

		var bonusAmount decimal.Decimal
		if currWin.Add(decimal.NewFromFloat(lastSpinWin)).GreaterThanOrEqual(maxWin) {
			s.isSpinOver = true
			if s.isFreeRound {
				s.scene.FreeNum = 0
			}
			bonusAmount = maxWin.Sub(decimal.NewFromFloat(lastSpinWin))
			spinLimit = true
		}
		if bonusAmount.GreaterThan(decimal.Zero) {
			s.bonusAmount = bonusAmount
		} else {
			bonusAmount = s.updateBonusAmount(stepMultiplierMulCombo)
		}
		s.updateSpinBonusAmount(bonusAmount)

		if s.debug.open && spinLimit {
			s.stepMultiplier = int64(s.bonusAmount.Round(2).InexactFloat64())
		}

		moveSymbolGrid := s.moveSymbols()
		s.fallingWinSymbols(moveSymbolGrid)

		s.winInfos = winInfos
		s.scene.NextStage = _spinTypeBaseEli
		if s.isFreeRound {
			s.scene.NextStage = _spinTypeFreeEli
		}
		if spinLimit {
			s.scene.Steps = 0
			s.isRoundOver = true
			s.scene.NextStage = _spinTypeBase
		}
	} else {
		s.scene.Steps = 0
		s.scene.NextStage = _spinTypeBase
		if s.isFreeRound {
			s.scene.NextStage = _spinTypeFree
		}

		s.isRoundOver = true
		scatterCount := s.getScatterCount()
		s.scatterCount = scatterCount

		if s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeBaseEli {
			if scatterCount >= s.gameConfig.FreeGameScatter {
				s.scene.NextStage = _spinTypeFree
				addFreeTimes := s.gameConfig.FreeGameTimes + (scatterCount-s.gameConfig.FreeGameScatter)*s.gameConfig.AddFreeTimes
				s.scene.FreeTimes = 0
				s.scene.FreeNum = addFreeTimes
				s.scene.TotalFree += addFreeTimes
				s.scene.GameMultiple = 0
				s.scene.RemoveMultiple = [_colCount]int64{}
			} else {
				s.isSpinOver = true
			}
		} else {
			if scatterCount >= s.gameConfig.FreeGameScatter {
				addFreeTimes := s.gameConfig.FreeGameTimes + (scatterCount-s.gameConfig.FreeGameScatter)*s.gameConfig.AddFreeTimes
				s.addFreeTime = addFreeTimes
				s.scene.FreeNum += addFreeTimes
				s.scene.TotalFree += addFreeTimes
			}
			if s.scene.FreeNum < 1 {
				s.scene.NextStage = _spinTypeBase
				s.isSpinOver = true
			}
		}
	}
	return nil
}
