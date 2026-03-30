package ajtm

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.debug.mark = 0
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}
	if s.isFreeRound && s.scene.FreeNum > 0 && s.scene.Stage == _spinTypeFree {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
	}

	// 判断是否为 round 的第一个 step
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.createMatrix()
	} else {
		s.handleSymbolGrid()
	}

	s.findWinInfos()
	s.processWinInfos()
	return nil
}
func (s *betOrderService) processWinInfos() {
	s.addFreeTime = 0 // 重置增加的免费次数
	s.debug.mark = 0  // 与 hbtr2 一致，便于 rtpx 日志读取
	s.scatterCount = s.getScatterCount()
	if len(s.winInfos) > 0 {
		s.processWin()
	} else {
		s.processNoWin()
	}
	s.updateBonusAmount(s.stepMultiplier)
}

// processWin：中奖后执行”符号消除→下落→填充”
func (s *betOrderService) processWin() {
	var totalOdds int64
	for _, w := range s.winInfos {
		totalOdds += w.Odds
	}
	s.lineMultiplier = totalOdds

	// 计算最终倍数：基础赔率 × 神秘符号倍数
	mysMul := s.scene.MysMultiplierTotal
	if mysMul <= 0 {
		mysMul = 1
	}
	s.stepMultiplier = s.lineMultiplier * mysMul
	s.isRoundOver = false

	s.scene.Steps++
	s.scene.RoundMultiplier += s.stepMultiplier

	s.nextSymbolGrid = s.moveSymbols()
	s.fallingWinSymbols(s.nextSymbolGrid)

	if s.isFreeRound {
		s.scene.NextStage = _spinTypeFreeEli
	} else {
		s.scene.NextStage = _spinTypeBaseEli
	}
}

func (s *betOrderService) processNoWin() {
	s.addFreeTime = 0
	s.lineMultiplier = 0
	s.stepMultiplier = 0
	s.isRoundOver = true

	s.scene.Steps = 0

	if s.isFreeRound {
		// 免费模式：保留神秘符号倍数，神秘符号下落并重新生成
		s.processFreeModeNoWin()

		if s.scene.FreeNum <= 0 {
			s.scene.FreeNum = 0
			s.scene.MysMultiplierTotal = 0 // 免费结束，重置倍数
			s.scene.NextStage = _spinTypeBase
		} else {
			s.scene.NextStage = _spinTypeFree
		}
	} else {
		s.scene.MysMultiplierTotal = 0 // 基础模式SPIN结束，重置倍数

		// 免费次数新增
		if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
			s.client.ClientOfFreeGame.Incr(uint64(newFree))
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
		}

		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
		} else {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
		}
	}
}
