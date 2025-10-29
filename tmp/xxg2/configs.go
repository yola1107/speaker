package xxg2

import (
	"fmt"

	jsoniter "github.com/json-iterator/go"
)

// 游戏配置JSON结构
type gameConfigJson struct {
	PayTable               [][]int64   `json:"pay_table"`                 // 赔付表（符号倍率表）
	BetSizeSlice           []float64   `json:"bet_size"`                  // 下注基础金额
	BetLevelSlice          []int64     `json:"bet_level"`                 // 下注倍数
	BaseBat                int64       `json:"base_bat"`                  // 基础倍数
	FreeGameTriggerScatter int64       `json:"free_game_trigger_scatter"` // 触发免费游戏的scatter数量
	FreeGameInitTimes      int64       `json:"free_game_init_times"`      // 免费游戏初始次数
	ExtraScatterExtraTime  int64       `json:"extra_scatter_extra_time"`  // 额外scatter增加的次数
	RollCfg                rollCfg     `json:"roll_cfg"`                  // 滚轴配置
	RealData               [][][]int64 `json:"real_data"`                 // 真实数据（预设滚轴数据）
}

// 滚轴配置
type rollConfig struct {
	UseKey []int64 `json:"use_key"` // 使用的key数组
	Weight []int64 `json:"weight"`  // 权重数组
}

// 滚轴配置集合
type rollCfg struct {
	Base rollConfig `json:"base"` // 基础游戏配置
	Free rollConfig `json:"free"` // 免费游戏配置
}

// 初始化游戏配置
func (s *betOrderService) initGameConfigs() {
	if s.gameConfig != nil {
		return
	}
	s.parseGameConfigs()
}

// 解析游戏配置
func (s *betOrderService) parseGameConfigs() {
	jsonConfigRaw := _gameJsonConfigsRaw

	// 先解析到临时变量
	tempConfig := &gameConfigJson{}
	if err := jsoniter.UnmarshalFromString(jsonConfigRaw, tempConfig); err != nil {
		panic("parse game config failed: " + err.Error())
	}

	// 校验配置完整性
	s.validateGameConfig(tempConfig)

	// 校验通过，赋值给gameConfig
	s.gameConfig = tempConfig
}

// 校验游戏配置
func (s *betOrderService) validateGameConfig(cfg *gameConfigJson) {
	// 1. 校验PayTable
	if len(cfg.PayTable) == 0 {
		panic("pay_table is empty")
	}
	if len(cfg.PayTable) != 10 {
		panic("pay_table must have 10 symbols")
	}
	for i, table := range cfg.PayTable {
		if len(table) != 3 {
			panic(fmt.Sprintf("pay_table[%d] must have 3 multipliers", i))
		}
		// 校验倍率递增
		if table[0] > table[1] || table[1] > table[2] {
			panic(fmt.Sprintf("pay_table[%d] multipliers must be in ascending order", i))
		}
	}

	// 2. 校验BetSizeSlice
	if len(cfg.BetSizeSlice) == 0 {
		panic("bet_size is empty")
	}
	for i, size := range cfg.BetSizeSlice {
		if size <= 0 {
			panic(fmt.Sprintf("bet_size[%d] must be positive", i))
		}
	}

	// 3. 校验BetLevelSlice
	if len(cfg.BetLevelSlice) == 0 {
		panic("bet_level is empty")
	}
	for i, level := range cfg.BetLevelSlice {
		if level <= 0 {
			panic(fmt.Sprintf("bet_level[%d] must be positive", i))
		}
	}

	// 4. 校验BaseBat
	if cfg.BaseBat <= 0 {
		panic("base_bat must be positive")
	}

	// 5. 校验FreeGameTriggerScatter
	if cfg.FreeGameTriggerScatter < 3 {
		panic("free_game_trigger_scatter must be at least 3")
	}

	// 6. 校验FreeGameInitTimes
	if cfg.FreeGameInitTimes <= 0 {
		panic("free_game_init_times must be positive")
	}

	// 7. 校验ExtraScatterExtraTime
	if cfg.ExtraScatterExtraTime < 0 {
		panic("extra_scatter_extra_time must be non-negative")
	}

	// 8. 校验RollCfg
	s.validateRollCfg(&cfg.RollCfg)

	// 9. 校验RealData
	s.validateRealData(cfg.RealData)
}

// 校验滚轴配置
func (s *betOrderService) validateRollCfg(cfg *rollCfg) {
	// 校验Base配置
	if len(cfg.Base.UseKey) == 0 {
		panic("roll_cfg.base.use_key is empty")
	}
	if len(cfg.Base.Weight) == 0 {
		panic("roll_cfg.base.weight is empty")
	}
	if len(cfg.Base.UseKey) != len(cfg.Base.Weight) {
		panic("roll_cfg.base.use_key and weight length mismatch")
	}
	for i, key := range cfg.Base.UseKey {
		if key < 0 {
			panic(fmt.Sprintf("roll_cfg.base.use_key[%d] must be non-negative", i))
		}
	}
	for i, weight := range cfg.Base.Weight {
		if weight <= 0 {
			panic(fmt.Sprintf("roll_cfg.base.weight[%d] must be positive", i))
		}
	}

	// 校验Free配置
	if len(cfg.Free.UseKey) == 0 {
		panic("roll_cfg.free.use_key is empty")
	}
	if len(cfg.Free.Weight) == 0 {
		panic("roll_cfg.free.weight is empty")
	}
	if len(cfg.Free.UseKey) != len(cfg.Free.Weight) {
		panic("roll_cfg.free.use_key and weight length mismatch")
	}
	for i, key := range cfg.Free.UseKey {
		if key < 0 {
			panic(fmt.Sprintf("roll_cfg.free.use_key[%d] must be non-negative", i))
		}
	}
	for i, weight := range cfg.Free.Weight {
		if weight <= 0 {
			panic(fmt.Sprintf("roll_cfg.free.weight[%d] must be positive", i))
		}
	}
}

// 校验预设滚轴数据
func (s *betOrderService) validateRealData(data [][][]int64) {
	if len(data) == 0 {
		panic("real_data is empty")
	}

	// 校验每个配置
	for i, config := range data {
		if len(config) != 5 {
			panic(fmt.Sprintf("real_data[%d] must have 5 reels", i))
		}
		// 校验每列数据
		for j, reel := range config {
			if len(reel) == 0 {
				panic(fmt.Sprintf("real_data[%d][%d] reel is empty", i, j))
			}
			// 校验符号ID范围
			for k, symbol := range reel {
				if symbol < 1 || symbol > 11 {
					panic(fmt.Sprintf("real_data[%d][%d][%d] symbol %d out of range (1-11)", i, j, k, symbol))
				}
			}
		}
	}
}
