package hbtr2

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.debug.mark = 0  // 每轮开始时重置mark
		s.syncGameStage() // RTP测试模式需要手动状态转换
	}
	if err := s.initialize(); err != nil {
		return err
	}
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.scene.SymbolRoller = s.initSpinSymbol()
	}
	s.handleSymbolGrid()
	s.findWinInfos()
	s.processWinInfos()
	return nil
}

func (s *betOrderService) processWinInfos() {
	s.addFreeTime = 0 // 重置免费次数
	s.nextSymbolGrid = nil
	s.debug.mark = 0 // 清空
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
	s.scene.RoundMultiplier += s.stepMultiplier

	// 执行消除和移动流程：消除 -> wild移动 -> 下落+左移动 -> 同步roller
	nextGrid := s.eliminateWinSymbols()
	s.bats = s.moveWildSymbols(nextGrid)
	nextGrid = s.moveSymbols(nextGrid)
	s.nextSymbolGrid = s.fallingWinSymbols(nextGrid)

	if s.isFreeRound {
		s.scene.NextStage = _spinTypeFreeEli
	} else {
		s.scene.NextStage = _spinTypeBaseEli
	}
	s.updateBonusAmount(s.stepMultiplier)
}

// processNoWin 处理未中奖情况
func (s *betOrderService) processNoWin() {
	// 重置游戏状态
	s.gameMultiple = 1
	s.lineMultiplier = 0
	s.stepMultiplier = 0
	s.isRoundOver = true
	s.scatterCount = s.getScatterCount()
	s.scene.Steps = 0

	s.updateBonusAmount(0)
	s.client.ClientOfFreeGame.SetLastWinId(0)

	if s.isFreeRound {
		// 13 + 15
		if newFree := s.scatterCount; newFree > 0 {
			s.client.ClientOfFreeGame.Incr(uint64(newFree))
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
		}

		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--

		if s.scene.FreeNum <= 0 {
			s.scene.FreeNum = 0
			s.scene.ScatterNum = 0
			s.scene.NextStage = _spinTypeBase
		} else {
			s.scene.NextStage = _spinTypeFree
		}

	} else {
		if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
			s.client.ClientOfFreeGame.Incr(uint64(newFree))
			s.scene.FreeNum += newFree
			s.addFreeTime = newFree
			s.scene.NextStage = _spinTypeFree
		} else {
			s.scene.NextStage = _spinTypeBase
		}
		s.scene.ScatterNum = s.scatterCount
	}

	// 免费模式回合结束，记录 wild 位置（左下偏移后）供下局继承
	if s.isFreeRound && s.isRoundOver {
		s.saveLsatWildPosForFree()
	}
}

func (s *betOrderService) handleWinElemsMultiplier(elems []WinInfo) int64 {
	var total int64
	for _, elem := range elems {
		total += elem.Multiplier
	}
	return total
}

// getStreakMultiplier 获取连击倍数（当前固定为1）
func (s *betOrderService) getStreakMultiplier() int64 {
	return 1
}

/*
	免费模式 Wild 位置继承机制：
	当前局结束时：
		收集所有 Wild 的位置
		每个 Wild 左下移动一格（r+1, c–1）
		过滤掉移动后落出盘面的 Wild
		将结果写入 scene.LastWildPositions

	下局初始化符号时（getSceneSymbol）：
		先标记上次免费模式中保存的wild的pos
		根据pos设置对应roller board的位置为wild
		再按原逻辑将roller board里非_wild的位置填充上

	作用：
	实现免费模式下 Wild 的延迟留存与移动，确保 Wild 不被随机逻辑覆盖。
*/

// 保存免费模式中一局结束后，当前盘面的wild并做一次左下移动后的坐标到scene，（剔除落出盘面的wild位置）
func (s *betOrderService) saveLsatWildPosForFree() {
	var pos [][2]int64
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if !isWild(s.symbolGrid[r][c]) {
				continue
			}
			nr, nc := r+1, c-1
			if nr >= _rowCount || nc < 0 || isBlockedCell(nr, nc) {
				continue
			}
			pos = append(pos, [2]int64{nr, nc})
		}
	}
	s.scene.LsatWildPos = pos
}
