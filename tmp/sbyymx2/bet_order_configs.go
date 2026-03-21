package sbyymx2

import (
	"fmt"
	"math/rand/v2"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

// symbolWeightTable 策划「符号 + 权重」一行表（权重可近似和为 1，解析时会归一化）
type symbolWeightTable struct {
	Symbols []int64   `json:"symbols"`
	Weights []float64 `json:"weights"`
}

type gameConfigJson struct {
	PayTable []int64 `json:"pay_table"` // 赔付表，索引=符号 ID-1（符号 1..7）

	// Reels 可选：三列条带（无四套权重时使用，兼容旧 Redis）
	Reels [][]int64 `json:"reels,omitempty"`

	// 四套权重：两边/中间列 × 中间行/上下行（与策划表对应）
	WeightSideMiddleRow       *symbolWeightTable `json:"weight_side_middle_row,omitempty"`
	WeightMiddleMiddleRow     *symbolWeightTable `json:"weight_middle_middle_row,omitempty"`
	WeightSideUpperLowerRow   *symbolWeightTable `json:"weight_side_upper_lower_row,omitempty"`
	WeightMiddleUpperLowerRow *symbolWeightTable `json:"weight_middle_upper_lower_row,omitempty"`

	// 中间格为「纯百搭 100」时，额外倍数 x2～x100 的离散分布（与策划「倍数说明」一致）
	PlainWildMultipliers     []int64   `json:"plain_wild_multipliers,omitempty"`
	PlainWildMultiplierProbs []float64 `json:"plain_wild_multiplier_probs,omitempty"`
}

func (c *gameConfigJson) useWeightTables() bool {
	return c != nil &&
		c.WeightSideMiddleRow != nil &&
		c.WeightMiddleMiddleRow != nil &&
		c.WeightSideUpperLowerRow != nil &&
		c.WeightMiddleUpperLowerRow != nil
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
	if len(s.gameConfig.PayTable) != 7 {
		panic("pay_table length must be 7 (symbols 1-7)")
	}

	if s.gameConfig.useWeightTables() {
		s.validateAndNormalizeWeightTable("weight_side_middle_row", s.gameConfig.WeightSideMiddleRow, true, true)
		s.validateAndNormalizeWeightTable("weight_middle_middle_row", s.gameConfig.WeightMiddleMiddleRow, false, true)
		s.validateAndNormalizeWeightTable("weight_side_upper_lower_row", s.gameConfig.WeightSideUpperLowerRow, true, false)
		s.validateAndNormalizeWeightTable("weight_middle_upper_lower_row", s.gameConfig.WeightMiddleUpperLowerRow, false, false)
		s.validatePlainWildMultiplierTable()
		return
	}

	if len(s.gameConfig.Reels) != _colCount {
		panic("reels must have 3 columns when weight tables are absent")
	}
	for c := 0; c < _colCount; c++ {
		if len(s.gameConfig.Reels[c]) == 0 {
			panic(fmt.Sprintf("reel column %d is empty", c))
		}
	}
	for _, col := range []int{0, 2} {
		for i, sym := range s.gameConfig.Reels[col] {
			if sym >= _wild {
				panic(fmt.Sprintf("side reel col %d index %d: symbol %d must be < %d (no wild on L/R)", col, i, sym, _wild))
			}
			if !isSideReelSymbolOK(sym) {
				panic(fmt.Sprintf("side reel col %d index %d: invalid symbol %d (allowed 0,9 or 1-7)", col, i, sym))
			}
		}
	}
	for i, sym := range s.gameConfig.Reels[1] {
		if !isMiddleReelSymbolOK(sym) {
			panic(fmt.Sprintf("middle reel index %d: invalid symbol %d", i, sym))
		}
	}
}

// validateAndNormalizeWeightTable sideCol=true 表示「左右列」表（不允许百搭）；middleRow=true 表示中间行表
func (s *betOrderService) validateAndNormalizeWeightTable(name string, t *symbolWeightTable, sideCol, middleRow bool) {
	if t == nil || len(t.Symbols) == 0 {
		panic(fmt.Sprintf("%s: empty", name))
	}
	if len(t.Symbols) != len(t.Weights) {
		panic(fmt.Sprintf("%s: symbols and weights length mismatch", name))
	}
	sum := 0.0
	for _, w := range t.Weights {
		if w < 0 {
			panic(fmt.Sprintf("%s: negative weight", name))
		}
		sum += w
	}
	if sum <= 0 {
		panic(fmt.Sprintf("%s: weight sum must be > 0", name))
	}
	for i := range t.Weights {
		t.Weights[i] /= sum
	}
	for _, sym := range t.Symbols {
		if sideCol {
			if sym >= _wild {
				panic(fmt.Sprintf("%s: side column must not contain wild (symbol %d)", name, sym))
			}
			if !isSideReelSymbolOK(sym) {
				panic(fmt.Sprintf("%s: invalid side symbol %d", name, sym))
			}
			continue
		}
		if middleRow {
			if !isMiddleMiddleRowSymbolOK(sym) {
				panic(fmt.Sprintf("%s: invalid middle column middle-row symbol %d", name, sym))
			}
		} else {
			if !isMiddleUpperLowerSymbolOK(sym) {
				panic(fmt.Sprintf("%s: invalid middle column upper/lower-row symbol %d", name, sym))
			}
		}
	}
}

func isMiddleMiddleRowSymbolOK(sym int64) bool {
	if sym == _blank || sym == _blankCell {
		return true
	}
	if sym >= 1 && sym <= 7 {
		return true
	}
	if sym == _wild {
		return true
	}
	return false
}

func isMiddleUpperLowerSymbolOK(sym int64) bool {
	if sym == _blank || sym == _blankCell {
		return true
	}
	if sym >= 1 && sym <= 7 {
		return true
	}
	if sym == _wild {
		return true
	}
	return false
}

func (s *betOrderService) validatePlainWildMultiplierTable() {
	m := s.gameConfig.PlainWildMultipliers
	p := s.gameConfig.PlainWildMultiplierProbs
	if len(m) == 0 && len(p) == 0 {
		return
	}
	if len(m) != len(p) || len(m) == 0 {
		panic("plain_wild_multipliers and plain_wild_multiplier_probs length mismatch or empty")
	}
	sum := 0.0
	for _, w := range p {
		if w < 0 {
			panic("plain_wild_multiplier_probs: negative prob")
		}
		sum += w
	}
	if sum <= 0 {
		panic("plain_wild_multiplier_probs: sum must be > 0")
	}
	for i := range p {
		p[i] /= sum
	}
	for _, mult := range m {
		if mult < 2 || mult > _maxWildExtraMultiplier {
			panic(fmt.Sprintf("plain_wild_multipliers: invalid mult %d (want 2..%d)", mult, _maxWildExtraMultiplier))
		}
	}
}

func pickFromWeightTable(t *symbolWeightTable) int64 {
	if t == nil || len(t.Symbols) == 0 {
		return 1
	}
	r := rand.Float64()
	cum := 0.0
	for i, w := range t.Weights {
		cum += w
		if r < cum {
			return t.Symbols[i]
		}
	}
	return t.Symbols[len(t.Symbols)-1]
}

// pickSymbolForCell 按策划四套表取 (row,col) 符号
func (s *betOrderService) pickSymbolForCell(row, col int) int64 {
	g := s.gameConfig
	middleRow := row == 1
	switch col {
	case 0, 2:
		if middleRow {
			return pickFromWeightTable(g.WeightSideMiddleRow)
		}
		return pickFromWeightTable(g.WeightSideUpperLowerRow)
	default:
		if middleRow {
			return pickFromWeightTable(g.WeightMiddleMiddleRow)
		}
		return pickFromWeightTable(g.WeightMiddleUpperLowerRow)
	}
}

// pickPlainWildMultiplier 中间格为 100 且中奖时，乘策划表 x2～x100；未配置则返回 1
func (s *betOrderService) pickPlainWildMultiplier() int64 {
	m := s.gameConfig.PlainWildMultipliers
	p := s.gameConfig.PlainWildMultiplierProbs
	if len(m) != len(p) || len(m) == 0 {
		return 1
	}
	r := rand.Float64()
	cum := 0.0
	for i, prob := range p {
		cum += prob
		if r < cum {
			return m[i]
		}
	}
	return m[len(m)-1]
}

func (s *betOrderService) getPayMultiplier(symbol int64) int64 {
	if symbol < 1 || symbol > 7 {
		return 0
	}
	return s.gameConfig.PayTable[symbol-1]
}

func isSideReelSymbolOK(sym int64) bool {
	if sym == _blank || sym == _blankCell {
		return true
	}
	return sym >= 1 && sym <= 7
}

func isMiddleReelSymbolOK(sym int64) bool {
	if sym == _blank || sym == _blankCell {
		return true
	}
	if sym >= 1 && sym <= 7 {
		return true
	}
	if sym >= _wild {
		return sym <= _wild+_maxWildExtraMultiplier
	}
	return false
}
