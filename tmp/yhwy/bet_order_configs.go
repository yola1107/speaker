package yhwy

import (
	"math/rand/v2"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

// gameConfigJson 与 game_json.go 中 JSON 顶层字段一致（扁平结构，无 free/mystery 嵌套）。
type gameConfigJson struct {
	PayTable          [][]int64 `json:"pay_table"`           // 赔付表，索引为符号 ID，列为 1~5 连的赔率
	Lines             [][]int   `json:"lines"`               // 25 条固定线定义，位置编号范围为 0~1
	LvCollectCount    []int64   `json:"lv_collect_count"`    // 樱花收集外观阈值（前端展示等）
	ChangeSymbolId    []int64   `json:"change_symbol_id"`    // 百变樱花可揭示为的符号 ID
	BaseOpenWeights   []int     `json:"base_open_weight"`    // BaseGame 揭示权重
	FreeOpenWeights   []int     `json:"free_open_weight"`    // FreeGame 揭示权重
	SakuraTriggerRate int64     `json:"sakura_trigger_rate"` // 樱吹雪触发：万分比
	SakuraReels       []int     `json:"sakura_reels"`        // 樱吹雪可替换到的最远列（3/4/5）
	SakuraReelsWeight []int     `json:"sakura_reels_weight"` // 对应权重
	ScatterCount      []int64   `json:"scatter_count"`       // 与 free_spins 等长、升序；达到该档 scatter 数时触发对应免费次数
	FreeSpins         []int64   `json:"free_spins"`          // 与 scatter_count 一一对应
	RollCfg           rollConf  `json:"roll_cfg"`            // 不同阶段使用的滚轴组配置
	RealData          []Reel    `json:"real_data"`           // 实际滚轴带数据

	_baseOpenWeightTotal int // BaseGame 权重总和，运行时预计算
	_freeOpenWeightTotal int // FreeGame 权重总和，运行时预计算
	_sakuraReelsTotal    int // 樱吹雪列数权重总和，运行时预计算
}

type rollConf struct {
	Base rollCfgType `json:"base"`
	Free rollCfgType `json:"free"`
}

type rollCfgType struct {
	UseKey []int `json:"use_key"`
	Weight []int `json:"weight"`
	WTotal int   `json:"-"`
}

type Reel [][]int64

type SymbolRoller struct {
	Real        int              `json:"real"`  // 使用的 reel set 索引
	Col         int              `json:"col"`   // 当前列号，0~4
	Len         int              `json:"len"`   // 原始滚轴长度
	Start       int              `json:"start"` // 本次抽样起始下标
	Fall        int              `json:"fall"`  // 本次展示结束下标
	BoardSymbol [_rowCount]int64 `json:"board"` // 停轮后该列 4 个可见符号
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

	s.validateMysteryConfigs()
	s.validateFreeSpinConfigs()
}

func (s *betOrderService) validateRollCfg(rollCfg *rollCfgType) {
	rollCfg.WTotal = 0
	if len(rollCfg.UseKey) == 0 || len(rollCfg.UseKey) != len(rollCfg.Weight) {
		panic("roll weight and use_key length not match")
	}
	for _, w := range rollCfg.Weight {
		rollCfg.WTotal += w
	}
	if rollCfg.WTotal <= 0 {
		panic("roll weight sum <= 0")
	}
}

func (s *betOrderService) validateFreeSpinConfigs() {
	sc := s.gameConfig.ScatterCount
	fs := s.gameConfig.FreeSpins
	if len(sc) == 0 || len(sc) != len(fs) {
		panic("invalid scatter_count / free_spins")
	}
	for _, spins := range fs {
		if spins <= 0 {
			panic("invalid free_spins value")
		}
	}
}

func (s *betOrderService) validateMysteryConfigs() {
	if cnt := len(s.gameConfig.ChangeSymbolId); cnt == 0 ||
		cnt != len(s.gameConfig.BaseOpenWeights) ||
		cnt != len(s.gameConfig.FreeOpenWeights) {
		panic("invalid mystery base config")
	}

	for _, w := range s.gameConfig.BaseOpenWeights {
		s.gameConfig._baseOpenWeightTotal += w
	}
	if s.gameConfig._baseOpenWeightTotal <= 0 {
		panic("invalid mystery base weight total")
	}
	for _, w := range s.gameConfig.FreeOpenWeights {
		s.gameConfig._freeOpenWeightTotal += w
	}
	if s.gameConfig._freeOpenWeightTotal <= 0 {
		panic("invalid mystery free weight total")
	}

	for _, reel := range s.gameConfig.SakuraReels {
		if reel < 3 || reel > _colCount {
			panic("invalid sakura reel")
		}
	}
	for _, w := range s.gameConfig.SakuraReelsWeight {
		s.gameConfig._sakuraReelsTotal += w
	}
	if s.gameConfig._sakuraReelsTotal <= 0 {
		panic("invalid sakura_reels_weight total")
	}
}

func (s *betOrderService) initSpinSymbol() {
	var cfg rollCfgType
	if s.isFreeRound {
		cfg = s.gameConfig.RollCfg.Free
	} else {
		cfg = s.gameConfig.RollCfg.Base
	}
	realIndex := cfg.UseKey[pickWeightIndex(cfg.Weight, cfg.WTotal)]
	s.scene.SymbolRoller = s.getSceneSymbol(realIndex)
}

func (s *betOrderService) getSceneSymbol(realIndex int) [_colCount]SymbolRoller {
	var symbols [_colCount]SymbolRoller
	realData := s.gameConfig.RealData[realIndex]

	for c := 0; c < _colCount; c++ {
		reel := realData[c]
		reelLen := len(reel)
		start := rand.IntN(reelLen)
		roller := SymbolRoller{
			Real:  realIndex,
			Col:   c,
			Len:   reelLen,
			Start: start,
			Fall:  (start + _rowCount - 1) % reelLen,
		}
		for r := 0; r < _rowCount; r++ {
			roller.BoardSymbol[r] = reel[(start+r)%reelLen]
		}
		symbols[c] = roller
	}

	return symbols
}

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

// calcNewFreeGameNum 按 scatter 档位取免费次数：scatter_count 与 free_spins 升序一一对应，取满足 scatterCount>=scatter_count[i] 的最高档。
func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if s.isFreeRound {
		return 0
	}
	if scatterCount < s.gameConfig.ScatterCount[0] {
		return 0
	}
	if scatterCount >= s.gameConfig.ScatterCount[len(s.gameConfig.ScatterCount)-1] {
		return s.gameConfig.FreeSpins[len(s.gameConfig.FreeSpins)-1]
	}
	sc := s.gameConfig.ScatterCount
	fs := s.gameConfig.FreeSpins
	for i := len(sc) - 1; i >= 0; i-- {
		if scatterCount >= sc[i] {
			return fs[i]
		}
	}
	return 0
}

func (s *betOrderService) pickMMysterySymbol() int64 {
	if s.isFreeRound {
		return s.gameConfig.ChangeSymbolId[pickWeightIndex(s.gameConfig.FreeOpenWeights, s.gameConfig._freeOpenWeightTotal)]
	}
	return s.gameConfig.ChangeSymbolId[pickWeightIndex(s.gameConfig.BaseOpenWeights, s.gameConfig._baseOpenWeightTotal)]
}

func (s *betOrderService) pickSakuraReels() int {
	return s.gameConfig.SakuraReels[pickWeightIndex(s.gameConfig.SakuraReelsWeight, s.gameConfig._sakuraReelsTotal)]
}

func (s *betOrderService) isHitSakuraTriggerRate() bool {
	r := s.gameConfig.SakuraTriggerRate
	if r <= 0 {
		return false
	}
	return rand.Int64N(10000)+1 <= r
}
