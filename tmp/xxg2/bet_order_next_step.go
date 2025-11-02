package xxg2

import (
	"github.com/shopspring/decimal"
)

// 初始化后续 step
func (s *betOrderService) initStepForNextStep() {
	// 恢复下注配置
	if s.debug.open {
		s.req.BaseMoney = 1
		s.req.Multiple = 1
	} else {
		s.req.BaseMoney = s.lastOrder.BaseAmount
		s.req.Multiple = s.lastOrder.Multiple
	}

	s.betAmount = decimal.NewFromFloat(s.client.ClientOfFreeGame.GetBetAmount())
	s.amount = decimal.Zero

	if s.debug.open {
		return
	}

	// 设置父订单号和免费订单号
	if s.isFreeRound() {
		// 免费模式：设置免费订单号
		switch {
		case s.lastOrder.FreeOrderSn != "":
			s.freeOrderSN = s.lastOrder.FreeOrderSn
		case s.lastOrder.ParentOrderSn != "":
			s.freeOrderSN = s.lastOrder.ParentOrderSn
		default:
			s.freeOrderSN = s.lastOrder.OrderSn
		}
	} else {
		// 基础模式：设置父订单号
		if s.lastOrder.ParentOrderSn != "" {
			s.parentOrderSN = s.lastOrder.ParentOrderSn
		} else {
			s.parentOrderSN = s.lastOrder.OrderSn
		}
	}
}
