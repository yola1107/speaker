package pjcd

import (
	"math/rand/v2"
	"strconv"

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

	RollCfg  RollConf `json:"roll_cfg"`  // 滚轴配置
	RealData []Reals  `json:"real_data"` // 真实数据
}

type RollConf struct {
	Base RollCfgType `json:"base"` // 普通游戏滚轴配置
	Free RollCfgType `json:"free"` // 免费游戏滚轴配置
}

type RollCfgType struct {
	UseKey []int `json:"use_key"` // 滚轴数据索引
	Weight []int `json:"weight"`  // 权重
	WTotal int   `json:"-"`       // 总权重（计算得出）
}

type Reals [][]int64

type SymbolRoller struct {
	Real        int              `json:"real"`  // 选择的第几个轮盘
	Col         int              `json:"col"`   // 第几列
	Len         int              `json:"len"`   // 长度
	Start       int              `json:"start"` // 开始索引
	Fall        int              `json:"fall"`  // 开始索引
	BoardSymbol [_rowCount]int64 `json:"board"` // 盘面符号
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

	// 预计算基础/免费模式权重总和
	s.calculateRollWeight(&s.gameConfig.RollCfg.Base)
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free)
}

func (s *betOrderService) calculateRollWeight(rollCfg *RollCfgType) {
	if len(rollCfg.Weight) != len(rollCfg.UseKey) {
		panic("roll weight and use_key length not match")
	}
	rollCfg.WTotal = 0
	for _, w := range rollCfg.Weight {
		rollCfg.WTotal += w
	}
	if rollCfg.WTotal <= 0 {
		panic("roll weight sum <= 0")
	}
}

func (s *betOrderService) initSpinSymbol() [_colCount]SymbolRoller {
	if !s.isFreeRound {
		return s.getSceneSymbol(s.gameConfig.RollCfg.Base)
	}
	// 免费模式使用统一配置
	return s.getSceneSymbol(s.gameConfig.RollCfg.Free)
}

func (s *betOrderService) getSceneSymbol(rollCfg RollCfgType) [_colCount]SymbolRoller {
	realIndex := 0
	r := rand.IntN(rollCfg.WTotal)
	for i, w := range rollCfg.Weight {
		if r < w {
			realIndex = rollCfg.UseKey[i]
			break
		}
		r -= w
	}
	if realIndex < 0 || realIndex >= len(s.gameConfig.RealData) {
		panic("real data index out of range: " + strconv.Itoa(realIndex))
	}
	realData := s.gameConfig.RealData[realIndex]

	var symbols [_colCount]SymbolRoller
	for col := 0; col < _colCount; col++ {
		data := realData[col]
		dataLen := len(data)
		if dataLen == 0 {
			panic("real data column is empty")
		}

		start := rand.IntN(dataLen)
		end := (start + _rowCount - 1) % dataLen
		roller := SymbolRoller{Real: realIndex, Start: start, Fall: end, Col: col, Len: dataLen}

		for row := 0; row < _rowCount; row++ {
			symbol := data[(start+row)%dataLen]
			roller.BoardSymbol[int(_rowCount)-1-row] = symbol
		}
		symbols[col] = roller
	}

	return symbols
}

func (rs *SymbolRoller) ringSymbol(gameConfig *gameConfigJson) {
	var newBoard [_rowCount]int64
	for i, s := range rs.BoardSymbol {
		if s != 0 {
			newBoard[i] = s
		}
	}
	for i := _rowCount - 1; i >= 0; i-- {
		if newBoard[i] == 0 {
			newBoard[i] = rs.getFallSymbol(gameConfig)
		}
	}
	rs.BoardSymbol = newBoard
}

func (rs *SymbolRoller) getFallSymbol(gameConfig *gameConfigJson) int64 {
	data := gameConfig.RealData[rs.Real][rs.Col]
	rs.Fall = (rs.Fall + 1) % len(data)
	return data[rs.Fall]
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
