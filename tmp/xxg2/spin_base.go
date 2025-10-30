package xxg2

import (
	"math/rand/v2"

	"github.com/shopspring/decimal"
)

// baseSpin 核心spin逻辑
func (s *betOrderService) baseSpin() (*BaseSpinResult, error) {
	// 初始化
	if err := s.initialize(); err != nil {
		return nil, err
	}

	// 首次step：生成符号网格
	if s.isRoundFirstStep {
		symbols := s.initSpinSymbol()
		s.stepMap = &stepMap{
			ID:         0,
			FreeNum:    0,
			IsFree:     0,
			New:        0,
			TreatCount: 0,
			Bat:        nil,
			Map:        symbols,
		}
		if s.isFreeRound() {
			s.stepMap.IsFree = 1

			// 免费游戏第一次时的特殊处理（参考 zcm2）
			if s.scene.IsFirstFree {
				s.scene.IsFirstFree = false // 标记已经不是第一次了
			}
		}
		s.isRoundFirstStep = false
	}

	// 加载符号数据到网格
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
		treasureCount:  s.treasureCount,
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
	if s.isFreeRound() {
		rollCfg = &s.gameConfig.RollCfg.Free
	}

	// 根据权重随机选择 RealData 索引
	totalWeight := int64(0)
	for _, w := range rollCfg.Weight {
		totalWeight += w
	}

	r := rand.Int64N(totalWeight)
	realIndex := 0
	for i, w := range rollCfg.Weight {
		if r < w {
			realIndex = int(rollCfg.UseKey[i])
			break
		}
		r -= w
	}

	if realIndex >= len(s.gameConfig.RealData) {
		panic("real data index out of range")
	}

	realData := s.gameConfig.RealData[realIndex]

	// 调试模式：使用固定测试数据
	if _debugMode {
		realData = [][]int64{
			{7, 8, 9, 7, 1, 2},  // 第1列
			{11, 2, 3, 4, 5, 6}, // 第2列
			{8, 9, 7, 8, 1, 2},  // 第3列
			{1, 11, 3, 4, 5, 6}, // 第4列
			{9, 7, 8, 9, 1, 2},  // 第5列
		}

		/*
			调试数据（startIndex=0）:

			RealData（每列取4个）:
			  7  11   8   1   9
			  8   2   9  11   7
			  9   3   7   3   8
			  7   4   8   4   9

			stepMap: [7,11,8,1,9, 8,2,9,11,7, 9,3,7,3,8, 7,4,8,4,9]

			symbolGrid:
			  7  11   8   1   9
			  8   2   9  11   7
			  9   3   7   3   8
			  7   4   8   4   9

			reversalSymbolGrid（上下翻转）:
			  7   4   8   4   9
			  9   3   7   3   8
			  8   2   9  11   7
			  7  11   8   1   9
			      ^^           treasure位置

			treasureCount=2, 可转换位置11个（7/8/9符号）
			随机选2个转为Wild(10)
		*/
	}

	var symbols [_rowCount * _colCount]int64

	// 从每列的 RealData 中随机选择起始位置，生成符号
	for col := 0; col < int(_colCount); col++ {
		columnData := realData[col]
		if len(columnData) < int(_rowCount) {
			panic("real data column too short")
		}

		// 随机选择起始位置（调试模式固定为0）
		startIdx := 0
		if !_debugMode {
			startIdx = rand.IntN(len(columnData))
		}

		// 填充该列的符号
		for row := 0; row < int(_rowCount); row++ {
			symbols[row*int(_colCount)+col] = columnData[(startIdx+row)%len(columnData)]
		}
	}

	return symbols
}

// 加载step数据
func (s *betOrderService) loadStepData() {
	var symbolGrid int64Grid
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			symbolGrid[row][col] = s.stepMap.Map[row*_colCount+col]
		}
	}
	s.symbolGrid = &symbolGrid
}

// 更新基础游戏步骤结果
func (s *betOrderService) updateBaseStepResult() {
	// 更新中奖金额
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		bonusFloat := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(bonusFloat)
		s.client.ClientOfFreeGame.IncRoundBonus(bonusFloat)
	}

	// 触发免费游戏（>=3个夺宝）
	if s.treasureCount >= _triggerTreasureCount {
		// 计算免费次数：10 + (夺宝数-3)×2
		s.newFreeCount = s.gameConfig.FreeGameInitTimes +
			(s.treasureCount-_triggerTreasureCount)*s.gameConfig.ExtraScatterExtraTime

		s.stepMap.New = s.newFreeCount
		s.stepMap.FreeNum = s.newFreeCount
		s.client.ClientOfFreeGame.SetFreeNum(uint64(s.newFreeCount))
		s.client.SetLastMaxFreeNum(uint64(s.newFreeCount))

		// 初始化免费游戏数据
		s.scene.BatPositions = s.treasurePositions
		s.scene.InitialBatCount = s.treasureCount
		s.scene.AccumulatedNewBat = 0
		s.scene.NextStage = _spinTypeFree
		s.scene.IsFirstFree = true
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
	if len(s.stepMap.Bat) > 0 {
		s.scene.BatPositions = make([]*position, len(s.stepMap.Bat))
		for i, bat := range s.stepMap.Bat {
			s.scene.BatPositions[i] = &position{Row: bat.TransX, Col: bat.TransY}
		}
	}

	// 处理新增夺宝（只在没有奖金时，参考xxg）
	s.newFreeCount = 0
	s.stepMap.New = 0

	if !s.bonusAmount.GreaterThan(decimal.Zero) && s.treasureCount > 0 {
		canAddCount := _maxBatPositions - int64(len(s.scene.BatPositions))
		if canAddCount > 0 {
			actualAddCount := min(s.treasureCount, canAddCount)
			s.scene.BatPositions = append(s.scene.BatPositions, s.treasurePositions[:actualAddCount]...)
			s.scene.AccumulatedNewBat += actualAddCount

			// 更新免费次数（每个新蝙蝠+2次）
			s.newFreeCount = actualAddCount * s.gameConfig.ExtraScatterExtraTime
			s.stepMap.New = s.newFreeCount
		}
	}

	s.stepMap.FreeNum = int64(s.client.ClientOfFreeGame.GetFreeNum()) + s.newFreeCount
	if s.newFreeCount > 0 {
		s.client.ClientOfFreeGame.SetFreeNum(uint64(s.stepMap.FreeNum))
		s.client.SetLastMaxFreeNum(uint64(s.stepMap.FreeNum))
	}

	// 检查免费游戏是否结束
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
