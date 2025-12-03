package mahjong4

import (
	"egame-grpc/global"

	"go.uber.org/zap"
)

func (s *betOrderService) baseSpin() error {
	// RTP 测试模式：直接调用时需要手动进行状态转换
	if s.debug.open {
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
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
	s.gameMultiple = s.getStreakMultiplier()
	s.lineMultiplier = s.handleWinElemsMultiplier(s.winInfos)
	s.stepMultiplier = s.lineMultiplier * s.gameMultiple
	s.isRoundOver = false

	s.scene.Steps++
	s.scene.ContinueNum++
	s.scene.RoundMultiplier += s.stepMultiplier

	s.nextSymbolGrid = s.moveSymbols()
	s.fallingWinSymbols(s.nextSymbolGrid)

	if s.isFreeRound {
		s.scene.NextStage = _spinTypeFreeEli
	} else {
		s.scene.NextStage = _spinTypeBaseEli
	}
	s.updateBonusAmount(s.stepMultiplier)
}

func (s *betOrderService) processNoWin() {
	s.gameMultiple = 1
	s.lineMultiplier = 0
	s.stepMultiplier = 0
	s.isRoundOver = true
	s.scatterCount = s.getScatterCount()

	s.scene.Steps = 0
	s.scene.ContinueNum = 0

	s.updateBonusAmount(0)
	s.client.ClientOfFreeGame.SetLastWinId(0)

	if s.isFreeRound {
		if trigger, newFreeRoundCount := s.checkNewFreeGameNum(s.scatterCount); trigger {
			s.client.ClientOfFreeGame.Incr(uint64(newFreeRoundCount))
			s.scene.FreeNum += newFreeRoundCount
			s.addFreeTime = newFreeRoundCount
		}

		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--

		if s.scene.FreeNum <= 0 {
			s.cleanupFreeGameState()
			s.scene.NextStage = _spinTypeBase
		} else {
			s.scene.NextStage = _spinTypeFree
		}

	} else {
		if s.scatterCount < int64(s.gameConfig.FreeGameScatterMin) {
			s.cleanupFreeGameState()
			s.scene.NextStage = _spinTypeBase
			return
		}

		s.scene.ScatterNum = s.scatterCount
		s.scene.BonusState = _bonusStatePending

		if s.debug.open {
			s.setupBonusNumAndFreeTimes(s.scatterCount, _bonusNum3)
		}

		// 根据 BonusNum 是否已设置决定下一步
		if s.scene.BonusNum > 0 {
			s.addFreeTime = s.scene.FreeNum
			s.scene.NextStage = _spinTypeFree
		} else {
			s.scene.NextStage = _spinTypeBase
		}
	}
}

func (s *betOrderService) cleanupFreeGameState() {
	s.scene.BonusNum = 0
	s.scene.FreeNum = 0
	s.scene.ScatterNum = 0
	s.scene.BonusState = 0
}

func (s *betOrderService) handleWinElemsMultiplier(elems []WinInfo) int64 {
	var stepMultiplier int64
	for _, elem := range elems {
		stepMultiplier += elem.Multiplier
	}
	return stepMultiplier
}

func (s *betOrderService) getStreakMultiplier() int64 {
	multiples := s.gameConfig.BaseStreakMulti
	if s.isFreeRound {
		bonusItem, ok := s.gameConfig.FreeBonusMap[s.scene.BonusNum]
		if !ok {
			global.GVA_LOG.Error("getStreakMultiplier: BonusNum not found", zap.Any("scene", s.scene))
			return 1
		}
		multiples = bonusItem.Multi
	}
	index := min(s.scene.ContinueNum, int64(len(multiples)-1))
	return int64(multiples[index])
}
