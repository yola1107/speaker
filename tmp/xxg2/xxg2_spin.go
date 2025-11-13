package xxg2

import (
	mathRand "math/rand"

	"github.com/shopspring/decimal"
)

// baseSpin 核心spin逻辑
func (s *betOrderService) baseSpin() (*BaseSpinResult, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}

	s.stepMap = &stepMap{Map: _cnf.initSpinSymbol(s.isFreeRound())}
	s.loadStepData()
	s.collectBat()
	s.findWinInfos()
	s.processWinInfos()
	s.updateBonusAmount()

	result := &BaseSpinResult{
		LineMultiplier: s.lineMultiplier,
		StepMultiplier: s.stepMultiplier,
		TreasureCount:  s.stepMap.TreatCount,
		SymbolGrid:     s.symbolGrid,
		WinGrid:        s.winGrid,
		WinResults:     s.winResults,
	}

	if s.isFreeRound() {
		s.updateFreeStepResult(result)
	} else {
		s.updateBaseStepResult(result)
	}

	return result, nil
}

// loadStepData 加载符号到网格并扫描treasure位置
func (s *betOrderService) loadStepData() {
	var grid int64Grid
	treasures := make([]*position, 0, 5)

	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			val := s.stepMap.Map[row*_colCount+col]
			grid[row][col] = val
			if val == _treasure {
				treasures = append(treasures, &position{Row: row, Col: col})
			}
		}
	}

	s.symbolGrid = &grid
	s.stepMap.TreatCount = int64(len(treasures))
	s.stepMap.TreatPos = treasures

	//if s.debug.open {
	gridCopy := grid
	s.debug.originalGrid = &gridCopy
	//}
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

	r := randPool.Get().(*mathRand.Rand)
	defer randPool.Put(r)

	count := min(int(s.stepMap.TreatCount), len(humanPos))
	bats := make([]*Bat, count)
	perm := r.Perm(len(humanPos))

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
	// 加载已有蝙蝠，限制数量
	allBats := append([]*position{}, s.scene.BatPositions...)
	if len(allBats) > int(_cnf.MaxBatPositions) {
		allBats = allBats[:int(_cnf.MaxBatPositions)]
	}

	// 添加新treasure（如果有空位）
	remainingSlots := int(_cnf.MaxBatPositions) - len(allBats)
	if remainingSlots > 0 && len(s.stepMap.TreatPos) > 0 {
		r := randPool.Get().(*mathRand.Rand)
		defer randPool.Put(r)

		newTreasures := append([]*position{}, s.stepMap.TreatPos...)
		if len(newTreasures) > remainingSlots {
			r.Shuffle(len(newTreasures), func(i, j int) {
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
			bats = append(bats, newBat(oldPos, newPos, targetSymbol, targetSymbol))
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

// findHumanSymbols 查找所有人符号位置（7/8/9）
func (s *betOrderService) findHumanSymbols() []*position {
	positions := make([]*position, 0, 12)
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			sym := s.symbolGrid[row][col]
			if isHumanSymbol(sym) {
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

	r := randPool.Get().(*mathRand.Rand)
	defer randPool.Put(r)

	dir := validDirs[r.Intn(validCount)]
	return &position{Row: pos.Row + dir.dRow, Col: pos.Col + dir.dCol}
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
		s.newFreeCount = _cnf.FreeGameInitTimes +
			(s.stepMap.TreatCount-_triggerTreasureCount)*_cnf.ExtraScatterExtraTime

		s.stepMap.New = s.newFreeCount
		s.stepMap.FreeNum = s.newFreeCount
		s.client.ClientOfFreeGame.SetFreeNum(uint64(s.newFreeCount))
		s.client.SetLastMaxFreeNum(uint64(s.newFreeCount))

		// 限制蝙蝠位置数量不超过 MaxBatPositions
		batPositions := s.stepMap.TreatPos
		if len(batPositions) > int(_cnf.MaxBatPositions) {
			batPositions = batPositions[:int(_cnf.MaxBatPositions)]
		}
		s.scene.BatPositions = batPositions
		s.scene.InitialBatCount = int64(len(batPositions))
		s.scene.AccumulatedNewBat = 0
		s.scene.NextStage = _spinTypeFree
	}

	s.stepMultiplier = s.lineMultiplier
	result.SpinOver = s.newFreeCount == 0
}

// updateFreeStepResult 更新免费模式结果
func (s *betOrderService) updateFreeStepResult(result *BaseSpinResult) {
	if s.debug.open {
		s.debug.initialBatCount = s.scene.InitialBatCount
	}

	s.client.ClientOfFreeGame.IncrFreeTimes()
	s.client.ClientOfFreeGame.Decr()

	if s.bonusAmount.GreaterThan(decimal.Zero) {
		bonusFloat := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(bonusFloat)
		s.client.ClientOfFreeGame.IncrFreeTotalMoney(bonusFloat)
		s.client.ClientOfFreeGame.IncRoundBonus(bonusFloat)
	}

	// 保存蝙蝠新位置（限制数量，保持顺序）
	batCount := len(s.stepMap.Bat)
	prevCount := len(s.scene.BatPositions)
	if batCount > 0 {
		newBatPositions := make([]*position, 0, batCount)
		for _, bat := range s.stepMap.Bat {
			newBatPositions = append(newBatPositions, &position{Row: bat.TransX, Col: bat.TransY})
		}
		// 限制数量不超过 MaxBatPositions
		if len(newBatPositions) > int(_cnf.MaxBatPositions) {
			newBatPositions = newBatPositions[:int(_cnf.MaxBatPositions)]
		}
		s.scene.BatPositions = newBatPositions
	} else {
		s.scene.BatPositions = nil
	}

	newCount := len(s.scene.BatPositions)
	prevCap := prevCount
	if prevCap > int(_cnf.MaxBatPositions) {
		prevCap = int(_cnf.MaxBatPositions)
	}
	if newCount > prevCap {
		s.scene.AccumulatedNewBat += int64(newCount - prevCap)
	}
	if s.debug.open {
		s.debug.accumulatedNewBat = s.scene.AccumulatedNewBat
	}

	// 计算新增免费次数（每个treasure+1次）
	s.newFreeCount = 0
	s.stepMap.New = 0
	if s.stepMap.TreatCount > 0 {
		s.newFreeCount = s.stepMap.TreatCount
		s.stepMap.New = s.newFreeCount

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

	s.stepMultiplier = s.lineMultiplier

	if s.debug.open {
		s.debug.isFreeGameEnding = result.SpinOver
	}
}
