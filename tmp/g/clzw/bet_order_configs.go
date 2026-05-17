package clzw

import (
	"fmt"
	"math/rand/v2"

	"egame-grpc/game/common"
	"egame-grpc/utils/jsonx"
)

type gameConfigJson struct {
	PayTable   [][]int64  `json:"pay_table"`     // 赔付表，索引=符号ID-1
	BaseMulti  []int64    `json:"base_multi"`    // 基础连消倍数
	FreeMulti  []int64    `json:"free_multi"`    // 免费连消倍数
	LionMulti  []int64    `json:"lion_multi"`    // 狮子王倍数 3~5个对应倍数
	MaxWinMuli int64      `json:"max_win_multi"` // 最大奖励倍数
	Free       FreeConfig `json:"free"`          // 免费游戏参数
	RollCfg    RollConf   `json:"roll_cfg"`      // 滚轴配置
	RealData   []Reel     `json:"real_data"`     // 滚轴数据 [模式][列]
}

type FreeConfig struct {
	ScatterMin         int64 `json:"scatter_min"`           // 触发免费最少夺宝数
	FreeTimes          int64 `json:"free_times"`            // 基础免费次数
	PerScatterAddTimes int64 `json:"per_scatter_add_times"` // 每多一个夺宝增加次数
}

type RollConf struct {
	Base    RollCfgType `json:"base"`
	Free    RollCfgType `json:"free"`
	BuyBase RollCfgType `json:"buy_base"`
	BuyFree RollCfgType `json:"buy_free"`
}

type RollCfgType struct {
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
	if err := s.gameConfig.validate(); err != nil {
		panic(err)
	}
}

func (c *gameConfigJson) validate() error {
	if len(c.BaseMulti) == 0 {
		return fmt.Errorf("base_multi is empty")
	}
	if len(c.FreeMulti) == 0 {
		return fmt.Errorf("free_multi is empty")
	}
	if c.Free.ScatterMin <= 0 || c.Free.FreeTimes <= 0 {
		return fmt.Errorf("invalid free config")
	}
	for name, rc := range map[string]*RollCfgType{
		"base":     &c.RollCfg.Base,
		"free":     &c.RollCfg.Free,
		"buy_base": &c.RollCfg.BuyBase,
		"buy_free": &c.RollCfg.BuyFree,
	} {
		if len(rc.Weight) != len(rc.UseKey) {
			return fmt.Errorf("roll weight and use_key length not match. %s", name)
		}
		rc.WTotal = 0
		for _, w := range rc.Weight {
			rc.WTotal += w
		}
		if rc.WTotal <= 0 {
			return fmt.Errorf("roll weight sum <= 0. %s", name)
		}
	}
	return nil
}

func (c *gameConfigJson) initSpinSymbol(isFree, isPurchase bool) [_colCount]SymbolRoller {
	switch {
	case isFree && isPurchase:
		return c.getSceneSymbol(c.RollCfg.BuyFree)
	case isFree:
		return c.getSceneSymbol(c.RollCfg.Free)
	case isPurchase:
		return c.getSceneSymbol(c.RollCfg.BuyBase)
	default:
		return c.getSceneSymbol(c.RollCfg.Base)
	}
}

func (c *gameConfigJson) getSceneSymbol(rollCfg RollCfgType) [_colCount]SymbolRoller {
	realIndex := rollCfg.UseKey[pickWeightIndex(rollCfg.Weight, rollCfg.WTotal)]
	realData := c.RealData[realIndex]
	var symbols [_colCount]SymbolRoller
	for col := 0; col < _colCount; col++ {
		reel := realData[col]
		reelLen := len(reel)
		start := rand.IntN(reelLen)
		symbols[col] = SymbolRoller{
			Real:     realIndex,
			Col:      col,
			Len:      reelLen,
			Start:    start,
			OriStart: start,
			Fall:     (start + _rowCount - 1) % reelLen,
		}
		for r := 0; r < _rowCount; r++ {
			symbols[col].BoardSymbol[r] = reel[(start+r)%reelLen]
		}
	}
	return symbols
}

func (c *gameConfigJson) getSymbolBaseMultiplier(symbol int64, starN int) int64 {
	if symbol < 1 || int(symbol) > len(c.PayTable) {
		return 0
	}
	table := c.PayTable[symbol-1]
	if len(table) < starN {
		starN = len(table)
	}
	return table[starN-1]
}

// calcNewFreeGameNum 计算触发免费游戏的次数
// 规则：3个夺宝触发10次免费，每多1个夺宝增加2次免费 -> 10 + (scatterCount-3)*2
func (c *gameConfigJson) calcNewFreeGameNum(scatterCount int64) int64 {
	if scatterCount < c.Free.ScatterMin {
		return 0
	}
	return c.Free.FreeTimes + (scatterCount-c.Free.ScatterMin)*c.Free.PerScatterAddTimes
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
