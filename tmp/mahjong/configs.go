package mahjong

import (
	"math/rand/v2"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable      [][]int64 `json:"pay_table"`             //赔付表
	BaseGameMulti []int64   `json:"base_streak_multi"`     //普通游戏额外倍数
	FreeGameMulti []int64   `json:"free_streak_multi"`     //免费游戏额外倍数
	GoldBaseProb  int64     `json:"gold_symbol_base_prob"` //普通模式金符号概率

	GoldFreeProb     int64 `json:"gold_symbol_free_prob"`           //免费模式金符号概率
	FreeGameTimes    int64 `json:"free_game_times"`                 //免费游戏次数
	FreeGameMin      int64 `json:"free_game_scatter_min"`           //免费符号最低个数
	FreeGameAddTimes int64 `json:"free_game_add_times_per_scatter"` //免费符号最低个数

	RollCfg  RollConf `json:"roll_cfg"` //滚轮符号
	RealData []Reals  `json:"real_data"`
}

type RollConf struct {
	Base RollCfgType `json:"base"`
	Free RollCfgType `json:"free"`
}

type RollCfgType struct {
	UseKey []int `json:"use_key"`
	Weight []int `json:"weight"`
	WTotal int   `json:"-"`
}
type Reals [][]int64

type SymbolRoller struct {
	Real        int              `json:"real"`  //选择的第几个轮盘
	Start       int              `json:"start"` //开始索引
	Fall        int              `json:"fall"`  //开始索引
	Col         int              `json:"col"`   //第几列
	BoardSymbol [_rowCount]int64 `json:"board"`
}

func (s *betOrderService) initGameConfigs() {
	if s.gameConfig != nil {
		return
	}
	s.parseGameConfigs()
}

func (s *betOrderService) parseGameConfigs() {

	jsonConfigRaw := ""
	jsonConfigRaw = _gameJsonConfigsRaw

	s.gameConfig = &gameConfigJson{}
	if err := jsoniter.UnmarshalFromString(jsonConfigRaw, s.gameConfig); err != nil {
		panic(err)
	}
	if len(s.gameConfig.RealData) == 0 {
		panic("no reals data")
	}
	if len(s.gameConfig.PayTable) == 0 {
		panic("no pay table conf exists")
	}

	s.gameConfig.RollCfg.Base.WTotal = 0
	for _, w := range s.gameConfig.RollCfg.Base.Weight {
		s.gameConfig.RollCfg.Base.WTotal += w
	}
	if s.gameConfig.RollCfg.Base.WTotal <= 0 {
		panic("real data base roll weight <= 0")
	}

	s.gameConfig.RollCfg.Free.WTotal = 0
	for _, w := range s.gameConfig.RollCfg.Free.Weight {
		s.gameConfig.RollCfg.Free.WTotal += w
	}
	if s.gameConfig.RollCfg.Free.WTotal <= 0 {
		panic("real data free roll weight <= 0")
	}

	if len(s.gameConfig.RollCfg.Base.Weight) != len(s.gameConfig.RollCfg.Base.UseKey) {
		panic("base roll weight and key not match")
	}
	if len(s.gameConfig.RollCfg.Free.Weight) != len(s.gameConfig.RollCfg.Free.UseKey) {
		panic("free roll weight and key not match")
	}
}

// 获取符号表
// scene值为 base/free
func (g *betOrderService) initSpinSymbol(stage int8) [_colCount]SymbolRoller {

	switch stage {
	case _spinTypeBase:
		return g.getSceneSymbol(g.gameConfig.RollCfg.Base, false, stage)
	case _spinTypeFree:
		return g.getSceneSymbol(g.gameConfig.RollCfg.Free, false, stage)
	default:
		return g.getSceneSymbol(g.gameConfig.RollCfg.Base, false, stage)
	}
}

func (g *betOrderService) getSceneSymbol(rollCfg RollCfgType, base bool, stage int8) [_colCount]SymbolRoller {
	r := rand.IntN(rollCfg.WTotal)
	realIndex := 0
	for i, w := range rollCfg.Weight {
		if r < w {
			realIndex = rollCfg.UseKey[i]
			break
		}
		r -= w
	}
	if len(g.gameConfig.RealData) <= realIndex {
		panic("real data index out of range")
	}
	realData := g.gameConfig.RealData[realIndex]

	var symbols [_colCount]SymbolRoller

	for i := 0; i < (_colCount); i++ {
		realLineLen := len(realData[i])
		startIndex := rand.IntN(realLineLen)

		//Debug codes
		if base && _baseSpecific[i] > -1 {
			startIndex = _baseSpecific[i]
		}
		fallIndex := startIndex
		symbols[i].Col = i

		for j := 0; j < _rowCount; j++ {

			index := (startIndex + j) % realLineLen
			sm := realData[i][index]
			if i > 0 && i < _colCount-1 {
				gold := rand.Int64N(10000)
				if stage == _spinTypeBase {
					if gold < g.gameConfig.GoldBaseProb {
						if sm < _treasure {
							sm += _goldSymbol
						}
					}
				} else {
					if gold < g.gameConfig.GoldFreeProb {
						if sm < _treasure {
							sm += _goldSymbol
						}

					}
				}
			}

			symbols[i].BoardSymbol[j] = sm
			fallIndex = index
		}

		symbols[i].Start = startIndex
		symbols[i].Fall = fallIndex
		symbols[i].Real = realIndex
	}

	return symbols
}

// 读取符号的赔率
func (g *betOrderService) getSymbolBaseMultiplier(symbol int64, starN int) int64 {
	if len(g.gameConfig.PayTable) < int(symbol) {
		return 0
	}
	table := g.gameConfig.PayTable[symbol-1]
	if len(table) < starN {
		starN = len(table)
	}
	return table[starN-1]
}

// 处理掉符号掉落
func (rs *SymbolRoller) getFallSymbol(gameConfig *gameConfigJson) int64 {

	rs.Start--
	if rs.Start < 0 {
		realLen := len(gameConfig.RealData[rs.Real][rs.Col])
		rs.Start = realLen - 1
	}

	return gameConfig.RealData[rs.Real][rs.Col][rs.Start]

}

// 补充掉下来导致的空缺位置，新的位置仍然按条件判断是否有可能产生金符号
func (rs *SymbolRoller) ringSymbol(gameConfig *gameConfigJson, stage int8, i int) {

	var newBoard [_rowCount]int64

	index := 0
	for i, s := range rs.BoardSymbol {
		if s != 0 {
			newBoard[i] = s
			index++
		}
	}

	needNewSymbol := _rowCount - index

	for k := needNewSymbol - 1; k >= 0; k-- {
		newSymbol := rs.getFallSymbol(gameConfig)

		if i > 0 && i < _colCount-1 {
			gold := rand.Int64N(10000)
			if stage == _spinTypeBase {
				if gold < gameConfig.GoldBaseProb {
					if newSymbol < _treasure {
						newSymbol += _goldSymbol
					}
				}
			} else {
				if gold < gameConfig.GoldFreeProb {
					if newSymbol < _treasure {
						newSymbol += _goldSymbol
					}
				}
			}
		}

		newBoard[k] = newSymbol

	}

	rs.BoardSymbol = newBoard

}
