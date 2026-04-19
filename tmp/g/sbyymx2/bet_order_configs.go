package sbyymx2

import (
	"fmt"
	"math/rand/v2"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable          [][]int64       `json:"pay_table"`           // 赔付表，索引=符号ID-1
	Lines             [][]int         `json:"lines"`               // 中奖线定义
	RespinRate        [][2]int64      `json:"respin_rate"`         // 必赢触发概率分数 [分子,分母]
	WildExpandRate    [][2]int64      `json:"wild_expand_rate"`    // 百搭变大概率分数 [分子,分母]
	ExpandMultiConfig expandMultiConf `json:"expand_multi_config"` // 长条百搭倍数与权重
	RollCfg           rollConf        `json:"roll_cfg"`            // 滚轴配置
	RealData          []Reel          `json:"real_data"`           // 滚轴数据 [模式][列]
}

type expandMultiConf struct {
	Multi  []int64 `json:"multi"`  // 长条百搭倍数
	Weight []int   `json:"weight"` // 长条百搭倍数权重
	WTotal int     `json:"-"`      // 权重总和
}

type rollConf struct {
	Base       rollCfgType `json:"base"`
	BaseRespin rollCfgType `json:"base_respin"`
}

type rollCfgType struct {
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

	if len(s.gameConfig.PayTable) < int(_wild) {
		panic(fmt.Sprintf("pay_table length < %d", _wild))
	}
	if len(s.gameConfig.RealData) < 2 {
		panic("real_data length < 2")
	}
	if len(s.gameConfig.RespinRate) != 1 {
		panic("respin_rate length != 1")
	}
	if len(s.gameConfig.WildExpandRate) != 1 {
		panic("wild_expand_rate length != 1")
	}

	s.calculateExpandWeight(&s.gameConfig.ExpandMultiConfig)

	s.calculateRollWeight(&s.gameConfig.RollCfg.Base)
	s.calculateRollWeight(&s.gameConfig.RollCfg.BaseRespin)
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

func (s *betOrderService) calculateExpandWeight(c *expandMultiConf) {
	if len(c.Multi) == 0 || len(c.Multi) != len(c.Weight) {
		panic("invalid expand_multi_config")
	}
	totalMultiWeight := 0
	for _, w := range c.Weight {
		totalMultiWeight += w
	}
	if totalMultiWeight <= 0 {
		panic("invalid expand_multi_config total")
	}
	c.WTotal = totalMultiWeight
}

func (s *betOrderService) initSpinSymbol() {
	// 根据 stepIsRespinMode 选择滚轴配置
	cfg := s.gameConfig.RollCfg.Base
	s.debug.mode = _spinTypeBase
	if s.stepIsRespinMode {
		cfg = s.gameConfig.RollCfg.BaseRespin
		s.debug.mode = _spinTypeBaseRespin
	}

	realIndex := cfg.UseKey[pickWeightIndex(cfg.Weight, cfg.WTotal)]
	realData := s.gameConfig.RealData[realIndex]

	// 生成盘面符号
	for c := 0; c < _colCount; c++ {
		reel := realData[c]
		reelLen := len(reel)
		start := rand.IntN(reelLen)
		roller := SymbolRoller{Real: realIndex, Start: start, Fall: (start + _rowCount - 1) % reelLen, Col: c, Len: reelLen}

		for r := 0; r < _rowCount; r++ {
			roller.BoardSymbol[_rowCount-1-r] = reel[(start+r)%reelLen]
		}
		s.scene.SymbolRoller[c] = roller
	}

	// 重转模式：固定中间列填满百搭（文档要求百搭只在中间列）
	if s.stepIsRespinMode {
		s.scene.SymbolRoller[1].BoardSymbol = [_rowCount]int64{_wild, _wild, _wild}
	}
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

func (s *betOrderService) weightWildMultiplier() int64 {
	idx := pickWeightIndex(s.gameConfig.ExpandMultiConfig.Weight, s.gameConfig.ExpandMultiConfig.WTotal)
	return s.gameConfig.ExpandMultiConfig.Multi[idx]
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

func (s *betOrderService) isHitRespinProb() bool {
	return isHit(s.gameConfig.RespinRate[0][0], s.gameConfig.RespinRate[0][1])
}

func (s *betOrderService) isHitWildExpandProb() bool {
	return isHit(s.gameConfig.WildExpandRate[0][0], s.gameConfig.WildExpandRate[0][1])
}

// isHit 按 num/den 概率命中：在 [0, den) 均匀取随机数，小于 num 则命中。
func isHit(num, den int64) bool {
	if num <= 0 || den <= 0 {
		return false
	}
	return rand.Int64N(den) < num
}
