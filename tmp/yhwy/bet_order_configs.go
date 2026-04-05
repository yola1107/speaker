package yhwy

import (
	"math/rand/v2"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

// gameConfigJson .
type gameConfigJson struct {
	PayTable [][]int64     `json:"pay_table"` // 赔付表，索引为符号 ID，列为 1~5 连的赔率
	Lines    [][]int       `json:"lines"`     // 25 条固定线定义，位置编号范围为 0~19
	Free     freeConfig    `json:"free"`      // 免费游戏触发配置
	Mystery  mysteryConfig `json:"mystery"`   // 百变樱花揭示配置
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
	SymbolId          []int64 `json:"symbol_id"`           // 百变樱花最终揭示为哪种符号
	BaseOpenWeights   []int   `json:"base_open_weight"`    // BaseGame 对应权重
	FreeOpenWeights   []int   `json:"free_open_weight"`    // FreeGame 对应权重
	LvCollectCount    []int64 `json:"lv_collect_count"`    // 樱花收集外观阈值
	SakuraTriggerRate float64 `json:"sakura_trigger_rate"` // 樱吹雪触发概率（百分比）
	SakuraReels       []int   `json:"sakura_reels"`        // 樱吹雪可替换到的最远列（3/4/5）
	SakuraReelsWeight []int   `json:"sakura_reels_weight"` // 对应权重

	_baseOpenWeightTotal int // BaseGame 权重总和，运行时预计算
	_freeOpenWeightTotal int // FreeGame 权重总和，运行时预计算
	_sakuraReelsTotal    int // 樱吹雪列数权重总和，运行时预计算
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
		if cacheText, _ := common.GetRedisGameJson(GameID); len(cacheText) > 0 {
			raw = cacheText
		}
	}

	s.gameConfig = &gameConfigJson{}
	if err := jsoniter.UnmarshalFromString(raw, s.gameConfig); err != nil {
		panic(err)
	}

	s.validateRollCfg(&s.gameConfig.RollCfg.Base)
	s.validateRollCfg(&s.gameConfig.RollCfg.Free)

	s.validateMysteryConfigs()
}

func (s *betOrderService) validateRollCfg(rollCfg *rollCfgType) {
	rollCfg.WTotal = 0
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

func (s *betOrderService) validateMysteryConfigs() {
	if cnt := len(s.gameConfig.Mystery.SymbolId); cnt == 0 ||
		cnt != len(s.gameConfig.Mystery.BaseOpenWeights) ||
		cnt != len(s.gameConfig.Mystery.FreeOpenWeights) {
		panic("invalid mystery base config")
	}

	for _, w := range s.gameConfig.Mystery.BaseOpenWeights {
		s.gameConfig.Mystery._baseOpenWeightTotal += w
	}
	if s.gameConfig.Mystery._baseOpenWeightTotal <= 0 {
		panic("invalid mystery base weight total")
	}
	for _, w := range s.gameConfig.Mystery.FreeOpenWeights {
		s.gameConfig.Mystery._freeOpenWeightTotal += w
	}
	if s.gameConfig.Mystery._freeOpenWeightTotal <= 0 {
		panic("invalid mystery free weight total")
	}

	for _, reel := range s.gameConfig.Mystery.SakuraReels {
		if reel < 3 || reel > _colCount {
			panic("invalid sakura reel")
		}
	}
	for _, w := range s.gameConfig.Mystery.SakuraReelsWeight {
		s.gameConfig.Mystery._sakuraReelsTotal += w
	}
	if s.gameConfig.Mystery._sakuraReelsTotal <= 0 {
		panic("invalid sakura_reels_weight total")
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

func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if s.isFreeRound || scatterCount < s.gameConfig.Free.ScatterMin {
		return 0
	}
	return s.gameConfig.Free.FreeTimes
}

func (s *betOrderService) pickMMysterySymbol() int64 {
	c := s.gameConfig.Mystery
	if s.isFreeRound {
		return c.SymbolId[pickWeightIndex(c.FreeOpenWeights, c._freeOpenWeightTotal)]
	}
	return c.SymbolId[pickWeightIndex(c.BaseOpenWeights, c._baseOpenWeightTotal)]
}

func (s *betOrderService) pickSakuraReels() int {
	c := s.gameConfig.Mystery
	return c.SakuraReels[pickWeightIndex(c.SakuraReelsWeight, c._sakuraReelsTotal)]
}

func (s *betOrderService) isHitSakuraTriggerRate() bool {
	return rand.Int64N(100) < int64(s.gameConfig.Mystery.SakuraTriggerRate)
}
