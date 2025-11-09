package xslm2

import (
	"fmt"
	"strconv"
	"time"

	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/utils/json"
	"egame-grpc/utils/snow"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// getRequestContext 获取请求上下文（商户、用户、游戏信息）
func (s *betOrderService) getRequestContext() bool {
	return s.mdbGetMerchant() && s.mdbGetMember() && s.mdbGetGame()
}

// initialize 初始化step数据
func (s *betOrderService) initialize() error {
	s.client.ClientOfFreeGame.ResetFreeClean()
	s.orderSN = strconv.FormatInt(snow.GenarotorID(s.member.ID), 10)

	if s.isFirst {
		return s.initStepForFirstStep()
	} else {
		return s.initStepForNextStep()
	}
}

// initStepForFirstStep 初始化首次step（回合第一局）
func (s *betOrderService) initStepForFirstStep() error {
	if !s.updateBetAmount() {
		return InvalidRequestParams
	}
	s.client.IsRoundOver = false

	// 如果是免费回合，不扣费且不重置免费游戏状态
	if s.isFreeRound {
		s.amount = decimal.Zero
		s.client.ClientOfFreeGame.ResetRoundBonus()
		s.client.ClientOfFreeGame.SetBetAmount(s.betAmount.Round(2).InexactFloat64())
	} else {
		// 基础回合：扣费并重置状态
		if !s.checkBalance() {
			return InsufficientBalance
		}
		s.client.SetLastMaxFreeNum(0)
		s.client.ClientOfFreeGame.Reset()
		s.client.ClientOfFreeGame.ResetGeneralWinTotal()
		s.client.ClientOfFreeGame.ResetRoundBonus()
		s.client.ClientOfFreeGame.SetBetAmount(s.betAmount.Round(2).InexactFloat64())
		s.amount = s.betAmount
	}
	return nil
}

// updateBetAmount 计算下注金额
func (s *betOrderService) updateBetAmount() bool {
	// 校验参数
	if !contains(_cnf.BetSizeSlice, s.req.BaseMoney) {
		global.GVA_LOG.Warn("invalid baseMoney", zap.Float64("value", s.req.BaseMoney))
		return false
	}
	if !contains(_cnf.BetLevelSlice, s.req.Multiple) {
		global.GVA_LOG.Warn("invalid multiple", zap.Int64("value", s.req.Multiple))
		return false
	}

	s.betAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_cnf.BaseBat))
	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount", zap.Error(fmt.Errorf("invalid params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
		return false
	}
	return true
}

// checkBalance 检查余额
func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

// initStepForNextStep 初始化后续step（回合内的连消step）
func (s *betOrderService) initStepForNextStep() error {
	s.req.BaseMoney = s.lastOrder.BaseAmount
	s.req.Multiple = s.lastOrder.Multiple
	s.betAmount = decimal.NewFromFloat(s.client.ClientOfFreeGame.GetBetAmount())
	s.amount = decimal.Zero

	if s.client.IsRoundOver {
		s.isFreeRound = true
		s.client.ClientOfFreeGame.ResetRoundBonus()
		if s.lastOrder.FreeOrderSn != "" {
			s.freeOrderSN = s.lastOrder.FreeOrderSn
		} else if s.lastOrder.ParentOrderSn != "" {
			s.freeOrderSN = s.lastOrder.ParentOrderSn
		} else {
			s.freeOrderSN = s.lastOrder.OrderSn
		}
	} else {
		s.isFreeRound = s.lastOrder.IsFree > 0
		if s.lastOrder.ParentOrderSn != "" {
			s.parentOrderSN = s.lastOrder.ParentOrderSn
		} else {
			s.parentOrderSN = s.lastOrder.OrderSn
		}
		s.freeOrderSN = s.lastOrder.FreeOrderSn
	}

	return nil
}

// updateStepResult 更新step结果
func (s *betOrderService) updateStepResult() {
	s.client.IsRoundOver = s.spin.isRoundOver

	// 更新奖金和余额
	if s.spin.stepMultiplier > 0 {
		s.updateBonusAmount()
		bonus := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(bonus)
		if s.isFreeRound {
			s.client.ClientOfFreeGame.IncrFreeTotalMoney(bonus)
		}
		s.client.ClientOfFreeGame.IncRoundBonus(bonus)
	}

	// 免费回合结束处理
	if s.isFreeRound && s.client.IsRoundOver {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
	}

	// 处理新增免费次数
	if s.client.IsRoundOver && s.spin.newFreeRoundCount > 0 {
		if !s.isFreeRound {
			s.client.ClientOfFreeGame.SetFreeNum(uint64(s.spin.newFreeRoundCount))
			s.client.SetLastMaxFreeNum(uint64(s.spin.newFreeRoundCount))
		} else {
			s.client.ClientOfFreeGame.Incr(uint64(s.spin.newFreeRoundCount))
			s.client.IncLastMaxFreeNum(uint64(s.spin.newFreeRoundCount))
		}
	}

	s.currBalance = decimal.NewFromFloat(s.member.Balance).Sub(s.amount).Add(s.bonusAmount)
}

// updateBonusAmount 计算奖金
func (s *betOrderService) updateBonusAmount() {
	if s.spin.stepMultiplier <= 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	s.bonusAmount = s.betAmount.Div(decimal.NewFromInt(_cnf.BaseBat)).Mul(decimal.NewFromInt(s.spin.stepMultiplier))
}

// ========== 订单更新和结算 ==========

// updateGameOrder 创建游戏订单
func (s *betOrderService) updateGameOrder() bool {
	gameOrder := game.GameOrder{
		MerchantID:        s.merchant.ID,
		Merchant:          s.merchant.Merchant,
		MemberID:          s.member.ID,
		Member:            s.member.MemberName,
		GameID:            s.game.ID,
		GameName:          s.game.GameName,
		BaseMultiple:      _cnf.BaseBat,
		Multiple:          s.req.Multiple,
		LineMultiple:      s.spin.stepMultiplier,
		BonusHeadMultiple: 1,
		BonusMultiple:     s.spin.stepMultiplier,
		BaseAmount:        s.req.BaseMoney,
		Amount:            s.amount.Round(2).InexactFloat64(),
		ValidAmount:       s.amount.Round(2).InexactFloat64(),
		BonusAmount:       s.bonusAmount.Round(2).InexactFloat64(),
		CurBalance:        s.currBalance.Round(2).InexactFloat64(),
		OrderSn:           s.orderSN,
		ParentOrderSn:     s.parentOrderSN,
		FreeOrderSn:       s.freeOrderSN,
		State:             1,
		BonusTimes:        0,
		HuNum:             s.spin.treasureCount,
		FreeNum:           s.spin.newFreeRoundCount,
		FreeTimes:         int64(s.client.ClientOfFreeGame.GetFreeTimes()),
	}
	if s.isFreeRound {
		gameOrder.IsFree = 1
	}
	s.gameOrder = &gameOrder
	return s.fillInGameOrderDetails()
}

// fillInGameOrderDetails 填充订单详情（序列化数据）
func (s *betOrderService) fillInGameOrderDetails() bool {
	betRawDetail, err := json.CJSON.MarshalToString(s.spin.symbolGrid)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return false
	}
	s.gameOrder.BetRawDetail = betRawDetail
	winRawDetail, err := json.CJSON.MarshalToString(s.spin.winGrid)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return false
	}
	s.gameOrder.BonusRawDetail = winRawDetail
	s.gameOrder.BetDetail = gridToString(s.spin.symbolGrid)
	s.gameOrder.BonusDetail = gridToString(s.spin.winGrid)
	winDetailsMap := s.buildResultMap()
	winDetails, err := json.CJSON.MarshalToString(winDetailsMap)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return false
	}
	s.gameOrder.WinDetails = winDetails
	return true
}

// settleStep 结算step（保存订单到数据库）
func (s *betOrderService) settleStep() bool {
	s.gameOrder.CreatedAt = time.Now().Unix()
	saveParam := &gamelogic.SaveTransferParam{
		Client:      s.client,
		GameOrder:   s.gameOrder,
		MerchantOne: s.merchant,
		MemberOne:   s.member,
		Ip:          s.req.Ip,
	}
	res := gamelogic.SaveTransfer(saveParam)
	return res.Err == nil
}
