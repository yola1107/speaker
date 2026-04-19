package hcsqy

import (
	"time"

	"egame-grpc/game/common"
	"egame-grpc/game/hcsqy/pb"
	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func (s *betOrderService) initialize() error {
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

	s.scene.Reset()
	//s.amount = s.betAmount
	if s.req.Purchase > 0 {
		s.scene.IsPurchase = true
		s.scene.PurchaseAmount = s.req.Purchase
		s.scene.NextStage = 0
		s.isFreeRound = false
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
	s.betAmount = decimal.NewFromFloat(s.lastOrder.BaseAmount * float64(s.lastOrder.BaseMultiple*s.lastOrder.Multiple))
	s.amount = decimal.Zero
	return nil
}

func (s *betOrderService) updateGameOrder() error {
	orderSn := &common.OrderSN{}
	if !s.debug.open {
		isBaseStage := s.scene.Stage == _spinTypeBase
		isFreeStage := s.scene.Stage == _spinTypeFree
		orderSn = common.GenerateOrderSn(s.member, s.lastOrder, isBaseStage, isFreeStage)
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
		HuNum:             1,
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
	winArr := make([]*pb.Hcsqy_WinArr, len(s.winInfos))
	for i, elem := range s.winInfos {
		winArr[i] = &pb.Hcsqy_WinArr{
			RoadNum: proto.Int64(elem.LineCount),
			Odds:    proto.Int64(elem.Odds),
		}
	}

	return &WinDetails{
		FreeWin:          s.scene.FreeWin,
		TotalWin:         s.scene.TotalWin,
		IsRoundOver:      s.isRoundOver,
		NewFreeTimes:     s.addFreeTime,
		IsPurchase:       s.stepIsPurchase,
		Next:             s.next,
		IsRespinUntilWin: s.stepIsRespinMode,
		RespinWildCol:    s.respinWildCol,
		WildMultiplier:   s.wildMultiplier,
		LineMultiplier:   s.lineMultiplier,
		ScatterCount:     s.scatterCount,
		WinArr:           winArr,
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
