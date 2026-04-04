// game/bxkh2/bet_order_spin.go
package bxkh2

import (
	"time"

	"egame-grpc/game/common"
	"egame-grpc/gamelogic"

	"github.com/shopspring/decimal"
)

func (s *betOrderService) baseSpin() error {
	if err := s.initialize(); err != nil {
		return err
	}

	if s.isFreeRound() {
		if s.isRoundFirst {
			s.client.ClientOfFreeGame.IncrFreeTimes()
			s.client.ClientOfFreeGame.Decr()
		}
	}

	// 判断是否为 round 的第一个 step
	if s.isRoundFirst {
		s.createMatrix(s.scene.Stage)
		s.isRoundFirst = false
		if s.isBaseRound() {
			s.freeMultiple = 1
			s.removeNum = 0
		}
	} else {
		s.symbolGrid = s.buildSymbolGrid()
		s.buildTailArrays()
	}

	s.scene.Steps++
	winInfos := s.checkSymbolGridWin()

	// 反转网格用于返回
	s.symbolGrid = s.reverseGrid(s.symbolGrid)
	s.winGrid = s.reverseGrid(s.winGrid)

	if len(winInfos) > 0 {
		// 有中奖
		s.scene.RoundOver = false
		lineMultiplier := s.handleWinInfosMultiplier(winInfos)
		stepMultiplier := lineMultiplier * s.freeMultiple

		s.scene.RoundMultiplier += stepMultiplier
		s.scene.SpinMultiplier += stepMultiplier
		if s.scene.IsFreeRound {
			s.scene.FreeMultiplier += stepMultiplier
		}

		s.updateBonusAmount(stepMultiplier)
		s.moveSymbolGrid = s.moveSymbols(s.scene.Stage)
		s.fallingSymbols()

		if s.isFreeRound() {
			s.removeNum++
			s.freeMultiple = s.removeNum + 1
		}

		s.scene.NextStage = _spinTypeBaseEli
		if s.isFreeRound() {
			s.scene.NextStage = _spinTypeFreeEli
		}
	} else {
		// 无中奖
		s.scene.Steps = 0
		s.scene.RoundOver = true
		s.scene.NextStage = _spinTypeBase
		if s.isFreeRound() {
			s.scene.NextStage = _spinTypeFree
		}

		if s.isBaseRound() {
			s.freeMultiple = 1
			s.removeNum = 0
		}
		s.moveSymbolGrid = s.symbolGrid
		s.scatterCount = s.getScatterCount(s.symbolGrid)

		if s.isBaseRound() {
			if s.scatterCount >= s.gameConfig.FreeGameScatter {
				s.scene.NextStage = _spinTypeFree
				addFreeTimes := uint64((s.scatterCount-s.gameConfig.FreeGameScatter)*s.gameConfig.AddFreeTimes + s.gameConfig.FreeTimes)
				s.client.SetMaxFreeNum(addFreeTimes)
				s.client.ClientOfFreeGame.SetFreeNum(addFreeTimes)
				s.addFreeTime = int64(addFreeTimes)
				s.freeMultiple = 1
			} else {
				s.scene.IsFreeRound = false
			}
		} else {
			if s.scatterCount >= s.gameConfig.FreeGameScatter {
				addFreeTimes := (s.scatterCount-s.gameConfig.FreeGameScatter)*s.gameConfig.AddFreeTimes + s.gameConfig.FreeTimes
				s.addFreeTime = addFreeTimes
				s.client.ClientOfFreeGame.Incr(uint64(s.addFreeTime))
			}
			if s.client.ClientOfFreeGame.GetFreeNum() < 1 {
				s.scene.NextStage = _spinTypeBase
				s.freeMultiple = 1
				s.removeNum = 0
			} else {
				s.scene.IsFreeRound = true
			}
		}
	}

	s.scene.RemoveNum = s.removeNum
	s.scene.FreeWinMultiple = s.freeMultiple
	return nil
}

func (s *betOrderService) initialize() error {
	s.client.ClientOfFreeGame.ResetFreeClean()

	if !s.debug.open {
		s.orderSn = common.GenerateOrderSn(s.member, s.lastOrder, s.isRoundFirst, s.scene.IsFreeRound)
	}

	if s.scene == nil || s.scene.Stage == _spinTypeBase || s.scene.Stage == 0 {
		if err := s.initFirstStep(); err != nil {
			return err
		}
		s.scene.SpinMultiplier = 0
		s.scene.FreeMultiplier = 0
	} else {
		s.initNextStep()
	}

	if s.scene.Steps == 0 {
		s.isRoundFirst = true
	}
	if s.isRoundFirst {
		s.scene.RoundMultiplier = 0
		s.client.ClientOfFreeGame.ResetRoundBonus()
		s.client.ClientOfFreeGame.ResetRoundBonusStaging()
	}
	s.originalSymbolGrid = int64Grid{}

	return nil
}

func (s *betOrderService) initFirstStep() error {
	if !s.debug.open {
		if !s.updateBetAmount() {
			return InvalidRequestParams
		}
		if !s.checkBalance() {
			return InsufficientBalance
		}
	}
	s.client.SetLastMaxFreeNum(0)
	s.client.ClientOfFreeGame.Reset()
	s.client.ClientOfFreeGame.ResetGeneralWinTotal()
	s.client.ClientOfFreeGame.ResetRoundBonus()
	s.client.ClientOfFreeGame.SetBetAmount(s.betAmount.Round(2).InexactFloat64())
	s.amount = s.betAmount
	s.client.ClientOfFreeGame.SetLastWinId(uint64(time.Now().UnixNano()))
	return nil
}

func (s *betOrderService) initNextStep() {
	if !s.debug.open {
		s.req.BaseMoney = s.lastOrder.BaseAmount
		s.req.Multiple = s.lastOrder.Multiple
	} else {
		s.req.BaseMoney = 1
		s.req.Multiple = 1
	}
	s.betAmount = decimal.NewFromFloat(s.client.ClientOfFreeGame.GetBetAmount())
	s.amount = decimal.Zero
}

func (s *betOrderService) updateBetAmount() bool {
	s.betAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_baseMultiplier))
	return s.betAmount.GreaterThan(decimal.Zero)
}

func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

func (s *betOrderService) updateBonusAmount(stepMultiplier int64) {
	if s.debug.open || stepMultiplier == 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	s.bonusAmount = s.betAmount.
		Mul(decimal.NewFromInt(stepMultiplier)).
		Div(decimal.NewFromInt(_baseMultiplier))

	if s.bonusAmount.GreaterThan(decimal.Zero) {
		rounded := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(rounded)
		s.client.ClientOfFreeGame.IncRoundBonus(rounded)
		if s.isFreeRound() {
			s.client.ClientOfFreeGame.IncrFreeTotalMoney(rounded)
		}
	}
}

func (s *betOrderService) isBaseRound() bool {
	return s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeBaseEli
}

func (s *betOrderService) isFreeRound() bool {
	return s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli
}

func (s *betOrderService) reverseGrid(grid int64Grid) int64Grid {
	for i := 0; i < len(grid)/2; i++ {
		j := len(grid) - 1 - i
		grid[i], grid[j] = grid[j], grid[i]
	}
	return grid
}
