package xxg2

import (
	"math/rand/v2"

	jsoniter "github.com/json-iterator/go"
)

var _cnf *gameConfig

type gameConfig struct {
	PayTable               [][]int64   `json:"pay_table"`                 // 赔付表（符号倍率表）
	BetSizeSlice           []float64   `json:"bet_size"`                  // 下注基础金额
	BetLevelSlice          []int64     `json:"bet_level"`                 // 下注倍数
	BaseBat                int64       `json:"base_bat"`                  // 基础倍数
	MaxBatPositions        int64       `json:"max_bat_positions"`         // 免费游戏中蝙蝠总数上限
	FreeGameTriggerScatter int64       `json:"free_game_trigger_scatter"` // 触发免费游戏的scatter数量
	FreeGameInitTimes      int64       `json:"free_game_init_times"`      // 免费游戏初始次数
	ExtraScatterExtraTime  int64       `json:"extra_scatter_extra_time"`  // 额外scatter增加的次数
	RollCfg                rollCfg     `json:"roll_cfg"`                  // 滚轴配置
	RealData               [][][]int64 `json:"real_data"`                 // 真实数据
}

type rollCfg struct {
	Base rollConfig `json:"base"` // 基础游戏配置
	Free rollConfig `json:"free"` // 免费游戏配置
}

type rollConfig struct {
	UseKey []int64 `json:"use_key"` // 使用的key数组
	Weight []int64 `json:"weight"`  // 权重数组
}

func init() {
	initGameConfigs()
}

// initGameConfigs 初始化游戏配置
func initGameConfigs() {
	if _cnf != nil {
		return
	}
	tmp := &gameConfig{}
	if err := jsoniter.UnmarshalFromString(_gameJsonConfigsRaw, tmp); err != nil {
		panic("parse game config failed: " + err.Error())
	}
	if len(tmp.BetSizeSlice) == 0 || len(tmp.BetLevelSlice) == 0 {
		panic("bet_size or bet_level config is empty")
	}
	_cnf = tmp
}

// initSpinSymbol 根据权重随机生成滚轴符号
func (c *gameConfig) initSpinSymbol(isFreeRound bool) [_rowCount * _colCount]int64 {
	cfg := &c.RollCfg.Base
	if isFreeRound {
		cfg = &c.RollCfg.Free
	}

	idx := 0
	if len(cfg.Weight) > 1 {
		total := int64(0)
		for _, w := range cfg.Weight {
			total += w
		}
		num := rand.Int64N(total)
		for i, w := range cfg.Weight {
			if num < w {
				idx = int(cfg.UseKey[i])
				break
			}
			num -= w
		}
	} else {
		idx = int(cfg.UseKey[0])
	}

	if idx >= len(c.RealData) {
		panic("real data index out of range")
	}

	var symbols [_rowCount * _colCount]int64
	for col := 0; col < int(_colCount); col++ {
		if col >= len(c.RealData[idx]) {
			panic("real data column out of range")
		}
		data := c.RealData[idx][col]
		if len(data) < int(_rowCount) {
			panic("real data column too short")
		}
		start := rand.IntN(len(data))
		for row := 0; row < int(_rowCount); row++ {
			symbols[row*int(_colCount)+col] = data[(start+row)%len(data)]
		}
	}
	return symbols
}

// getSymbolMultiplier 获取符号倍率
func (c *gameConfig) getSymbolMultiplier(symbol int64, symbolCount int64) int64 {
	if len(c.PayTable) < int(symbol) {
		return 0
	}
	table := c.PayTable[symbol-1]
	index := int(symbolCount - _minMatchCount)
	if index < 0 || index >= len(table) {
		return 0
	}
	return table[index]
}

// validateBetSize 校验投注金额
func (c *gameConfig) validateBetSize(baseMoney float64) bool {
	return contains(c.BetSizeSlice, baseMoney)
}

// validateBetLevel 校验投注倍数
func (c *gameConfig) validateBetLevel(multiple int64) bool {
	return contains(c.BetLevelSlice, multiple)
}
