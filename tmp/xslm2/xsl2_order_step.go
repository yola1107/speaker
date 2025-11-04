package xslm2

import (
	"fmt"
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

// getRequestContext 获取请求上下文（商户、用户、游戏信息）
func (s *betOrderService) getRequestContext() bool {
	return s.mdbGetMerchant() && s.mdbGetMember() && s.mdbGetGame()
}

// selectGameRedis 选择游戏Redis（根据gameID取模）
func (s *betOrderService) selectGameRedis() {
	if s.debug.open {
		return // RTP测试模式不需要Redis
	}
	index := _gameID % int64(len(global.GVA_GAME_REDIS))
	s.gameRedis = global.GVA_GAME_REDIS[index]
}

// updateBetAmount 计算下注金额
func (s *betOrderService) updateBetAmount() bool {
	// 校验参数
	if !_cnf.validateBetSize(s.req.BaseMoney) {
		global.GVA_LOG.Warn("invalid baseMoney", zap.Float64("value", s.req.BaseMoney))
		return false
	}
	if !_cnf.validateBetLevel(s.req.Multiple) {
		global.GVA_LOG.Warn("invalid multiple", zap.Int64("value", s.req.Multiple))
		return false
	}

	s.betAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_cnf.BaseBat))
	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount", zap.Error(fmt.Errorf("invalid params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
		return false
	}
	return true
}

// checkBalance 检查余额
func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

// updateBonusAmount 计算奖金
func (s *betOrderService) updateBonusAmount() {
	if s.spin.stepMultiplier <= 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	s.bonusAmount = s.betAmount.Div(decimal.NewFromInt(_cnf.BaseBat)).Mul(decimal.NewFromInt(s.spin.stepMultiplier))
}

// symbolGridToString 符号网格转字符串
func (s *betOrderService) symbolGridToString() string {
	builder := ""
	for r := int64(0); r < _rowCount; r++ {
		builder += "["
		for c := int64(0); c < _colCount; c++ {
			builder += strconv.FormatInt(s.spin.symbolGrid[r][c], 10)
			if c < _colCount-1 {
				builder += ","
			}
		}
		builder += "]"
		if r < _rowCount-1 {
			builder += "\n"
		}
	}
	return builder
}

// winGridToString 中奖网格转字符串
func (s *betOrderService) winGridToString() string {
	builder := ""
	for r := int64(0); r < _rowCount; r++ {
		builder += "["
		for c := int64(0); c < _colCount; c++ {
			builder += strconv.FormatInt(s.spin.winGrid[r][c], 10)
			if c < _colCount-1 {
				builder += ","
			}
		}
		builder += "]"
		if r < _rowCount-1 {
			builder += "\n"
		}
	}
	return builder
}

// initialize 初始化step数据
func (s *betOrderService) initialize() error {
	s.client.ClientOfFreeGame.ResetFreeClean()
	s.orderSN = strconv.FormatInt(snow.GenarotorID(s.member.ID), 10)
	var err error
	if s.isFirst {
		err = s.initStepForFirstStep()
	} else {
		err = s.initStepForNextStep()
	}
	if err != nil {
		return err
	}
	return nil
}

// initStepForFirstStep 初始化首次step（回合第一局）
func (s *betOrderService) initStepForFirstStep() error {
	if !s.updateBetAmount() {
		return InvalidRequestParams
	}
	if !s.checkBalance() {
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

// initStepForNextStep 初始化后续step（回合内的连消step）
func (s *betOrderService) initStepForNextStep() error {
	s.req.BaseMoney = s.lastOrder.BaseAmount
	s.req.Multiple = s.lastOrder.Multiple
	s.betAmount = decimal.NewFromFloat(s.client.ClientOfFreeGame.GetBetAmount())
	s.amount = decimal.Zero
	if s.client.IsRoundOver {
		s.isFreeRound = true
		s.client.ClientOfFreeGame.ResetRoundBonus()
		if s.lastOrder.FreeOrderSn != "" {
			s.freeOrderSN = s.lastOrder.FreeOrderSn
		} else if s.lastOrder.ParentOrderSn != "" {
			s.freeOrderSN = s.lastOrder.ParentOrderSn
		} else {
			s.freeOrderSN = s.lastOrder.OrderSn
		}
	} else {
		s.isFreeRound = s.lastOrder.IsFree > 0
		if s.lastOrder.ParentOrderSn != "" {
			s.parentOrderSN = s.lastOrder.ParentOrderSn
		} else {
			s.parentOrderSN = s.lastOrder.OrderSn
		}
		s.freeOrderSN = s.lastOrder.FreeOrderSn
	}

	return nil
}

// updateCurrentBalance 更新当前余额
func (s *betOrderService) updateCurrentBalance() {
	s.currBalance = decimal.NewFromFloat(s.member.Balance).Sub(s.amount).Add(s.bonusAmount)
}

// updateStepResult 更新step结果
func (s *betOrderService) updateStepResult() {
	s.client.IsRoundOver = s.spin.isRoundOver
	if s.spin.stepMultiplier > 0 {
		s.updateBonusAmount()
		bonus := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(bonus)
		if s.isFreeRound {
			s.client.ClientOfFreeGame.IncrFreeTotalMoney(bonus)
		}
		s.client.ClientOfFreeGame.IncRoundBonus(bonus)
	}
	if s.isFreeRound && s.client.IsRoundOver {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
	}
	if s.spin.newFreeRoundCount > 0 {
		freeCount := uint64(s.spin.newFreeRoundCount)
		if !s.isFreeRound {
			s.client.ClientOfFreeGame.SetFreeNum(freeCount)
			s.client.SetLastMaxFreeNum(freeCount)
		} else {
			s.client.ClientOfFreeGame.Incr(freeCount)
			s.client.IncLastMaxFreeNum(freeCount)
		}
	}
	s.updateCurrentBalance()
}

// getBetResultMap 获取下注结果（返回给前端）
func (s *betOrderService) getBetResultMap() map[string]any {
	return map[string]any{
		"orderSN":                 s.gameOrder.OrderSn,
		"isFreeRound":             s.isFreeRound,
		"femaleCountsForFree":     s.spin.femaleCountsForFree,
		"enableFullElimination":   s.spin.enableFullElimination,
		"symbolGrid":              s.spin.symbolGrid,
		"winGrid":                 s.spin.winGrid,
		"winResults":              s.spin.winResults,
		"baseBet":                 s.req.BaseMoney,
		"multiplier":              s.req.Multiple,
		"betAmount":               s.betAmount.Round(2).InexactFloat64(),
		"bonusAmount":             s.bonusAmount.Round(2).InexactFloat64(),
		"spinBonusAmount":         s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		"freeBonusAmount":         s.client.ClientOfFreeGame.GetFreeTotalMoney(),
		"roundBonus":              s.client.ClientOfFreeGame.RoundBonus,
		"currentBalance":          s.gameOrder.CurBalance,
		"isRoundOver":             s.spin.isRoundOver,
		"hasFemaleWin":            s.spin.hasFemaleWin,
		"nextFemaleCountsForFree": s.spin.nextFemaleCountsForFree,
		"newFreeRoundCount":       s.spin.newFreeRoundCount,
		"totalFreeRoundCount":     s.client.GetLastMaxFreeNum(),
		"remainingFreeRoundCount": s.client.ClientOfFreeGame.GetFreeNum(),
		"lineMultiplier":          s.spin.lineMultiplier,
		"stepMultiplier":          s.spin.stepMultiplier,
	}
}

// ========== 订单更新和结算 ==========

// updateGameOrder 创建游戏订单
func (s *betOrderService) updateGameOrder() bool {
	gameOrder := game.GameOrder{
		MerchantID:        s.merchant.ID,
		Merchant:          s.merchant.Merchant,
		MemberID:          s.member.ID,
		Member:            s.member.MemberName,
		GameID:            s.game.ID,
		GameName:          s.game.GameName,
		BaseMultiple:      _cnf.BaseBat,
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

// fillInGameOrderDetails 填充订单详情（序列化数据）
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

// getWinDetailsMap 获取中奖详情map（返回给前端）
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

// settleStep 结算step（保存订单到数据库）
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
