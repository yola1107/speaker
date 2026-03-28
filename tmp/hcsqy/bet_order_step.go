package hcsqy

import (
	"time"

	"egame-grpc/game/common"
	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

func (s *betOrderService) initialize() error {
	s.client.ClientOfFreeGame.ResetFreeClean()
	switch {
	case !s.isFreeRound && !s.scene.IsRespinMode:
		return s.initFirstStepForSpin()
	default:
		return s.initStepForNextStep()
	}
}

func (s *betOrderService) initFirstStepForSpin() error {
	if s.debug.open {
		s.betAmount = decimal.NewFromInt(_baseMultiplier)
		s.amount = s.betAmount
		return nil
	}

	switch {
	case !s.updateBetAmount():
		return InvalidRequestParams
	case s.checkPurchase() != nil:
		return ErrorPurchase
	case !s.checkBalance():
		return InsufficientBalance
	}
	s.client.SetLastMaxFreeNum(0)
	s.client.ClientOfFreeGame.Reset()
	s.client.ClientOfFreeGame.ResetGeneralWinTotal()
	s.client.ClientOfFreeGame.ResetRoundBonus()
	s.client.ClientOfFreeGame.ResetRoundBonusStaging()
	s.client.ClientOfFreeGame.SetBetAmount(s.betAmount.Round(2).InexactFloat64())
	s.client.ClientOfFreeGame.SetPurchaseAmount(s.req.Purchase)
	s.client.ClientOfFreeGame.SetLastWinId(uint64(time.Now().UnixNano()))
	if s.req.Purchase > 0 {

		s.scene.IsPurchase = true
		s.scene.NextStage = 0 //s.scene.Stage = _spinTypeBase
		//freeNum := uint64(s.gameConfig.Free.FreeTimes)
		//s.scene.FreeNum = s.gameConfig.Free.FreeTimes
		//s.isFreeRound = false
		//s.client.ClientOfFreeGame.SetFreeNum(freeNum)
		//s.client.SetMaxFreeNum(freeNum)
		//s.client.SetLastMaxFreeNum(freeNum)
	}
	return nil
}

// checkPurchase 校验购买请求合法性：
// 1) 只能在基础非重转起手下注购买；
// 2) 购买金额必须等于 betAmount * buy.price。
func (s *betOrderService) checkPurchase() error {
	if s.req.Purchase <= 0 {
		return nil
	}

	// initialize 分支已经保证是基础非重转首手，这里保留显式校验，避免未来流程调整引入隐性问题。
	if s.isFreeRound || s.scene.IsRespinMode || s.scene.FreeNum > 0 {
		global.GVA_LOG.Error("checkPurchase: invalid purchase stage",
			zap.Int64("purchase", s.req.Purchase),
			zap.Bool("isFreeRound", s.isFreeRound),
			zap.Bool("isRespinMode", s.scene.IsRespinMode),
			zap.Int64("freeNum", s.scene.FreeNum),
		)
		return ErrorPurchase
	}

	expect := s.betAmount.Mul(decimal.NewFromInt(s.gameConfig.Buy.Price))
	if !expect.Equal(decimal.NewFromInt(s.req.Purchase)) {
		global.GVA_LOG.Error("checkPurchase: invalid purchase amount",
			zap.Int64("purchase", s.req.Purchase),
			zap.String("expect", expect.String()),
			zap.String("betAmount", s.betAmount.String()),
		)
		return ErrorPurchase
	}
	return nil
}

func (s *betOrderService) initStepForNextStep() error {
	if s.debug.open {
		s.req.BaseMoney = 1
		s.req.Multiple = 1

		s.betAmount = decimal.NewFromInt(_baseMultiplier)
		s.amount = decimal.Zero
		return nil
	}

	s.req.BaseMoney = s.lastOrder.BaseAmount
	s.req.Multiple = s.lastOrder.Multiple

	s.betAmount = decimal.NewFromFloat(s.client.ClientOfFreeGame.GetBetAmount())
	s.amount = decimal.Zero
	return nil
}

func (s *betOrderService) updateGameOrder() error {
	if !s.debug.open {
		isBaseStage := s.scene.Stage == _spinTypeBase
		isFreeStage := s.scene.Stage == _spinTypeFree
		s.orderSn = common.GenerateOrderSn(s.member, s.lastOrder, isBaseStage, isFreeStage)
	}
	if s.orderSn == nil {
		s.orderSn = &common.OrderSN{}
	}
	s.gameOrder = &game.GameOrder{
		MerchantID:        s.merchant.ID,
		Merchant:          s.merchant.Merchant,
		MemberID:          s.member.ID,
		Member:            s.member.MemberName,
		GameID:            s.game.ID,
		GameName:          s.game.GameName,
		BaseMultiple:      _baseMultiplier,
		Multiple:          s.req.Multiple,
		LineMultiple:      s.stepMultiplier,
		BonusHeadMultiple: 1,
		BonusMultiple:     1,
		BaseAmount:        s.req.BaseMoney,
		Amount:            s.amount.Round(2).InexactFloat64(),
		ValidAmount:       s.amount.Round(2).InexactFloat64(),
		BonusAmount:       s.bonusAmount.Round(2).InexactFloat64(),
		CurBalance:        decimal.NewFromFloat(s.member.Balance).Sub(s.amount).Add(s.bonusAmount).Round(2).InexactFloat64(),
		OrderSn:           s.orderSn.OrderSN,
		ParentOrderSn:     s.orderSn.ParentOrderSN,
		FreeOrderSn:       s.orderSn.FreeOrderSN,
		State:             1,
		BonusTimes:        1,
		HuNum:             s.scatterCount,
		FreeNum:           s.scene.FreeNum,
		FreeTimes:         int64(s.client.ClientOfFreeGame.GetFreeTimes()),
	}
	if s.isFreeRound {
		s.gameOrder.IsFree = 1
		//if s.stepIsPurchase {
		//	s.gameOrder.IsFree = 2
		//}
	}
	return s.fillInGameOrderDetails()
}

func (s *betOrderService) fillInGameOrderDetails() error {
	var err error
	if s.gameOrder.BetRawDetail, err = json.CJSON.MarshalToString(s.symbolGrid); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal symbolGrid", zap.Error(err))
		return err
	}
	if s.gameOrder.BonusRawDetail, err = json.CJSON.MarshalToString(s.winGrid); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal winGrid", zap.Error(err))
		return err
	}
	s.gameOrder.BetDetail = s.symbolGridToString(s.symbolGrid)
	s.gameOrder.BonusDetail = s.symbolGridToString(s.winGrid)
	winDetails, err := json.CJSON.MarshalToString(s.buildWinInfo())
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return err
	}
	s.gameOrder.WinDetails = winDetails
	return nil
}

func (s *betOrderService) settleStep() error {
	s.gameOrder.CreatedAt = time.Now().Unix()
	saveParam := &gamelogic.SaveTransferParam{
		Client:      s.client,
		GameOrder:   s.gameOrder,
		MerchantOne: s.merchant,
		MemberOne:   s.member,
		Ip:          s.req.Ip,
	}
	return gamelogic.SaveTransfer(saveParam).Err
}
