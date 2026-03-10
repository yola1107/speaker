package pjcd

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage() // RTP 测试模式：手动进行状态转换
	}
	if err := s.initialize(); err != nil {
		return err
	}
	// 在 Round 首 Step 时扣减免费次数
	if s.isFreeRound && s.scene.IsRoundFirstStep {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
		s.scene.IsRoundFirstStep = false
	}
	// 轮轴初始化
	if s.scene.Stage == _spinTypeBase {
		// 基础模式：按间隔重新生成轮轴
		needRegen := len(s.scene.BaseReelData) == 0 ||
			s.scene.BaseReelSpinCount <= 0 ||
			s.scene.BaseReelSpinCount >= s.gameConfig.BaseReelGenerateInterval
		if needRegen {
			s.scene.BaseReelData = s.generateFullReelData(false)
			s.scene.BaseReelSpinCount = 0
		}
		s.scene.BaseReelSpinCount++
		// 溢出保护
		if s.scene.BaseReelSpinCount < 0 {
			s.scene.BaseReelSpinCount = 1
		}
		s.scene.SymbolRoller = s.getBoardFromReelData(s.scene.BaseReelData)
	} else if s.scene.Stage == _spinTypeFree && s.scene.Steps == 0 {
		// 免费模式：每轮首次生成
		if len(s.scene.FreeReelData) == 0 {
			s.scene.FreeReelData = s.generateFullReelData(true)
		}
		s.scene.SymbolRoller = s.getBoardFromReelData(s.scene.FreeReelData)
	}
	s.handleSymbolGrid()
	s.initWildStatesForNewSymbols() // 初始化新百搭为毛虫状态
	s.checkSymbolGridWin()
	s.processWinInfos()
	return nil
}

// processWinInfos 处理中奖信息
func (s *betOrderService) processWinInfos() {
	s.addFreeTime = 0
	if len(s.winInfos) > 0 {
		s.processWin()
	} else {
		s.processNoWin()
	}
}

// processWin 处理中奖
func (s *betOrderService) processWin() {
	// 计算线倍数（所有中奖线的赔率总和）
	s.lineMultiplier = s.handleWinElemsMultiplier(s.winInfos)

	// 获取当前轮次倍数
	roundMult := s.getRoundMultiplier()

	// Step倍数 = 线倍数 × 轮次倍数
	s.stepMultiplier = s.lineMultiplier * roundMult

	// 标记回合未结束（连消继续）
	s.isRoundOver = false

	// 更新场景状态
	s.scene.Steps++
	s.scene.ContinueNum++
	// 增加轮次索引，但限制在倍数数组范围内
	if s.isFreeRound {
		maxIndex := len(s.gameConfig.FreeRoundMultipliers) - 1
		if int(s.scene.MultipleIndex) < maxIndex {
			s.scene.MultipleIndex++
		}
	} else {
		maxIndex := len(s.gameConfig.BaseRoundMultipliers) - 1
		if int(s.scene.MultipleIndex) < maxIndex {
			s.scene.MultipleIndex++
		}
	}
	s.scene.RoundMultiplier += s.stepMultiplier

	// 处理蝴蝶百搭加成（蝴蝶参与中奖时累加倍数）
	// 注意：必须在状态推进之前调用，因为需要统计参与中奖的蝴蝶
	s.processButterflyBonus()

	// 处理中奖位置的百搭形态升级（毛虫→蝶茧→蝴蝶）
	s.processWildStatesOnWin()

	// 清除被消除位置的蝴蝶百搭状态
	s.clearWildStatesAfterElimination()

	// 生成下一盘面（消除中奖符号，下落填充）
	s.nextSymbolGrid = s.moveSymbols()
	s.fallingWinSymbols(s.nextSymbolGrid)

	// 设置下一阶段
	if s.isFreeRound {
		s.scene.NextStage = _spinTypeFreeEli
	} else {
		s.scene.NextStage = _spinTypeBaseEli
	}

	// 更新奖金
	s.updateBonusAmount(s.stepMultiplier)

	// 同步百搭状态和蝴蝶累加倍数到场景
	s.scene.WildStates = s.wildStates
	s.scene.ButterflyBonus = s.butterflyBonus
}

// processNoWin 处理未中奖
func (s *betOrderService) processNoWin() {
	s.stepMultiplier = 0
	s.lineMultiplier = 0
	s.isRoundOver = true
	s.scatterCount = s.getScatterCount()

	// 重置连消状态
	s.scene.Steps = 0
	s.scene.ContinueNum = 0
	s.scene.MultipleIndex = 0

	// 注意：无中奖时，百搭形态不变（只有参与中奖才升级）

	// 更新奖金（0）
	s.updateBonusAmount(0)
	s.client.ClientOfFreeGame.SetLastWinId(0)

	// 计算免费次数
	if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
		s.client.ClientOfFreeGame.Incr(uint64(newFree))
		s.scene.FreeNum += newFree
		s.addFreeTime = newFree
	}

	// 处理阶段转换
	if s.isFreeRound {
		if s.scene.FreeNum <= 0 {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
			s.scene.IsRoundFirstStep = false
			// 免费模式结束，重置百搭累加倍数
			s.butterflyBonus = 0
			s.scene.ButterflyBonus = 0
		} else {
			s.scene.NextStage = _spinTypeFree
			s.scene.IsRoundFirstStep = true
		}
	} else {
		// 基础模式每轮结束时重置蝴蝶百搭累加倍数
		s.butterflyBonus = 0
		s.scene.ButterflyBonus = 0

		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
			s.scene.IsRoundFirstStep = true
		} else {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
			s.scene.IsRoundFirstStep = false
		}
	}
}

// handleWinElemsMultiplier 计算中奖元素总倍数
func (s *betOrderService) handleWinElemsMultiplier(elems []WinInfo) int64 {
	var mul int64
	for _, elem := range elems {
		mul += elem.Odds
	}
	return mul
}
