package xslm2

import (
	"fmt"
	"strconv"
	"time"

	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/model/pool"
	"egame-grpc/strategy"
	"egame-grpc/utils/json"
	"egame-grpc/utils/snow"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// ========== 步骤结果更新函数（基础模式和免费模式） ==========

// updateStepResultForBase 更新基础模式步骤结果
func (s *betOrderService) updateStepResultForBase() {
	s.client.IsRoundOver = s.spin.isRoundOver
	if s.spin.stepMultiplier > 0 {
		s.updateBonusAmount()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncRoundBonus(s.bonusAmount.Round(2).InexactFloat64())
	}
	if s.spin.newFreeRoundCount > 0 {
		s.client.ClientOfFreeGame.SetFreeNum(uint64(s.spin.newFreeRoundCount))
		s.client.SetLastMaxFreeNum(uint64(s.spin.newFreeRoundCount))
	}
	lastMapID := s.client.ClientOfFreeGame.GetLastMapId()
	freeNum := s.client.ClientOfFreeGame.GetFreeNum()
	switch {
	case lastMapID > 0 && s.client.IsRoundOver && freeNum == 0:
		s.showPostUpdateErrorLog()
	case lastMapID == 0 && (!s.client.IsRoundOver || freeNum > 0):
		s.showPostUpdateErrorLog()
	}
}

// updateStepResultForFree 更新免费模式步骤结果
func (s *betOrderService) updateStepResultForFree() {
	if s.client.IsRoundOver {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
	}
	s.client.IsRoundOver = s.spin.isRoundOver
	if s.spin.stepMultiplier > 0 {
		s.updateBonusAmount()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncrFreeTotalMoney(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncRoundBonus(s.bonusAmount.Round(2).InexactFloat64())
	}
	if s.client.IsRoundOver && s.spin.newFreeRoundCount > 0 {
		s.client.ClientOfFreeGame.Incr(uint64(s.spin.newFreeRoundCount))
		s.client.IncLastMaxFreeNum(uint64(s.spin.newFreeRoundCount))
	}
	lastMapID := s.client.ClientOfFreeGame.GetLastMapId()
	freeNum := s.client.ClientOfFreeGame.GetFreeNum()
	switch {
	case lastMapID > 0 && s.client.IsRoundOver && freeNum == 0:
		s.showPostUpdateErrorLog()
	case lastMapID == 0 && (!s.client.IsRoundOver || freeNum > 0):
		s.showPostUpdateErrorLog()
	}
}

// ==========

func (s *betOrderService) initialize() error {
	s.client.ClientOfFreeGame.ResetFreeClean()
	s.orderSN = strconv.FormatInt(snow.GenarotorID(s.member.ID), 10)
	var err error
	switch {
	case s.isFirst:
		err = s.initStepForFirstStep()
	default:
		err = s.initStepForNextStep()
	}
	if err != nil {
		return err
	}
	s.strategy = strategy.NewStrategy(s.game.ID, s.merchant.ID, s.member.ID, false)
	s.gameType = s.strategy.Get()
	return nil
}

func (s *betOrderService) initPreset() bool {
	switch s.isFirst {
	case true:
		return s.initPresetForFirstStep()
	default:
		return s.initPresetForNextStep()
	}
}

func (s *betOrderService) initStepMap() bool {
	var stepMaps []*stepMap
	if err := json.CJSON.Unmarshal([]byte(s.spin.preset.SpinMaps), &stepMaps); err != nil {
		global.GVA_LOG.Error("initStepMap", zap.Error(err), zap.Int64("id", s.spin.preset.ID))
		return false
	}
	if len(stepMaps) == 0 {
		global.GVA_LOG.Error("initStepMap", zap.Error(fmt.Errorf("empty maps: %v", s.spin.preset.ID)))
		return false
	}
	lastMapID := s.client.ClientOfFreeGame.GetLastMapId()
	switch {
	case lastMapID < uint64(len(stepMaps))-1:
		s.saveScene(uint64(s.spin.preset.ID), uint64(stepMaps[lastMapID].ID))
	case lastMapID == uint64(len(stepMaps))-1:
		s.saveScene(0, 0)
	default:
		global.GVA_LOG.Error(
			"initStepMap",
			zap.Error(fmt.Errorf("invalid map id: %v", lastMapID)),
			zap.Int64("id", s.spin.preset.ID),
		)
		s.saveScene(0, 0)
		return false
	}
	s.spin.stepMap = stepMaps[lastMapID]
	return true
}

func (s *betOrderService) updateStepResult() bool {
	if !s.spin.updateStepResult(s.isFreeRound) {
		return false
	}
	switch {
	case !s.isFreeRound:
		s.updateStepResultForBase()
	default:
		s.updateStepResultForFree()
	}
	s.updateCurrentBalance()
	return true
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

func (s *betOrderService) finalize() {
	switch {
	case s.amount.GreaterThan(decimal.Zero):
		s.strategy.Update(s.betAmount.Round(2).InexactFloat64(), s.gameOrder.Amount, s.gameOrder.BonusAmount)
	default:
		s.strategy.Update(0, 0, s.gameOrder.BonusAmount)
	}

}

func (s *betOrderService) updateCurrentBalance() {
	currBalance := decimal.NewFromFloat(s.member.Balance).
		Sub(s.amount).
		Add(s.bonusAmount)
	s.currBalance = currBalance
}

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
