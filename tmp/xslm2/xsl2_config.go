package xslm2

import (
	mathRand "math/rand"

	jsoniter "github.com/json-iterator/go"
)

var _cnf *gameConfig

// gameConfig 游戏配置
type gameConfig struct {
	PayTable      [][]int64   `json:"pay_table"`       // 赔付表（符号倍率表）
	BetSizeSlice  []float64   `json:"bet_size"`        // 下注基础金额列表
	BetLevelSlice []int64     `json:"bet_level"`       // 下注倍数列表
	BaseBat       int64       `json:"base_bat"`        // 基础线数倍数
	FreeSpinCount []int64     `json:"free_spin_count"` // 免费次数配置   对应scatter1~5个时，奖励的免费游戏数量。 [0,0,7,10,15],
	RollCfg       rollCfg     `json:"roll_cfg"`        // 滚轴配置
	RealData      [][][]int64 `json:"real_data"`       // 真实数据（预设滚轴数据）
}

type rollCfg struct {
	Base rollConfig            `json:"base"` // 基础游戏配置
	Free map[string]rollConfig `json:"free"` // 免费游戏配置（key为女性符号收集状态，如"000", "001"等）
}

type rollConfig struct {
	UseKey []int64 `json:"use_key"` // 使用的key数组
	Weight []int64 `json:"weight"`  // 权重数组
	WTotal int64   `json:"-"`       // 权重总和（预计算）
}

// SymbolRoller 符号滚轴（每列一个滚轴）
type SymbolRoller struct {
	Real  int `json:"real"`  // 使用的 RealData 索引
	Start int `json:"start"` // 当前起始位置（会递减）
	Col   int `json:"col"`   // 列索引 (0-4)
}

func init() {
	initGameConfigs()
}

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

	// 预计算基础模式权重总和
	_cnf.RollCfg.Base.WTotal = 0
	for _, w := range _cnf.RollCfg.Base.Weight {
		_cnf.RollCfg.Base.WTotal += w
	}
	if _cnf.RollCfg.Base.WTotal <= 0 {
		panic("base roll weight sum <= 0")
	}

	// 预计算免费模式权重总和
	for key, cfg := range _cnf.RollCfg.Free {
		cfg.WTotal = 0
		for _, w := range cfg.Weight {
			cfg.WTotal += w
		}
		if cfg.WTotal <= 0 {
			panic("free roll weight sum <= 0 for key: " + key)
		}
		_cnf.RollCfg.Free[key] = cfg
	}
}

// initSpinSymbol 生成滚轴符号网格
func (c *gameConfig) initSpinSymbol(isFreeRound bool, femaleCounts [3]int64) (int64Grid, [_colCount]SymbolRoller) {
	// 选择配置
	var cfg rollConfig
	if isFreeRound {
		key := getFemaleCountsKey(femaleCounts)
		if freeCfg, ok := c.RollCfg.Free[key]; ok {
			cfg = freeCfg
		} else {
			cfg = c.RollCfg.Free["000"] // 默认配置
		}
	} else {
		cfg = c.RollCfg.Base
	}

	// 根据权重选择数据集索引
	r := randPool.Get().(*mathRand.Rand)
	defer randPool.Put(r)

	num := r.Int63n(cfg.WTotal)
	realIdx := 0
	for i, w := range cfg.Weight {
		if num < w {
			realIdx = int(cfg.UseKey[i])
			break
		}
		num -= w
	}

	if realIdx >= len(c.RealData) {
		panic("real data index out of range")
	}

	// 生成符号网格和滚轴
	var symbolGrid int64Grid
	var rollers [_colCount]SymbolRoller

	for col := 0; col < int(_colCount); col++ {
		data := c.RealData[realIdx][col]
		if len(data) == 0 {
			panic("real data column is empty")
		}

		start := r.Intn(len(data))
		rollers[col] = SymbolRoller{Real: realIdx, Start: start, Col: col}

		// 生成符号网格（连续取4个）
		for row := 0; row < int(_rowCount); row++ {
			symbolGrid[row][col] = data[(start+row)%len(data)]
		}
	}

	return symbolGrid, rollers
}

// getFallSymbol 从滚轴获取下一个符号（Start递减）
func (r *SymbolRoller) getFallSymbol() int64 {
	data := _cnf.RealData[r.Real][r.Col]
	r.Start--
	if r.Start < 0 {
		r.Start = len(data) - 1
	}
	return data[r.Start]
}

// getSymbolMultiplier 获取符号倍率
func (c *gameConfig) getSymbolMultiplier(symbol, symbolCount int64) int64 {
	if symbol <= 0 || symbol > int64(len(c.PayTable)) {
		return 0
	}
	if symbolCount < _minMatchCount || symbolCount > _colCount {
		return 0
	}
	// pay_table 数组索引对应：索引0=1列，索引1=2列，索引2=3列，索引3=4列，索引4=5列
	return c.PayTable[symbol-1][symbolCount-1]
}

// getFreeRoundCount 获取免费次数
// treasureCount: 夺宝符号数量（1-5）
// free_spin_count数组索引对应：索引0=1个scatter，索引1=2个scatter，索引2=3个scatter，索引3=4个scatter，索引4=5个scatter
func (c *gameConfig) getFreeRoundCount(treasureCount int64) int64 {
	if treasureCount < 1 || treasureCount > 5 {
		return 0
	}
	idx := treasureCount - 1 // 索引0=1个scatter，索引1=2个scatter...
	if idx >= int64(len(c.FreeSpinCount)) {
		idx = int64(len(c.FreeSpinCount)) - 1
	}
	return c.FreeSpinCount[idx]
}
