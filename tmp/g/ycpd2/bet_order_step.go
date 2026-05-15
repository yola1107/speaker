package ycpd

import (
	"fmt"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/game/ycpd2/pb"
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
		return err
	}
	s.merchant, s.member, s.game = mer, mem, ga
	return nil
}

func (s *betOrderService) initialize() error {
	var err error
	switch {
	case !s.isFreeRound && s.scene.Steps == 0:
		err = s.initFirstStepForSpin()
	default:
		err = s.initStepForNextStep()
	}
	if err != nil {
		return err
	}
	s.initRoundState()
	return nil
}

func (s *betOrderService) initRoundState() {
	if s.scene.Steps == 0 {
		s.scene.RoundWin = 0
		if s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeBaseEli {
			s.scene.GameMultiple = 0
			s.scene.RemoveMultiple = [_colCount]int64{}
		} else {
			s.scene.FreeTimes += 1
			s.scene.Decr()
		}
	}

	s.gameMultiple = 0
	s.lineMultiplier = 0
	s.curGameMultiple = [_colCount]int64{}
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

func (s *betOrderService) updateBonusAmount(stepMultiplier int64) decimal.Decimal {
	bonusAmount := s.calcBonusAmount(stepMultiplier)
	s.bonusAmount = bonusAmount
	return bonusAmount
}

func (s *betOrderService) calcBonusAmount(multiplier int64) decimal.Decimal {
	return decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(multiplier))
}

func (s *betOrderService) updateGameOrder() error {
	orderSn := common.GenerateOrderSn(s.member, s.lastOrder, s.scene.Stage == _spinTypeBase, s.isFreeRound)
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
		BonusHeadMultiple: 1,
		BonusMultiple:     s.gameMultiple,
		BaseAmount:        s.req.BaseMoney,
		Amount:            s.amount.Round(2).InexactFloat64(),
		ValidAmount:       s.amount.Round(2).InexactFloat64(),
		BonusAmount:       s.bonusAmount.Round(2).InexactFloat64(),
		CurBalance:        decimal.NewFromFloat(s.member.Balance).Sub(s.amount).Add(s.bonusAmount).Round(2).InexactFloat64(),
		OrderSn:           orderSn.OrderSN,
		ParentOrderSn:     orderSn.ParentOrderSN,
		FreeOrderSn:       orderSn.FreeOrderSN,
		State:             1,
		BonusTimes:        0,
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
	winArr := make([]*pb.Ycpd_WinArr, len(s.winInfos))
	for i, info := range s.winInfos {
		winArr[i] = &pb.Ycpd_WinArr{
			Sym:     proto.Int64(info.Symbol),
			Scnt:    proto.Int64(info.SymbolCount),
			Lcnt:    proto.Int64(info.LineCount),
			BaseMu:  proto.Int64(info.Odds),
			TotalMu: proto.Int64(info.Multiplier),
		}
	}
	return &WinDetails{
		FreeNum:      s.scene.FreeNum,
		FreeTimes:    s.scene.FreeTimes,
		TotalWin:     s.scene.TotalWin,
		FreeWin:      s.scene.FreeWin,
		RoundWin:     s.scene.RoundWin,
		TotalFree:    s.scene.TotalFree,
		NewFreeTimes: s.addFreeTime,
		IsRoundOver:  s.isRoundOver,
		IsSpinOver:   s.isSpinOver,
		IsFree:       s.isFreeRound,
		State:        s.scene.Stage,
		LineMul:      s.lineMultiplier,
		StepMul:      s.stepMultiplier,
		GameMul:      s.gameOrder.BonusMultiple,
		RemoveMul:    colMultipliers(s.scene.RemoveMultiple),
		CurGameMul:   colMultipliers(s.curGameMultiple),
		WinArr:       winArr,
	}
}

func gameOrderToResponse(gameOrder *game.GameOrder) (*pb.Ycpd_BetOrderResponse, error) {
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
	resp := &pb.Ycpd_BetOrderResponse{
		Sn:                 &gameOrder.OrderSn,
		Balance:            &gameOrder.CurBalance,
		BaseBet:            &gameOrder.BaseAmount,
		BetAmount:          &bet,
		Multiplier:         &gameOrder.Multiple,
		CurWin:             &gameOrder.BonusAmount,
		RoundWin:           &winDetail.RoundWin,
		TotalWin:           &winDetail.TotalWin,
		FreeTotalWin:       &winDetail.FreeWin,
		IsRoundOver:        &winDetail.IsRoundOver,
		IsSpinOver:         &winDetail.IsSpinOver,
		IsFree:             &winDetail.IsFree,
		NewFreeTimes:       &winDetail.NewFreeTimes,
		RemainingFreeTimes: &winDetail.FreeNum,
		TotalFreeTimes:     &winDetail.TotalFree,
		SymGrid:            int64GridToArray(symbolGrid),
		State:              &winDetail.State,
		WinArr:             winDetail.WinArr,
		WinGird:            int64GridToArray(winGrid),
		LineMul:            &winDetail.LineMul,
		StepMul:            &winDetail.StepMul,
		GameMul:            &winDetail.GameMul,
		RemoveMul:          winDetail.RemoveMul,
		CurGameMul:         winDetail.CurGameMul,
	}
	return resp, nil
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
