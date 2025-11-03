package xxg2

import (
	"math/rand/v2"

	"github.com/shopspring/decimal"
)

// baseSpin 核心spin逻辑
func (s *betOrderService) baseSpin() (*BaseSpinResult, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}

	s.stepMap = &stepMap{Map: s.initSpinSymbol()}
	s.loadStepData()
	s.collectBat()
	s.findWinInfos()
	s.processWinInfos()
	s.updateBonusAmount()

	result := &BaseSpinResult{
		lineMultiplier: s.lineMultiplier,
		stepMultiplier: s.stepMultiplier,
		treasureCount:  s.stepMap.TreatCount,
		symbolGrid:     s.symbolGrid,
		winGrid:        s.winGrid,
		winResults:     s.winResults,
	}

	if s.isFreeRound() {
		s.updateFreeStepResult(result)
	} else {
		s.updateBaseStepResult(result)
	}

	return result, nil
}

// initSpinSymbol 根据权重随机生成滚轴符号
func (s *betOrderService) initSpinSymbol() [_rowCount * _colCount]int64 {
	rollCfg := &s.gameConfig.RollCfg.Base
	if s.isFreeRound() {
		rollCfg = &s.gameConfig.RollCfg.Free
	}

	// 根据权重选择RealData索引
	realIndex := 0
	if len(rollCfg.Weight) > 1 {
		totalWeight := int64(0)
		for _, w := range rollCfg.Weight {
			totalWeight += w
		}
		r := rand.Int64N(totalWeight)
		for i, w := range rollCfg.Weight {
			if r < w {
				realIndex = int(rollCfg.UseKey[i])
				break
			}
			r -= w
		}
	} else {
		realIndex = int(rollCfg.UseKey[0])
	}

	if realIndex >= len(s.gameConfig.RealData) {
		panic("real data index out of range")
	}

	realData := s.gameConfig.RealData[realIndex]
	var symbols [_rowCount * _colCount]int64

	// 每列随机选择起始位置生成符号
	for col := 0; col < int(_colCount); col++ {
		columnData := realData[col]
		if len(columnData) < int(_rowCount) {
			panic("real data column too short")
		}

		startIdx := rand.IntN(len(columnData))
		for row := 0; row < int(_rowCount); row++ {
			symbols[row*int(_colCount)+col] = columnData[(startIdx+row)%len(columnData)]
		}

		if s.debug.open {
			s.debug.reelPositions[col] = reelPosition{startIdx: startIdx, length: len(columnData)}
		}
	}

	return symbols
}

// loadStepData 加载符号到网格并扫描treasure位置
func (s *betOrderService) loadStepData() {
	positions := make([]*position, 0, 5)
	var grid int64Grid

	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			val := s.stepMap.Map[row*_colCount+col]
			grid[row][col] = val
			if val == _treasure {
				positions = append(positions, &position{Row: row, Col: col})
			}
		}
	}

	s.symbolGrid = &grid
	s.stepMap.TreatCount = int64(len(positions))
	s.stepMap.TreatPos = positions

	if s.debug.open {
		gridCopy := grid
		s.debug.originalGrid = &gridCopy
	}
}

// collectBat 执行Wind转换（基础模式=射线映射，免费模式=蝙蝠移动）
func (s *betOrderService) collectBat() {
	if s.isFreeRound() {
		s.stepMap.Bat = s.transformToWildFreeMode()
	} else {
		s.stepMap.Bat = s.transformToWildBaseMode()
	}
}

// transformToWildBaseMode 基础模式：treasure射线到Wind转Wild（1-2个treasure）
func (s *betOrderService) transformToWildBaseMode() []*Bat {
	if s.stepMap.TreatCount < 1 || s.stepMap.TreatCount > 2 {
		return nil
	}

	humanPos := s.findHumanSymbols()
	if len(humanPos) == 0 {
		return nil
	}

	count := min(int(s.stepMap.TreatCount), len(humanPos))
	bats := make([]*Bat, count)
	perm := rand.Perm(len(humanPos))

	for i := 0; i < count; i++ {
		pos := humanPos[perm[i]]
		oldSymbol := s.symbolGrid[pos.Row][pos.Col]
		s.symbolGrid[pos.Row][pos.Col] = _wild

		treasurePos := s.stepMap.TreatPos[i%len(s.stepMap.TreatPos)]
		bats[i] = newBat(treasurePos, pos, oldSymbol, _wild)
	}

	return bats
}

// transformToWildFreeMode 免费模式：蝙蝠持续移动并转换Wind为Wild
func (s *betOrderService) transformToWildFreeMode() []*Bat {
	allBats := append([]*position{}, s.scene.BatPositions...)
	remainingSlots := int(s.gameConfig.MaxBatPositions) - len(allBats)

	// 添加新treasure（如果有空位）
	if remainingSlots > 0 && len(s.stepMap.TreatPos) > 0 {
		newTreasures := s.stepMap.TreatPos
		if len(newTreasures) > remainingSlots {
			rand.Shuffle(len(newTreasures), func(i, j int) {
				newTreasures[i], newTreasures[j] = newTreasures[j], newTreasures[i]
			})
			newTreasures = newTreasures[:remainingSlots]
		}
		allBats = append(allBats, newTreasures...)
	}

	// 移动蝙蝠并检查Wind转换
	bats := make([]*Bat, 0, len(allBats))
	visited := make(map[int64]int64, len(allBats))

	for _, oldPos := range allBats {
		newPos := s.moveBat(oldPos)
		targetSymbol := s.getCachedSymbol(newPos, visited)

		if isHumanSymbol(targetSymbol) {
			s.symbolGrid[newPos.Row][newPos.Col] = _wild
			bats = append(bats, newBat(oldPos, newPos, targetSymbol, _wild))
		} else {
			oldSymbol := s.symbolGrid[oldPos.Row][oldPos.Col]
			bats = append(bats, newBat(oldPos, newPos, oldSymbol, targetSymbol))
		}
	}

	return bats
}

// getCachedSymbol 获取符号（缓存防止多蝙蝠移入同格冲突）
func (s *betOrderService) getCachedSymbol(pos *position, cache map[int64]int64) int64 {
	key := pos.Row*_colCount + pos.Col
	if symbol, ok := cache[key]; ok {
		return symbol
	}
	cache[key] = s.symbolGrid[pos.Row][pos.Col]
	return cache[key]
}

// findHumanSymbols 查找所有Wind符号位置（7/8/9）
func (s *betOrderService) findHumanSymbols() []*position {
	positions := make([]*position, 0, 12)
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			sym := s.symbolGrid[row][col]
			if sym == _child || sym == _woman || sym == _oldMan {
				positions = append(positions, &position{Row: row, Col: col})
			}
		}
	}
	return positions
}

// moveBat 蝙蝠随机移动一格（8方向）
func (s *betOrderService) moveBat(pos *position) *position {
	validCount := 0
	var validDirs [8]direction

	for _, dir := range allDirections {
		newRow, newCol := pos.Row+dir.dRow, pos.Col+dir.dCol
		if newRow >= 0 && newRow < _rowCount && newCol >= 0 && newCol < _colCount {
			validDirs[validCount] = dir
			validCount++
		}
	}

	if validCount == 0 {
		return pos
	}

	dir := validDirs[rand.IntN(validCount)]
	return &position{Row: pos.Row + dir.dRow, Col: pos.Col + dir.dCol}
}

// isHumanSymbol 判断是否为Wind符号（7/8/9）
func isHumanSymbol(symbol int64) bool {
	return symbol == _child || symbol == _woman || symbol == _oldMan
}

// newBat 创建蝙蝠移动记录
func newBat(from, to *position, oldSym, newSym int64) *Bat {
	return &Bat{
		X:      from.Row,
		Y:      from.Col,
		TransX: to.Row,
		TransY: to.Col,
		Syb:    oldSym,
		Sybn:   newSym,
	}
}

// updateBaseStepResult 更新基础模式结果
func (s *betOrderService) updateBaseStepResult(result *BaseSpinResult) {
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		bonusFloat := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(bonusFloat)
		s.client.ClientOfFreeGame.IncRoundBonus(bonusFloat)
	}

	// 触发免费游戏（>=3个treasure）
	if s.stepMap.TreatCount >= _triggerTreasureCount {
		s.newFreeCount = s.gameConfig.FreeGameInitTimes +
			(s.stepMap.TreatCount-_triggerTreasureCount)*s.gameConfig.ExtraScatterExtraTime

		s.stepMap.New = s.newFreeCount
		s.stepMap.FreeNum = s.newFreeCount
		s.client.ClientOfFreeGame.SetFreeNum(uint64(s.newFreeCount))
		s.client.SetLastMaxFreeNum(uint64(s.newFreeCount))

		s.scene.BatPositions = s.stepMap.TreatPos
		s.scene.InitialBatCount = s.stepMap.TreatCount
		s.scene.AccumulatedNewBat = 0
		s.scene.NextStage = _spinTypeFree
	}

	s.validateGameState()
	s.stepMultiplier = s.lineMultiplier
	result.SpinOver = s.newFreeCount == 0
}

// updateFreeStepResult 更新免费模式结果
func (s *betOrderService) updateFreeStepResult(result *BaseSpinResult) {
	if s.debug.open {
		s.debug.initialBatCount = s.scene.InitialBatCount
		s.debug.accumulatedNewBat = s.scene.AccumulatedNewBat
	}

	s.client.ClientOfFreeGame.IncrFreeTimes()
	s.client.ClientOfFreeGame.Decr()

	if s.bonusAmount.GreaterThan(decimal.Zero) {
		bonusFloat := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(bonusFloat)
		s.client.ClientOfFreeGame.IncrFreeTotalMoney(bonusFloat)
		s.client.ClientOfFreeGame.IncRoundBonus(bonusFloat)
	}

	// 保存蝙蝠新位置
	batCount := len(s.stepMap.Bat)
	actualAddedCount := 0
	if batCount > 0 {
		newBatPositions := make([]*position, batCount)
		for i, bat := range s.stepMap.Bat {
			newBatPositions[i] = &position{Row: bat.TransX, Col: bat.TransY}
			if bat.TransX == bat.X && bat.TransY == bat.Y && bat.Syb == _treasure {
				actualAddedCount++
			}
		}
		s.scene.BatPositions = newBatPositions
	} else {
		s.scene.BatPositions = nil
	}

	// 计算新增免费次数（每个treasure+1次）
	s.newFreeCount = 0
	s.stepMap.New = 0
	if s.stepMap.TreatCount > 0 {
		s.newFreeCount = s.stepMap.TreatCount
		s.stepMap.New = s.newFreeCount
		s.scene.AccumulatedNewBat += int64(actualAddedCount)

		newTotal := s.client.ClientOfFreeGame.GetFreeNum() + uint64(s.newFreeCount)
		s.stepMap.FreeNum = int64(newTotal)
		s.client.ClientOfFreeGame.SetFreeNum(newTotal)
		s.client.SetLastMaxFreeNum(newTotal)
	} else {
		s.stepMap.FreeNum = int64(s.client.ClientOfFreeGame.GetFreeNum())
	}

	// 判断是否结束
	result.SpinOver = s.client.ClientOfFreeGame.GetFreeNum() < 1
	if result.SpinOver {
		s.scene.BatPositions = nil
		s.scene.InitialBatCount = 0
		s.scene.AccumulatedNewBat = 0
		s.scene.NextStage = _spinTypeBase
		s.client.ClientOfFreeGame.SetLastWinId(0)
	}

	s.validateGameState()
	s.stepMultiplier = s.lineMultiplier

	if s.debug.open {
		s.debug.isFreeGameEnding = result.SpinOver
	}
}

// validateGameState 校验游戏状态一致性
func (s *betOrderService) validateGameState() {
	if s.debug.open {
		return
	}
	lastMapID := s.client.ClientOfFreeGame.GetLastMapId()
	freeNum := s.client.ClientOfFreeGame.GetFreeNum()
	if (lastMapID > 0 && freeNum == 0) || (lastMapID == 0 && freeNum > 0) {
		s.showPostUpdateErrorLog()
	}
}
