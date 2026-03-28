package ycj

import (
	"math/rand/v2"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	SymbolMul []float64 `json:"symbol_mul"` // 符号值：1-5=数字，7-14=翻倍，15-17=夺宝(免费次数)
	RollCfg   rollConf  `json:"roll_cfg"`   // 滚轴配置
	RealData  []Reel    `json:"real_data"`  // 滚轴数据 [索引][列]
}

type rollConf struct {
	Base rollCfgType `json:"base"` // 普通模式
	Free rollCfgType `json:"free"` // 免费模式
}

type rollCfgType struct {
	UseKey []int `json:"use_key"` // 使用的滚轴索引
	Weight []int `json:"weight"`  // 选择权重
	WTotal int   `json:"-"`       // 权重总和
}

type Reel [][]int64 // [列][符号列表]

// SymbolRoller 滚轮状态
type SymbolRoller struct {
	Real        int            `json:"real"`  // 选择的第几个轮盘
	Start       int            `json:"start"` // 开始索引
	Fall        int            `json:"fall"`  // 下落索引
	Col         int            `json:"col"`   // 第几列
	BoardSymbol [_rowCount]int `json:"board"` // 当前显示的符号
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

	if len(s.gameConfig.RealData) < 2 {
		panic("real_data length < 2")
	}

	s.calculateRollWeight(&s.gameConfig.RollCfg.Base)
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free)
}

func (s *betOrderService) calculateRollWeight(rollCfg *rollCfgType) {
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

func (s *betOrderService) initSpinSymbol() {
	cfg := s.gameConfig.RollCfg.Base
	if s.isFreeRound {
		cfg = s.gameConfig.RollCfg.Free
	}

	realIndex := cfg.UseKey[pickWeightIndex(cfg.Weight, cfg.WTotal)]
	s.scene.SymbolRoller = s.getSceneSymbol(realIndex)
}

func (s *betOrderService) getSceneSymbol(realIndex int) [_colCount]SymbolRoller {
	var rollers [_colCount]SymbolRoller
	realData := s.gameConfig.RealData[realIndex]

	for c := 0; c < _colCount; c++ {
		reel := realData[c]
		reelLen := len(reel)
		start := rand.IntN(reelLen)

		rollers[c] = SymbolRoller{
			Real:        realIndex,
			Start:       start,
			Fall:        (start + _rowCount - 1) % reelLen,
			Col:         c,
			BoardSymbol: [_rowCount]int{int(reel[start])},
		}
	}
	return rollers
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

// getFallSymbol 获取指定列的下一个符号（顺序下落）
func (s *betOrderService) getFallSymbol(col int) int {
	roller := &s.scene.SymbolRoller[col]
	data := s.gameConfig.RealData[roller.Real][roller.Col]
	nextPos := (roller.Fall + 1) % len(data)
	roller.Fall = nextPos
	return int(data[nextPos])
}

// getRandomSymbol 获取指定列的随机符号
func (s *betOrderService) getRandomSymbol(col int) int {
	roller := &s.scene.SymbolRoller[col]
	data := s.gameConfig.RealData[roller.Real][roller.Col]
	idx := rand.IntN(len(data))
	roller.Start = idx
	roller.Fall = idx
	return int(data[idx])
}
