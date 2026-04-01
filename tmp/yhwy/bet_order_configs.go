package yhwy

import (
	"fmt"
	"math/rand/v2"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

// gameConfigJson 是 yhwy 的完整配置镜像，来源于 game_json.go 或 Redis 热更配置。
type gameConfigJson struct {
	PayTable [][]int64     `json:"pay_table"` // 赔付表，索引为符号 ID，列为 1~5 连的赔率
	Lines    [][]int       `json:"lines"`     // 25 条固定线定义，位置编号范围为 0~19
	Free     freeConfig    `json:"free"`      // 免费游戏触发配置
	Mystery  mysteryConfig `json:"mystery"`   // 百变樱花揭示配置
	Spread   spreadConfig  `json:"spread"`    // 樱花扩散配置
	RollCfg  rollConf      `json:"roll_cfg"`  // 不同阶段使用的滚轴组配置
	RealData []Reel        `json:"real_data"` // 实际滚轴带数据
}

// freeConfig 定义免费游戏的基础规则。
type freeConfig struct {
	ScatterMin int64 `json:"scatter_min"` // 触发免费所需最少 Scatter 数量
	FreeTimes  int64 `json:"free_times"`  // 单次触发赠送的免费次数
}

// mysteryConfig 定义百变樱花最终揭示为哪种符号。
type mysteryConfig struct {
	BaseSymbols []int64 `json:"base_symbols"` // BaseGame 可揭示出的符号集合
	BaseWeights []int   `json:"base_weights"` // BaseGame 对应权重
	FreeSymbols []int64 `json:"free_symbols"` // FreeGame 可揭示出的符号集合
	FreeWeights []int   `json:"free_weights"` // FreeGame 对应权重

	baseTotal int // BaseGame 权重总和，运行时预计算
	freeTotal int // FreeGame 权重总和，运行时预计算
}

// spreadConfig 定义首列满列樱花后，最远可以扩散到哪一列。
type spreadConfig struct {
	ReelNum    []int `json:"reel_num"`    // 候选扩散终点列，取值 2~5
	BaseWeight []int `json:"base_weight"` // BaseGame 权重
	FreeWeight []int `json:"free_weight"` // FreeGame 权重

	baseTotal int // BaseGame 权重总和，运行时预计算
	freeTotal int // FreeGame 权重总和，运行时预计算
}

type rollConf struct {
	Base rollCfgType `json:"base"`
	Free rollCfgType `json:"free"`
}

type rollCfgType struct {
	UseKey []int `json:"use_key"`
	Weight []int `json:"weight"`
	WTotal int   `json:"-"`
}

type Reel [][]int64

type SymbolRoller struct {
	Real        int              `json:"real"`  // 使用的 reel set 索引
	Col         int              `json:"col"`   // 当前列号，0~4
	Len         int              `json:"len"`   // 原始滚轴长度
	Start       int              `json:"start"` // 本次抽样起始下标
	Fall        int              `json:"fall"`  // 本次展示结束下标
	BoardSymbol [_rowCount]int64 `json:"board"` // 停轮后该列 4 个可见符号
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

	if len(s.gameConfig.PayTable) != _symbolCount {
		panic(fmt.Sprintf("pay_table length != %d", _symbolCount))
	}
	if len(s.gameConfig.Lines) != _baseMultiplier {
		panic(fmt.Sprintf("lines length != %d", _baseMultiplier))
	}
	if len(s.gameConfig.RealData) < 2 {
		panic("real_data length < 2")
	}
	if s.gameConfig.Free.ScatterMin <= 0 || s.gameConfig.Free.FreeTimes <= 0 {
		panic("invalid free config")
	}

	s.calculateRollWeight(&s.gameConfig.RollCfg.Base)
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free)
	s.calculateMysteryWeight()
	s.calculateSpreadWeight()
	s.validateLines()
}

func (s *betOrderService) calculateRollWeight(rollCfg *rollCfgType) {
	if len(rollCfg.UseKey) == 0 || len(rollCfg.UseKey) != len(rollCfg.Weight) {
		panic("roll weight and use_key length not match")
	}
	for _, w := range rollCfg.Weight {
		rollCfg.WTotal += w
	}
	if rollCfg.WTotal <= 0 {
		panic("roll weight sum <= 0")
	}
}

func (s *betOrderService) calculateMysteryWeight() {
	if len(s.gameConfig.Mystery.BaseSymbols) == 0 || len(s.gameConfig.Mystery.BaseSymbols) != len(s.gameConfig.Mystery.BaseWeights) {
		panic("invalid mystery base config")
	}
	if len(s.gameConfig.Mystery.FreeSymbols) == 0 || len(s.gameConfig.Mystery.FreeSymbols) != len(s.gameConfig.Mystery.FreeWeights) {
		panic("invalid mystery free config")
	}

	for _, w := range s.gameConfig.Mystery.BaseWeights {
		s.gameConfig.Mystery.baseTotal += w
	}
	for _, w := range s.gameConfig.Mystery.FreeWeights {
		s.gameConfig.Mystery.freeTotal += w
	}
	if s.gameConfig.Mystery.baseTotal <= 0 || s.gameConfig.Mystery.freeTotal <= 0 {
		panic("invalid mystery weight total")
	}
}

func (s *betOrderService) calculateSpreadWeight() {
	if len(s.gameConfig.Spread.ReelNum) == 0 ||
		len(s.gameConfig.Spread.ReelNum) != len(s.gameConfig.Spread.BaseWeight) ||
		len(s.gameConfig.Spread.ReelNum) != len(s.gameConfig.Spread.FreeWeight) {
		panic("invalid spread config")
	}
	for _, reel := range s.gameConfig.Spread.ReelNum {
		if reel < 2 || reel > _colCount {
			panic("invalid spread reel")
		}
	}
	for _, w := range s.gameConfig.Spread.BaseWeight {
		s.gameConfig.Spread.baseTotal += w
	}
	for _, w := range s.gameConfig.Spread.FreeWeight {
		s.gameConfig.Spread.freeTotal += w
	}
	if s.gameConfig.Spread.baseTotal <= 0 || s.gameConfig.Spread.freeTotal <= 0 {
		panic("invalid spread weight total")
	}
}

func (s *betOrderService) validateLines() {
	maxPos := _rowCount*_colCount - 1
	for i, line := range s.gameConfig.Lines {
		if len(line) != _colCount {
			panic(fmt.Sprintf("lines[%d] length != %d", i, _colCount))
		}
		for _, pos := range line {
			if pos < 0 || pos > maxPos {
				panic(fmt.Sprintf("lines[%d] pos out of range: %d", i, pos))
			}
		}
	}
}

func (s *betOrderService) initSpinSymbol() {
	var cfg rollCfgType
	if s.isFreeRound {
		cfg = s.gameConfig.RollCfg.Free
	} else {
		cfg = s.gameConfig.RollCfg.Base
	}
	realIndex := cfg.UseKey[pickWeightIndex(cfg.Weight, cfg.WTotal)]
	s.scene.SymbolRoller = s.getSceneSymbol(realIndex)
}

func (s *betOrderService) getSceneSymbol(realIndex int) [_colCount]SymbolRoller {
	var symbols [_colCount]SymbolRoller
	realData := s.gameConfig.RealData[realIndex]

	for c := 0; c < _colCount; c++ {
		reel := realData[c]
		reelLen := len(reel)
		start := rand.IntN(reelLen)
		roller := SymbolRoller{
			Real:  realIndex,
			Col:   c,
			Len:   reelLen,
			Start: start,
			Fall:  (start + _rowCount - 1) % reelLen,
		}
		for r := 0; r < _rowCount; r++ {
			roller.BoardSymbol[r] = reel[(start+r)%reelLen]
		}
		symbols[c] = roller
	}
	return symbols
}

func pickWeightIndex(weights []int, total int) int {
	if len(weights) <= 1 || total <= 0 {
		return 0
	}
	r := rand.IntN(total)
	curr := 0
	for i, w := range weights {
		curr += w
		if r < curr {
			return i
		}
	}
	return 0
}

func (s *betOrderService) pickRevealSymbol() int64 {
	cfg := s.gameConfig.Mystery
	if s.isFreeRound {
		return cfg.FreeSymbols[pickWeightIndex(cfg.FreeWeights, cfg.freeTotal)]
	}
	return cfg.BaseSymbols[pickWeightIndex(cfg.BaseWeights, cfg.baseTotal)]
}

func (s *betOrderService) pickSpreadToReel() int64 {
	cfg := s.gameConfig.Spread
	if s.isFreeRound {
		return int64(cfg.ReelNum[pickWeightIndex(cfg.FreeWeight, cfg.freeTotal)])
	}
	return int64(cfg.ReelNum[pickWeightIndex(cfg.BaseWeight, cfg.baseTotal)])
}

func (s *betOrderService) getSymbolBaseMultiplier(symbol int64, count int) int64 {
	if symbol < 0 || int(symbol) >= len(s.gameConfig.PayTable) {
		return 0
	}
	table := s.gameConfig.PayTable[symbol]
	if count <= 0 {
		return 0
	}
	if count > len(table) {
		count = len(table)
	}
	return table[count-1]
}

func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if s.isFreeRound || scatterCount < s.gameConfig.Free.ScatterMin {
		return 0
	}
	return s.gameConfig.Free.FreeTimes
}
