package xxg2

import (
	"fmt"
	"math/rand/v2"

	"github.com/shopspring/decimal"
)

// baseSpin 核心spin逻辑
func (s *betOrderService) baseSpin() (*BaseSpinResult, error) {
	// 初始化
	if err := s.initialize(); err != nil {
		return nil, err
	}

	// XXG2每次spin都生成新符号（不像mahjong的消除机制）
	symbols := s.initSpinSymbol()
	s.stepMap = &stepMap{
		ID:         0,
		FreeNum:    0,
		IsFree:     0,
		New:        0,
		TreatCount: 0,
		TreatPos:   nil,
		Bat:        nil,
		Map:        symbols,
	}
	if s.isFreeRound() {
		s.stepMap.IsFree = 1
	}

	// 加载符号数据到网格，扫描treasure位置（保存到stepMap）
	s.loadStepData()

	// 蝙蝠移动和Wind转换（xxg2核心逻辑）
	s.collectBat()

	// 查找中奖信息（在符号变换之后）
	s.findWinInfos()

	// 处理中奖信息并计算倍率
	s.processWinInfos()

	// 更新奖金金额
	s.updateBonusAmount()

	// 根据游戏模式更新结果
	result := &BaseSpinResult{
		lineMultiplier: s.lineMultiplier,
		stepMultiplier: s.stepMultiplier,
		treasureCount:  s.stepMap.TreatCount,
		symbolGrid:     s.symbolGrid,
		winGrid:        s.winGrid,
		winResults:     s.winResults,
	}

	if s.isFreeRound() {
		// 保存统计数据（在updateFreeStepResult清空前）
		result.InitialBatCount = s.scene.InitialBatCount
		result.AccumulatedNewBat = s.scene.AccumulatedNewBat

		s.updateFreeStepResult()

		result.SpinOver = (s.client.ClientOfFreeGame.GetFreeNum() < 1)
		result.IsFreeGameEnding = result.SpinOver
	} else {
		s.updateBaseStepResult()
		result.SpinOver = (s.newFreeCount == 0)
	}

	return result, nil
}

// initSpinSymbol 初始化滚轴符号（使用配置中的 RealData，参考 mahjong）
func (s *betOrderService) initSpinSymbol() [_rowCount * _colCount]int64 {
	// 获取配置
	rollCfg := &s.gameConfig.RollCfg.Base
	isFree := s.isFreeRound()
	if isFree {
		rollCfg = &s.gameConfig.RollCfg.Free
	}

	// 根据权重随机选择 RealData 索引（优化：只有1个权重时直接使用）
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

	//// 调试模式：使用固定测试数据
	//if _debugModeOpen {
	//	realData = [][]int64{
	//		{7, 8, 9, 7, 1, 2},  // 第1列
	//		{11, 2, 3, 4, 5, 6}, // 第2列
	//		{8, 9, 7, 8, 1, 2},  // 第3列
	//		{1, 11, 3, 4, 5, 6}, // 第4列
	//		{9, 7, 8, 9, 1, 2},  // 第5列
	//	}
	//
	//	/*
	//		调试数据（startIndex=0）:
	//
	//		RealData（每列取4个）:
	//		  7  11   8   1   9
	//		  8   2   9  11   7
	//		  9   3   7   3   8
	//		  7   4   8   4   9
	//
	//		stepMap: [7,11,8,1,9, 8,2,9,11,7, 9,3,7,3,8, 7,4,8,4,9]
	//
	//		symbolGrid:
	//		  7  11   8   1   9
	//		  8   2   9  11   7
	//		  9   3   7   3   8
	//		  7   4   8   4   9
	//
	//		reversalSymbolGrid（上下翻转）:
	//		  7   4   8   4   9
	//		  9   3   7   3   8
	//		  8   2   9  11   7
	//		  7  11   8   1   9
	//		      ^^           treasure位置
	//
	//		treasureCount=2, 可转换位置11个（7/8/9符号）
	//		随机选2个转为Wild(10)
	//	*/
	//}

	var symbols [_rowCount * _colCount]int64

	// 从每列的 RealData 中随机选择起始位置，生成符号
	for col := 0; col < int(_colCount); col++ {
		columnData := realData[col]
		if len(columnData) < int(_rowCount) {
			panic("real data column too short")
		}

		// 随机选择起始位置（调试模式固定为0）
		startIdx := rand.IntN(len(columnData))
		if _debugModeOpen {
			startIdx = 0
		}

		// 初始化转轮起始位置记录（用于调试或RTP测试）
		if s.forRtpBench {
			s.debug.col[col] = statColInfo{
				startIdx: startIdx,
				len:      len(columnData),
			}
		}

		// 填充该列的符号
		for row := 0; row < int(_rowCount); row++ {
			symbols[row*int(_colCount)+col] = columnData[(startIdx+row)%len(columnData)]
		}
	}

	return symbols
}

// loadStepData 加载step数据（统计夺宝符号数量及位置）
func (s *betOrderService) loadStepData() {
	var positions []*position // 夺宝符号的坐标
	var symbolGrid int64Grid

	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			val := s.stepMap.Map[row*_colCount+col]
			symbolGrid[row][col] = val
			if val == _treasure {
				positions = append(positions, &position{Row: row, Col: col})
			}
		}
	}
	s.symbolGrid = &symbolGrid                   // 初始网格
	s.stepMap.TreatCount = int64(len(positions)) // treasure数量
	s.stepMap.TreatPos = positions               // treasure位置（统一数据源）

	// 保存初始符号网格（转换前，用于RTP测试）
	if s.forRtpBench {
		gridCopy := symbolGrid
		s.originalGrid = &gridCopy
	}
}

// collectBat 收集蝙蝠移动和Wind转换信息
func (s *betOrderService) collectBat() {
	if s.isFreeRound() {
		s.stepMap.Bat = s.transformToWildFreeMode()
	} else {
		s.stepMap.Bat = s.transformToWildBaseMode()
	}
}

// transformToWildBaseMode 基础模式Wind转换
func (s *betOrderService) transformToWildBaseMode() []*Bat {
	treasureCount := s.stepMap.TreatCount
	if treasureCount < 1 || treasureCount > 2 {
		return nil
	}

	// 扫描所有人符号位置(7/8/9)
	var windPositions []*position
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			symbol := s.symbolGrid[row][col]
			if symbol == _child || symbol == _woman || symbol == _oldMan {
				windPositions = append(windPositions, &position{Row: row, Col: col})
			}
		}
	}

	if len(windPositions) == 0 {
		return nil
	}

	// 随机选择N个人符号转换为Wild
	selectCount := min(int(treasureCount), len(windPositions))
	bats := make([]*Bat, 0, selectCount)

	for i, idx := range rand.Perm(len(windPositions))[:selectCount] {
		pos := windPositions[idx]
		oldSymbol := s.symbolGrid[pos.Row][pos.Col]
		s.symbolGrid[pos.Row][pos.Col] = _wild

		// 射线起点：循环使用treasure位置（从stepMap读取）
		treasureIdx := i % len(s.stepMap.TreatPos)
		bats = append(bats, createBat(s.stepMap.TreatPos[treasureIdx], pos, oldSymbol, _wild))
	}

	return bats
}

// transformToWildFreeMode 免费模式Wind转换（蝙蝠持续移动）
func (s *betOrderService) transformToWildFreeMode() []*Bat {
	// 步骤1：合并所有蝙蝠位置（旧蝙蝠 + 新treasure）
	allBatPositions := make([]*position, 0, _maxBatPositions)
	allBatPositions = append(allBatPositions, s.scene.BatPositions...)
	allBatPositions = append(allBatPositions, s.stepMap.TreatPos...)

	// 步骤2：如果超过5个，打乱后只取前5个
	if len(allBatPositions) > _maxBatPositions {
		// 打乱顺序，随机选择
		rand.Shuffle(len(allBatPositions), func(i, j int) {
			allBatPositions[i], allBatPositions[j] = allBatPositions[j], allBatPositions[i]
		})
		allBatPositions = allBatPositions[:_maxBatPositions]
	}

	// 步骤3：统一处理所有蝙蝠的移动和转换
	var bats []*Bat
	// 缓存已检查位置的原始符号，防止多只蝙蝠移动到同一格时符号被覆盖
	visitedSymbols := make(map[string]int64)

	for _, oldPos := range allBatPositions {
		newPos := moveBatOneStep(oldPos)
		posKey := fmt.Sprintf("%d_%d", newPos.Row, newPos.Col)

		// 获取目标位置的原始符号
		targetSymbol, cached := visitedSymbols[posKey]
		if !cached {
			targetSymbol = s.symbolGrid[newPos.Row][newPos.Col]
			visitedSymbols[posKey] = targetSymbol
		}

		// 判断是否能转换为Wild（目标是人符号7/8/9）
		if targetSymbol == _child || targetSymbol == _woman || targetSymbol == _oldMan {
			s.symbolGrid[newPos.Row][newPos.Col] = _wild
			bats = append(bats, createBat(oldPos, newPos, targetSymbol, _wild))
		} else {
			// 不转换，保持原符号
			oldSymbol := s.symbolGrid[oldPos.Row][oldPos.Col]
			bats = append(bats, createBat(oldPos, newPos, oldSymbol, targetSymbol))
		}
	}

	return bats
}

// direction 方向结构
type direction struct {
	dRow int64
	dCol int64
}

var allDirections = []direction{
	{-1, 0}, {1, 0}, {0, -1}, {0, 1}, // 上、下、左、右
	{-1, -1}, {-1, 1}, {1, -1}, {1, 1}, // 左上、右上、左下、右下
}

// isValidPosition 检查位置是否在边界内
func isValidPosition(row, col int64) bool {
	return row >= 0 && row < _rowCount && col >= 0 && col < _colCount
}

// moveBatOneStep 蝙蝠随机移动一格（8个方向）
func moveBatOneStep(pos *position) *position {
	var validDirs []direction
	for _, dir := range allDirections {
		if isValidPosition(pos.Row+dir.dRow, pos.Col+dir.dCol) {
			validDirs = append(validDirs, dir)
		}
	}

	if len(validDirs) == 0 {
		return pos
	}

	dir := validDirs[rand.IntN(len(validDirs))]
	return &position{Row: pos.Row + dir.dRow, Col: pos.Col + dir.dCol}
}

// createBat 创建蝙蝠移动记录
func createBat(fromPos, toPos *position, oldSymbol, newSymbol int64) *Bat {
	return &Bat{
		X:      fromPos.Row,
		Y:      fromPos.Col,
		TransX: toPos.Row,
		TransY: toPos.Col,
		Syb:    oldSymbol,
		Sybn:   newSymbol,
	}
}

// updateBaseStepResult 更新基础模式结果
func (s *betOrderService) updateBaseStepResult() {
	// 更新中奖金额
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		bonusFloat := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(bonusFloat)
		s.client.ClientOfFreeGame.IncRoundBonus(bonusFloat)
	}

	// 触发免费游戏（>=3个夺宝）
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
}

// validateGameState 校验游戏状态一致性
func (s *betOrderService) validateGameState() {
	// RTP 测试模式下跳过状态验证
	if s.forRtpBench {
		return
	}

	lastMapID := s.client.ClientOfFreeGame.GetLastMapId()
	freeNum := s.client.ClientOfFreeGame.GetFreeNum()

	if (lastMapID > 0 && freeNum == 0) || (lastMapID == 0 && freeNum > 0) {
		s.showPostUpdateErrorLog()
	}
}

// 更新免费游戏步骤结果
func (s *betOrderService) updateFreeStepResult() {
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
}
