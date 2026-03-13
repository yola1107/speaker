package sgz

import (
	"go.uber.org/zap"

	"egame-grpc/global"
)

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage() // RTP 测试模式：手动进行状态转换
	}
	if err := s.initialize(); err != nil {
		return err
	}
	// 在 Round 首 Step 时扣减免费次数（参考 mahjong 实现）
	if s.isFreeRound && s.scene.IsRoundFirstStep {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
		s.scene.IsRoundFirstStep = false // 标记已处理，避免重复执行
	}
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.scene.SymbolRoller = s.initSpinSymbol()
	}
	s.handleSymbolGrid()
	s.checkSymbolGridWin()
	s.processWinInfos()
	return nil
}

func (s *betOrderService) processWinInfos() {
	s.addFreeTime = 0 // 重置增加的免费次数
	if len(s.winInfos) > 0 {
		s.processWin()
	} else {
		s.processNoWin()
	}
}

func (s *betOrderService) processWin() {
	s.gameMultiple, s.freeGameMultiples, s.freeGameMultipleIndex = s.getStreakMultiplier()
	s.lineMultiplier = s.handleWinElemsMultiplier(s.winInfos)
	s.stepMultiplier = s.lineMultiplier * s.gameMultiple
	s.isRoundOver = false

	s.scene.Steps++
	s.scene.ContinueNum++
	s.scene.RoundMultiplier += s.stepMultiplier

	s.nextSymbolGrid = s.moveSymbols()
	s.fallingWinSymbols(s.nextSymbolGrid)

	if s.isFreeRound {
		s.scene.CityValue += s.lineMultiplier // 战斗力++
		s.scene.NextStage = _spinTypeFreeEli
	} else {
		s.scene.NextStage = _spinTypeBaseEli
	}
	s.updateBonusAmount(s.stepMultiplier)
}

func (s *betOrderService) processNoWin() {
	_, s.freeGameMultiples, s.freeGameMultipleIndex = s.getStreakMultiplier()
	s.gameMultiple = 0
	s.lineMultiplier = 0
	s.stepMultiplier = 0
	s.isRoundOver = true
	s.scatterCount = s.getScatterCount()

	s.scene.Steps = 0
	s.scene.ContinueNum = 0

	s.updateBonusAmount(0)
	s.client.ClientOfFreeGame.SetLastWinId(0)

	// 免费次数新增
	if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
		s.client.ClientOfFreeGame.Incr(uint64(newFree))
		s.scene.FreeNum += newFree
		s.addFreeTime = newFree
	}

	if s.isFreeRound {
		if s.scene.FreeNum <= 0 {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
			s.scene.IsRoundFirstStep = false // 免费模式结束

			// 判断是否解锁新英雄
			s.tryUnlockNextHero()

		} else {
			s.scene.NextStage = _spinTypeFree
			s.scene.IsRoundFirstStep = true // 下一轮免费回合的首 Step
		}
	} else {
		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
			s.scene.IsRoundFirstStep = true // 新进入免费模式，标记为首 Step

		} else {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
			s.scene.IsRoundFirstStep = false // 普通模式不需要此标志

			// 免费游戏时触发的英雄ID，使用随机英雄确定滚轴
			s.scene.FreeHeroID = s.pickFreeHeroID()
		}
	}
}

func (s *betOrderService) tryUnlockNextHero() {
	if s.scene.LastMaxUnlockHero >= _heroID8 {
		return
	}

	currMaxUnlockHero := s.scene.CurrMaxUnlockHeroID
	s.scene.LastMaxUnlockHero = currMaxUnlockHero
	kd, ok := s.gameConfig.KingdomMap[currMaxUnlockHero]
	if !ok {
		global.GVA_LOG.Error("cityUnlockHero: no such city", zap.Any("currUnlockMaxHeroID", currMaxUnlockHero))
		return
	}
	if int32(s.scene.CityValue) >= kd.GetNextCityForce() {
		s.scene.CurrMaxUnlockHeroID = int64(kd.GetNextCommanderIndex())
	}
}

func (s *betOrderService) handleWinElemsMultiplier(elems []WinInfo) int64 {
	var mul int64
	for _, elem := range elems {
		mul += elem.Odds
	}
	return mul
}

func (s *betOrderService) pickFreeHeroID() int64 {
	if s.scene.CurrMaxUnlockHeroID < _heroID1 {
		return _heroID1
	}
	if s.scene.CurrMaxUnlockHeroID >= _heroID8 {
		return _heroID8
	}
	return RandIntInclusive(_heroID1, s.scene.CurrMaxUnlockHeroID)
}
