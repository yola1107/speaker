package pjcd

import (
	"math/rand/v2"

	jsoniter "github.com/json-iterator/go"

	"egame-grpc/game/common"
)

type gameConfigJson struct {
	PayTable                   [][]int64 `json:"pay_table"`                       // 赔付表
	Lines                      [][]int64 `json:"lines"`                           // 中奖线定义（20条支付线）
	BaseSymbolWeights          []int     `json:"base_symbol_weights"`             // 基础模式符号权重（万分比）
	FreeSymbolWeights          []int     `json:"free_symbol_weights"`             // 免费模式符号权重（万分比）
	SymbolPermutationWeights   []int     `json:"symbol_permutation_weights"`      // 符号排列权重（单符号/二连/三连）
	BaseScatterProb            int       `json:"base_scatter_prob"`               // 基础模式夺宝符号替换概率（万分比）
	BaseWildProb               int       `json:"base_wild_prob"`                  // 基础模式百搭符号替换概率（万分比）
	FreeScatterProb            int       `json:"free_scatter_prob"`               // 免费模式夺宝符号替换概率（万分比）
	FreeWildProb               int       `json:"free_wild_prob"`                  // 免费模式百搭符号替换概率（万分比）
	BaseRoundMultipliers       []int64   `json:"base_round_multipliers"`          // 基础模式轮次倍数 [1,2,3,5]
	FreeRoundMultipliers       []int64   `json:"free_round_multipliers"`          // 免费模式轮次倍数 [3,6,9,15]
	WildAddFourthMultiple      int64     `json:"wild_add_fourth_multiple"`        // 蝴蝶百搭增加第4轮倍数值（配置里拼写为 multipier）
	BaseReelGenerateInterval   int       `json:"base_reel_generate_interval"`     // 基础轮轴重新生成间隔
	FreeGameSpins              int64     `json:"free_game_spins"`                 // 免费游戏基础次数
	FreeGameScatterMin         int64     `json:"free_game_scatter_min"`           // 触发免费游戏最小夺宝符号数
	FreeGameAddSpinsPerScatter int64     `json:"free_game_add_spins_per_scatter"` // 免费游戏每个额外夺宝符号增加次数
	FreeGameTwoScatterAddTimes int64     `json:"free_game_two_scatter_add_times"` // 免费模式 2 个夺宝时额外增加次数（再触发基础值）
}

type SymbolRoller struct {
	Real        int              `json:"real"`  // 当前轮轴模式：0基础/1免费
	Col         int              `json:"col"`   // 第几列
	Len         int              `json:"len"`   // 长度
	Start       int              `json:"start"` // 开始索引
	Fall        int              `json:"fall"`  // 下落索引（用于连消时获取下一个符号）
	BoardSymbol [_rowCount]int64 `json:"board"` // 盘面符号
	Reel        []int64          `json:"reel"`  // 当前列完整轮轴数据
}

func (s *betOrderService) initGameConfigs() {
	if s.gameConfig != nil {
		return
	}
	s.parseGameConfigs()
}

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

const _reelLength = 100
const _probabilityBase = 10000
const _reelModeBase = 0
const _reelModeFree = 1

func (s *betOrderService) initSpinSymbol() [_colCount]SymbolRoller {
	reelMode := _reelModeBase
	reelData := s.getOrBuildBaseReelData()
	if s.isFreeRound {
		reelMode = _reelModeFree
		reelData = s.getOrBuildFreeReelData()
	}
	return s.buildBoardFromReelData(reelData, reelMode)
}

func (s *betOrderService) getOrBuildBaseReelData() [][]int64 {
	interval := s.gameConfig.BaseReelGenerateInterval
	if interval <= 0 {
		interval = 1
	}
	needGenerate := !s.hasValidReelData(s.scene.BaseReelData)
	if !needGenerate && s.scene.BaseReelUseCount >= int64(interval) {
		needGenerate = true
	}
	if needGenerate {
		s.scene.BaseReelData = s.cloneReelData(s.generateFullReelData(false))
		s.scene.BaseReelUseCount = 0
	}
	s.scene.BaseReelUseCount++
	if len(s.scene.FreeReelData) > 0 {
		s.scene.FreeReelData = nil
	}
	return s.scene.BaseReelData
}

func (s *betOrderService) getOrBuildFreeReelData() [][]int64 {
	if !s.hasValidReelData(s.scene.FreeReelData) {
		s.scene.FreeReelData = s.cloneReelData(s.generateFullReelData(true))
	}
	return s.scene.FreeReelData
}

func (s *betOrderService) hasValidReelData(reelData [][]int64) bool {
	if len(reelData) != _colCount {
		return false
	}
	for col := 0; col < _colCount; col++ {
		if len(reelData[col]) == 0 {
			return false
		}
	}
	return true
}

func (s *betOrderService) cloneReelData(src [][]int64) [][]int64 {
	dst := make([][]int64, len(src))
	for col := range src {
		if len(src[col]) == 0 {
			continue
		}
		dst[col] = make([]int64, len(src[col]))
		copy(dst[col], src[col])
	}
	return dst
}

func (s *betOrderService) buildBoardFromReelData(reelData [][]int64, reelMode int) [_colCount]SymbolRoller {
	var symbols [_colCount]SymbolRoller
	for col := 0; col < _colCount; col++ {
		reel := reelData[col]
		reelLen := len(reel)
		if reelLen == 0 {
			continue
		}
		start := rand.IntN(reelLen)
		end := (start + _rowCount - 1) % reelLen

		roller := SymbolRoller{
			Real:  reelMode,
			Col:   col,
			Len:   reelLen,
			Start: start,
			Fall:  end,
			Reel:  reel,
		}
		for row := 0; row < _rowCount; row++ {
			symbol := reel[(start+row)%reelLen]
			roller.BoardSymbol[int(_rowCount)-1-row] = symbol
		}
		symbols[col] = roller
	}
	return symbols
}

func (s *betOrderService) generateFullReelData(isFree bool) [][]int64 {
	reelData := make([][]int64, _colCount)
	for col := 0; col < _colCount; col++ {
		reelData[col] = s.generateReelSymbolsFull(col, isFree)
	}
	return reelData
}

func (s *betOrderService) generateReelSymbolsFull(col int, isFree bool) []int64 {
	weights := s.gameConfig.BaseSymbolWeights
	scatterProb := s.gameConfig.BaseScatterProb
	wildProb := s.gameConfig.BaseWildProb
	if isFree {
		weights = s.gameConfig.FreeSymbolWeights
		scatterProb = s.gameConfig.FreeScatterProb
		wildProb = s.gameConfig.FreeWildProb
	}
	allowWild := col >= 1 && col <= 3
	return s.generateReelSymbols(weights, scatterProb, wildProb, allowWild)
}

func (s *betOrderService) generateReelSymbols(weights []int, scatterProb, wildProb int, allowWild bool) []int64 {
	permWeights := s.gameConfig.SymbolPermutationWeights
	if len(permWeights) == 0 {
		permWeights = []int{8100, 1700, 200}
	}

	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}
	if totalWeight <= 0 {
		totalWeight = _probabilityBase
	}

	reel := make([]int64, 0, _reelLength)
	lastSymbol := int64(-1)
	for len(reel) < _reelLength {
		consecutiveCount := s.weightedRandomConsecutiveCount(permWeights)
		baseSymbol := s.weightedRandomSymbolExcluding(weights, totalWeight, lastSymbol)
		for i := 0; i < consecutiveCount && len(reel) < _reelLength; i++ {
			reel = append(reel, maybeReplaceSpecial(baseSymbol, scatterProb, wildProb, allowWild))
		}
		lastSymbol = baseSymbol
	}
	return reel
}

func (s *betOrderService) weightedRandomConsecutiveCount(weights []int) int {
	total := 0
	for _, w := range weights {
		total += w
	}
	if total <= 0 {
		return 1
	}
	r := rand.IntN(total)
	for i, w := range weights {
		if r < w {
			return i + 1
		}
		r -= w
	}
	return len(weights)
}

func (s *betOrderService) weightedRandomSymbolExcluding(weights []int, totalWeight int, exclude int64) int64 {
	if len(weights) == 0 {
		return 1
	}
	validWeight := totalWeight
	if exclude >= 1 && exclude <= int64(len(weights)) {
		validWeight -= weights[exclude-1]
	}
	if validWeight <= 0 {
		for symbol := int64(1); symbol <= int64(len(weights)); symbol++ {
			if symbol != exclude {
				return symbol
			}
		}
		return 1
	}
	r := rand.IntN(validWeight)
	for i, w := range weights {
		symbol := int64(i + 1)
		if symbol == exclude {
			continue
		}
		if r < w {
			return symbol
		}
		r -= w
	}
	for symbol := int64(1); symbol <= int64(len(weights)); symbol++ {
		if symbol != exclude {
			return symbol
		}
	}
	return 1
}

func maybeReplaceSpecial(symbol int64, scatterProb, wildProb int, allowWild bool) int64 {
	r := rand.IntN(_probabilityBase)
	if r < scatterProb {
		return _treasure
	}
	if allowWild && r < scatterProb+wildProb {
		return _wild
	}
	return symbol
}

func (rs *SymbolRoller) ringSymbol() {
	var newBoard [_rowCount]int64
	for i, s := range rs.BoardSymbol {
		if s != 0 {
			newBoard[i] = s
		}
	}
	for i := int(_rowCount) - 1; i >= 0; i-- {
		if newBoard[i] == 0 {
			newBoard[i] = rs.getFallSymbol()
		}
	}
	rs.BoardSymbol = newBoard
}

func (rs *SymbolRoller) getFallSymbol() int64 {
	if len(rs.Reel) == 0 {
		return 0
	}
	rs.Fall = (rs.Fall + 1) % len(rs.Reel)
	return rs.Reel[rs.Fall]
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

// calcNewFreeGameNum 计算触发免费游戏的次数
// 基础模式：scatter >= free_game_scatter_min → free_game_spins + (scatter - min) * free_game_add_spins_per_scatter
// 免费模式再触发：scatter >= 2 → free_game_two_scatter_add_times + (scatter - 2) * free_game_add_spins_per_scatter
func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if s.isFreeRound {
		// 免费模式再触发：2 个夺宝起算
		const freeRetriggerScatterMin = 2
		if scatterCount < freeRetriggerScatterMin {
			return 0
		}
		return s.gameConfig.FreeGameTwoScatterAddTimes +
			(scatterCount-freeRetriggerScatterMin)*s.gameConfig.FreeGameAddSpinsPerScatter
	}

	// 基础模式触发
	if scatterCount < s.gameConfig.FreeGameScatterMin {
		return 0
	}
	return s.gameConfig.FreeGameSpins +
		(scatterCount-s.gameConfig.FreeGameScatterMin)*s.gameConfig.FreeGameAddSpinsPerScatter
}
