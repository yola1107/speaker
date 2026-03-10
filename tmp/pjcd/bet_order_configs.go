package pjcd

import (
	"math/rand/v2"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

// gameConfigJson 游戏配置
type gameConfigJson struct {
	PayTable                       [][]int64 `json:"pay_table"`                            // 赔付表
	Lines                          [][]int   `json:"lines"`                                // 中奖线定义（20条支付线）
	BaseSymbolWeights              []int     `json:"base_symbol_weights"`                  // 基础模式符号权重（万分比）
	FreeSymbolWeights              []int     `json:"free_symbol_weights"`                  // 免费模式符号权重（万分比）
	SymbolPermutationWeights       []int     `json:"symbol_permutation_weights"`           // 符号排列权重（单符号/二连/三连）
	BaseScatterProb                int       `json:"base_scatter_prob"`                    // 基础模式夺宝符号替换概率（万分比）
	BaseWildProb                   int       `json:"base_wild_prob"`                       // 基础模式百搭符号替换概率（万分比）
	FreeScatterProb                int       `json:"free_scatter_prob"`                    // 免费模式夺宝符号替换概率（万分比）
	FreeWildProb                   int       `json:"free_wild_prob"`                       // 免费模式百搭符号替换概率（万分比）
	BaseRoundMultipliers           []int64   `json:"base_round_multipliers"`               // 基础模式轮次倍数 [1,2,3,5]
	FreeRoundMultipliers           []int64   `json:"free_round_multipliers"`               // 免费模式轮次倍数 [3,6,9,15]
	WildAddFourthMultiplier        int64     `json:"wild_add_fourth_multiplier"`           // 蝴蝶百搭增加第4轮倍数值
	BaseReelGenerateInterval       int64     `json:"base_reel_generate_interval"`          // 基础轮轴重新生成间隔
	FreeGameTimes                  int64     `json:"free_game_times"`                      // 免费游戏基础次数
	FreeGameScatterMin             int64     `json:"free_game_scatter_min"`                // 触发免费游戏最小夺宝符号数
	FreeGameAddTimesPerScatter     int64     `json:"free_game_add_times_per_scatter"`      // 免费游戏每个额外夺宝符号增加次数
	FreeGameAddTimes               int64     `json:"free_game_add_times"`                  // 免费模式再触发基础增加次数
	FreeGameAddTimesScatterMin     int64     `json:"free_game_add_times_scatter_min"`      // 免费模式再触发最小夺宝符号数
	FreeGameAddMoreTimesPerScatter int64     `json:"free_game_add_more_times_per_scatter"` // 免费模式再触发每个额外夺宝符号增加次数
}

// initGameConfigs 初始化游戏配置
func (s *betOrderService) initGameConfigs() {
	if s.gameConfig != nil {
		return
	}
	s.parseGameConfigs()
}

// parseGameConfigs 解析游戏配置
func (s *betOrderService) parseGameConfigs() {
	raw := _gameJsonConfigsRaw
	if !s.debug.open {
		cacheText, _ := common.GetRedisGameJson(GameID)
		if len(cacheText) > 0 {
			raw = cacheText
		}
	}
	s.gameConfig = &gameConfigJson{}
	if err := jsoniter.UnmarshalFromString(raw, s.gameConfig); err != nil {
		panic(err)
	}
}

// GetSymbolOdds 获取符号指定连数的赔率
func (c *gameConfigJson) GetSymbolOdds(symbol int64, count int) int64 {
	if symbol < 1 || symbol > 7 {
		return 0
	}
	if count < 3 || count > 5 {
		return 0
	}
	return c.PayTable[symbol-1][count-1]
}

// GetRoundMultiplier 获取轮次倍数
func (c *gameConfigJson) GetRoundMultiplier(roundIndex int, isFree bool) int64 {
	multipliers := c.BaseRoundMultipliers
	if isFree {
		multipliers = c.FreeRoundMultipliers
	}
	if roundIndex < 0 {
		roundIndex = 0
	}
	if roundIndex >= len(multipliers) {
		roundIndex = len(multipliers) - 1
	}
	return multipliers[roundIndex]
}

// CalcInitialFreeSpins 计算初始免费次数（基础模式触发）
func (c *gameConfigJson) CalcInitialFreeSpins(scatterCount int64) int64 {
	if scatterCount < c.FreeGameScatterMin {
		return 0
	}
	extraScatters := scatterCount - c.FreeGameScatterMin
	return c.FreeGameTimes + extraScatters*c.FreeGameAddTimesPerScatter
}

// CalcRetriggerSpins 计算再触发新增次数（免费模式内）
func (c *gameConfigJson) CalcRetriggerSpins(scatterCount int64) int64 {
	if scatterCount < c.FreeGameAddTimesScatterMin {
		return 0
	}
	extraScatters := scatterCount - c.FreeGameAddTimesScatterMin
	return c.FreeGameAddTimes + extraScatters*c.FreeGameAddMoreTimesPerScatter
}

// initSpinSymbol 初始化轮轴（动态生成）
func (s *betOrderService) initSpinSymbol() [_colCount]SymbolRoller {
	var rollers [_colCount]SymbolRoller
	for col := 0; col < _colCount; col++ {
		rollers[col] = s.generateReel(col)
	}
	return rollers
}

// generateFullReelData 生成完整轮轴数据（5列×100符号）
func (s *betOrderService) generateFullReelData(isFree bool) [][]int64 {
	reelData := make([][]int64, _colCount)
	for col := 0; col < _colCount; col++ {
		reelData[col] = s.generateReelSymbolsFull(col, isFree)
	}
	return reelData
}

// generateReelSymbolsFull 生成单列完整轮轴符号序列（100个符号）
func (s *betOrderService) generateReelSymbolsFull(col int, isFree bool) []int64 {
	weights := s.gameConfig.BaseSymbolWeights
	scatterProb := s.gameConfig.BaseScatterProb
	wildProb := s.gameConfig.BaseWildProb

	if isFree {
		weights = s.gameConfig.FreeSymbolWeights
		scatterProb = s.gameConfig.FreeScatterProb
		wildProb = s.gameConfig.FreeWildProb
	}

	allowWild := col >= 1 && col <= 3 // WILD 只能在中间三列
	return s.generateReelSymbols(weights, scatterProb, wildProb, allowWild)
}

// getBoardFromReelData 从完整轮轴数据提取当前盘面
func (s *betOrderService) getBoardFromReelData(reelData [][]int64) [_colCount]SymbolRoller {
	var rollers [_colCount]SymbolRoller
	for col := 0; col < _colCount; col++ {
		reel := reelData[col]
		reelLen := len(reel)
		if reelLen == 0 {
			continue
		}
		// 随机选择起始位置
		startPos := rand.IntN(reelLen)
		// 提取盘面符号（从下到上）
		boardSymbol := make([]int64, _rowCount)
		for row := 0; row < _rowCount; row++ {
			idx := (startPos + row) % reelLen
			boardSymbol[_rowCount-1-row] = reel[idx]
		}
		rollers[col] = SymbolRoller{
			BoardSymbol: boardSymbol,
			Position:    startPos,
			Length:      reelLen,
		}
	}
	return rollers
}

// generateReel 生成单列轮轴
// 规则：WILD 只能在中间三列（列索引1,2,3）出现
func (s *betOrderService) generateReel(col int) SymbolRoller {
	weights := s.gameConfig.BaseSymbolWeights
	scatterProb := s.gameConfig.BaseScatterProb
	wildProb := s.gameConfig.BaseWildProb

	if s.isFreeRound {
		weights = s.gameConfig.FreeSymbolWeights
		scatterProb = s.gameConfig.FreeScatterProb
		wildProb = s.gameConfig.FreeWildProb
	}

	// WILD 只能在中间三列（列索引1,2,3）出现
	allowWild := col >= 1 && col <= 3

	// 生成轮轴符号（使用排列权重决定单符号/二连/三连）
	reel := s.generateReelSymbols(weights, scatterProb, wildProb, allowWild)

	// 随机选择起始位置
	startPos := 0
	if len(reel) > _rowCount {
		startPos = rand.IntN(len(reel) - _rowCount + 1)
	}

	// 提取盘面符号（从下到上）
	boardSymbol := make([]int64, _rowCount)
	for row := 0; row < _rowCount; row++ {
		idx := (startPos + row) % len(reel)
		boardSymbol[_rowCount-1-row] = reel[idx]
	}

	return SymbolRoller{
		BoardSymbol: boardSymbol,
		Position:    startPos,
		Length:      len(reel),
	}
}

// generateReelSymbols 基于权重生成轮轴符号序列
// 规则：按排列权重生成符号组，相邻组不能是相同符号
// allowWild: 是否允许替换为WILD（仅中间三列允许）
func (s *betOrderService) generateReelSymbols(weights []int, scatterProb, wildProb int, allowWild bool) []int64 {
	// 符号ID范围：1-7（普通符号），8=wild，9=scatter
	const symbolCount = 7

	// 计算权重总和
	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}
	if totalWeight == 0 {
		totalWeight = 10000 // 默认值
	}

	// 根据排列权重生成符号序列
	permWeights := s.gameConfig.SymbolPermutationWeights
	if len(permWeights) == 0 {
		permWeights = []int{8100, 1700, 200} // 默认：单符号81%，二连17%，三连2%
	}

	var reel []int64
	targetLength := _reelLength
	var lastSymbol int64 = -1 // 上一个填充的基础符号（用于避免连续相同）

	for len(reel) < targetLength {
		// 决定连续数量（1/2/3个相同符号）
		consecCount := s.weightedRandom(permWeights)
		if consecCount < 1 {
			consecCount = 1
		}
		if consecCount > 3 {
			consecCount = 3
		}

		// 选择符号（避免与上一个符号相同）
		symbol := s.weightedRandomSymbolExcluding(weights, totalWeight, symbolCount, lastSymbol)

		// 填充连续符号
		for i := 0; i < consecCount && len(reel) < targetLength; i++ {
			// 可能替换为 scatter 或 wild（wild仅中间三列）
			finalSymbol := s.maybeReplaceSpecial(symbol, scatterProb, wildProb, allowWild)
			reel = append(reel, finalSymbol)
		}
		lastSymbol = symbol
	}

	return reel
}

// weightedRandom 基于权重随机选择索引
func (s *betOrderService) weightedRandom(weights []int) int {
	if len(weights) == 0 {
		return 0
	}
	total := 0
	for _, w := range weights {
		total += w
	}
	if total == 0 {
		return 0
	}
	r := rand.IntN(total)
	for i, w := range weights {
		if r < w {
			return i + 1 // 返回1-based值（连续数量）
		}
		r -= w
	}
	return len(weights)
}

// weightedRandomSymbol 基于权重随机选择符号
func (s *betOrderService) weightedRandomSymbol(weights []int, totalWeight, symbolCount int) int64 {
	if totalWeight == 0 {
		return int64(rand.IntN(symbolCount) + 1)
	}
	r := rand.IntN(totalWeight)
	for i, w := range weights {
		if r < w {
			return int64(i + 1)
		}
		r -= w
	}
	return int64(symbolCount)
}

// weightedRandomSymbolExcluding 基于权重随机选择符号，排除指定符号
func (s *betOrderService) weightedRandomSymbolExcluding(weights []int, totalWeight, symbolCount int, exclude int64) int64 {
	if totalWeight == 0 {
		symbol := int64(rand.IntN(symbolCount) + 1)
		for symbol == exclude && symbolCount > 1 {
			symbol = int64(rand.IntN(symbolCount) + 1)
		}
		return symbol
	}

	// 如果只有一个符号，直接返回
	if symbolCount <= 1 {
		return 1
	}

	// 计算排除后的有效权重
	validWeight := totalWeight
	if exclude > 0 && int(exclude) <= len(weights) {
		validWeight -= weights[exclude-1]
	}

	if validWeight <= 0 {
		// 排除后无有效选择，随机选一个非排除的
		for {
			symbol := int64(rand.IntN(symbolCount) + 1)
			if symbol != exclude {
				return symbol
			}
		}
	}

	r := rand.IntN(validWeight)
	for i, w := range weights {
		symbol := int64(i + 1)
		if symbol == exclude {
			continue // 跳过排除的符号
		}
		if r < w {
			return symbol
		}
		r -= w
	}

	// 回退：返回第一个非排除符号
	for i := int64(1); i <= int64(symbolCount); i++ {
		if i != exclude {
			return i
		}
	}
	return 1
}

// maybeReplaceSpecial 可能替换为特殊符号（scatter/wild）
// allowWild: 是否允许替换为WILD（仅中间三列允许）
func (s *betOrderService) maybeReplaceSpecial(symbol int64, scatterProb, wildProb int, allowWild bool) int64 {
	// 万分比概率判断
	r := rand.IntN(10000)
	if r < scatterProb {
		return _scatter
	}
	// WILD 替换仅限中间三列
	if allowWild && r < scatterProb+wildProb {
		return _wild
	}
	return symbol
}

// ringSymbol 轮轴滚动填充新符号
// col: 当前列索引（用于判断是否允许WILD）
func (rs *SymbolRoller) ringSymbol(svc *betOrderService, col int) {
	// 找到空位（值为0的位置）并填充新符号
	for i := range rs.BoardSymbol {
		if rs.BoardSymbol[i] == 0 {
			rs.BoardSymbol[i] = svc.generateSingleSymbol(col)
		}
	}
}

// generateSingleSymbol 生成单个符号（用于下落填充）
// col: 当前列索引（WILD仅允许在中间三列）
func (s *betOrderService) generateSingleSymbol(col int) int64 {
	weights := s.gameConfig.BaseSymbolWeights
	scatterProb := s.gameConfig.BaseScatterProb
	wildProb := s.gameConfig.BaseWildProb

	if s.isFreeRound {
		weights = s.gameConfig.FreeSymbolWeights
		scatterProb = s.gameConfig.FreeScatterProb
		wildProb = s.gameConfig.FreeWildProb
	}

	// 计算权重总和
	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}
	if totalWeight == 0 {
		totalWeight = 10000
	}

	// 选择符号
	symbol := s.weightedRandomSymbol(weights, totalWeight, 7)

	// WILD 仅允许在中间三列（列索引1,2,3）
	allowWild := col >= 1 && col <= 3
	return s.maybeReplaceSpecial(symbol, scatterProb, wildProb, allowWild)
}
