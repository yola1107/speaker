package xslm2

import (
	"time"

	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/model/pool"
	"egame-grpc/utils/json"

	"go.uber.org/zap"
)

// updateGameOrder 更新游戏订单
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
		LineMultiple:      s.spin.lineMultiplier,
		BonusHeadMultiple: 1,
		BonusMultiple:     s.spin.stepMultiplier,
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
		HuNum:             s.spin.treasureCount,
		FreeNum:           s.spin.newFreeRoundCount,
		FreeTimes:         int64(s.client.ClientOfFreeGame.GetFreeTimes()),
	}
	if s.isFreeRound {
		gameOrder.IsFree = 1
	}
	s.gameOrder = &gameOrder
	return s.fillInGameOrderDetails()
}

// fillInGameOrderDetails 填充订单详情
func (s *betOrderService) fillInGameOrderDetails() bool {
	betRawDetail, err := json.CJSON.MarshalToString(s.spin.symbolGrid)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return false
	}
	s.gameOrder.BetRawDetail = betRawDetail
	winRawDetail, err := json.CJSON.MarshalToString(s.spin.winGrid)
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

// getWinDetailsMap 获取中奖详情map
func (s *betOrderService) getWinDetailsMap() map[string]any {
	var winDetailsMap = make(map[string]any)
	winDetailsMap["orderSN"] = s.gameOrder.OrderSn
	winDetailsMap["isFreeRound"] = s.isFreeRound
	winDetailsMap["femaleCountsForFree"] = s.spin.femaleCountsForFree
	winDetailsMap["enableFullElimination"] = s.spin.enableFullElimination
	winDetailsMap["symbolGrid"] = s.spin.symbolGrid
	winDetailsMap["winGrid"] = s.spin.winGrid
	winDetailsMap["winResults"] = s.spin.winResults
	winDetailsMap["baseBet"] = s.req.BaseMoney
	winDetailsMap["multiplier"] = s.req.Multiple
	winDetailsMap["betAmount"] = s.betAmount.Round(2).InexactFloat64()
	winDetailsMap["bonusAmount"] = s.bonusAmount.Round(2).InexactFloat64()
	winDetailsMap["spinBonusAmount"] = s.client.ClientOfFreeGame.GetGeneralWinTotal()
	winDetailsMap["freeBonusAmount"] = s.client.ClientOfFreeGame.GetFreeTotalMoney()
	winDetailsMap["roundBonus"] = s.client.ClientOfFreeGame.RoundBonus
	winDetailsMap["currentBalance"] = s.gameOrder.CurBalance
	winDetailsMap["isRoundOver"] = s.spin.isRoundOver
	winDetailsMap["hasFemaleWin"] = s.spin.hasFemaleWin
	winDetailsMap["newFreeRoundCount"] = s.spin.newFreeRoundCount
	winDetailsMap["totalFreeRoundCount"] = s.client.GetLastMaxFreeNum()
	winDetailsMap["remainingFreeRoundCount"] = s.client.ClientOfFreeGame.GetFreeNum()
	winDetailsMap["lineMultiplier"] = s.spin.lineMultiplier
	winDetailsMap["stepMultiplier"] = s.spin.stepMultiplier
	return winDetailsMap
}

// settleStep 结算步骤
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
