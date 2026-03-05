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
	nextGrid := s.symbolGrid
	s.eliminateWinSymbols(&nextGrid)
	s.moveWildSymbols(&nextGrid)
	s.moveSymbols(&nextGrid)
	s.nextSymbolGrid = s.fallingWinSymbols(&nextGrid)

	if s.isFreeRound {
		s.scene.NextStage = _spinTypeFreeEli
	} else {
		s.scene.NextStage = _spinTypeBaseEli
	}
	s.updateBonusAmount(s.stepMultiplier)
}

// processNoWin 处理未中奖
func (s *betOrderService) processNoWin() {
	// 重置游戏状态
	if !s.isFreeRound {
		s.gameMultiple = 0
	}
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
			s.scene.ContinueNum = 0   // 清理
			s.scene.LsatWildPos = nil // 免费模式结束，清理 wild 位置
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
		s.scene.ContinueNum = 0   // 清理
		s.scene.LsatWildPos = nil // 清理 wild 位置
	}

	if s.isFreeRound && s.isRoundOver {
		s.saveLsatWildPosForFree() // 免费模式结束时记录wild位置
		s.scene.ContinueNum++      // 免费模式连续局数 用于计算gameMultiple
	}
}

func (s *betOrderService) handleWinElemsMultiplier(elems []WinInfo) int64 {
	var total int64
	for _, elem := range elems {
		total += elem.Multiplier
	}
	return total
}

// getStreakMultiplier 获取连击倍数
func (s *betOrderService) getStreakMultiplier() int64 {
	if s.isFreeRound {
		return s.scene.ContinueNum + 1
	}
	return 1
}

// 免费模式 Wild 位置继承：收集并移动 Wild 位置，用于下局初始化
// saveLsatWildPosForFree 保存免费模式Wild位置
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
