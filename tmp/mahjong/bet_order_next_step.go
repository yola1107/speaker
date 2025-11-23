package mahjong

import (
	"github.com/shopspring/decimal"
)

// 初始化spin后续step
func (s *betOrderService) initStepForNextStep() {
	if !s.forRtpBench {
		s.req.BaseMoney = s.lastOrder.BaseAmount
		s.req.Multiple = s.lastOrder.Multiple
	} else {
		s.req.BaseMoney = 1
		s.req.Multiple = 1
	}

	s.betAmount = decimal.NewFromFloat(s.client.ClientOfFreeGame.GetBetAmount())
	s.amount = decimal.Zero

	if s.forRtpBench {
		return
	}

	if s.scene.IsFreeRound {
		switch {
		case s.lastOrder.FreeOrderSn != "":
			s.freeOrderSN = s.lastOrder.FreeOrderSn
		case s.lastOrder.ParentOrderSn != "":
			s.freeOrderSN = s.lastOrder.ParentOrderSn
		default:
			s.freeOrderSN = s.lastOrder.OrderSn
		}
	} else {
		if s.lastOrder.ParentOrderSn != "" {
			s.parentOrderSN = s.lastOrder.ParentOrderSn
		} else {
			s.parentOrderSN = s.lastOrder.OrderSn
		}
	}

}
