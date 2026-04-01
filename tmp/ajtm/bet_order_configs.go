package ajtm

import (
	"math/rand/v2"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable          [][]int64  `json:"pay_table"`                      // 赔率表 [symbol-1][列数-1]
	FreeGameTimes     int64      `json:"free_game_times"`                // 基础免费次数
	FreeGameBonus     int64      `json:"free_game_bonus"`                // 触发免费时的基础奖励倍数
	ExtraAddFreeBonus int64      `json:"extra_add_free_bonus"`           // 每多一个夺宝追加的奖励倍数
	FreeGameScatter   int64      `json:"trigger_free_game_need_scatter"` // 触发免费所需夺宝数
	BaseBigSyWeights  []int64    `json:"base_mysterious_symbol_weights"` // 基础模式每列长符号数量权重
	BigSyMultiples    []Location `json:"big_symbol_multiples"`           // 长符号布局模板
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

// SymbolRoller 表示单列滚轴当前窗口状态。
type SymbolRoller struct {
	Real        int              `json:"real"`  // 当前使用的 reel 下标
	Col         int              `json:"col"`   // 当前列号
	Len         int              `json:"len"`   // reel 总长度
	Start       int              `json:"start"` // 当前补位读取起点
	Fall        int              `json:"fall"`  // 当前窗口底部索引
	BoardSymbol [_rowCount]int64 `json:"board"` // 当前列窗口
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
	return s.gameConfig.FreeGameTimes, mul
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

// initSpinSymbol 根据 base/free 配置选 reel，并生成初始盘面。
func (s *betOrderService) initSpinSymbol() [_colCount]SymbolRoller {
	cfg := s.gameConfig.RollCfg.Base
	if s.isFreeRound {
		cfg = s.gameConfig.RollCfg.Free
	}
	realIndex := cfg.UseKey[pickWeightIndex(cfg.Weight)]
	return s.getSceneSymbol(realIndex)
}

func (s *betOrderService) getSceneSymbol(realIndex int) [_colCount]SymbolRoller {
	var symbols [_colCount]SymbolRoller
	realData := s.gameConfig.RealData[realIndex]

	for c := 0; c < _colCount; c++ {
		symbols[c] = buildSymbolRoller(c, realIndex, realData[c])
	}

	spinLongBlocks := s.buildSpinLongBlocks(&symbols)
	for _, block := range spinLongBlocks {
		s.writeLongBlockToBoard(&symbols, block)
	}

	for c := 0; c < _colCount; c++ {
		s.scene.LongCount[c] = 0
	}
	if s.isFreeRound {
		for _, block := range spinLongBlocks {
			if block.Col > 0 && block.Col < _colCount-1 {
				s.scene.LongCount[int(block.Col)]++
			}
		}
	}

	return symbols
}

func buildSymbolRoller(col, realIndex int, reel []int64) SymbolRoller {
	reelLen := len(reel)
	start := rand.IntN(reelLen)
	roller := SymbolRoller{
		Real:  realIndex,
		Col:   col,
		Len:   reelLen,
		Start: start,
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

// writeLongBlockToBoard 把一个长符号块写入当前列盘面，并同步补位索引。
func (s *betOrderService) writeLongBlockToBoard(symbols *[_colCount]SymbolRoller, block Block) {
	if block.Col <= 0 || block.Col >= _colCount-1 {
		return
	}
	if block.HeadRow < 0 || block.TailRow < 0 || block.HeadRow >= _rowCount || block.TailRow >= _rowCount {
		return
	}
	if block.TailRow != block.HeadRow+1 {
		return
	}

	col := int(block.Col)
	symbols[col].Fall--
	if symbols[col].Fall < 0 {
		symbols[col].Fall = symbols[col].Len - 1
	}

	board := &symbols[col].BoardSymbol
	head := (*board)[block.HeadRow]
	for r := _rowCount - 1; r > int(block.TailRow); r-- {
		(*board)[r] = (*board)[r-1]
	}
	(*board)[block.TailRow] = _longSymbol + head
}

// buildSpinLongBlocks 构建本局初始盘面要写入的长符号块。
// 基础模式按列随机生成，免费模式先恢复继承块，再尝试新增 1 个。
func (s *betOrderService) buildSpinLongBlocks(symbols *[_colCount]SymbolRoller) []Block {
	blocks := make([]Block, 0, _maxLongBlocks)

	if s.isFreeRound {
		for c := 1; c < _colCount-1; c++ {
			for count := 0; count < s.scene.LongCount[c]; count++ {
				tailRow := int64(_rowCount - 1 - count*2)
				if tailRow < 1 {
					continue
				}
				blocks = append(blocks, Block{
					Col:     int64(c),
					HeadRow: tailRow - 1,
					TailRow: tailRow,
				})
			}
		}
		if len(blocks) >= _maxLongBlocks {
			return blocks
		}

		patterns := s.gameConfig.BigSyMultiples[0]
		if len(patterns) == 0 {
			return blocks
		}
		cols := []int{1, 2, 3}
		rand.Shuffle(len(cols), func(i, j int) { cols[i], cols[j] = cols[j], cols[i] })
		for _, col := range cols {
			columnBlocks := collectLongBlocksByColumn(blocks, col)
			startIdx := rand.IntN(len(patterns))
			for i := 0; i < len(patterns); i++ {
				pattern := patterns[(startIdx+i)%len(patterns)]
				if patternBlocks, ok := s.buildColumnLongBlocksByPattern(pattern, columnBlocks, col, symbols[col].BoardSymbol, false); ok {
					return append(blocks, patternBlocks...)
				}
			}
		}
		return blocks
	}

	for c := 1; c < _colCount-1; c++ {
		longCount := pickWeightIndex(s.gameConfig.BaseBigSyWeights)
		if longCount == 0 || longCount > len(s.gameConfig.BigSyMultiples) {
			continue
		}

		patterns := s.gameConfig.BigSyMultiples[longCount-1]
		if len(patterns) == 0 {
			continue
		}

		columnBlocks := collectLongBlocksByColumn(blocks, c)
		startIdx := rand.IntN(len(patterns))
		for i := 0; i < len(patterns); i++ {
			pattern := patterns[(startIdx+i)%len(patterns)]
			if patternBlocks, ok := s.buildColumnLongBlocksByPattern(pattern, columnBlocks, c, symbols[c].BoardSymbol, true); ok {
				blocks = append(blocks, patternBlocks...)
				break
			}
		}
	}
	return blocks
}

func collectLongBlocksByColumn(blocks []Block, col int) []Block {
	result := make([]Block, 0, 3)
	for _, block := range blocks {
		if block.Col == int64(col) {
			result = append(result, block)
		}
	}
	return result
}

// buildColumnLongBlocksByPattern 按单列模板构建长符号块。
// forbidTreasure=true 时，长符号头部不能落在夺宝符号上。
func (s *betOrderService) buildColumnLongBlocksByPattern(pattern []int64, columnBlocks []Block, col int, board [_rowCount]int64, forbidTreasure bool) ([]Block, bool) {
	var patternBlocks []Block
	row := 1

	for _, value := range pattern {
		if value > 1 {
			headRow, tailRow := int64(row-1), int64(row)
			if tailRow >= _rowCount {
				return nil, false
			}
			if forbidTreasure && board[int(headRow)] == _treasure {
				return nil, false
			}
			if hasLongBlockRowConflict(columnBlocks, headRow, tailRow) || hasLongBlockRowConflict(patternBlocks, headRow, tailRow) {
				return nil, false
			}

			patternBlocks = append(patternBlocks, Block{
				Col:     int64(col),
				HeadRow: headRow,
				TailRow: tailRow,
			})
			row++
		}
		row++
	}
	return patternBlocks, true
}

func hasLongBlockRowConflict(blocks []Block, headRow, tailRow int64) bool {
	for _, block := range blocks {
		if block.HeadRow == headRow || block.TailRow == headRow ||
			block.HeadRow == tailRow || block.TailRow == tailRow {
			return true
		}
	}
	return false
}
