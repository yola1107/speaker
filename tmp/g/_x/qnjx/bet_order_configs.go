package qnjx

import (
	"math/rand/v2"

	"egame-grpc/game/common"
	"egame-grpc/utils/jsonx"
)

type gameConfigJson struct {
	PayTable                     [][]int64  `json:"pay_table"`                      // 赔率表 [symbol-1][列数-1]
	FreeGameTimes                int64      `json:"free_game_times"`                // 基础免费次数 12
	AddFreeTimes                 int64      `json:"extra_add_free_times"`           // 每多一个夺宝追加的免费次数 2
	FreeGameScatter              int64      `json:"trigger_free_game_need_scatter"` // 触发免费所需夺宝数 4
	MaxWinMultiplier             int64      `json:"max_win_multiplier"`             // 最大奖励倍数
	BigSymbolNums                []int      `json:"free_big_symbol_nums"`           // 基础局首屏每列长符号个数候选（值即长块层级）
	BaseBigSymbolWeights         []int      `json:"base_big_symbol_weights"`        // 基础模式每列长符号数量权重
	FreeBigSymbolWeights         []int      `json:"free_big_symbol_weights"`        // 免费模式每列长符号数量权重
	BigSymbolMultiples           []Location `json:"big_symbol_multiples"`           // 首屏长符号布局模板
	BaseFallBigSymbolProbability int64      `json:"base_fill_symbol_probability"`   // 基础模式消除后补位尝试长块概率(万分比)
	FreeFallBigSymbolProbability int64      `json:"free_fill_symbol_probability"`   // 基础模式消除后补位尝试长块概率(万分比)
	FillBigSymbolMultiples       []Location `json:"fill_big_symbol_multiples"`      // 补位长符号布局模板，与首屏结构相同
	RollCfg                      RollConf   `json:"roll_cfg"`                       //
	RealData                     []Reals    `json:"real_data"`                      //

	baseBigSymbolW int `json:"-"`
	freeBigSymbolW int `json:"-"`
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
type Location [][]int64

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

	if s.gameConfig.MaxWinMultiplier <= 0 {
		panic(" s.gameConfig.MaxWinMultiplier <= 0 ")
	}

	s.validateRollCfg(&s.gameConfig.RollCfg.Base)
	s.validateRollCfg(&s.gameConfig.RollCfg.Free)

	for _, weight := range s.gameConfig.BaseBigSymbolWeights {
		s.gameConfig.baseBigSymbolW += weight
	}
	for _, weight := range s.gameConfig.FreeBigSymbolWeights {
		s.gameConfig.freeBigSymbolW += weight
	}
}

func (s *betOrderService) validateRollCfg(rollCfg *RollCfgType) {
	if len(rollCfg.Weight) != len(rollCfg.UseKey) {
		panic("roll weight and use_key length not match")
	}
	rollCfg.WTotal = 0
	for _, weight := range rollCfg.Weight {
		rollCfg.WTotal += weight
	}
	if rollCfg.WTotal <= 0 {
		panic("roll weight total <= 0")
	}
}

func pickWeightIndexWithSum(weights []int, weightSum int) int {
	if len(weights) <= 1 {
		return 0
	}
	random := rand.IntN(weightSum)
	current := 0
	for i, weight := range weights {
		current += weight
		if random < current {
			return i
		}
	}
	return 0
}

// calcNewFreeGameNum 返回夺宝触发的新增免费次数。
func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if scatterCount < s.gameConfig.FreeGameScatter {
		return 0
	}
	return s.gameConfig.FreeGameTimes +
		(scatterCount-s.gameConfig.FreeGameScatter)*s.gameConfig.AddFreeTimes
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

// initSpinSymbol 根据当前模式生成初始盘面，并完成长符号落盘。
func (s *betOrderService) initSpinSymbol() [_colCount]SymbolRoller {
	c := s.gameConfig

	rollCfg := c.RollCfg.Base
	longWeights := c.BaseBigSymbolWeights
	longWeightSum := c.baseBigSymbolW
	if s.isFreeRound {
		rollCfg = c.RollCfg.Free
		longWeights = c.FreeBigSymbolWeights
		longWeightSum = c.freeBigSymbolW
	}
	realIndex := rollCfg.UseKey[pickWeightIndexWithSum(rollCfg.Weight, rollCfg.WTotal)]
	realData := c.RealData[realIndex]

	var symbols [_colCount]SymbolRoller
	for col := 0; col < _colCount; col++ {
		reel := realData[col]
		reelLen := len(reel)
		start := rand.IntN(reelLen)

		roller := SymbolRoller{
			Real:     realIndex,
			Col:      col,
			Len:      reelLen,
			Start:    start,
			OriStart: start,
		}
		roller.Fall = (start + _rowCount - 1) % reelLen
		for r := 0; r < _rowCount; r++ {
			roller.BoardSymbol[r] = reel[(start+r)%reelLen]
		}
		// 生成长符号
		if col > 0 && col < _colCount-1 && len(c.BigSymbolNums) > 0 {
			longCount := 0
			if longWeightSum > 0 {
				longCount = c.BigSymbolNums[pickWeightIndexWithSum(longWeights, longWeightSum)]
			}
			if longCount > 0 && longCount <= len(c.BigSymbolMultiples) {
				buildLongBlock(&roller, longCount, c.BigSymbolMultiples, reel)
			}
		}
		symbols[col] = roller
	}
	return symbols
}

func buildLongBlock(rs *SymbolRoller, longCount int, bigSymbolMultiples []Location, reel []int64) {
	patterns := bigSymbolMultiples[longCount-1]
	count := len(patterns)
	if count == 0 {
		return
	}
	startIdx := rand.IntN(count)
	for i := 0; i < count; i++ {
		ok, write, newTailIndex, newBoard := matchPattern(patterns[(startIdx+i)%count], _rowCount, rs.Fall, rs.Len, reel)
		if !ok {
			continue
		}
		copy(rs.BoardSymbol[:], newBoard)
		rs.Start = newTailIndex
		rs.OriStart = newTailIndex
		rs.Fall = (rs.Start + write - 1) % rs.Len
		return
	}
}

func (rs *SymbolRoller) fillByFallPatternOrRing(c *gameConfigJson, isFreeRound bool) {
	// 边列不走长符号补位。
	if rs.Col == 0 || rs.Col == _colCount-1 {
		rs.fillByRing(c)
		return
	}

	// 先做概率判定，失败直接回退 ring。
	rate := c.BaseFallBigSymbolProbability
	if isFreeRound {
		rate = c.FreeFallBigSymbolProbability
	}
	if rate <= 0 || rand.Int64N(10000) >= rate {
		rs.fillByRing(c)
		return
	}

	// 统计顶部连续空位。仅顶部连续空位才可套长符号模板。
	empty := 0
	for row := 0; row < _rowCount && rs.BoardSymbol[row] == 0; row++ {
		empty++
	}
	if empty < 2 {
		rs.fillByRing(c)
		return
	}

	patternIdx := empty - 2
	if patternIdx >= len(c.FillBigSymbolMultiples) {
		rs.fillByRing(c)
		return
	}

	patterns := c.FillBigSymbolMultiples[patternIdx]
	patternN := len(patterns)
	if patternN == 0 {
		rs.fillByRing(c)
		return
	}

	startIdx := rand.IntN(patternN)
	reel := c.RealData[rs.Real][rs.Col]
	for i := 0; i < patternN; i++ {
		pattern := patterns[(startIdx+i)%patternN]
		if len(pattern) == 0 {
			continue
		}
		ok, _, newTailIndex, newBoard := matchPattern(pattern, empty, rs.Start, rs.Len, reel)
		if !ok {
			continue
		}
		rs.Start = newTailIndex
		copy(rs.BoardSymbol[:], newBoard)
		return
	}

	rs.fillByRing(c)
}

func (rs *SymbolRoller) fillByRing(c *gameConfigJson) {
	reel := c.RealData[rs.Real][rs.Col]
	reelLen := len(reel)
	start := rs.Start
	for r := _rowCount - 1; r >= 0; r-- {
		if rs.BoardSymbol[r] == 0 {
			start--
			if start < 0 {
				start = reelLen - 1
			}
			rs.BoardSymbol[r] = reel[start]
		}
	}
	rs.Start = start
}

/*
		_treasure int64 = 11 // 夺宝符号
		reel      [2,1,11,4,6]
	    board     [0,0,0,0,6]
		pattern   [1,1,2],
		newBoard  [11,4,6,1006,6]
*/
// matchPattern 将 pattern 写入顶部空位区：
// - readIdx 以 tailIndex 为起点向前读取（--，越界回环到 reelLen-1）；
// - writePos 从空位区底部向上写入，长符号按 [head, tail...] 编码。
func matchPattern(pattern []int64, empty, readIdx, reelLen int, reel []int64) (ok bool, write, newReadIdx int, board []int64) {
	covered := 0
	for _, value := range pattern {
		if value <= 0 {
			return
		}
		if covered+int(value) > empty {
			return
		}
		write++
		covered += int(value)
		if covered == empty {
			break
		}
	}
	if covered != empty {
		return
	}

	board = make([]int64, empty)
	writePos := empty - 1
	for i := write - 1; i >= 0; i-- {
		readIdx--
		if readIdx < 0 {
			readIdx = reelLen - 1
		}
		sym := reel[readIdx]

		// 长符号不含百搭/夺宝
		if pattern[i] >= 2 && !canBeBigSymbol(sym) {
			return
		}

		value := int(pattern[i])
		blockStart := writePos - value + 1
		if blockStart < 0 {
			return
		}
		for k := 0; k < value; k++ {
			if k == 0 {
				board[blockStart+k] = sym
			} else {
				board[blockStart+k] = sym + _longSymbol
			}
		}
		writePos = blockStart - 1
	}
	return true, write, readIdx, board
}

func canBeBigSymbol(symbol int64) bool {
	return symbol > _blank && symbol < _wild
}
