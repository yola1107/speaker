package tmtg

import (
	"fmt"
	"math/rand/v2"

	"egame-grpc/game/common"
	"egame-grpc/utils/jsonx"
)

type gameConfigJson struct {
	PayTable        [][]int64   `json:"pay_table"`
	PayTableScatter []int64     `json:"pay_table_scatter"`
	WildMaxLimit    int         `json:"wild_max_limit"`
	MaxWinCap       int64       `json:"max_win_cap"`
	FreeTrigger     FreeTrigger `json:"free_trigger"`
	BaseGame        ModeConfig  `json:"base_game"`
	FreeGame        ModeConfig  `json:"free_game"`
	FreeBuy         ModeConfig  `json:"free_buy"`
	RollCfg         RollConf    `json:"roll_cfg"`
	RealData        []Reel      `json:"real_data"`
}

type FreeTrigger struct {
	InitialSpins           int   `json:"initial_spins"`
	RetriggerCount         int   `json:"retrigger_count"`
	RetriggerAdd           int   `json:"retrigger_add"`
	InitialScatterByBuy    []int `json:"initial_scatter_by_buy"` // 初始化4-6个Scatter权重
	_initialScatterByBuyWT int
}

type ModeConfig struct {
	BombGen BombGenConfig `json:"bomb_gen"`
	WildGen WildGenConfig `json:"wild_gen"`
}

type BombGenConfig struct {
	ProbPerCol float64 `json:"prob_per_col"`
	Multiplier []int64 `json:"multiplier"`
	Weight     []int   `json:"weight"`
	WTotal     int     `json:"-"`
}

type WildGenConfig struct {
	InitialSpawn    []int `json:"initial_spawn"`
	TumbleRefill    []int `json:"tumble_refill"`
	_initialSpawnWT int
	_tumbleRefillWT int
}

type RollConf struct {
	BaseGame    RollCfgType `json:"base_game"`
	FreeGame    RollCfgType `json:"free_game"`
	FreeBuy     RollCfgType `json:"free_buy"`
	FreeBuyBase RollCfgType `json:"free_buy_base"`
}

type RollCfgType struct {
	UseKey []int `json:"use_key"`
	Weight []int `json:"weight"`
	WTotal int   `json:"-"`
}

type Reel [][]int64

type SymbolRoller struct {
	Real        int              `json:"real"`  // 选择的第几个轮盘
	Col         int              `json:"col"`   // 第几列
	Len         int              `json:"len"`   // 长度
	Start       int              `json:"start"` // 开始索引
	Fall        int              `json:"fall"`  // 结束索引
	BoardSymbol [_rowCount]int64 `json:"board"` // 盘面符号
	OriStart    int              `json:"-"`     // 原始补位读取起点
}

func (s *betOrderService) initGameConfigs() {
	if s.gameConfig != nil {
		return
	}
	raw := _gameJsonConfigsRaw
	if !s.debug.open {
		if cacheText, _ := common.GetRedisGameJson(GameID); len(cacheText) > 0 {
			raw = cacheText
		}
	}
	s.gameConfig = &gameConfigJson{}
	if err := jsonx.UnmarshalString(raw, s.gameConfig); err != nil {
		panic(err)
	}
	if err := s.gameConfig.validate(); err != nil {
		panic(err)
	}
}

func (c *gameConfigJson) validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if len(c.PayTable) == 0 {
		return fmt.Errorf("pay_table is empty")
	}
	for name, rc := range map[string]*RollCfgType{
		"base_game":     &c.RollCfg.BaseGame,
		"free_game":     &c.RollCfg.FreeGame,
		"free_buy":      &c.RollCfg.FreeBuy,
		"free_buy_base": &c.RollCfg.FreeBuyBase,
	} {
		if len(rc.Weight) != len(rc.UseKey) {
			return fmt.Errorf("roll weight and use_key length not match. %s", name)
		}
		rc.WTotal = 0
		for _, w := range rc.Weight {
			rc.WTotal += w
		}
		if rc.WTotal <= 0 {
			return fmt.Errorf("roll weight sum <= 0. %s", name)
		}
	}
	if len(c.RealData) == 0 {
		return fmt.Errorf("real_data is empty")
	}

	for name, mode := range map[string]*ModeConfig{
		"base_game": &c.BaseGame,
		"free_game": &c.FreeGame,
		"free_buy":  &c.FreeBuy,
	} {
		if len(mode.BombGen.Weight) != len(mode.BombGen.Multiplier) {
			return fmt.Errorf("multipler/weight not match. %s", name)
		}
		mode.BombGen.WTotal = 0
		for _, w := range mode.BombGen.Weight {
			mode.BombGen.WTotal += w
		}
		if mode.BombGen.WTotal <= 0 {
			return fmt.Errorf("multipler weight sum <= 0. %s", name)
		}
		mode.WildGen._initialSpawnWT = 0
		for _, w := range mode.WildGen.InitialSpawn {
			mode.WildGen._initialSpawnWT += w
		}
		if mode.WildGen._initialSpawnWT <= 0 {
			return fmt.Errorf("initial_spawn weight sum <= 0. %s", name)
		}
		mode.WildGen._tumbleRefillWT = 0
		for _, w := range mode.WildGen.TumbleRefill {
			mode.WildGen._tumbleRefillWT += w
		}
		if mode.WildGen._tumbleRefillWT <= 0 {
			return fmt.Errorf("tumble_refill weight sum <= 0. %s", name)
		}
	}
	c.FreeTrigger._initialScatterByBuyWT = 0
	for _, w := range c.FreeTrigger.InitialScatterByBuy {
		c.FreeTrigger._initialScatterByBuyWT += w
	}
	return nil
}

func (c *gameConfigJson) getSceneSymbol(rollCfg RollCfgType) [_colCount]SymbolRoller {
	realIndex := rollCfg.UseKey[pickWeightIndex(rollCfg.Weight, rollCfg.WTotal)]
	realData := c.RealData[realIndex]
	var symbols [_colCount]SymbolRoller
	for col := 0; col < _colCount; col++ {
		reel := realData[col]
		reelLen := len(reel)
		start := rand.IntN(reelLen)
		symbols[col] = SymbolRoller{
			Real:     realIndex,
			Col:      col,
			Len:      reelLen,
			Start:    start,
			OriStart: start,
			Fall:     (start + _rowCount - 1) % reelLen,
		}
		for r := 0; r < _rowCount; r++ {
			symbols[col].BoardSymbol[r] = reel[(start+r)%reelLen]
		}
	}
	return symbols
}

// getSymbolBaseMultiplier pay_table 索引从 matchCount=8 开始（index = matchCount - 8）
// scatterPayMultiplier 夺宝赔付倍数（与 demo min(sc,6) 取档一致：4→[0], 5→[1], 6+→[2]）。
func (c *gameConfigJson) scatterPayMultiplier(scatterCount int64) int64 {
	if scatterCount < _scatterEntryMin || len(c.PayTableScatter) == 0 {
		return 0
	}
	idx := int(scatterCount - _scatterEntryMin)
	if idx >= len(c.PayTableScatter) {
		idx = len(c.PayTableScatter) - 1
	}
	return c.PayTableScatter[idx]
}

func (c *gameConfigJson) getSymbolBaseMultiplier(symbol int64, starN int) int64 {
	if symbol < 1 || int(symbol) > len(c.PayTable) {
		return 0
	}
	table := c.PayTable[symbol-1]
	idx := starN - _minMatchCount
	if idx < 0 {
		return 0
	}
	if idx >= len(table) {
		idx = len(table) - 1
	}
	return table[idx]
}

func (c *gameConfigJson) calcNewFreeGameNum(isFree bool, scatterCount int64) int64 {
	if isFree {
		if scatterCount < int64(c.FreeTrigger.RetriggerCount) {
			return 0
		}
		return int64(c.FreeTrigger.RetriggerAdd)
	}
	if scatterCount < _scatterEntryMin {
		return 0
	}
	return int64(c.FreeTrigger.InitialSpins)
}

// pickWeightIndex 按权重随机选择索引
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
