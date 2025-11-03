package xslm2

import (
	"github.com/shopspring/decimal"
)

func (s *betOrderService) initStepForNextStep() error {
	s.req.BaseMoney = s.lastOrder.BaseAmount
	s.req.Multiple = s.lastOrder.Multiple
	s.betAmount = decimal.NewFromFloat(s.client.ClientOfFreeGame.GetBetAmount())
	s.amount = decimal.Zero
	switch {
	case s.client.IsRoundOver:
		s.isFreeRound = true
		s.client.ClientOfFreeGame.ResetRoundBonus()
		switch {
		case s.lastOrder.FreeOrderSn != "":
			s.freeOrderSN = s.lastOrder.FreeOrderSn
		case s.lastOrder.ParentOrderSn != "":
			s.freeOrderSN = s.lastOrder.ParentOrderSn
		default:
			s.freeOrderSN = s.lastOrder.OrderSn
		}
	default:
		s.isFreeRound = s.lastOrder.IsFree > 0
		switch {
		case s.lastOrder.ParentOrderSn != "":
			s.parentOrderSN = s.lastOrder.ParentOrderSn
		default:
			s.parentOrderSN = s.lastOrder.OrderSn
		}
		s.freeOrderSN = s.lastOrder.FreeOrderSn
	}
	return nil
}

func (s *betOrderService) initPresetForNextStep() bool {
	lastPresetID := s.client.ClientOfFreeGame.GetLastWinId()
	return s.rdbGetPresetByID(int64(lastPresetID))
}
