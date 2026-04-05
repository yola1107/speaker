package ajtm

import (
	"math/rand/v2"
	"sort"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable          [][]int64  `json:"pay_table"`                      // 赔率表 [symbol-1][列数-1]
	FreeGameTimes     int64      `json:"free_game_times"`                // 基础免费次数
	FreeGameBonus     int64      `json:"free_game_bonus"`                // 触发免费时的基础奖励倍数
	ExtraAddFreeBonus int64      `json:"extra_add_free_bonus"`           // 每多一个夺宝追加的奖励倍数
	FreeGameScatter   int64      `json:"trigger_free_game_need_scatter"` // 触发免费所需夺宝数
	MaxWinMultiplier  int64      `json:"max_win_multiplier"`             // 最大奖励倍数
	BaseBigSyWeights  []int64    `json:"base_mysterious_symbol_weights"` // 基础模式每列长符号数量权重
	BigSyMultiples    []Location `json:"mystery_symbol_multiples"`       // 长符号布局模板
	RollCfg           RollConf   `json:"roll_cfg"`
	RealData          []Reals    `json:"real_data"`
}

type RollConf struct {
	Base RollCfgType `json:"base"`
	Free RollCfgType `json:"free"`
}

type RollCfgType struct {
	UseKey []int `json:"use_key"`
	Weight []int `json:"weight"`
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
	if err := jsoniter.UnmarshalFromString(raw, s.gameConfig); err != nil {
		panic(err)
	}

	if s.gameConfig.MaxWinMultiplier <= 0 {
		panic(" s.gameConfig.MaxWinMultiplier <= 0 ")
	}

	s.validateRollCfg(&s.gameConfig.RollCfg.Base)
	s.validateRollCfg(&s.gameConfig.RollCfg.Free)
}

func (s *betOrderService) validateRollCfg(rollCfg *RollCfgType) {
	if len(rollCfg.Weight) != len(rollCfg.UseKey) {
		panic("roll weight and use_key length not match")
	}
}

// ringSymbol 按下落方向补齐当前列中的 0，边列四角不参与补位。
func (rs *SymbolRoller) ringSymbol(gameConfig *gameConfigJson) {
	isEdge := rs.Col == 0 || rs.Col == _colCount-1
	for r := _rowCount - 1; r >= 0; r-- {
		if isEdge && (r == 0 || r == _rowCount-1) {
			continue
		}
		if rs.BoardSymbol[r] == 0 {
			rs.BoardSymbol[r] = rs.getFallSymbol(gameConfig)
		}
	}
}

func (rs *SymbolRoller) getFallSymbol(gameConfig *gameConfigJson) int64 {
	rs.Start--
	if rs.Start < 0 {
		reelLen := len(gameConfig.RealData[rs.Real][rs.Col])
		rs.Start = reelLen - 1
	}
	return gameConfig.RealData[rs.Real][rs.Col][rs.Start]
}

func pickWeightIndex[T int | int64](weights []T) int {
	if len(weights) <= 1 {
		return 0
	}

	var total T
	for _, weight := range weights {
		total += weight
	}
	if total <= 0 {
		return 0
	}

	var random T
	switch any(total).(type) {
	case int:
		random = T(rand.IntN(int(total)))
	case int64:
		random = T(rand.Int64N(int64(total)))
	}

	var current T
	for i, weight := range weights {
		current += weight
		if random < current {
			return i
		}
	}
	return 0
}

// calcNewFreeGameNum 仅在基础模式生效，返回新增免费次数和触发奖励倍数。
func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) (int64, int64) {
	if s.isFreeRound || scatterCount < s.gameConfig.FreeGameScatter {
		return 0, 0
	}
	mul := s.gameConfig.FreeGameBonus +
		(scatterCount-s.gameConfig.FreeGameScatter)*s.gameConfig.ExtraAddFreeBonus
	return s.gameConfig.FreeGameTimes, mul * _baseMultiplier
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
	rollCfg := s.gameConfig.RollCfg.Base
	if s.isFreeRound {
		rollCfg = s.gameConfig.RollCfg.Free
	}
	realIndex := rollCfg.UseKey[pickWeightIndex(rollCfg.Weight)]
	realData := s.gameConfig.RealData[realIndex]

	// 构建不含长符号的基础盘面。
	var symbols [_colCount]SymbolRoller
	for col := 0; col < _colCount; col++ {
		symbols[col] = buildSymbolRoller(col, realIndex, realData[col])
	}

	// 规划并落盘长符号。
	positions := s.buildSpinLongPositions(&symbols)
	nextDownCount := [_colCount]int{}
	for _, pos := range positions {
		s.writeLongToBoard(&symbols, pos.Col, pos.HeadRow)
		if s.isFreeRound && pos.Col > 0 && pos.Col < _colCount-1 {
			nextDownCount[pos.Col]++
		}
	}
	s.scene.DownCount = nextDownCount
	return symbols
}

func buildSymbolRoller(col, realIndex int, reel []int64) SymbolRoller {
	reelLen := len(reel)
	start := rand.IntN(reelLen)
	roller := SymbolRoller{
		Real:     realIndex,
		Col:      col,
		Len:      reelLen,
		Start:    start,
		OriStart: start,
	}

	if col == 0 || col == _colCount-1 {
		roller.Fall = (start + 3) % reelLen
		for r := 1; r < _rowCount-1; r++ {
			roller.BoardSymbol[r] = reel[(start+r-1)%reelLen]
		}
		return roller
	}

	roller.Fall = (start + _rowCount - 1) % reelLen
	for r := 0; r < _rowCount; r++ {
		roller.BoardSymbol[r] = reel[(start+r)%reelLen]
	}
	return roller
}

// writeLongToBoard 把一个 2 格长符号写入当前列盘面，并同步补位索引。
func (s *betOrderService) writeLongToBoard(symbols *[_colCount]SymbolRoller, col, headRow int) {
	tailRow := headRow + 1
	if col <= 0 || col >= _colCount-1 || headRow < 0 || tailRow >= _rowCount || tailRow != headRow+1 {
		return
	}

	roller := &symbols[col]
	roller.Fall--
	if roller.Fall < 0 {
		roller.Fall = roller.Len - 1
	}

	board := &roller.BoardSymbol
	head := (*board)[headRow]

	if s.isFreeRound {
		origin := *board
		head = randomMysSymbol(_treasure)
		(*board)[headRow] = head
		(*board)[tailRow] = _longSymbol + head

		if tailRow+1 < _rowCount {
			dst := (*board)[tailRow+1:]
			srcEnd := headRow + len(dst)
			copy(dst, origin[headRow:srcEnd])
		}
		return
	}

	if tailRow+1 < _rowCount {
		copy((*board)[tailRow+1:], (*board)[tailRow:_rowCount-1])
	}
	(*board)[tailRow] = _longSymbol + head
}

// buildSpinLongPositions 规划本局需要写入的长符号坐标。
func (s *betOrderService) buildSpinLongPositions(symbols *[_colCount]SymbolRoller) []initLongPos {
	if s.isFreeRound {
		maxPerCol := _rowCount / 2
		maxBlocks := (_colCount - 2) * maxPerCol
		positions := make([]initLongPos, 0, maxBlocks)
		candidates := make([]initLongPos, 0, maxBlocks)

		for col := 1; col < _colCount-1; col++ {
			inherited := s.scene.DownCount[col]
			if inherited < 0 {
				inherited = 0
			}
			if inherited > maxPerCol {
				inherited = maxPerCol
			}

			var occupied [_rowCount]bool
			firstHeadRow := _rowCount - inherited*2
			for i := 0; i < inherited; i++ {
				headRow := firstHeadRow + i*2
				tailRow := headRow + 1
				occupied[headRow], occupied[tailRow] = true, true
				positions = append(positions, initLongPos{Col: col, HeadRow: headRow})
			}

			if inherited == maxPerCol {
				continue
			}
			for headRow := 0; headRow < _rowCount-1; headRow++ {
				if occupied[headRow] || occupied[headRow+1] {
					continue
				}
				candidates = append(candidates, initLongPos{Col: col, HeadRow: headRow})
			}
		}

		if len(positions) < maxBlocks && len(candidates) > 0 {
			added := candidates[rand.IntN(len(candidates))]
			positions = append(positions, added)
			if s.debug.open {
				s.debug.freeAddMystery = [2]int64{int64(added.Col), int64(added.HeadRow)}
			}
		}

		sort.Slice(positions, func(i, j int) bool {
			if positions[i].Col != positions[j].Col {
				return positions[i].Col < positions[j].Col
			}
			return positions[i].HeadRow < positions[j].HeadRow
		})
		return positions
	}

	positions := make([]initLongPos, 0, _maxLongBlocks)
	for col := 1; col < _colCount-1; col++ {
		longCount := pickWeightIndex(s.gameConfig.BaseBigSyWeights)
		if longCount == 0 || longCount > len(s.gameConfig.BigSyMultiples) {
			continue
		}

		patterns := s.gameConfig.BigSyMultiples[longCount-1]
		if len(patterns) == 0 {
			continue
		}

		startIdx := rand.IntN(len(patterns))
		for i := 0; i < len(patterns); i++ {
			idx := (startIdx + i) % len(patterns)
			heads, ok := buildColumnLongHeadsByPattern(patterns[idx], symbols[col].BoardSymbol, true)
			if !ok {
				continue
			}
			for _, headRow := range heads {
				positions = append(positions, initLongPos{Col: col, HeadRow: headRow})
			}
			if s.debug.open {
				s.debug.realIndex[col-1] += len(heads)
				s.debug.randomIndex[col-1] = idx + 1
			}
			break
		}
	}
	return positions
}

// buildColumnLongHeadsByPattern 按单列模板构建长符号 headRow 列表。
func buildColumnLongHeadsByPattern(pattern []int64, board [_rowCount]int64, forbidTreasure bool) ([]int, bool) {
	heads := make([]int, 0, len(pattern))
	row := 0

	for _, value := range pattern {
		if value <= 1 {
			row++
			continue
		}

		headRow := row
		tailRow := row + 1
		if tailRow >= _rowCount {
			return nil, false
		}
		if forbidTreasure && board[headRow] == _treasure {
			return nil, false
		}

		heads = append(heads, headRow)
		row += 2
	}
	return heads, true
}

func randomMysSymbol(oldSymbol int64) int64 {
	//return 1 // TODO delete

	for {
		// 仅在 1~13 中随机，排除自身和夺宝。
		n := int64(rand.IntN(13) + 1)
		if n != oldSymbol && n != _treasure {
			return n
		}
	}
}
