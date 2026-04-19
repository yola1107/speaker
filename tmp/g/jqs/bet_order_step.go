package jqs

import (
	"fmt"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/game/jqs/pb"
	"egame-grpc/gamelogic"
	"egame-grpc/gamelogic/game_replay"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

func (s *betOrderService) initialize() error {
	if s.scene.Stage == _spinTypeFree {
		return s.initStepForNextStep()
	}
	return s.initStepForFirstStep()
}

func (s *betOrderService) initStepForFirstStep() error {
	if s.debug.open {
		s.betAmount = decimal.NewFromInt(_baseMultiplier)
		s.amount = s.betAmount
		return nil
	}
	switch {
	case !s.updateBetAmount():
		return InvalidRequestParams
	case !s.checkBalance():
		return InsufficientBalance
	}
	s.scene.SceneFreeGame.Reset()
	s.amount = s.betAmount
	return nil
}

// 初始化spin后续step
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

// 更新订单
func (s *betOrderService) updateGameOrder() error {
	orderSn := &common.OrderSN{}
	if !s.debug.open {
		orderSn = common.GenerateOrderSn(s.member, s.lastOrder, !s.isFreeRound, s.isFreeRound)
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
		BonusMultiple:     s.stepMultiplier,
		BaseAmount:        s.req.BaseMoney,
		Amount:            s.amount.Round(2).InexactFloat64(),
		ValidAmount:       s.amount.Round(2).InexactFloat64(),
		BonusAmount:       s.bonusAmount.Round(2).InexactFloat64(),
		CurBalance:        decimal.NewFromFloat(s.member.Balance).Sub(s.amount).Add(s.bonusAmount).Round(2).InexactFloat64(),
		OrderSn:           orderSn.OrderSN,
		ParentOrderSn:     orderSn.ParentOrderSN,
		FreeOrderSn:       orderSn.FreeOrderSN,
		State:             s.state,
		BonusTimes:        0,
		HuNum:             0,
		CreatedAt:         time.Now().Unix(),
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
	if s.gameOrder.BonusRawDetail, err = json.CJSON.MarshalToString(s.winGrid); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal winGrid", zap.Error(err))
		return err
	}
	if s.gameOrder.WinDetails, err = json.CJSON.MarshalToString(s.getWinDetails()); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return err
	}
	return nil
}

func (s *betOrderService) getWinDetails() *WinDetails {
	winArr := make([]*pb.Jqs_WinArr, len(s.winInfos))
	for i, w := range s.winInfos {
		winArr[i] = &pb.Jqs_WinArr{
			Val:     &w.Symbol,
			RoadNum: &w.LineCount,
			StarNum: &w.SymbolCount,
			Odds:    &w.Odds,
		}
	}
	return &WinDetails{
		FreeWin:  s.scene.FreeWin,
		TotalWin: s.scene.TotalWin,
		Next:     s.scene.NextStage == _spinTypeFree,
		WinArr:   winArr,
	}
}

func (s *betOrderService) settleStep() error {
	return gamelogic.SaveTransfer(&gamelogic.SaveTransferParam{
		Client:      s.client,
		GameOrder:   s.gameOrder,
		MerchantOne: s.merchant,
		MemberOne:   s.member,
		Ip:          s.req.Ip,
	}).Err
}

func (s *betOrderService) replayByOrder(_ *request.BetOrderReq, gameOrder *game.GameOrder) (*game_replay.InternalResponse, error) {
	ret, err := gameOrderToResponse(gameOrder)
	if err != nil {
		return nil, fmt.Errorf("jqs replayByOrder gameOrderToResponse: %w", err)
	}
	pbData, jsonData, err := marshalProtoMessage(ret)
	if err != nil {
		return nil, fmt.Errorf("jqs replayByOrder marshal: %w", err)
	}
	return game_replay.NewPbInternalResponse(jsonData, pbData), nil
}
