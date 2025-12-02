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

	s.symbolGrid = s.handleSymbolGrid()
	s.winInfos, s.winGrid = s.checkSymbolGridWin(s.symbolGrid)

	s.processWinInfos()
	return nil
}

func (s *betOrderService) processWinInfos() {
	runState := runStateNormal
	if s.isFreeRound {
		runState = runStateFreeGame
	}
	s.winData = winData{State: runState, WinArr: make([]WinElem, 0, len(s.winInfos))}

	if len(s.winInfos) > 0 {
		s.processWin()
	} else {
		s.processNoWin()
	}

	s.fillResultCommonFields()
}

func (s *betOrderService) processWin() {
	s.gameMultiple = s.getStreakMultiplier()
	s.lineMultiplier = s.handleWinInfosMultiplier(s.winInfos)
	s.stepMultiplier = s.lineMultiplier * s.gameMultiple

	s.scene.RoundMultiplier += s.stepMultiplier
	s.winData.Multi = s.scene.RoundMultiplier
	s.updateBonusAmount(s.stepMultiplier) // 更新奖金 倍数=stepMultiplier

	s.cardTypes = make([]CardType, 0, len(s.winInfos))
	for _, info := range s.winInfos {
		s.winData.WinArr = append(s.winData.WinArr, WinElem{
			Val:     info.Symbol,
			RoadNum: info.LineCount,
			StarNum: info.SymbolCount,
			Odds:    info.Odds,
			Mul:     info.Multiplier,
			Loc:     info.WinGrid,
		})
		s.cardTypes = append(s.cardTypes, CardType{
			Type:     int(info.Symbol),
			Way:      int(info.LineCount),
			Multiple: int(info.Odds),
			Route:    int(info.SymbolCount),
		})
	}

	s.scene.Steps++
	s.isRoundOver = false
	s.scene.ContinueNum++

	s.nextSymbolGrid = s.moveSymbols()
	s.winData.Next = true
	s.fallingWinSymbols(s.nextSymbolGrid)

	if s.isFreeRound {
		s.scene.NextStage = _spinTypeFreeEli
	} else {
		s.scene.NextStage = _spinTypeBaseEli
	}
}

func (s *betOrderService) processNoWin() {
	s.gameMultiple = 1
	s.lineMultiplier = 0
	s.stepMultiplier = 0
	s.scene.Steps = 0
	s.scene.ContinueNum = 0
	s.isRoundOver = true
	s.cardTypes = nil
	s.scatterCount = s.getScatterCount()

	s.updateBonusAmount(0) // 更新奖金 倍数=0
	s.client.ClientOfFreeGame.SetLastWinId(0)
	s.winData.ScatterCount = s.scatterCount

	if s.isFreeRound {
		if trigger, newFreeRoundCount := s.checkNewFreeGameNum(s.scatterCount); trigger {
			s.client.ClientOfFreeGame.Incr(uint64(newFreeRoundCount))
			s.scene.FreeNum += newFreeRoundCount
			s.winData.AddFreeTime = newFreeRoundCount
		}

		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--

		if s.scene.FreeNum <= 0 {
			s.cleanupFreeGameState()
			s.setNextStage(_spinTypeBase, runStateNormal, false)
		} else {
			s.setNextStage(_spinTypeFree, runStateFreeGame, true)
		}

	} else {
		if s.scatterCount < int64(s.gameConfig.FreeGameScatterMin) {
			s.cleanupFreeGameState()
			s.setNextStage(_spinTypeBase, runStateNormal, false)
			return
		}

		s.scene.ScatterNum = s.scatterCount
		s.scene.BonusState = _bonusStatePending

		if s.debug.open {
			s.autoSetupBonusNumAndFreeTimes(s.scatterCount)
		}

		// 根据 BonusNum 是否已设置决定下一步
		if s.scene.BonusNum > 0 {
			s.setNextStage(_spinTypeFree, runStateFreeGame, true)
		} else {
			s.setNextStage(_spinTypeBase, runStateNormal, false)
		}
	}
}

func (s *betOrderService) setNextStage(stage int8, state int8, next bool) {
	s.scene.NextStage = stage
	s.winData.State = state
	s.winData.Next = next
}

func (s *betOrderService) cleanupFreeGameState() {
	s.scene.BonusNum = 0
	s.scene.FreeNum = 0
	s.scene.ScatterNum = 0
	s.scene.BonusState = 0
}

func (s *betOrderService) fillResultCommonFields() {
	s.winData.FreeNum = uint64(s.scene.FreeNum)
	s.winData.FreeTime = s.client.ClientOfFreeGame.GetFreeTimes()
	s.winData.TotalFreeTime = s.winData.FreeNum + s.winData.FreeTime
	s.winData.Over = !s.winData.Next
	s.winData.IsRoundOver = s.isRoundOver
	s.winData.WinGrid = convertToRewardGrid(s.winGrid)
	s.winData.NextSymbolGrid = s.nextSymbolGrid
	s.winData.FreeMultiple = s.gameMultiple
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
