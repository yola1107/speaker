package bxkh2

import (
	"fmt"
	"time"

	"egame-grpc/game/bxkh2/pb"
	"egame-grpc/game/common"
	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/utils/jsonx"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func (s *betOrderService) getRequestContext() error {
	mer, mem, ga, err := common.GetRequestContext(s.req)
	if err != nil {
		global.GVA_LOG.Error("failed to get request context", zap.Error(err))
		return err
	}
	s.merchant, s.member, s.game = mer, mem, ga
	return nil
}

func (s *betOrderService) initialize() error {
	switch {
	case !s.isFreeRound && s.scene.Steps == 0:
		return s.initFirstStepForSpin()
	default:
		return s.initStepForNextStep()
	}
}

func (s *betOrderService) initFirstStepForSpin() error {
	if s.debug.open {
		s.betAmount = decimal.NewFromInt(_baseMultiplier)
		s.amount = s.betAmount
		s.scene.Reset()
		return nil
	}
	switch {
	case !s.updateBetAmount():
		return InvalidRequestParams
	case !s.checkBalance():
		return InsufficientBalance
	}
	s.scene.Reset()
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

func (s *betOrderService) updateBetAmount() bool {
	s.betAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_baseMultiplier))
	s.amount = s.betAmount
	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v,%v]", s.req.BaseMoney, s.req.Multiple, s.req.Purchase)))
		return false
	}
	return true
}

func (s *betOrderService) checkBalance() bool {
	return gamelogic.CheckMemberBalance(s.amount.Round(2).InexactFloat64(), s.member)
}

func (s *betOrderService) updateBonusAmount(stepMultiplier int64) {
	if s.debug.open || stepMultiplier == 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	s.bonusAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(s.stepMultiplier))
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		rounded := s.bonusAmount.Round(2).InexactFloat64()
		s.scene.TotalWin += rounded
		s.scene.RoundWin += rounded
		if s.isFreeRound {
			s.scene.FreeWin += rounded
		}
	}
}

func (s *betOrderService) updateGameOrder() error {
	orderSn := &common.OrderSN{}
	if !s.debug.open {
		orderSn = common.GenerateOrderSn(s.member, s.lastOrder, s.scene.Stage == _spinTypeBase,
			s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli)
	}

	s.gameOrder = &game.GameOrder{
		MerchantID:    s.merchant.ID,
		Merchant:      s.merchant.Merchant,
		MemberID:      s.member.ID,
		Member:        s.member.MemberName,
		GameID:        s.game.ID,
		GameName:      s.game.GameName,
		BaseMultiple:  _baseMultiplier,
		Multiple:      s.req.Multiple,
		BonusMultiple: s.stepMultiplier,
		BaseAmount:    s.req.BaseMoney,
		Amount:        s.amount.Round(2).InexactFloat64(),
		ValidAmount:   s.amount.Round(2).InexactFloat64(),
		BonusAmount:   s.bonusAmount.Round(2).InexactFloat64(),
		CurBalance:    decimal.NewFromFloat(s.member.Balance).Sub(s.amount).Add(s.bonusAmount).Round(2).InexactFloat64(),
		OrderSn:       orderSn.OrderSN,
		ParentOrderSn: orderSn.ParentOrderSN,
		FreeOrderSn:   orderSn.FreeOrderSN,
		State:         1,
		HuNum:         s.scatterCount,
		FreeNum:       int64(s.scene.FreeNum),
		FreeTimes:     int64(s.scene.FreeTimes),
		CreatedAt:     time.Now().Unix(),
	}
	if s.isFreeRound {
		s.gameOrder.IsFree = 1
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
	winArr := make([]*pb.Bxkh2_WinArr, len(s.winInfos))
	for i, elem := range s.winInfos {
		winArr[i] = &pb.Bxkh2_WinArr{
			Val:     proto.Int64(elem.Symbol),
			RoadNum: proto.Int64(elem.LineCount),
			StarNum: proto.Int64(elem.SymbolCount),
			Odds:    proto.Int64(elem.Odds),
			Mul:     proto.Int64(elem.Multiplier),
		}
	}
	return &WinDetails{
		RoundWin:     s.scene.RoundWin,
		TotalWin:     s.scene.TotalWin,
		FreeTotalWin: s.scene.FreeWin,
		FreeMultiple: s.scene.FreeWinMultiple,
		IsRoundOver:  s.isRoundOver,
		AddFreeTime:  s.addFreeTime,
		WinArr:       winArr,
	}
}

func gameOrderToResponse(gameOrder *game.GameOrder) (*pb.Bxkh2_BetOrderResponse, error) {
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
	isFree := gameOrder.IsFree == 1
	return &pb.Bxkh2_BetOrderResponse{
		Sn:            &gameOrder.OrderSn,
		Balance:       &gameOrder.CurBalance,
		BaseBet:       &gameOrder.BaseAmount,
		Multiplier:    &gameOrder.Multiple,
		BetAmount:     &bet,
		CurWin:        &gameOrder.BonusAmount,
		RoundWin:      &winDetail.RoundWin,
		TotalWin:      &winDetail.TotalWin,
		FreeWin:       &winDetail.FreeTotalWin,
		IsRoundOver:   &winDetail.IsRoundOver,
		IsFree:        &isFree,
		FreeNum:       &gameOrder.FreeNum,
		FreeTime:      &gameOrder.FreeTimes,
		NewFreeTimes:  &winDetail.AddFreeTime,
		SymGrid:       int64GridToArray(symbolGrid),
		WinGrid:       int64GridToArray(winGrid),
		WinArr:        winDetail.WinArr,
		FreeMultiple:  &winDetail.FreeMultiple,
		OriginalCards: int64GridToArray(symbolGrid),
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
