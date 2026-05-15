package gcd

import (
	"fmt"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/game/gcd/pb"
	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/utils/jsonx"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

func (s *betOrderService) settleStep() error {
	return gamelogic.SaveTransfer(&gamelogic.SaveTransferParam{
		Client:      s.client,
		GameOrder:   s.gameOrder,
		MerchantOne: s.merchant,
		MemberOne:   s.member,
		Ip:          s.req.Ip,
	}).Err
}

func (s *betOrderService) updateBetAmount() bool {
	betAmount := decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_baseMultiplier))
	s.betAmount = betAmount
	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
		return false
	}
	return true
}

func (s *betOrderService) updateBonusAmount() error {
	s.bonusAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(s.stepMultiplier))
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		rounded := s.bonusAmount.Round(2).InexactFloat64()
		s.scene.TotalWin += rounded
		s.scene.RoundWin += rounded
		if s.isFreeMode() {
			s.scene.FreeWin += rounded
		}
	}
	return nil
}

func (s *betOrderService) updateGameOrder() error {
	orderSn := common.GenerateOrderSn(s.member, s.lastOrder, s.scene.Stage == _normalMode, s.isFreeMode())
	isFree := int64(0)
	if s.scene.Stage == _freeMode {
		isFree = 1
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
		BonusHeadMultiple: 1,
		BonusMultiple:     s.stepMultiplier,
		BaseAmount:        s.req.BaseMoney,
		Amount:            s.amount.Round(2).InexactFloat64(),
		ValidAmount:       s.amount.Round(2).InexactFloat64(),
		BonusAmount:       s.bonusAmount.Round(2).InexactFloat64(),
		CurBalance:        0,
		OrderSn:           orderSn.OrderSN,
		ParentOrderSn:     orderSn.ParentOrderSN,
		FreeOrderSn:       orderSn.FreeOrderSN,
		State:             1,
		BonusTimes:        0,
		HuNum:             s.treasureNum,
		FreeNum:           s.scene.FreeNum,
		FreeTimes:         s.scene.FreeTimes,
		CreatedAt:         time.Now().Unix(),
		IsFree:            isFree,
	}
	return s.fillInGameOrderDetails()
}

func (s *betOrderService) fillInGameOrderDetails() error {
	betRawDetail, err := jsonx.MarshalString(s.clientSymbolGrid)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return err
	}
	s.gameOrder.BetRawDetail = betRawDetail
	winRawDetail, err := jsonx.MarshalString(s.winGrid)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return err
	}
	s.gameOrder.BonusRawDetail = winRawDetail
	winDetails, err := jsonx.MarshalString(s.getWinDetailsMap())
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return err
	}
	s.gameOrder.WinDetails = winDetails
	return nil
}

func (s *betOrderService) getWinDetailsMap() any {
	s.winArr = make([]*pb.WinResult, len(s.winResults))
	for i, row := range s.winResults {
		s.winArr[i] = &pb.WinResult{
			Symbol:     &row.Symbol,
			SymbolNum:  &row.SymbolCount,
			Multiplier: &row.TotalMultiplier,
			LineNo:     &row.LineNo,
			Grid:       int64GridToPbBoard(row.Position),
		}
	}
	return &WinDetails{
		StepIndex:    s.stepIndex,
		IsRoundOver:  s.isRoundOver,
		BonusState:   s.scene.BonusState,
		Stage:        s.scene.Stage,
		NextStage:    s.scene.NextStage,
		FreeNum:      s.scene.FreeNum,
		FreeTimes:    s.scene.FreeTimes,
		RoundWin:     s.scene.RoundWin,
		FreeWin:      s.scene.FreeWin,
		TotalWin:     s.scene.TotalWin,
		WinArr:       s.winArr,
		RoundMulti:   s.roundMulti,
		FreeType:     s.scene.FreeType,
		NewFreeTimes: s.newFreeTimes,
	}
}
