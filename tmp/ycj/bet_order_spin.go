package ycj

// baseSpin 基础旋转流程
func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}

	// 免费模式扣减次数
	if s.isFreeRound && !s.scene.IsExtendMode && !s.scene.IsRespinMode {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
	}

	// 生成符号
	if !s.scene.IsExtendMode && !s.scene.IsRespinMode {
		s.initSpinSymbol()
	}

	s.handleSymbolGrid()
	s.processGame()
	return nil
}

// processGame 处理游戏主流程
func (s *betOrderService) processGame() {
	s.addFreeTime = 0
	s.stepMultiplier = 0
	s.scene.IsExtendMode = false
	s.scene.IsRespinMode = false
	s.next = false

	// 初始判奖
	result := s.findWinInfos()

	// 处理中奖结果
	if result.Win {
		s.stepMultiplier = result.Multiplier
	}

	// 处理推展模式：中间空 + 左右相同数字
	if result.TriggerExtend {
		s.scene.IsExtendMode = true
		s.next = true

		// 推展模式：中间列符号下落一个
		s.scene.SymbolRoller[1].BoardSymbol[0] = s.getFallSymbol(1)
	}

	// 处理重转模式：中间非空 + 左右不同（仅免费模式）
	if result.TriggerRespin && s.isFreeRound {
		s.scene.IsRespinMode = true
		s.next = true

		// 重转模式：左右列符号随机一个
		s.scene.SymbolRoller[0].BoardSymbol[0] = s.getRandomSymbol(0)
		s.scene.SymbolRoller[2].BoardSymbol[0] = s.getRandomSymbol(2)
	}

	// 夺宝模式：累积免费次数
	if result.TriggerFree {
		s.addFreeTime = result.FreeSpinNum
		s.client.ClientOfFreeGame.Incr(uint64(s.addFreeTime))
		s.scene.FreeNum += s.addFreeTime
	}

	// 更新免费模式状态
	if s.scene.FreeNum <= 0 {
		s.scene.FreeNum = 0
		s.scene.NextStage = _spinTypeBase
	} else {
		s.scene.NextStage = _spinTypeFree
	}

	// 判断回合是否结束
	s.isRoundOver = !(s.scene.IsExtendMode || s.scene.IsRespinMode || s.addFreeTime > 0)

	// 更新奖金
	s.updateBonusAmount(s.stepMultiplier)
}
