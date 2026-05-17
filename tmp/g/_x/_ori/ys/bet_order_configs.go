package ys

import (
	"fmt"
	"math/rand/v2"

	"egame-grpc/game/common"
	"egame-grpc/utils/jsonx"
)

type gameConfigJson struct {
	PayTable  [][]int64  `json:"pay_table"`  // 赔付表，索引=符号ID-1
	Lines     [][]int    `json:"lines"`      // 中奖线
	BaseMulti []int64    `json:"base_multi"` // 基础倍数
	Free      FreeConfig `json:"free_game"`  // 免费游戏配置
	RollCfg   RollConf   `json:"roll_cfg"`   // 滚轴配置
	RealData  []Reel     `json:"real_data"`  // 滚轴数据 [模式][列]

	MaxWinMuli int64 `json:"max_win_multi"` // 最大赢倍数
}

type FreeConfig struct {
	ScatterMin int64       `json:"scatter_min"`
	Free1      FreeGameSub `json:"free1"`
	Free2      FreeGameSub `json:"free2"`
	Free3      FreeGameSub `json:"free3"`
}

type FreeGameSub struct {
	Times int64   `json:"times"`
	Multi []int64 `json:"multi"`
}

type RollConf struct {
	Base  RollCfgType `json:"base"`
	Free1 RollCfgType `json:"free1"`
	Free2 RollCfgType `json:"free2"`
	Free3 RollCfgType `json:"free3"`
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
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if len(c.PayTable) == 0 {
		return fmt.Errorf("pay_table is empty")
	}
	for i, row := range c.PayTable {
		if len(row) != _colCount {
			return fmt.Errorf("pay_table[%d] length %d != %d", i, len(row), _colCount)
		}
	}
	for i, line := range c.Lines {
		if len(line) != _colCount {
			return fmt.Errorf("lines[%d] length %d != %d", i, len(line), _colCount)
		}
	}
	if len(c.BaseMulti) == 0 {
		return fmt.Errorf("base_multi is empty")
	}
	for name, sub := range map[string]FreeGameSub{
		"free1": c.Free.Free1,
		"free2": c.Free.Free2,
		"free3": c.Free.Free3,
	} {
		if sub.Times < 0 {
			return fmt.Errorf("free_game.%s.times invalid", name)
		}
		if len(sub.Multi) == 0 {
			return fmt.Errorf("free_game.%s.multi is empty", name)
		}
	}
	if len(c.RealData) == 0 {
		return fmt.Errorf("real_data is empty")
	}
	for i, reel := range c.RealData {
		if len(reel) != _colCount {
			return fmt.Errorf("real_data[%d] cols %d != %d", i, len(reel), _colCount)
		}
		for j, col := range reel {
			if len(col) == 0 {
				return fmt.Errorf("real_data[%d][%d] is empty", i, j)
			}
		}
	}

	for name, rc := range map[string]*RollCfgType{
		"base":  &c.RollCfg.Base,
		"free1": &c.RollCfg.Free1,
		"free2": &c.RollCfg.Free2,
		"free3": &c.RollCfg.Free3,
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

func (c *gameConfigJson) initSpinSymbol(isFree bool, bonusNum int64) [_colCount]SymbolRoller {
	if !isFree {
		return c.getSceneSymbol(c.RollCfg.Base)
	}
	switch bonusNum {
	case 2:
		return c.getSceneSymbol(c.RollCfg.Free2)
	case 3:
		return c.getSceneSymbol(c.RollCfg.Free3)
	default:
		return c.getSceneSymbol(c.RollCfg.Free1)
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

func (c *gameConfigJson) getFreeCfgByType(ty int64) FreeGameSub {
	switch ty {
	case 3:
		return c.Free.Free3
	case 2:
		return c.Free.Free2
	case 1:
		return c.Free.Free1
	}
	return FreeGameSub{}
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
