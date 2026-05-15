package sbyymx2

import (
	"time"

	"egame-grpc/game/common"
	"egame-grpc/game/sbyymx2/pb"
	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/utils/jsonx"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func (s *betOrderService) initialize() error {
	switch {
	case !s.scene.IsRespinMode:
		return s.initFirstStepForSpin()
	default:
		return s.initStepForNextStep()
	}
}

func (s *betOrderService) initFirstStepForSpin() error {
	if s.debug.open {
		s.betAmount = decimal.NewFromInt(_baseMultiplier)
		s.amount = s.betAmount
	} else {
		switch {
		case !s.updateBetAmount():
			return InvalidRequestParams
		case !s.checkBalance():
			return InsufficientBalance
		}
	}
	s.amount = s.betAmount
	s.scene.TotalWin = 0
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
	s.betAmount = decimal.NewFromFloat(s.lastOrder.BaseAmount * float64(s.lastOrder.BaseMultiple*s.lastOrder.Multiple))
	s.amount = decimal.Zero
	return nil
}

func (s *betOrderService) updateGameOrder() error {
	orderSn := &common.OrderSN{}
	if !s.debug.open {
		orderSn = common.GenerateOrderSn(s.member, s.lastOrder, true, false)
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
		OrderSn:           orderSn.OrderSN,
		ParentOrderSn:     orderSn.ParentOrderSN,
		FreeOrderSn:       orderSn.FreeOrderSN,
		State:             1,
		BonusTimes:        1,
		CreatedAt:         time.Now().Unix(),
	}
	return s.fillInGameOrderDetails()
}

func (s *betOrderService) fillInGameOrderDetails() error {
	var err error
	if s.gameOrder.BetRawDetail, err = jsonx.MarshalString(s.symbolGrid); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal symbolGrid", zap.Error(err))
		return err
	}
	if s.gameOrder.BonusRawDetail, err = jsonx.MarshalString(s.winGrid); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal winGrid", zap.Error(err))
		return err
	}
	if s.gameOrder.WinDetails, err = jsonx.MarshalString(s.getWinDetails()); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return err
	}
	return nil
}

func (s *betOrderService) getWinDetails() *WinDetails {
	winArr := make([]*pb.Sbyymx_WinArr, len(s.winInfos))
	for i, elem := range s.winInfos {
		winArr[i] = &pb.Sbyymx_WinArr{
			RoadNum: proto.Int64(elem.LineCount),
			Odds:    proto.Int64(elem.Odds),
		}
	}
	return &WinDetails{
		TotalWin:         s.scene.TotalWin,
		IsRespinUntilWin: s.stepIsRespinMode,
		WildMultiplier:   s.wildMultiplier,
		LineMultiplier:   s.lineMultiplier,
		StepMultiple:     s.stepMultiplier,
		IsInstrumentWin:  s.isInstrumentWin,
		IsRoundOver:      s.isRoundOver,
		WinArr:           winArr,
	}
}

func gameOrderToResponse(gameOrder *game.GameOrder) (*pb.Sbyymx_BetOrderResponse, error) {
	winDetail := WinDetails{}
	if err := jsonx.UnmarshalString(gameOrder.WinDetails, &winDetail); err != nil {
		return nil, err
	}
	symbolGrid := int64Grid{}
	if err := jsonx.UnmarshalString(gameOrder.BetRawDetail, &symbolGrid); err != nil {
		return nil, err
	}
	winGrid := int64Grid{}
	if err := jsonx.UnmarshalString(gameOrder.BonusRawDetail, &winGrid); err != nil {
		return nil, err
	}
	bet := gameOrder.BaseAmount * float64(gameOrder.BaseMultiple*gameOrder.Multiple)
	return &pb.Sbyymx_BetOrderResponse{
		OrderSN:          &gameOrder.OrderSn,
		Balance:          &gameOrder.CurBalance,
		BetAmount:        &bet,
		CurrentWin:       &gameOrder.BonusAmount,
		TotalWin:         &winDetail.TotalWin,
		Review:           proto.Int64(0),
		WinArr:           winDetail.WinArr,
		Cards:            int64GridToArray(symbolGrid),
		IsRoundOver:      &winDetail.IsRoundOver,
		WinGrid:          int64GridToArray(winGrid),
		IsRespinUntilWin: &winDetail.IsRespinUntilWin,
		WildMultiplier:   &winDetail.WildMultiplier,
		LineMultiplier:   &winDetail.LineMultiplier,
		StepMultiple:     &winDetail.StepMultiple,
		IsInstrumentWin:  &winDetail.IsInstrumentWin,
	}, nil
}

func (s *betOrderService) settleStep() error {
	saveParam := &gamelogic.SaveTransferParam{
		Client:      s.client,
		GameOrder:   s.gameOrder,
		MerchantOne: s.merchant,
		MemberOne:   s.member,
		Ip:          s.req.Ip,
	}
	return gamelogic.SaveTransfer(saveParam).Err
}
