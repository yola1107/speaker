package xxg2

import (
	"fmt"
	"math/rand/v2"

	"github.com/shopspring/decimal"
)

// baseSpin 核心spin逻辑
func (s *betOrderService) baseSpin() (*BaseSpinResult, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}

	// 初始化stepMap（零值字段无需显式赋值）
	symbols := s.initSpinSymbol()
	s.stepMap = &stepMap{
		Map: symbols,
	}

	// 加载网格，扫描treasure
	s.loadStepData()

	// Wind转换
	s.collectBat()

	// 计算中奖
	s.findWinInfos()
	s.processWinInfos()
	s.updateBonusAmount()

	// 构建并更新结果
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

// initSpinSymbol 初始化滚轴符号
func (s *betOrderService) initSpinSymbol() [_rowCount * _colCount]int64 {
	// 获取配置
	rollCfg := &s.gameConfig.RollCfg.Base
	if s.isFreeRound() {
		rollCfg = &s.gameConfig.RollCfg.Free
	}

	// 根据权重随机选择 RealData 索引
	realIndex := 0
	if len(rollCfg.Weight) == 1 {
		realIndex = int(rollCfg.UseKey[0])
	} else {
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
	}

	if realIndex >= len(s.gameConfig.RealData) {
		panic("real data index out of range")
	}

	realData := s.gameConfig.RealData[realIndex]
	var symbols [_rowCount * _colCount]int64

	// 从每列的 RealData 中随机选择起始位置，生成符号
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

// loadStepData 加载符号网格并扫描treasure
func (s *betOrderService) loadStepData() {
	positions := make([]*position, 0, _rowCount*_colCount)
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

// collectBat Wind转换调度
func (s *betOrderService) collectBat() {
	if s.isFreeRound() {
		s.stepMap.Bat = s.transformToWildFreeMode()
	} else {
		s.stepMap.Bat = s.transformToWildBaseMode()
	}
}

// transformToWildBaseMode 基础模式Wind转换（1-2个treasure触发）
func (s *betOrderService) transformToWildBaseMode() []*Bat {
	if s.stepMap.TreatCount < 1 || s.stepMap.TreatCount > 2 {
		return nil
	}

	// 收集所有人符号位置
	humanPos := s.findHumanSymbols()
	if len(humanPos) == 0 {
		return nil
	}

	// 随机选择N个转为Wild
	count := min(int(s.stepMap.TreatCount), len(humanPos))
	bats := make([]*Bat, 0, count)

	for i, idx := range rand.Perm(len(humanPos))[:count] {
		pos := humanPos[idx]
		oldSymbol := s.symbolGrid[pos.Row][pos.Col]
		s.symbolGrid[pos.Row][pos.Col] = _wild

		treasureIdx := i % len(s.stepMap.TreatPos)
		bats = append(bats, newBat(s.stepMap.TreatPos[treasureIdx], pos, oldSymbol, _wild))
	}

	return bats
}

// transformToWildFreeMode 免费模式Wind转换（蝙蝠持续移动）
func (s *betOrderService) transformToWildFreeMode() []*Bat {
	// 合并所有蝙蝠位置（旧蝙蝠 + 新treasure）
	allBats := append([]*position{}, s.scene.BatPositions...)
	allBats = append(allBats, s.stepMap.TreatPos...)

	// 超过5个则随机选择
	if len(allBats) > _maxBatPositions {
		rand.Shuffle(len(allBats), func(i, j int) {
			allBats[i], allBats[j] = allBats[j], allBats[i]
		})
		allBats = allBats[:_maxBatPositions]
	}

	// 统一移动和转换
	var bats []*Bat
	visited := make(map[string]int64)

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

// getCachedSymbol 获取符号（带缓存，防止多只蝙蝠移入同格时冲突）
func (s *betOrderService) getCachedSymbol(pos *position, cache map[string]int64) int64 {
	key := fmt.Sprintf("%d_%d", pos.Row, pos.Col)
	if symbol, ok := cache[key]; ok {
		return symbol
	}
	cache[key] = s.symbolGrid[pos.Row][pos.Col]
	return cache[key]
}

// findHumanSymbols 查找所有人符号位置(7/8/9)
func (s *betOrderService) findHumanSymbols() []*position {
	positions := make([]*position, 0, _rowCount*_colCount)
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			if isHumanSymbol(s.symbolGrid[row][col]) {
				positions = append(positions, &position{Row: row, Col: col})
			}
		}
	}
	return positions
}

// moveBat 蝙蝠随机移动一格（8方向）
func (s *betOrderService) moveBat(pos *position) *position {
	validDirs := make([]direction, 0, 8)
	for _, dir := range allDirections {
		newRow, newCol := pos.Row+dir.dRow, pos.Col+dir.dCol
		if newRow >= 0 && newRow < _rowCount && newCol >= 0 && newCol < _colCount {
			validDirs = append(validDirs, dir)
		}
	}

	if len(validDirs) == 0 {
		return pos
	}

	dir := validDirs[rand.IntN(len(validDirs))]
	return &position{Row: pos.Row + dir.dRow, Col: pos.Col + dir.dCol}
}

// isHumanSymbol 判断是否为人符号(7/8/9)
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

type direction struct {
	dRow, dCol int64
}

var allDirections = []direction{
	{-1, 0}, {1, 0}, {0, -1}, {0, 1}, // 上下左右
	{-1, -1}, {-1, 1}, {1, -1}, {1, 1}, // 四个斜角
}

// updateBaseStepResult 更新基础模式结果
func (s *betOrderService) updateBaseStepResult(result *BaseSpinResult) {
	// 更新中奖金额
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		bonusFloat := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(bonusFloat)
		s.client.ClientOfFreeGame.IncRoundBonus(bonusFloat)
	}

	// 触发免费游戏（>=3个treasure）
	if s.stepMap.TreatCount >= _triggerTreasureCount {
		// 计算免费次数：10 + (夺宝数-3)×2
		s.newFreeCount = s.gameConfig.FreeGameInitTimes +
			(s.stepMap.TreatCount-_triggerTreasureCount)*s.gameConfig.ExtraScatterExtraTime

		s.stepMap.New = s.newFreeCount
		s.stepMap.FreeNum = s.newFreeCount
		s.client.ClientOfFreeGame.SetFreeNum(uint64(s.newFreeCount))
		s.client.SetLastMaxFreeNum(uint64(s.newFreeCount))

		// 初始化免费游戏数据：treasure位置变成初始蝙蝠位置（从stepMap读取）
		s.scene.BatPositions = s.stepMap.TreatPos
		s.scene.InitialBatCount = s.stepMap.TreatCount
		s.scene.AccumulatedNewBat = 0
		s.scene.NextStage = _spinTypeFree
	}

	s.validateGameState()
	s.stepMultiplier = s.lineMultiplier

	result.SpinOver = s.newFreeCount == 0
}

// 更新免费游戏步骤结果
func (s *betOrderService) updateFreeStepResult(result *BaseSpinResult) {
	// RTP测试统计信息
	if s.debug.open {
		s.debug.initialBatCount = s.scene.InitialBatCount
		s.debug.accumulatedNewBat = s.scene.AccumulatedNewBat
	}

	// 更新计数器
	s.client.ClientOfFreeGame.IncrFreeTimes()
	s.client.ClientOfFreeGame.Decr()

	// 更新中奖金额
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		bonusFloat := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(bonusFloat)
		s.client.ClientOfFreeGame.IncrFreeTotalMoney(bonusFloat)
		s.client.ClientOfFreeGame.IncRoundBonus(bonusFloat)
	}

	// 保存蝙蝠移动后的新位置
	var newBatPositions []*position
	actualAddedBatCount := 0
	for _, bat := range s.stepMap.Bat {
		newBatPositions = append(newBatPositions, &position{Row: bat.TransX, Col: bat.TransY})
		// 统计实际添加的蝙蝠（位置未移动 且 是夺宝符号）
		if bat.TransX == bat.X && bat.TransY == bat.Y && bat.Syb == _treasure {
			actualAddedBatCount++
		}
	}
	s.scene.BatPositions = newBatPositions

	// 计算新增免费次数（参考XXG逻辑）
	// 规则：根据当前新生成盘面的夺宝数量，每个夺宝+1次免费
	s.newFreeCount = 0
	s.stepMap.New = 0

	// 只在没有奖金时，根据当前盘面的夺宝数量计算免费次数
	if !s.bonusAmount.GreaterThan(decimal.Zero) && s.stepMap.TreatCount > 0 {
		// 每个夺宝符号+1次免费（独立于蝙蝠生成的5个上限）
		s.newFreeCount = s.stepMap.TreatCount
		s.stepMap.New = s.newFreeCount
		s.scene.AccumulatedNewBat += int64(actualAddedBatCount) // 累计实际添加的蝙蝠（用于统计）

		newTotal := s.client.ClientOfFreeGame.GetFreeNum() + uint64(s.newFreeCount)
		s.stepMap.FreeNum = int64(newTotal)
		s.client.ClientOfFreeGame.SetFreeNum(newTotal)
		s.client.SetLastMaxFreeNum(newTotal)
	} else {
		s.stepMap.FreeNum = int64(s.client.ClientOfFreeGame.GetFreeNum())
	}

	// 免费游戏结束时清理
	if s.client.ClientOfFreeGame.GetFreeNum() < 1 {
		s.scene.BatPositions = nil
		s.scene.InitialBatCount = 0
		s.scene.AccumulatedNewBat = 0
		s.scene.NextStage = _spinTypeBase
		s.client.ClientOfFreeGame.SetLastWinId(0)
	}

	s.validateGameState()
	s.stepMultiplier = s.lineMultiplier

	result.SpinOver = s.client.ClientOfFreeGame.GetFreeNum() < 1

	// RTP测试统计：记录免费游戏是否结束
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
