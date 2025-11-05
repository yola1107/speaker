package xslm2

import (
	mathRand "math/rand"

	jsoniter "github.com/json-iterator/go"
)

var _cnf *gameConfig

type gameConfig struct {
	PayTable             [][]int64   `json:"pay_table"`              // 赔付表（符号倍率表）
	BetSizeSlice         []float64   `json:"bet_size"`               // 下注基础金额列表
	BetLevelSlice        []int64     `json:"bet_level"`              // 下注倍数列表
	BaseBat              int64       `json:"base_bat"`               // 基础线数倍数
	FreeRounds           []int64     `json:"free_rounds"`            // 免费次数配置（按夺宝数量索引）
	TriggerTreasureCount int64       `json:"trigger_treasure_count"` // 触发免费的最少夺宝符号数量
	RollCfg              rollCfg     `json:"roll_cfg"`               // 滚轴配置
	RealData             [][][]int64 `json:"real_data"`              // 真实数据（预设滚轴数据）
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
	_cnf = &gameConfig{}
	if err := jsoniter.UnmarshalFromString(_gameJsonConfigsRaw, _cnf); err != nil {
		panic("parse game config failed: " + err.Error())
	}
	if len(_cnf.BetSizeSlice) == 0 || len(_cnf.BetLevelSlice) == 0 {
		panic("bet_size or bet_level config is empty")
	}
}

// initSpinSymbol 根据权重随机生成滚轴符号
func (c *gameConfig) initSpinSymbol(isFreeRound bool) int64Grid {
	cfg := &c.RollCfg.Base
	if isFreeRound {
		cfg = &c.RollCfg.Free
	}

	r := randPool.Get().(*mathRand.Rand)
	defer randPool.Put(r)

	// 根据权重选择数据集索引
	idx := 0
	if len(cfg.Weight) > 1 {
		total := int64(0)
		for _, w := range cfg.Weight {
			total += w
		}
		num := r.Int63n(total)
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

	// 生成符号网格
	var symbolGrid int64Grid
	for col := 0; col < int(_colCount); col++ {
		data := c.RealData[idx][col]
		start := r.Intn(len(data))
		for row := 0; row < int(_rowCount); row++ {
			symbolGrid[row][col] = data[(start+row)%len(data)]
		}
	}
	return symbolGrid
}

// getSymbolMultiplier 获取符号倍率
func (c *gameConfig) getSymbolMultiplier(symbol, symbolCount int64) int64 {
	if symbol <= 0 || symbol > int64(len(c.PayTable)) {
		return 0
	}
	if symbolCount < _minMatchCount || symbolCount > _colCount {
		return 0
	}
	return c.PayTable[symbol-1][symbolCount-_minMatchCount]
}

// getFreeRoundCount 获取免费次数
func (c *gameConfig) getFreeRoundCount(treasureCount int64) int64 {
	if treasureCount < c.TriggerTreasureCount {
		return 0
	}
	idx := treasureCount - c.TriggerTreasureCount
	if idx >= int64(len(c.FreeRounds)) {
		idx = int64(len(c.FreeRounds)) - 1
	}
	return c.FreeRounds[idx]
}

// validateBetSize 校验投注金额
func (c *gameConfig) validateBetSize(baseMoney float64) bool {
	return contains(c.BetSizeSlice, baseMoney)
}

// validateBetLevel 校验投注倍数
func (c *gameConfig) validateBetLevel(multiple int64) bool {
	return contains(c.BetLevelSlice, multiple)
}
