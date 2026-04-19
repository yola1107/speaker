package jqs

import (
	"math/rand/v2"

	"egame-grpc/game/common"
	//"egame-grpc/game/common/rand"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable                   [][]int64 `json:"pay_table"`                     // 赔付表：[符号类型][连续个数]=赔率倍数
	Lines                      [][]int   `json:"lines"`                         // 中奖线定义：每条线包含3个位置编号(0-8)
	MidSymbolWeight            []int     `json:"mid_symbol_weight"`             // BaseGame中间位置符号权重：[鼠,福字,红包,福袋,鞭炮,年桔,花生]
	BaseTwoConsecutiveProb     int       `json:"base_two_consecutive_prob"`     // BaseGame二连判定概率(万分比)：6000=60%
	BaseComplementSymbolWeight []int     `json:"base_complement_symbol_weight"` // BaseGame补位符号权重：用于生成与中间符号不同的符号
	RespinTriggerRate          int64     `json:"respin_trigger_rate"`           // Re-spin真实触发概率(万分比)：102=1.02%
	FakeRespinTriggerRate      int64     `json:"fake_respin_trigger_rate"`      // Re-spin假触发概率(万分比)：300=3%
	FreeThreeConsecutiveProb   int       `json:"free_three_consecutive_prob"`   // FreeGame三连判定概率(万分比)：6000=60%
	FreeThreeSymbolWeight      []int     `json:"free_three_symbol_weight"`      // FreeGame三连符号权重：[鼠,福字,红包,福袋,鞭炮,年桔,花生]
	FreeTwoSymbolWeight        []int     `json:"free_two_symbol_weight"`        // FreeGame二连符号权重：[鼠,福字,红包,福袋,鞭炮,年桔,花生]
	FreeComplementSymbolWeight []int     `json:"free_complement_symbol_weight"` // FreeGame补位符号权重：用于生成与主符号不同的符号
	MaxPayMultiple             int       `json:"max_pay_multiple"`              // 最大中奖倍数：九个位置全百搭时的奖励倍数
}

func (s *betOrderService) initGameConfigs() {
	if s.gameConfig != nil {
		return
	}
	jsonConfigRaw := _gameJsonConfigsRaw
	if !s.debug.open {
		cacheText, _ := common.GetRedisGameJson(GameID)
		if len(cacheText) > 0 {
			jsonConfigRaw = cacheText
		}
	}
	s.gameConfig = &gameConfigJson{}
	if err := jsoniter.UnmarshalFromString(jsonConfigRaw, s.gameConfig); err != nil {
		panic(err)
	}
}

// 生成BaseGame符号 - 按列生成算法
func (s *betOrderService) initBaseGameSymbols() int64Grid {
	var grid int64Grid

	// 按列生成符号 (第0列, 第1列, 第2列)
	for col := 0; col < _colCount; col++ {
		// 1. 先生成中间位置 (行1)：按权重从7种符号中生成
		middleSymbol := s.randSymbol(s.gameConfig.MidSymbolWeight)
		grid[1][col] = middleSymbol

		// 2. 判定是否纵向形成二连（概率配置：base_two_consecutive_prob）
		if isHit(int64(s.gameConfig.BaseTwoConsecutiveProb)) {
			// 二连判定成功：随机在上方或下方生成一个相同符号
			randomPos := rand.IntN(2) // 0=上方(行0), 1=下方(行2)
			if randomPos == 0 {
				grid[0][col] = middleSymbol
				// 另一个位置按补位权重生成一个符号
				// 文档："直接按权重取补位符号即可，若取到与中间符号相同的符号，则形成一列三连相同符号，这是正常的"
				grid[2][col] = s.randSymbol(s.gameConfig.BaseComplementSymbolWeight)
			} else {
				grid[2][col] = middleSymbol
				// 上方位置按补位权重生成一个符号
				grid[0][col] = s.randSymbol(s.gameConfig.BaseComplementSymbolWeight)
			}
		} else {
			// 二连判定失败：上下两个位置独立按补位权重取两个符号
			// 文档："若取到与中间符号相同的符号，则重新取，直到与中间符号不同为止"
			grid[0][col] = s.diffSymbolRetry(middleSymbol, s.gameConfig.BaseComplementSymbolWeight)
			grid[2][col] = s.diffSymbolRetry(middleSymbol, s.gameConfig.BaseComplementSymbolWeight)
		}
	}

	return grid
}

// Respin模式符号生成
func (s *betOrderService) initRespinSymbols() int64Grid {
	var symbols int64Grid

	// 中间列固定为百搭
	symbols[0][1] = _wild
	symbols[1][1] = _wild
	symbols[2][1] = _wild

	// 第一列和第三列生成
	for _, col := range []int{0, 2} {
		// 判定是否形成三连
		if isHit(int64(s.gameConfig.FreeThreeConsecutiveProb)) {
			// 三连成功
			mainSymbol := s.randSymbol(s.gameConfig.FreeThreeSymbolWeight)
			symbols[0][col] = mainSymbol
			symbols[1][col] = mainSymbol
			symbols[2][col] = mainSymbol
		} else {
			// 必定形成二连：按免费二连权重取符号
			mainSymbol := s.randSymbol(s.gameConfig.FreeTwoSymbolWeight)
			// 在中间位置和上下两位置中随机一个位置生成该符号形成二连
			if rand.IntN(2) == 0 {
				symbols[0][col] = mainSymbol
				symbols[1][col] = mainSymbol
				// 补位位置按免费补位权重取符号，需与主符号不同
				symbols[2][col] = s.diffSymbolRetry(mainSymbol, s.gameConfig.FreeComplementSymbolWeight)
			} else {
				// 上位置按免费补位权重取符号，需与主符号不同
				symbols[0][col] = s.diffSymbolRetry(mainSymbol, s.gameConfig.FreeComplementSymbolWeight)
				symbols[1][col] = mainSymbol
				symbols[2][col] = mainSymbol
			}
		}
	}

	return symbols
}

// 按权重随机生成符号
func (s *betOrderService) randSymbol(weights []int) int64 {
	if len(weights) == 0 {
		return 1
	}

	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}

	if totalWeight <= 0 {
		return 1
	}

	randomValue := rand.IntN(totalWeight)
	cumulativeWeight := 0

	for i, weight := range weights {
		cumulativeWeight += weight
		if randomValue < cumulativeWeight {
			return int64(i + 1) // 符号编号从1开始
		}
	}

	return 1 // 默认返回第一个符号
}

// 根据权重概率随机生成一个符号，可能形成三连（这是正常的）
// 文档说明：“若取到与中间符号相同的符号，则形成一列三连相同符号，这是正常的”
func (s *betOrderService) complementSymbol(excludeSymbol int64, weights []int) int64 {
	if len(weights) == 0 {
		return excludeSymbol%7 + 1
	}

	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}

	if totalWeight <= 0 {
		return excludeSymbol%7 + 1
	}

	// 按权重概率随机生成
	randomValue := rand.IntN(totalWeight)
	cumulativeWeight := 0

	for i, weight := range weights {
		cumulativeWeight += weight
		if randomValue < cumulativeWeight {
			symbol := int64(i + 1)
			// 如果与excludeSymbol相同，形成三连（这是正常的）
			// 如果不同，直接返回
			return symbol
		}
	}

	// 边界保护：返回第一个不同符号
	return excludeSymbol%7 + 1
}

// 不排除任何符号，按权重独立生成
func (s *betOrderService) randIndependent(weights []int) int64 {
	return s.randSymbol(weights)
}

// 按权重独立生成补位符号
func (s *betOrderService) randIndependentComplement(weights []int) int64 {
	return s.randSymbol(weights)
}

// 获取一个与指定符号不同的符号（用于确保二连失败时上下位置与中间符号不同）
func (s *betOrderService) diffSymbol(excludeSymbol int64, weights []int) int64 {
	if len(weights) == 0 {
		return excludeSymbol%7 + 1
	}

	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}

	if totalWeight <= 0 {
		return excludeSymbol%7 + 1
	}

	// 循环直到取到不同符号
	for {
		randomValue := rand.IntN(totalWeight)
		cumulativeWeight := 0
		for i, weight := range weights {
			cumulativeWeight += weight
			if randomValue < cumulativeWeight {
				symbol := int64(i + 1)
				if symbol != excludeSymbol {
					return symbol
				}
				// 相同则继续循环
				break
			}
		}
	}
}

// 生成不同符号（用于二连失败情况，必须与指定符号不同）
// 文档说明："若取到与中间符号相同的符号，则重新取，直到与中间符号不同为止"
func (s *betOrderService) diffSymbolRetry(excludeSymbol int64, weights []int) int64 {
	if len(weights) == 0 {
		return excludeSymbol%7 + 1
	}

	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}

	if totalWeight <= 0 {
		return excludeSymbol%7 + 1
	}

	// 计算有效权重（排除excludeSymbol）
	validWeight := 0
	for i := range weights {
		if int64(i+1) != excludeSymbol {
			validWeight += weights[i]
		}
	}

	// 如果所有符号都与excludeSymbol相同，返回不同的默认符号
	if validWeight <= 0 {
		return excludeSymbol%7 + 1
	}

	// 按权重概率随机生成，直到取到不同符号
	for {
		randomValue := rand.IntN(totalWeight)
		cumulativeWeight := 0
		for i, weight := range weights {
			cumulativeWeight += weight
			if randomValue < cumulativeWeight {
				symbol := int64(i + 1)
				if symbol != excludeSymbol {
					return symbol
				}
				// 相同则继续循环
				break
			}
		}
	}
}

func (s *betOrderService) getSymbolBaseMultiplier(symbol int64, starN int) int64 {
	if len(s.gameConfig.PayTable) < int(symbol) {
		return 0
	}
	table := s.gameConfig.PayTable[symbol-1]
	if len(table) < starN {
		starN = len(table)
	}
	return table[starN-1]
}

// 原有的场景符号生成方法
func (s *betOrderService) getSceneSymbol() int64Grid {
	stage := s.scene.Stage
	if s.scene.NextStage > 0 {
		stage = s.scene.NextStage
	}
	switch stage {
	case _spinTypeBase:
		return s.initBaseGameSymbols()
	case _spinTypeFree:
		return s.initRespinSymbols()
	default:
		return s.initBaseGameSymbols()
	}
}

func isHit(prob int64) bool {
	if prob <= 0 {
		return false
	}
	// 确保概率计算正确：[0,9999] < prob
	return rand.Int64N(10000) < prob
}
