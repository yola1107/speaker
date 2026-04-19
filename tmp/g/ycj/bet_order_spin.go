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
	if s.isFreeRound && !s.scene.pendingSpin() {
		s.scene.FreeTimes++
		s.scene.FreeNum--

	}

	// 生成符号
	if !s.scene.pendingSpin() {
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
	s.scene.Pend = _pendNone
	s.next = false

	// 初始判奖
	result := s.findWinInfos()

	// 处理中奖结果
	if result.Win {
		s.stepMultiplier = result.Multiplier
	}

	// 推展 / 重转（各回合至多一次，由 Done 位控制）
	switch {
	case result.TriggerExtend:
		s.scene.Done |= _doneExtend
		s.scene.Pend = _pendExtend
		s.next = true
		s.applyExtendFall(1)
	case result.TriggerRespin && s.isFreeRound:
		s.scene.Done |= _doneRespin
		s.scene.Pend = _pendRespin
		s.next = true
		s.respinSides()
	}

	// 夺宝模式：累积免费次数
	if result.TriggerFree {
		s.addFreeTime = result.FreeSpinNum
		s.scene.FreeNum += s.addFreeTime
	}

	// 更新下一阶段：
	// 免费最后一把若命中补判（Pend!=0），补判步骤仍应留在免费态执行。
	if s.isFreeRound && s.scene.pendingSpin() {
		s.scene.NextStage = _spinTypeFree
	} else if s.scene.FreeNum == 0 {
		s.scene.NextStage = _spinTypeBase
	} else {
		s.scene.NextStage = _spinTypeFree
	}

	s.isRoundOver = !(s.scene.pendingSpin() || s.addFreeTime > 0)

	// 更新奖金
	s.updateBonusAmount(s.stepMultiplier)
}
