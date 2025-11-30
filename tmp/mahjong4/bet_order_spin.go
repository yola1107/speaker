package mahjong

import (
	"go.uber.org/zap"

	"egame-grpc/global"
)

// baseSpin 核心旋转逻辑
func (s *betOrderService) baseSpin() (*BaseSpinResult, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}

	// 状态验证
	s.handleStageTransition()

	// 新 Round 初始化：生成符号、重置计数
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.scene.SymbolRoller = s.initSpinSymbol()
	}

	// 生成符号网格并检测中奖
	s.symbolGrid = s.handleSymbolGrid()
	s.winInfos, s.winGrid = s.checkSymbolGridWin(s.symbolGrid)

	// 处理消除和结果返回
	result := s.precessWinInfos()
	return result, nil
}

func (s *betOrderService) precessWinInfos() *BaseSpinResult {
	runState := runStateNormal
	if s.isFreeRound {
		runState = runStateFreeGame
	}
	result := &BaseSpinResult{
		winGrid:        toWinGridReward(s.winGrid),
		cards:          s.symbolGrid,
		nextSymbolGrid: int64Grid{},
		winInfo:        WinInfo{State: runState, WinArr: make([]WinElem, 0, len(s.winInfos))},
	}

	if len(s.winInfos) > 0 {
		s.processWin(result)
	} else {
		s.processNoWin(result)
	}

	s.fillResultCommonFields(result)
	return result
}

func (s *betOrderService) processWin(result *BaseSpinResult) {
	// 计算倍数
	lineMultiplier := s.handleWinInfosMultiplier(s.winInfos)
	gameMultiple := s.getStreakMultiplier(s.isFreeRound)
	stepMultiplier := lineMultiplier * gameMultiple
	result.lineMultiplier = lineMultiplier
	result.gameMultiple = gameMultiple
	result.stepMultiplier = stepMultiplier
	result.bonusHeadMultiple = gameMultiple
	s.gameMultiple = gameMultiple
	s.stepMultiplier = stepMultiplier

	// 更新奖金
	s.scene.RoundMultiplier += stepMultiplier
	result.winInfo.Multi = s.scene.RoundMultiplier
	s.updateSpinBonusAmount(s.updateBonusAmount(stepMultiplier))

	// 填充中奖信息
	result.winResult = make([]CardType, 0, len(s.winInfos))
	for _, info := range s.winInfos {
		result.winInfo.WinArr = append(result.winInfo.WinArr, WinElem{
			Val:     info.Symbol,
			RoadNum: info.LineCount,
			StarNum: info.SymbolCount,
			Odds:    info.Odds,
			Mul:     info.Multiplier,
			Loc:     info.WinGrid,
		})
		result.winResult = append(result.winResult, CardType{
			Type:     int(info.Symbol),
			Way:      int(info.LineCount),
			Multiple: int(info.Odds),
			Route:    int(info.SymbolCount),
		})
	}

	// 消除及场景数据处理
	s.scene.Steps++
	s.isRoundOver = false
	s.scene.ContinueNum++

	s.nextSymbolGrid = s.moveSymbols()
	result.nextSymbolGrid = s.nextSymbolGrid
	result.winInfo.Next = true
	s.fallingWinSymbols(s.nextSymbolGrid, s.scene.Stage)
	/*	// fallingWinSymbols 会填充新符号，需要更新 nextSymbolGrid 以反映填充后的状态
		// 从更新后的 BoardSymbol 重新生成 nextSymbolGrid
		s.nextSymbolGrid = s.handleSymbolGrid()
		result.nextSymbolGrid = s.nextSymbolGrid*/

	// 设置下一阶段（消除状态）
	if s.isFreeRound {
		s.scene.NextStage = _spinTypeFreeEli
	} else {
		s.scene.NextStage = _spinTypeBaseEli
	}
}

func (s *betOrderService) processNoWin(result *BaseSpinResult) {
	scatterCount := s.getScatterCount()
	result.bonusHeadMultiple = s.gameMultiple
	result.bonusTimes = s.scene.ContinueNum
	result.nextSymbolGrid = s.symbolGrid
	result.scatterCount = scatterCount
	result.winInfo.ScatterCount = scatterCount

	// 更新奖金 倍数=0
	s.stepMultiplier = 0
	result.stepMultiplier = 0
	result.lineMultiplier = 0
	s.updateSpinBonusAmount(s.updateBonusAmount(0))

	s.scene.Steps = 0
	s.scene.ContinueNum = 0
	s.gameMultiple = 1
	s.isRoundOver = true
	s.client.ClientOfFreeGame.SetLastWinId(0)

	if s.isFreeRound {
		if scatterCount > 0 {
			if bonusItem, ok := s.gameConfig.FreeBonusMap[s.scene.BonusNum]; ok {
				newFreeRoundCount := int64(bonusItem.AddTimes) * scatterCount
				result.addFreeTime = newFreeRoundCount
				result.freeTime = newFreeRoundCount
				s.client.ClientOfFreeGame.Incr(uint64(newFreeRoundCount))
				s.scene.FreeNum += newFreeRoundCount
			}
		}

		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--

		if s.scene.FreeNum <= 0 {
			// 免费游戏结束，清理状态
			s.cleanupFreeGameState()
			s.setNextStage(result, _spinTypeBase, runStateNormal, false, true)
		} else {
			s.setNextStage(result, _spinTypeFree, runStateFreeGame, true, false)
		}

	} else {
		// 基础模式：处理免费游戏触发
		if scatterCount < int64(s.gameConfig.FreeGameScatterMin) {
			// 未触发免费游戏，清理状态
			s.cleanupFreeGameState()
			s.setNextStage(result, _spinTypeBase, runStateNormal, false, true)
			return
		}

		// 满足触发条件：设置 BonusState 和 ScatterNum
		s.scene.ScatterNum = scatterCount
		s.scene.BonusState = _bonusStatePending // 标记等待客户端选择

		// Debug 模式自动设置 BonusNum
		if s.debug.open {
			s.setupBonusNumAndFreeTimes(scatterCount, _bonusNum3)
		}

		// 根据 BonusNum 是否已设置决定下一步
		if s.scene.BonusNum > 0 {
			// 已选择（Debug 模式或客户端已选择），进入免费游戏
			s.setNextStage(result, _spinTypeFree, runStateFreeGame, true, false)
		} else {
			// 等待客户端选择 BonusNum
			s.setNextStage(result, _spinTypeBase, runStateNormal, false, true)
		}
	}
}

func (s *betOrderService) setNextStage(result *BaseSpinResult, stage int8, state int8, next, spinOver bool) {
	s.scene.NextStage = stage
	result.winInfo.State = state
	result.winInfo.Next = next
	result.SpinOver = spinOver
}

// cleanupFreeGameState 清理免费游戏相关状态
func (s *betOrderService) cleanupFreeGameState() {
	s.scene.BonusNum = 0
	s.scene.FreeNum = 0
	s.scene.ScatterNum = 0
	s.scene.BonusState = 0 // 清理状态，重置为 0
}

func (s *betOrderService) fillResultCommonFields(result *BaseSpinResult) {
	result.gameMultiple = s.gameMultiple
	result.winInfo.FreeNum = uint64(s.scene.FreeNum)
	result.winInfo.FreeTime = s.client.ClientOfFreeGame.GetFreeTimes()
	result.winInfo.TotalFreeTime = result.winInfo.FreeNum + result.winInfo.FreeTime
	result.winInfo.Over = !result.winInfo.Next
	result.winInfo.IsRoundOver = s.isRoundOver
	result.winInfo.AddFreeTime = result.addFreeTime
	result.winInfo.WinGrid = result.winGrid // 复用已设置的 winGrid
	result.winInfo.NextSymbolGrid = s.nextSymbolGrid
	result.winInfo.FreeMultiple = s.gameMultiple
	result.stepWin = s.bonusAmount.Round(2).InexactFloat64()
}

// getStreakMultiplier 计算连续消除倍数
func (s *betOrderService) getStreakMultiplier(isFreeRound bool) int64 {
	multiples := s.gameConfig.BaseStreakMulti
	if isFreeRound {
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
