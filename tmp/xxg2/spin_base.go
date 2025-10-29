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

	s.isFree = s.client.ClientOfFreeGame.GetFreeNum() > 0

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
		if s.isFree {
			s.stepMap.IsFree = 1
		}
		s.isRoundFirstStep = false
	}

	// 加载符号数据到网格
	s.loadStepData()

	// 计算夺宝符号数量和位置
	s.countTreasureSymbols()

	// 蝙蝠移动和Wind转换（xxg2核心逻辑）
	s.collectBat()

	// 查找中奖信息（在符号变换之后）
	s.findWinInfos()

	// 处理中奖信息并计算倍率
	s.processWinInfos()

	// 更新奖金金额
	s.updateBonusAmount()

	// 根据游戏模式更新结果
	if s.isFree {
		s.updateFreeStepResult()
	} else {
		s.updateBaseStepResult()
	}

	// 更新免费状态
	s.isFree = s.client.ClientOfFreeGame.GetFreeNum() > 0

	// 构建返回结果
	return &BaseSpinResult{
		lineMultiplier: s.lineMultiplier,
		stepMultiplier: s.stepMultiplier,
		treasureCount:  s.treasureCount,
		symbolGrid:     s.symbolGrid,
		winGrid:        s.winGrid,
		winResults:     s.winResults,
	}, nil
}

// initSpinSymbol 初始化滚轴符号（使用配置中的 RealData，参考 mahjong）
func (s *betOrderService) initSpinSymbol() [_rowCount * _colCount]int64 {
	// 判断是基础游戏还是免费游戏
	var rollCfg *rollConfig
	if s.isFree {
		rollCfg = &s.gameConfig.RollCfg.Free
	} else {
		rollCfg = &s.gameConfig.RollCfg.Base
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

	// 检查索引范围
	if realIndex >= len(s.gameConfig.RealData) {
		panic("real data index out of range")
	}

	realData := s.gameConfig.RealData[realIndex]
	var symbols [_rowCount * _colCount]int64

	// 从每列的 RealData 中随机选择起始位置，生成符号
	for col := 0; col < int(_colCount); col++ {
		columnData := realData[col]
		realLineLen := len(columnData)

		if realLineLen < int(_rowCount) {
			panic("real data column too short")
		}

		// 随机选择起始位置
		startIndex := rand.IntN(realLineLen)

		//// TODO del test
		//if true {
		//	startIndex = 0
		//}

		// 填充该列的符号（按行列索引）
		for row := 0; row < int(_rowCount); row++ {
			index := (startIndex + row) % realLineLen
			idx := row*int(_colCount) + col
			symbols[idx] = columnData[index]
		}
	}

	return symbols
}

// 更新基础游戏步骤结果
func (s *betOrderService) updateBaseStepResult() {
	// 更新中奖金额
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		bonusFloat := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(bonusFloat)
		s.client.ClientOfFreeGame.IncRoundBonus(bonusFloat)
	}

	// 检查是否触发免费游戏（>=3个夺宝符号）
	if s.treasureCount >= _triggerTreasureCount {
		freeCount := s.calculateFreeTimes(s.treasureCount)

		s.newFreeCount = freeCount
		s.stepMap.New = freeCount
		s.stepMap.FreeNum = freeCount

		s.client.ClientOfFreeGame.SetFreeNum(uint64(freeCount))
		s.client.SetLastMaxFreeNum(uint64(freeCount))

		// 初始化蝙蝠位置：保存触发时的treasure位置，供第一次免费spin使用
		s.scene.BatPositions = s.treasurePositions
	}

	// 校验状态一致性
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
	// 更新免费游戏计数器
	s.client.ClientOfFreeGame.IncrFreeTimes() // 已玩次数+1
	s.client.ClientOfFreeGame.Decr()          // 剩余次数-1

	// 更新中奖金额
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		bonusFloat := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(bonusFloat)
		s.client.ClientOfFreeGame.IncrFreeTotalMoney(bonusFloat)
		s.client.ClientOfFreeGame.IncRoundBonus(bonusFloat)
	}

	// 免费游戏中每个夺宝符号增加免费次数（每个+2次）
	if s.treasureCount > 0 {
		addFreeCount := s.calculateFreeAddTimes(s.treasureCount)
		currentFreeNum := s.client.ClientOfFreeGame.GetFreeNum()
		newFreeNum := currentFreeNum + uint64(addFreeCount)

		s.newFreeCount = addFreeCount
		s.stepMap.New = addFreeCount
		s.stepMap.FreeNum = int64(newFreeNum)

		s.client.ClientOfFreeGame.SetFreeNum(newFreeNum)
		s.client.SetLastMaxFreeNum(newFreeNum)
	} else {
		s.newFreeCount = 0
		s.stepMap.New = 0
		s.stepMap.FreeNum = int64(s.client.ClientOfFreeGame.GetFreeNum())
	}

	// 免费游戏结束时，清空蝙蝠位置
	if s.client.ClientOfFreeGame.GetFreeNum() == 0 {
		s.scene.BatPositions = nil
	}

	// 校验状态一致性
	s.validateGameState()

	s.stepMultiplier = s.lineMultiplier
}
