package xslm2

import (
	"strconv"
	"time"

	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/model/pool"
	"egame-grpc/utils/json"
	"egame-grpc/utils/snow"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

func (s *betOrderService) initialize() error {
	s.client.ClientOfFreeGame.ResetFreeClean()
	if !s.debug.open {
		s.orderSN = strconv.FormatInt(snow.GenarotorID(s.member.ID), 10)
	}
	switch {
	case s.scene.Steps == 0 && s.scene.Stage == _spinTypeBase:
		return s.initStepForFirstStep()
	default:
		return s.initStepForNextStep()
	}
}

func (s *betOrderService) initStepForFirstStep() error {
	if !s.debug.open {
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
	s.client.IsRoundOver = false
	s.client.SetLastMaxFreeNum(0)
	s.client.ClientOfFreeGame.Reset()
	s.client.ClientOfFreeGame.ResetGeneralWinTotal()
	s.client.ClientOfFreeGame.ResetRoundBonus()
	s.client.ClientOfFreeGame.SetBetAmount(s.betAmount.Round(2).InexactFloat64())
	s.amount = s.betAmount

	return nil
}

func (s *betOrderService) initStepForNextStep() error {
	if s.debug.open {
		s.req.BaseMoney = 1
		s.req.Multiple = 1
		s.betAmount = decimal.NewFromFloat(s.client.ClientOfFreeGame.GetBetAmount())
		s.amount = decimal.Zero
		return nil
	}

	s.req.BaseMoney = s.lastOrder.BaseAmount
	s.req.Multiple = s.lastOrder.Multiple

	s.betAmount = decimal.NewFromFloat(s.client.ClientOfFreeGame.GetBetAmount())
	s.amount = decimal.Zero

	switch {
	case s.client.IsRoundOver:
		s.isFreeRound = true
		s.client.ClientOfFreeGame.ResetRoundBonus()
		switch {
		case s.lastOrder.FreeOrderSn != "":
			s.freeOrderSN = s.lastOrder.FreeOrderSn
		case s.lastOrder.ParentOrderSn != "":
			s.freeOrderSN = s.lastOrder.ParentOrderSn
		default:
			s.freeOrderSN = s.lastOrder.OrderSn
		}
	default:
		s.isFreeRound = s.lastOrder.IsFree > 0
		switch {
		case s.lastOrder.ParentOrderSn != "":
			s.parentOrderSN = s.lastOrder.ParentOrderSn
		default:
			s.parentOrderSN = s.lastOrder.OrderSn
		}
		s.freeOrderSN = s.lastOrder.FreeOrderSn
	}
	return nil
}

func (s *betOrderService) updateCurrentBalance() {
	s.currBalance = decimal.NewFromFloat(s.member.Balance).
		Sub(s.amount).
		Add(s.bonusAmount)
}

func (s *betOrderService) updateGameOrder() bool {
	gameOrder := game.GameOrder{
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
		CurBalance:        s.currBalance.Round(2).InexactFloat64(),
		OrderSn:           s.orderSN,
		ParentOrderSn:     s.parentOrderSN,
		FreeOrderSn:       s.freeOrderSN,
		State:             1,
		BonusTimes:        0,
		HuNum:             s.treasureCount,
		FreeNum:           s.newFreeRoundCount,
		FreeTimes:         int64(s.client.ClientOfFreeGame.GetFreeTimes()),
	}
	if s.isFreeRound {
		gameOrder.IsFree = 1
	}
	s.gameOrder = &gameOrder
	return s.fillInGameOrderDetails()
}

func (s *betOrderService) fillInGameOrderDetails() bool {
	betRawDetail, err := json.CJSON.MarshalToString(s.symbolGrid)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return false
	}
	s.gameOrder.BetRawDetail = betRawDetail
	winRawDetail, err := json.CJSON.MarshalToString(s.winGrid)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return false
	}
	s.gameOrder.BonusRawDetail = winRawDetail
	s.gameOrder.BetDetail = s.symbolGridToString()
	s.gameOrder.BonusDetail = s.winGridToString()
	winDetailsMap := s.getWinDetailsMap()
	winDetails, err := json.CJSON.MarshalToString(winDetailsMap)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return false
	}
	s.gameOrder.WinDetails = winDetails
	return true
}

func (s *betOrderService) getWinDetailsMap() map[string]any {
	var winDetailsMap = make(map[string]any)
	winDetailsMap["orderSN"] = s.gameOrder.OrderSn
	winDetailsMap["isFreeRound"] = s.isFreeRound
	winDetailsMap["femaleCountsForFree"] = s.femaleCountsForFree
	winDetailsMap["enableFullElimination"] = s.enableFullElimination
	winDetailsMap["symbolGrid"] = s.symbolGrid
	winDetailsMap["winGrid"] = s.winGrid
	winDetailsMap["winResults"] = s.winResults
	winDetailsMap["baseBet"] = s.req.BaseMoney
	winDetailsMap["multiplier"] = s.req.Multiple
	winDetailsMap["betAmount"] = s.betAmount.Round(2).InexactFloat64()
	winDetailsMap["bonusAmount"] = s.bonusAmount.Round(2).InexactFloat64()
	winDetailsMap["spinBonusAmount"] = s.client.ClientOfFreeGame.GetGeneralWinTotal()
	winDetailsMap["freeBonusAmount"] = s.client.ClientOfFreeGame.GetFreeTotalMoney()
	winDetailsMap["roundBonus"] = s.client.ClientOfFreeGame.RoundBonus
	winDetailsMap["currentBalance"] = s.gameOrder.CurBalance
	winDetailsMap["isRoundOver"] = s.isRoundOver
	winDetailsMap["hasFemaleWin"] = s.hasFemaleWin
	winDetailsMap["newFreeRoundCount"] = s.newFreeRoundCount
	winDetailsMap["totalFreeRoundCount"] = s.client.GetLastMaxFreeNum()
	winDetailsMap["remainingFreeRoundCount"] = s.client.ClientOfFreeGame.GetFreeNum()
	winDetailsMap["lineMultiplier"] = s.lineMultiplier
	winDetailsMap["stepMultiplier"] = s.stepMultiplier
	return winDetailsMap
}

func (s *betOrderService) settleStep() bool {
	poolRecord := pool.GamePoolRecord{
		OrderId:      s.gameOrder.OrderSn,
		MemberId:     s.gameOrder.MemberID,
		GameType:     1,
		GameId:       s.game.ID,
		GameName:     s.game.GameName,
		MerchantID:   s.merchant.ID,
		Merchant:     s.merchant.Merchant,
		Amount:       0,
		BeforeAmount: 0,
		AfterAmount:  0,
		EventType:    1,
		EventName:    "自然蓄水",
		EventDesc:    "",
		CreatedBy:    "SYSTEM",
	}
	s.gameOrder.CreatedAt = time.Now().Unix()
	poolRecord.CreatedAt = time.Now().Unix()
	saveParam := &gamelogic.SaveTransferParam{
		Client:      s.client,
		GameOrder:   s.gameOrder,
		MerchantOne: s.merchant,
		MemberOne:   s.member,
		Ip:          s.req.Ip,
	}
	res := gamelogic.SaveTransfer(saveParam)
	//res.CurBalance 当前余额，已兼容转账、单一
	if res.Err != nil {
		return false
	}
	return true
}
