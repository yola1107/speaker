package ycpd

import (
	"egame-grpc/game/common"
	"egame-grpc/game/common/rand"
	"egame-grpc/global"
	"egame-grpc/utils/jsonx"

	"go.uber.org/zap"
)

type gameConfigJson struct {
	PayTable         [][]int64 `json:"pay_table"`                      // 赔付表
	FreeGameTimes    int64     `json:"free_game_times"`                // 免费局数
	AddFreeTimes     int64     `json:"extra_add_free_times"`           // 每多 1 夺宝加次
	FreeGameScatter  int64     `json:"trigger_free_game_need_scatter"` // 触发免费最少夺宝
	MaxWinMultiplier int64     `json:"max_win_multiplier"`             // 最大赢倍封顶
	RollCfg          RollConf  `json:"roll_cfg"`                       // 滚轴权重
	RealData         []Reals   `json:"real_data"`                      // 滚轴符号
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
	Real        int              `json:"real"`  // 轮盘组索引
	Start       int              `json:"start"` // 起始索引
	Fall        int              `json:"fall"`  // 下落索引
	Col         int              `json:"col"`   // 列号
	BoardSymbol [_rowCount]int64 `json:"board"`
}

var _gameJsonConfig *gameConfigJson

func (s *betOrderService) initGameConfigs() {
	if s.gameConfig != nil {
		return
	}
	raw := _gameJsonConfigsRaw
	if !s.debug.open {
		if cacheText, err := common.GetRedisGameJson(GameID); err != nil {
			global.GVA_LOG.Warn("initGameConfigs: use default config", zap.Error(err))
		} else if len(cacheText) > 0 {
			raw = cacheText
		}
	}
	parseGameConfigs(raw)
	s.gameConfig = _gameJsonConfig
}

func parseGameConfigs(text string) {
	tmpCnf := &gameConfigJson{}
	if err := jsonx.UnmarshalString(text, tmpCnf); err != nil {
		panic(err)
	}
	if len(tmpCnf.RealData) == 0 {
		panic("no reals data")
	}
	if len(tmpCnf.PayTable) == 0 {
		panic("no pay table conf exists")
	}
	if tmpCnf.MaxWinMultiplier <= 0 {
		tmpCnf.MaxWinMultiplier = 5000
	}

	tmpCnf.RollCfg.Base.WTotal = 0
	for _, w := range tmpCnf.RollCfg.Base.Weight {
		tmpCnf.RollCfg.Base.WTotal += w
	}
	if tmpCnf.RollCfg.Base.WTotal <= 0 {
		panic("real data base roll weight <= 0")
	}

	tmpCnf.RollCfg.Free.WTotal = 0
	for _, w := range tmpCnf.RollCfg.Free.Weight {
		tmpCnf.RollCfg.Free.WTotal += w
	}
	if tmpCnf.RollCfg.Free.WTotal <= 0 {
		panic("real data free roll weight <= 0")
	}

	if len(tmpCnf.RollCfg.Base.Weight) != len(tmpCnf.RollCfg.Base.UseKey) {
		panic("base roll weight and key not match")
	}
	if len(tmpCnf.RollCfg.Free.Weight) != len(tmpCnf.RollCfg.Free.UseKey) {
		panic("free roll weight and key not match")
	}
	_gameJsonConfig = tmpCnf
}

func (s *betOrderService) initSpinSymbol(stage int64) [_colCount]SymbolRoller {
	switch stage {
	case _spinTypeBase:
		return s.getSceneSymbol(s.gameConfig.RollCfg.Base)
	case _spinTypeFree:
		return s.getSceneSymbol(s.gameConfig.RollCfg.Free)
	default:
		return s.getSceneSymbol(s.gameConfig.RollCfg.Base)
	}
}

func (s *betOrderService) getSceneSymbol(rollCfg RollCfgType) [_colCount]SymbolRoller {
	r := rand.IntN(rollCfg.WTotal)
	realIndex := 0
	for i, w := range rollCfg.Weight {
		if r < w {
			realIndex = rollCfg.UseKey[i]
			break
		}
		r -= w
	}
	if len(s.gameConfig.RealData) <= realIndex {
		panic("real data index out of range")
	}
	realData := s.gameConfig.RealData[realIndex]

	var symbols [_colCount]SymbolRoller

	for i := 0; i < _colCount; i++ {
		realLineLen := len(realData[i])
		startIndex := rand.IntN(realLineLen)

		fallIndex := startIndex
		symbols[i].Col = i

		for j := 0; j < _rowCount; j++ {
			index := (startIndex + j) % realLineLen
			sm := realData[i][index]
			if j >= _lineNumber[i] {
				sm = _kong
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

func (rs *SymbolRoller) getFallSymbol(gameConfig *gameConfigJson) int64 {
	rs.Start--
	if rs.Start < 0 {
		realLen := len(gameConfig.RealData[rs.Real][rs.Col])
		rs.Start = realLen - 1
	}

	return gameConfig.RealData[rs.Real][rs.Col][rs.Start]
}

func (rs *SymbolRoller) ringSymbol(gameConfig *gameConfigJson, i int) {
	var newBoard [_rowCount]int64
	index := 0
	for i, sym := range rs.BoardSymbol {
		if sym != 0 {
			newBoard[i] = sym
			index++
		}
	}

	needNewSymbol := _rowCount - index

	for k := needNewSymbol - 1; k >= 0; k-- {
		newSymbol := rs.getFallSymbol(gameConfig)
		newBoard[k] = newSymbol
	}

	rs.BoardSymbol = newBoard
}
