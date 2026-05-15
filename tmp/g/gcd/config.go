package gcd

import (
	"fmt"

	"egame-grpc/utils/jsonx"
)

type GameConfig struct {
	PayTable  [][]int64  `json:"pay_table"`
	BetSize   []float64  `json:"bet_size"`
	BetLevel  []int      `json:"bet_level"`
	LinesNum  int        `json:"linesNum"`
	Lines     [][]int    `json:"lines"`
	BaseMulti []int64    `json:"base_multi"`
	Free      FreeConfig `json:"free_game"`
	RollCfg   RollConf   `json:"roll_cfg"`
	RealData  []Reals    `json:"real_data"`
}

type FreeConfig struct {
	ScatterMin int64       `json:"scatter_min"`
	Free1      FreeGameSub `json:"free1"`
	Free2      FreeGameSub `json:"free2"`
	Free3      FreeGameSub `json:"free3"`
}

type FreeGameSub struct {
	Times int64   `json:"times"`
	Multi []int64 `json:"multi"`
}

type RollConf struct {
	Base  RollCfgType `json:"base"`
	Free1 RollCfgType `json:"free1"`
	Free2 RollCfgType `json:"free2"`
	Free3 RollCfgType `json:"free3"`
}

type RollCfgType struct {
	UseKey []int64 `json:"use_key"`
	Weight []int   `json:"weight"`
}

type Reals [][]int64

var defaultGameConfig *GameConfig

func newGameConfig(cacheText string) *GameConfig {
	jsonConfigRaw := _gameJsonConfigsRaw
	if len(cacheText) > 0 {
		jsonConfigRaw = cacheText
	} else if defaultGameConfig != nil {
		return defaultGameConfig
	}
	gameConfig := new(GameConfig)
	if err := jsonx.UnmarshalString(jsonConfigRaw, gameConfig); err != nil {
		panic(err)
	}
	if err := gameConfig.validateConfig(); err != nil {
		panic(err)
	}
	if len(cacheText) == 0 {
		defaultGameConfig = gameConfig
	}
	return gameConfig
}

func (c *GameConfig) validateConfig() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if len(c.PayTable) == 0 {
		return fmt.Errorf("pay_table is empty")
	}
	payCols := int(_colCount)
	for i, row := range c.PayTable {
		if len(row) != payCols {
			return fmt.Errorf("pay_table[%d] length is %d, expected %d", i, len(row), payCols)
		}
	}
	if len(c.BetSize) == 0 {
		return fmt.Errorf("bet_size is empty")
	}
	if len(c.BetLevel) == 0 {
		return fmt.Errorf("bet_level is empty")
	}
	if c.LinesNum <= 0 {
		return fmt.Errorf("linesNum must be positive, got %d", c.LinesNum)
	}
	if len(c.Lines) != c.LinesNum {
		return fmt.Errorf("lines length %d != linesNum %d", len(c.Lines), c.LinesNum)
	}
	for i, line := range c.Lines {
		if len(line) != int(_colCount) {
			return fmt.Errorf("lines[%d] length is %d, expected %d", i, len(line), _colCount)
		}
	}
	if len(c.BaseMulti) == 0 {
		return fmt.Errorf("base_multi is empty")
	}
	if c.Free.ScatterMin < 0 {
		return fmt.Errorf("free_game.scatter_min must be >= 0, got %d", c.Free.ScatterMin)
	}
	for name, sub := range map[string]FreeGameSub{"free1": c.Free.Free1, "free2": c.Free.Free2, "free3": c.Free.Free3} {
		if sub.Times < 0 {
			return fmt.Errorf("free_game.%s.times must be >= 0, got %d", name, sub.Times)
		}
		if len(sub.Multi) == 0 {
			return fmt.Errorf("free_game.%s.multi is empty", name)
		}
	}
	for name, r := range map[string]RollCfgType{
		"base": c.RollCfg.Base, "free1": c.RollCfg.Free1, "free2": c.RollCfg.Free2, "free3": c.RollCfg.Free3,
	} {
		if len(r.UseKey) == 0 {
			return fmt.Errorf("roll_cfg.%s.use_key is empty", name)
		}
		if len(r.Weight) != len(r.UseKey) {
			return fmt.Errorf("roll_cfg.%s: weight length %d != use_key length %d", name, len(r.Weight), len(r.UseKey))
		}
		for _, w := range r.Weight {
			if w <= 0 {
				return fmt.Errorf("roll_cfg.%s weight must be positive", name)
			}
		}
	}
	if len(c.RealData) == 0 {
		return fmt.Errorf("real_data is empty")
	}
	for i, col := range c.RealData {
		if len(col) != int(_colCount) {
			return fmt.Errorf("real_data[%d] column count is %d, expected %d", i, len(col), _colCount)
		}
		for j, c1 := range col {
			if len(c1) == 0 {
				return fmt.Errorf("real_data[%d][%d] is empty", i, j)
			}
		}
	}
	return nil
}

func (c *GameConfig) GetFreeCfgByType(ty int64) FreeGameSub {
	if ty == 3 {
		return c.Free.Free3
	}
	if ty == 2 {
		return c.Free.Free2
	}
	return c.Free.Free1
}

func (c *GameConfig) GetRollCfgByType(ty int64) RollCfgType {
	if ty == 3 {
		return c.RollCfg.Free3
	}
	if ty == 2 {
		return c.RollCfg.Free2
	}
	return c.RollCfg.Free1
}
