package xslm3

// baseSpin2 优化后的baseSpin逻辑（使用滚轴配置生成网格）
// 流程：
// 1. 更新上一step里保存的女性符号收集情况到当前femaleCountsForFree nextFemaleCountsForFree
// 2. 新生成网格（免费游戏需要使用femaleCountsForFree生成key）/ 使用上一step需要消除的roller转成网格
// 3. 当前盘面网格转换
// 4. 计算当前网格的得分及是否女性符号中奖，女性百搭中奖标识
// 5. 计算下一个step需要的消除、下落、填充（更新roller到scene）
// 6. 下一次的女性符号收集（供新游戏或需要使用滚轴选key使用）
// 7. 下一次女性符号转女性百搭情况
// 8. 下一次的状态保存及清理的数据
// 9. 当前是否有新增的免费次数
// 10. 当前状态更新（用于状态流转到下个step）
func (s *betOrderService) baseSpin2() error {
	// 1-2. 初始化和网格准备
	s.handleStageTransition() // 状态跳转
	s.loadSceneFemaleCount()  // 加载女性符号计数（从scene加载到femaleCountsForFree和nextFemaleCountsForFree）

	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		// 新回合开始，生成新网格（免费模式使用femaleCountsForFree生成key）
		s.scene.SymbolRoller = s.getSceneSymbol()
	}

	// 3-4. 当前盘面处理和算分
	s.handleSymbolGrid() // 从SymbolRoller转换为symbolGrid
	s.findWinInfos()     // 查找中奖，设置hasFemaleWin、hasFemaleWildWin等标识

	// 免费模式全屏情况下，补充查找只有女性百搭的way
	if s.isFreeRound && s.enableFullElimination {
		s.findAllWinInfosForFullElimination()
	}

	s.updateStepResults(false) // 计算当前网格的得分（计算全部得分）

	// 5. 消除处理：检查是否需要消除并执行消除、下落、填充（更新roller到scene）
	hasElimination := s.checkAndExecuteElimination()

	// 6-7. 女性符号收集和状态更新
	if s.isFreeRound {
		s.collectFemaleSymbol2() // 基于当前网格的中奖收集女性符号，更新nextFemaleCountsForFree
		s.femaleCountsForFree = s.nextFemaleCountsForFree
		// enableFullElimination在loadSceneFemaleCount中已经设置，代表初始盘面状态，不应该改变
	}

	// 8-9. 状态保存和清理（包括客户端状态更新）
	if s.isFreeRound {
		s.eliminateResultForFree2(hasElimination)
	} else {
		s.eliminateResultForBase2(hasElimination)
	}

	// 10. 更新余额
	s.updateCurrentBalance()

	return nil
}

// checkAndExecuteElimination 检查是否需要消除并执行消除、下落、填充
// 返回是否有消除
// 注意：当前盘面（symbolGrid）保持不变，用于返回给客户端
// 如果有消除，会更新SymbolRoller到scene，下一次baseSpin会从更新后的SymbolRoller读取
func (s *betOrderService) checkAndExecuteElimination() bool {
	// 检查是否需要消除
	if !s.isFreeRound {
		// 基础模式：有女性中奖且有百搭时，才消除（规则4）
		if !s.hasFemaleWin || !s.hasWildSymbol() {
			return false
		}
	} else {
		// 免费模式：有女性中奖或全屏情况下有女性百搭中奖时，才消除（规则5、8）
		// enableFullElimination是初始盘面的状态，不应该在过程中改变
		if s.enableFullElimination {
			if !s.hasFemaleWildWin {
				return false
			}
		} else {
			if !s.hasFemaleWin {
				return false
			}
		}
	}

	// 执行消除、下落、填充
	if len(s.winInfos) == 0 || s.stepMultiplier == 0 || s.winGrid == nil {
		return false
	}

	// 复制当前网格用于消除处理
	nextGrid := *s.symbolGrid

	// 根据模式执行不同的消除逻辑（规则10）
	var cnt int
	if !s.isFreeRound {
		// 基础模式：消除中奖的女性符号（7，8，9）及百搭，如果盘面有夺宝则百搭不消除
		cnt = s.fillElimBase(&nextGrid)
	} else if s.enableFullElimination {
		// 免费模式全屏情况：每个中奖Way找女性百搭，找到则改way除百搭13之外的符号都全部消除
		cnt = s.fillElimFreeFull(&nextGrid)
	} else {
		// 免费模式非全屏情况：每个中奖way找女性，找到该way女性及女性百搭都消除
		cnt = s.fillElimFreePartial(&nextGrid)
	}

	if cnt == 0 {
		return false
	}

	// 有消除，执行掉落和填充
	s.dropSymbols(&nextGrid)       // 消除后掉落
	s.fallingWinSymbols2(nextGrid) // 掉落后填充，更新 SymbolRoller 到 scene（用于下一次）

	return true
}

// fallingWinSymbols2 优化后的填充函数
// 将掉落后的网格更新到 SymbolRoller，并填充新符号
// 免费模式下，根据当前的 femaleCountsForFree 转换女性符号为女性百搭
func (s *betOrderService) fallingWinSymbols2(nextSymbolGrid int64Grid) {
	// 将掉落后的网格更新到 SymbolRoller
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			// BoardSymbol 从下往上存储，所以需要反转索引
			s.scene.SymbolRoller[c].BoardSymbol[r] = nextSymbolGrid[_rowCount-1-r][c]
		}
	}

	// 填充新符号
	for i := range s.scene.SymbolRoller {
		s.scene.SymbolRoller[i].ringSymbol(s.gameConfig)
	}

	// 免费模式下，填充后需要根据当前的 femaleCountsForFree 转换符号
	// 注意：这里使用 femaleCountsForFree（上一step保存的），因为当前step的女性符号收集在后续步骤处理
	if s.isFreeRound {
		for col := 0; col < int(_colCount); col++ {
			for row := 0; row < int(_rowCount); row++ {
				symbol := s.scene.SymbolRoller[col].BoardSymbol[row]
				// 检查是否是女性符号（A/B/C），且对应的计数 >= 10
				if symbol >= _femaleA && symbol <= _femaleC {
					idx := symbol - _femaleA
					if idx >= 0 && idx < 3 && s.femaleCountsForFree[idx] >= _femaleFullCount {
						// 转换为对应的 wild 版本
						s.scene.SymbolRoller[col].BoardSymbol[row] = _wildFemaleA + idx
					}
				}
			}
		}
	}
}

// collectFemaleSymbol2 基于当前网格的中奖收集女性符号，更新到nextFemaleCountsForFree
// 用于下一次生成网格时判断是否需要转换为女性百搭
func (s *betOrderService) collectFemaleSymbol2() {
	// 基于当前网格的中奖（s.winGrid）收集女性符号
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbol := s.winGrid[r][c]
			if symbol >= _femaleA && symbol <= _femaleC {
				idx := symbol - _femaleA
				if idx >= 0 && idx < 3 && s.nextFemaleCountsForFree[idx] < _femaleFullCount {
					s.nextFemaleCountsForFree[idx]++
				}
			}
		}
	}
}

// eliminateResultForBase2 处理基础模式的状态保存及清理
func (s *betOrderService) eliminateResultForBase2(hasElimination bool) {
	if hasElimination {
		// 有消除，继续消除状态
		s.isRoundOver = false
		s.scene.Steps++
		s.scene.NextStage = _spinTypeBaseEli
		s.scene.FemaleCountsForFree = [3]int64{} // 基础模式不保存女性符号计数
		s.newFreeRoundCount = 0
	} else {
		// 没有消除，结束当前回合（roundOver）
		s.isRoundOver = true
		s.scene.Steps = 0
		s.scene.FemaleCountsForFree = [3]int64{}

		// 基础模式：只在 roundOver 时统计夺宝数量并判断是否进入免费（规则6）
		s.treasureCount = s.getTreasureCount()
		s.newFreeRoundCount = s.getFreeRoundCountFromTreasure()

		// 基础模式结束时，如果触发免费，直接设置 NextStage = _spinTypeFree（参考 mahjong）
		// 如果未触发免费，回到基础模式
		if s.newFreeRoundCount > 0 {
			// 触发免费模式：直接进入免费模式
			s.scene.FreeNum = s.newFreeRoundCount
			s.scene.NextStage = _spinTypeFree
		} else {
			// 不触发免费模式，继续基础模式
			s.scene.NextStage = _spinTypeBase
		}
		s.scene.TreasureNum = 0
	}

	// 更新客户端状态
	s.client.IsRoundOver = s.isRoundOver
	if s.newFreeRoundCount > 0 {
		// 基础模式：如果有新增的免费次数，更新client
		s.client.ClientOfFreeGame.SetFreeNum(uint64(s.newFreeRoundCount))
		s.client.SetLastMaxFreeNum(uint64(s.newFreeRoundCount))
	}

	// 更新奖金
	if s.stepMultiplier > 0 {
		s.updateBonusAmount()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncRoundBonus(s.bonusAmount.Round(2).InexactFloat64())
	}
}

// eliminateResultForFree2 处理免费模式的状态保存及清理
func (s *betOrderService) eliminateResultForFree2(hasElimination bool) {
	// 计算本步骤增加的夺宝数量（规则6）
	s.treasureCount = s.getTreasureCount()
	s.stepAddTreasure = s.treasureCount - s.scene.TreasureNum
	s.scene.TreasureNum = s.treasureCount

	// 免费模式：每收集1个夺宝符号则免费游戏次数+1
	s.newFreeRoundCount = s.stepAddTreasure
	if s.newFreeRoundCount > 0 {
		s.client.ClientOfFreeGame.Incr(uint64(s.newFreeRoundCount))
		s.client.IncLastMaxFreeNum(uint64(s.newFreeRoundCount))
		s.scene.FreeNum += s.newFreeRoundCount
	}

	if hasElimination {
		// 有消除，继续消除状态
		s.isRoundOver = false
		s.scene.Steps++
		s.scene.NextStage = _spinTypeFreeEli
		s.scene.FemaleCountsForFree = s.nextFemaleCountsForFree // 保存女性符号计数到scene，供下一次使用
	} else {
		// 没有消除，结束当前回合
		s.isRoundOver = true
		s.scene.Steps = 0

		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
		if s.scene.FreeNum < 0 {
			s.scene.FreeNum = 0
		}

		// 更新状态
		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
			s.scene.FemaleCountsForFree = s.nextFemaleCountsForFree // 保存女性符号计数到scene，供下一次使用
		} else {
			s.scene.NextStage = _spinTypeBase
			s.scene.FemaleCountsForFree = [3]int64{} // 免费模式结束，清理女性符号计数
			s.scene.TreasureNum = 0
		}
	}

	// 更新客户端状态
	s.client.IsRoundOver = s.isRoundOver
	// 免费模式：夺宝统计和免费次数更新已经在上面处理（Incr, IncLastMaxFreeNum）

	// 更新奖金
	if s.stepMultiplier > 0 {
		s.updateBonusAmount()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncrFreeTotalMoney(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncRoundBonus(s.bonusAmount.Round(2).InexactFloat64())
	}
}
