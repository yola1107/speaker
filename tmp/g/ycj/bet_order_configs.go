package ycj

import (
	"math/rand/v2"

	"egame-grpc/game/common"
	//"egame-grpc/game/common/rand"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable  [][]float64      `json:"pay_table"`  // 数字符号赔率表
	Multi     []float64        `json:"multi"`      // 中间符号倍数表（索引=符号ID）
	FreeCount map[string]int64 `json:"free_count"` // 免费符号ID -> 免费次数（原始配置）
	RollCfg   rollConf         `json:"roll_cfg"`   // 滚轴配置
	RawReels  [][][][]int64    `json:"real_data"`  // [轮盘组][列][0=符号序列,1=权重序列]
	RealData  [][]weightedReel `json:"-"`          // 解析后的滚轴数据 [轮盘组][列]
}

type rollConf struct {
	Base       rollCfgType `json:"base"`        // 普通模式
	Free       rollCfgType `json:"free"`        // 免费模式
	FreeRespin rollCfgType `json:"free_respin"` // 免费重转模式
}

type rollCfgType struct {
	UseKey []int `json:"use_key"` // 使用的滚轴索引
	Weight []int `json:"weight"`  // 选择权重
	WTotal int   `json:"-"`       // 权重总和
}

type weightedReel struct {
	Symbols []int64
	Weights []int
	WTotal  int
}

// SymbolRoller 滚轮状态
type SymbolRoller struct {
	Real        int            `json:"real"`  // 选择的第几个轮盘
	Start       int            `json:"start"` // 开始索引
	Fall        int            `json:"fall"`  // 下落索引
	Col         int            `json:"col"`   // 第几列
	BoardSymbol [_rowCount]int `json:"board"` // 当前显示的符号
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
	s.gameConfig = &gameConfigJson{FreeCount: map[string]int64{}}
	if err := jsoniter.UnmarshalFromString(raw, s.gameConfig); err != nil {
		panic(err)
	}
	s.validateGameConfig()
	s.validateRollWeight(&s.gameConfig.RollCfg.Base)
	s.validateRollWeight(&s.gameConfig.RollCfg.Free)
	s.validateRollWeight(&s.gameConfig.RollCfg.FreeRespin)
}

func (s *betOrderService) validateGameConfig() {
	cfg := s.gameConfig
	for i := range cfg.PayTable {
		if len(cfg.PayTable[i]) < 3 {
			panic("pay_table row length < 3")
		}
	}
	if len(cfg.Multi) <= int(_mul100) {
		panic("multi length not enough")
	}
	if cfg.FreeCount == nil {
		cfg.FreeCount = make(map[string]int64)
	}

	if len(cfg.RawReels) < 3 {
		panic("real_data length < 3")
	}
	cfg.RealData = make([][]weightedReel, len(cfg.RawReels))
	for r := range cfg.RawReels {
		cols := cfg.RawReels[r]
		if len(cols) != _colCount {
			panic("real_data col count not match")
		}
		cfg.RealData[r] = make([]weightedReel, _colCount)
		for c := 0; c < _colCount; c++ {
			col := cols[c]
			if len(col) < 2 {
				panic("real_data col format invalid")
			}
			symbols := col[0]
			rawWeights := col[1]
			if len(symbols) == 0 || len(symbols) != len(rawWeights) {
				panic("real_data symbols and weights length not match")
			}
			weights := make([]int, len(rawWeights))
			total := 0
			for i, w := range rawWeights {
				weights[i] = int(w)
				total += int(w)
			}
			if total <= 0 {
				panic("real_data weights sum <= 0")
			}
			cfg.RealData[r][c] = weightedReel{
				Symbols: symbols,
				Weights: weights,
				WTotal:  total,
			}
		}
	}
}

func (s *betOrderService) validateRollWeight(rollCfg *rollCfgType) {
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

func (s *betOrderService) initSpinSymbol() {
	s.scene.Done = 0

	cfg := s.gameConfig.RollCfg.Base
	if s.isFreeRound {
		cfg = s.gameConfig.RollCfg.Free
	}
	realIndex := cfg.UseKey[pickWeightIndex(cfg.Weight, cfg.WTotal)]

	var rollers [_colCount]SymbolRoller
	realData := s.gameConfig.RealData[realIndex]
	for c := 0; c < _colCount; c++ {
		reel := realData[c]
		reelLen := len(reel.Symbols)
		picked := pickWeightIndex(reel.Weights, reel.WTotal) //根据权重选取中间行符号下标
		start := (picked - _midRow + reelLen) % reelLen
		board := [_rowCount]int{}
		for r := 0; r < _rowCount; r++ {
			board[r] = int(reel.Symbols[(start+r)%reelLen])
		}
		rollers[c] = SymbolRoller{
			Real:        realIndex,
			Start:       start,
			Fall:        (start + _rowCount - 1) % reelLen,
			Col:         c,
			BoardSymbol: board,
		}
	}
	//if !s.isFreeRound {
	//	// 强制中间行为 [3,7,3]，用于固定判奖调试。
	//	rollers[0].BoardSymbol[_midRow] = 3  //int(_num1)
	//	rollers[1].BoardSymbol[_midRow] = 16 //int(_free5)
	//	rollers[2].BoardSymbol[_midRow] = 3  //int(_num1)
	//}

	s.scene.SymbolRoller = rollers
}

// applyExtendFall 推展：中间列整列下移一格，新顶格符号沿 reel 顺序取 Start-- 方向一格。
func (s *betOrderService) applyExtendFall(col int) {
	roller := &s.scene.SymbolRoller[col]
	data := s.gameConfig.RealData[roller.Real][roller.Col].Symbols
	dataLen := len(data)
	roller.Start--
	if roller.Start < 0 {
		roller.Start = dataLen - 1
	}
	roller.Fall = (roller.Start + _rowCount - 1) % dataLen
	newTop := int(data[roller.Start])
	for r := _rowCount - 1; r > 0; r-- {
		roller.BoardSymbol[r] = roller.BoardSymbol[r-1]
	}
	roller.BoardSymbol[0] = newTop
}

func (s *betOrderService) respinSides() {
	cfg := s.gameConfig.RollCfg.FreeRespin
	realIndex := cfg.UseKey[pickWeightIndex(cfg.Weight, cfg.WTotal)]
	for _, col := range []int{0, 2} {
		reel := s.gameConfig.RealData[realIndex][col]
		reelLen := len(reel.Symbols)
		picked := pickWeightIndex(reel.Weights, reel.WTotal)
		start := (picked - _midRow + reelLen) % reelLen
		board := [_rowCount]int{}
		for r := 0; r < _rowCount; r++ {
			board[r] = int(reel.Symbols[(start+r)%reelLen])
		}
		s.scene.SymbolRoller[col] = SymbolRoller{
			Real:        realIndex,
			Start:       start,
			Fall:        (start + _rowCount - 1) % reelLen,
			Col:         col,
			BoardSymbol: board,
		}
	}
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

func (s *betOrderService) getNumberPay(symbol int64) float64 {
	idx := int(symbol - _num01)
	if idx < 0 || idx >= len(s.gameConfig.PayTable) {
		return 0
	}
	// 配置以整数存储(1/5/10/50/100)，实际数字为 0.1/0.5/1/5/10
	return s.gameConfig.PayTable[idx][2] / 10 // s.gameConfig.PayTable[idx][2]/10
}

func (s *betOrderService) getMiddleMul(symbol int64) float64 {
	if symbol < 0 || int(symbol) >= len(s.gameConfig.Multi) {
		return 1
	}
	v := s.gameConfig.Multi[symbol]
	if v <= 0 {
		return 1
	}
	return v
}

func (s *betOrderService) getFreeSpinCount(symbol int64) int64 {
	switch symbol {
	case _free5:
		return s.gameConfig.FreeCount["7"]
	case _free10:
		return s.gameConfig.FreeCount["8"]
	case _free20:
		return s.gameConfig.FreeCount["9"]
	default:
		return 0
	}
}
