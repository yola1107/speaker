package qnjx

import (
	"fmt"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/game/qnjx/pb"
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
		global.GVA_LOG.Error("getRequestContext error.")
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
		s.scene.SceneFreeGame.Reset()
		return nil
	}
	switch {
	case !s.updateBetAmount():
		return InvalidRequestParams
	case !s.checkBalance():
		return InsufficientBalance
	}
	s.scene.SceneFreeGame.Reset()
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
	orderSn := common.GenerateOrderSn(s.member, s.lastOrder, s.scene.Stage == _spinTypeBase,
		s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli)
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
		HuNum:             s.scatterCount,
		FreeNum:           s.scene.FreeNum,
		FreeTimes:         s.scene.FreeTimes,
		CreatedAt:         time.Now().Unix(),
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
	winArr := make([]*pb.Qnjx_WinArr, len(s.winInfos))
	for i, elem := range s.winInfos {
		winArr[i] = &pb.Qnjx_WinArr{
			Val:     &elem.Symbol,
			RoadNum: &elem.LineCount,
			Odds:    &elem.Odds,
		}
	}
	return &WinDetails{
		FreeWin:      s.scene.FreeWin,
		RoundWin:     s.scene.RoundWin,
		IsRoundOver:  s.isRoundOver,
		State:        int64(s.scene.Stage),
		NewFreeTimes: s.addFreeTime,
		MysMul:       s.mysMul,
		Limit:        s.limit,
		ColorMul:     s.scene.ColorMul[:],
		WinArr:       winArr,
	}
}

func gameOrderToResponse(gameOrder *game.GameOrder) (*pb.Qnjx_BetOrderResponse, error) {
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
	return &pb.Qnjx_BetOrderResponse{
		Sn:           &gameOrder.OrderSn,
		Balance:      &gameOrder.CurBalance,
		BetAmount:    &bet,
		CurWin:       &gameOrder.BonusAmount,
		FreeWin:      &winDetail.FreeWin,
		RoundWin:     &winDetail.RoundWin,
		IsRoundOver:  &winDetail.IsRoundOver,
		IsFree:       proto.Bool(gameOrder.IsFree == 1),
		State:        &winDetail.State,
		FreeNum:      &gameOrder.FreeNum,
		FreeTime:     &gameOrder.FreeTimes,
		NewFreeTimes: &winDetail.NewFreeTimes,
		ScatterCount: &gameOrder.HuNum,
		SymGrid:      int64GridToArray(symbolGrid),
		WinGrid:      int64GridToArray(winGrid),
		StepMul:      &gameOrder.LineMultiple,
		Limit:        &winDetail.Limit,
		ColorMul:     winDetail.ColorMul,
		WinArr:       winDetail.WinArr,
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
