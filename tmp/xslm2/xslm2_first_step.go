package xslm2

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"egame-grpc/global"

	"go.uber.org/zap"
)

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
