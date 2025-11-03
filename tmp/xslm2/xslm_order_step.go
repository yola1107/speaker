package xslm2

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"

	"egame-grpc/global"
	"egame-grpc/strategy"
	"egame-grpc/utils/json"
	"egame-grpc/utils/snow"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// ========== 步骤初始化 ==========

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

// ========== 首局初始化 ==========

func (s *betOrderService) initStepForFirstStep() error {
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

func (s *betOrderService) initPresetForFirstStep() bool {
	switch {
	case _presetID > 0:
		return s.rdbGetPresetByID(_presetID)
	case !s.initProb():
		return false
	case !s.initPresetExpectedParam():
		return false
	case !s.rdbGetPresetIDByExpectedParam():
		return false
	default:
		return s.rdbGetPresetByID(s.presetID)
	}
}

func (s *betOrderService) initProb() bool {
	switch {
	case global.GVA_DYNAMIC_PROB == nil:
		global.GVA_LOG.Error("initProb", zap.Error(errors.New("game dynamic prob is nil")))
		return false
	case global.GVA_DYNAMIC_PROB[_gameID] == nil:
		err := fmt.Errorf("game dynamic prob is nil")
		global.GVA_LOG.Error("initProb", zap.Error(err))
		return false
	case global.GVA_DYNAMIC_PROB[_gameID][s.gameType] == nil:
		err := fmt.Errorf("game dynamic prob is nil: [%v]", s.gameType)
		global.GVA_LOG.Error("initProb", zap.Error(err))
		return false
	}
	probMap := global.GVA_DYNAMIC_PROB[_gameID][s.gameType]
	var muls []int64
	weightSum := int64(0)
	for _, value := range probMap {
		weightSum += value.Probability
		muls = append(muls, value.Multiple)
	}
	if weightSum <= 0 {
		global.GVA_LOG.Error("initProb", zap.Error(errors.New("invalid weights")))
		return false
	}
	sort.Slice(muls, func(i, j int) bool {
		return muls[i] < muls[j]
	})
	s.probMap = probMap
	s.probMultipliers = muls
	s.probWeightSum = weightSum
	return true
}

func (s *betOrderService) initPresetExpectedParam() bool {
	r := randPool.Get().(*rand.Rand)
	defer randPool.Put(r)
	num := r.Int63n(s.probWeightSum)
	sum := int64(0)
	for _, mul := range s.probMultipliers {
		sum += s.probMap[mul].Probability
		if num < sum {
			s.expectedMultiplier = mul
			switch {
			case mul >= _maxMultiplierForBaseOnly || r.Int63n(10_000) < s.probMap[mul].FreeProbability:
				s.presetKind = _presetKindNormalFree
			default:
				s.presetKind = _presetKindNormalBase
			}
			return true
		}
	}
	return false
}

// ========== 后续局初始化 ==========

func (s *betOrderService) initStepForNextStep() error {
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

func (s *betOrderService) initPresetForNextStep() bool {
	lastPresetID := s.client.ClientOfFreeGame.GetLastWinId()
	return s.rdbGetPresetByID(int64(lastPresetID))
}

// ========== 预设数据初始化 ==========

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

// ========== 步骤结果更新 ==========

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

func (s *betOrderService) updateCurrentBalance() {
	currBalance := decimal.NewFromFloat(s.member.Balance).
		Sub(s.amount).
		Add(s.bonusAmount)
	s.currBalance = currBalance
}

func (s *betOrderService) finalize() {
	switch {
	case s.amount.GreaterThan(decimal.Zero):
		s.strategy.Update(s.betAmount.Round(2).InexactFloat64(), s.gameOrder.Amount, s.gameOrder.BonusAmount)
	default:
		s.strategy.Update(0, 0, s.gameOrder.BonusAmount)
	}
}
