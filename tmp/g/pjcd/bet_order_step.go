package pjcd

import (
	"time"

	"egame-grpc/game/common"
	"egame-grpc/game/pjcd/pb"
	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

var _baseMultiplierDec = decimal.NewFromInt(_baseMultiplier)

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
		s.betAmount = _baseMultiplierDec
		s.amount = s.betAmount
		return nil
	}

	switch {
	case !s.updateBetAmount():
		return InvalidRequestParams
	case !s.checkBalance():
		return InsufficientBalance
	}
	s.scene.Reset()
	s.amount = s.betAmount
	return nil
}

func (s *betOrderService) initStepForNextStep() error {
	if s.debug.open {
		s.req.BaseMoney = 1
		s.req.Multiple = 1
		s.betAmount = _baseMultiplierDec
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
		orderSn = common.GenerateOrderSn(s.member, s.lastOrder, s.scene.Stage == _spinTypeBase, s.scene.Stage == _spinTypeFree)
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
		BonusMultiple:     s.stepMultiplier,
		BaseAmount:        s.req.BaseMoney,
		Amount:            s.amount.Round(2).InexactFloat64(),
		ValidAmount:       s.amount.Round(2).InexactFloat64(),
		BonusAmount:       s.bonusAmount.Round(2).InexactFloat64(),
		CurBalance:        decimal.NewFromFloat(s.member.Balance).Sub(s.amount).Add(s.bonusAmount).Round(2).InexactFloat64(),
		OrderSn:           orderSn.OrderSN,
		ParentOrderSn:     orderSn.ParentOrderSN,
		FreeOrderSn:       orderSn.FreeOrderSN,
		State:             1,
		BonusTimes:        s.scene.ContinueNum,
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
	if s.gameOrder.BetRawDetail, err = json.CJSON.MarshalToString(s.symbolGrid); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal symbolGrid", zap.Error(err))
		return err
	}
	if s.gameOrder.BonusRawDetail, err = json.CJSON.MarshalToString(s.winGrid); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal winGridReward", zap.Error(err))
		return err
	}
	if s.gameOrder.WinDetails, err = json.CJSON.MarshalToString(s.getWinDetails()); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return err
	}
	return nil
}

func (s *betOrderService) getWinDetails() *WinDetails {
	winArr := make([]*pb.Pjcd_WinArr, len(s.winInfos))
	for i, elem := range s.winInfos {
		winArr[i] = &pb.Pjcd_WinArr{
			RoadNum: proto.Int64(elem.LineCount),
			Odds:    proto.Int64(elem.Odds),
		}
	}

	return &WinDetails{
		FreeWin:           s.scene.FreeWin,
		RoundWin:          s.scene.RoundWin,
		TotalWin:          s.scene.TotalWin,
		IsRoundOver:       s.isRoundOver,
		NewFreeTimes:      s.addFreeTime,
		MulIndex:          s.gameMultipleIndex,
		StepMultiplier:    s.stepMultiplier,
		WildEliCount:      s.addWildEliCount,
		TotalWildEliCount: s.scene.TotalWildEliCount,
		WildEliMultiple:   s.wildMultiplier,
		WinArr:            winArr,
	}
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
