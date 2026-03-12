package pjcd

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

var _baseMultiplierDec = decimal.NewFromInt(_baseMultiplier)

func (s *betOrderService) initialize() error {
	s.client.ClientOfFreeGame.ResetFreeClean()
	switch {
	case !s.isFreeRound && s.scene.Steps == 0:
		return s.initFirstStepForSpin()
	default:
		return s.initStepForNextStep()
	}
}

func (s *betOrderService) initFirstStepForSpin() error {
	if s.debug.open {
		s.betAmount = _baseMultiplierDec // decimal.NewFromInt(_baseMultiplier)
		s.amount = s.betAmount
		return nil
	}

	switch {
	case !s.updateBetAmount():
		return InvalidRequestParams
	case !s.checkBalance():
		return InsufficientBalance
	}
	s.client.SetLastMaxFreeNum(0)
	s.client.ClientOfFreeGame.Reset()
	s.client.ClientOfFreeGame.ResetGeneralWinTotal()
	s.client.ClientOfFreeGame.ResetRoundBonus()
	s.client.ClientOfFreeGame.ResetRoundBonusStaging()
	s.client.ClientOfFreeGame.SetBetAmount(s.betAmount.Round(2).InexactFloat64())
	s.amount = s.betAmount
	s.client.ClientOfFreeGame.SetLastWinId(uint64(time.Now().UnixNano()))
	return nil
}

func (s *betOrderService) initStepForNextStep() error {
	if s.debug.open {
		s.req.BaseMoney = 1
		s.req.Multiple = 1

		s.betAmount = _baseMultiplierDec // decimal.NewFromInt(_baseMultiplier)
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
		s.orderSn = common.GenerateOrderSn(s.member, s.lastOrder, s.scene.Stage == _spinTypeBase, s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli)
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
		LineMultiple:      s.lineMultiplier,
		BonusHeadMultiple: s.gameMultiple,
		BonusMultiple:     s.gameMultiple,
		BaseAmount:        s.req.BaseMoney,
		Amount:            s.amount.Round(2).InexactFloat64(),
		ValidAmount:       s.amount.Round(2).InexactFloat64(),
		BonusAmount:       s.bonusAmount.Round(2).InexactFloat64(),
		CurBalance:        decimal.NewFromFloat(s.member.Balance).Sub(s.amount).Add(s.bonusAmount).Round(2).InexactFloat64(),
		OrderSn:           s.orderSn.OrderSN,
		ParentOrderSn:     s.orderSn.ParentOrderSN,
		FreeOrderSn:       s.orderSn.FreeOrderSN,
		State:             1,
		BonusTimes:        s.scene.ContinueNum,
		HuNum:             s.scatterCount,
		FreeNum:           s.scene.FreeNum,
		FreeTimes:         int64(s.client.ClientOfFreeGame.GetFreeTimes()),
	}
	if s.isFreeRound {
		s.gameOrder.IsFree = 1
	}
	return s.fillInGameOrderDetails()
}

func (s *betOrderService) fillInGameOrderDetails() error {
	var err error
	if s.gameOrder.BetRawDetail, err = json.CJSON.MarshalToString(s.symbolGrid); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal symbolGrid", zap.Error(err))
		return err
	}
	// 使用3行奖励格式保存
	if s.gameOrder.BonusRawDetail, err = json.CJSON.MarshalToString(s.winGridReward); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal WinGridReward", zap.Error(err))
		return err
	}
	s.gameOrder.BetDetail = s.symbolGridToString(s.symbolGrid)
	s.gameOrder.BonusDetail = s.winGridToString(s.winGridReward)
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
