package ys

import (
	"math/rand/v2"

	"egame-grpc/game/common"
	"egame-grpc/utils/jsonx"
	//"egame-grpc/game/common/rand"
)

type gameConfigJson struct {
	PayTable [][]int64  `json:"pay_table"` // 赔付表，索引=符号ID-1
	Lines    [][]int    `json:"lines"`     // 中奖线定义
	Free     freeConfig `json:"free"`      // 免费参数
	RollCfg  rollConf   `json:"roll_cfg"`  // 滚轴配置
	RealData []Reel     `json:"real_data"` // 滚轴数据 [模式][列]
}

type freeConfig struct {
	ScatterMin         int64 `json:"scatter_min"`           // 触发免费最少夺宝数
	FreeTimes          int64 `json:"free_times"`            // 基础免费次数
	PerScatterAddTimes int64 `json:"per_scatter_add_times"` // 每多一个夺宝增加次数
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
	if err := jsonx.UnmarshalString(raw, s.gameConfig); err != nil {
		panic(err)
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

func (s *betOrderService) initSpinSymbol() [_colCount]SymbolRoller {
	var cfg rollCfgType
	switch {
	case s.isFreeRound:
		cfg = s.gameConfig.RollCfg.Free
	default:
		cfg = s.gameConfig.RollCfg.Base
	}

	realIndex := cfg.UseKey[pickWeightIndex(cfg.Weight, cfg.WTotal)]
	return s.getSceneSymbol(realIndex)
}

func (s *betOrderService) getSceneSymbol(realIndex int) [_colCount]SymbolRoller {
	var symbols [_colCount]SymbolRoller
	realData := s.gameConfig.RealData[realIndex]

	for c := 0; c < _colCount; c++ {
		reel := realData[c]
		reelLen := len(reel)
		start := rand.IntN(reelLen)
		roller := SymbolRoller{Real: realIndex, Start: start, Fall: (start + _rowCount - 1) % reelLen, Col: c, Len: reelLen, OriStart: start}

		for r := 0; r < _rowCount; r++ {
			roller.BoardSymbol[r] = reel[(start+r)%reelLen]
		}
		symbols[c] = roller
	}
	return symbols
}

// ringSymbol 按 qnjx 的下落方向补齐当前列中的 0。
func (rs *SymbolRoller) ringSymbol(gameConfig *gameConfigJson) {
	for r := _rowCount - 1; r >= 0; r-- {
		if rs.BoardSymbol[r] == 0 {
			rs.BoardSymbol[r] = rs.getFallSymbol(gameConfig)
		}
	}
}

func (rs *SymbolRoller) getFallSymbol(gameConfig *gameConfigJson) int64 {
	data := gameConfig.RealData[rs.Real][rs.Col]
	rs.Start--
	if rs.Start < 0 {
		rs.Start = len(data) - 1
	}
	return data[rs.Start]
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
	if scatterCount < s.gameConfig.Free.ScatterMin {
		return 0
	}
	return s.gameConfig.Free.FreeTimes + (scatterCount-s.gameConfig.Free.ScatterMin)*s.gameConfig.Free.PerScatterAddTimes
}
