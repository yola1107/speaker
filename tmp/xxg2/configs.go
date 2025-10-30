package xxg2

import (
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
	tempConfig := gameConfigJson{}
	if err := jsoniter.UnmarshalFromString(jsonConfigRaw, &tempConfig); err != nil {
		panic("parse game config failed: " + err.Error())
	}

	// 校验通过，赋值给gameConfig
	s.gameConfig = &tempConfig
}
