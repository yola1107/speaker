package hcsqy

import (
	"math/rand/v2"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable                []int64   `json:"pay_table"`                  // 赔付表，索引=符号ID-1
	Lines                   [][]int   `json:"lines"`                      // 中奖线定义
	FreeTriggerCount        int64     `json:"free_trigger_count"`         // 触发免费最少夺宝数
	FreeBaseTimes           int64     `json:"free_base_times"`            // 基础免费次数
	FreeExtraPerScatter     int64     `json:"free_extra_per_scatter"`     // 每多一个夺宝增加次数
	BuyFreeMultiplier       int64     `json:"buy_free_multiplier"`        // 购买免费价格倍数
	MustWinProb             float64   `json:"must_win_prob"`              // 必赢触发概率
	WildExpandProb          float64   `json:"wild_expand_prob"`           // 百搭变大触发概率
	LongWildMultipliers     []int64   `json:"long_wild_multipliers"`      // 长条百搭倍数
	LongWildMultiplierProbs []float64 `json:"long_wild_multiplier_probs"` // 长条百搭倍数概率
	RealData                []Reel    `json:"real_data"`                  // 滚轴数据 [模式][列]
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

	if len(s.gameConfig.PayTable) < 8 {
		panic("pay_table length < 8")
	}
	if len(s.gameConfig.RealData) < 2 {
		panic("real_data length < 2")
	}
	if len(s.gameConfig.LongWildMultipliers) != len(s.gameConfig.LongWildMultiplierProbs) {
		panic("multipliers and probs length mismatch")
	}
}

func (s *betOrderService) initSpinSymbol() [_colCount]SymbolRoller {
	realIndex := 0
	if s.isFreeRound {
		realIndex = 1
	}
	return s.getSceneSymbol(realIndex)
}

func (s *betOrderService) getSceneSymbol(realIndex int) [_colCount]SymbolRoller {
	realData := s.gameConfig.RealData[realIndex]
	var symbols [_colCount]SymbolRoller

	for col := 0; col < _colCount; col++ {
		reel := realData[col]
		reelLen := len(reel)
		start := rand.IntN(reelLen)
		roller := SymbolRoller{Real: realIndex, Start: start, Fall: (start + _rowCount - 1) % reelLen, Col: col, Len: reelLen}

		for row := 0; row < _rowCount; row++ {
			roller.BoardSymbol[_rowCount-1-row] = reel[(start+row)%reelLen]
		}
		symbols[col] = roller
	}
	return symbols
}

func (s *betOrderService) getSymbolBaseMultiplier(symbol int64) int64 {
	idx := int(symbol - 1)
	if idx < 0 || idx >= len(s.gameConfig.PayTable) {
		return 0
	}
	return s.gameConfig.PayTable[idx]
}

func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if scatterCount < s.gameConfig.FreeTriggerCount {
		return 0
	}
	return s.gameConfig.FreeBaseTimes + (scatterCount-s.gameConfig.FreeTriggerCount)*s.gameConfig.FreeExtraPerScatter
}

func (s *betOrderService) pickWildMultiplier() int64 {
	r := rand.Float64()
	cumProb := 0.0
	for i, prob := range s.gameConfig.LongWildMultiplierProbs {
		cumProb += prob
		if r < cumProb {
			return s.gameConfig.LongWildMultipliers[i]
		}
	}
	return s.gameConfig.LongWildMultipliers[0]
}
