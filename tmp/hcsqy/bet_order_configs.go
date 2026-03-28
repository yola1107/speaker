package hcsqy

import (
	"fmt"
	"math/rand/v2"

	"egame-grpc/game/common"
	//"egame-grpc/game/common/rand"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable          [][]int64       `json:"pay_table"`           // 赔付表，索引=符号ID-1
	Lines             [][]int         `json:"lines"`               // 中奖线定义
	Free              freeConfig      `json:"free"`                // 免费参数
	Buy               buyConfig       `json:"buy"`                 // 购买配置
	RespinRate        [][2]int64      `json:"respin_rate"`         // 必赢触发概率分数 [分子,分母]，index 0=基础，1=免费
	WildExpandRate    [][2]int64      `json:"wild_expand_rate"`    // 百搭变大概率分数 [分子,分母]，index 0=基础，1=免费
	ExpandMultiConfig expandMultiConf `json:"expand_multi_config"` // 长条百搭倍数与权重
	RollCfg           rollConf        `json:"roll_cfg"`            // 滚轴配置
	RealData          []Reel          `json:"real_data"`           // 滚轴数据 [模式][列]
}

type freeConfig struct {
	ScatterMin         int64 `json:"scatter_min"`           // 触发免费最少夺宝数
	FreeTimes          int64 `json:"free_times"`            // 基础免费次数
	PerScatterAddTimes int64 `json:"per_scatter_add_times"` // 每多一个夺宝增加次数
}

type buyConfig struct {
	Price           int64 `json:"price"`              // 购买免费价格倍数
	MaxBuyBetAmount int64 `json:"max_buy_bet_amount"` // 最大可购买投注金额
}

type expandMultiConf struct {
	Multi  []int64 `json:"multi"`  // 长条百搭倍数
	Weight []int   `json:"weight"` // 长条百搭倍数权重
	WTotal int     `json:"-"`      // 权重总和
}

type rollConf struct {
	Base          rollCfgType `json:"base"`
	BaseRespin    rollCfgType `json:"base_respin"`
	Free          rollCfgType `json:"free"`
	FreeRespin    rollCfgType `json:"free_respin"`
	BuyBase       rollCfgType `json:"buy_base"`        // 基础模式下 购买
	BuyFree       rollCfgType `json:"buy_free"`        // 购买后 免费模式且非Respin模式
	BuyFreeRespin rollCfgType `json:"buy_free_respin"` // 购买后 免费且Respin模式
}

type rollCfgType struct {
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
}

type spinExecMode uint8

const (
	spinExecBase spinExecMode = iota
	spinExecBaseRespin
	spinExecFree
	spinExecFreeRespin
	spinExecBuyBase
	spinExecBuyFree
	spinExecBuyFreeRespin
)

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

	if len(s.gameConfig.PayTable) < int(_wild) {
		panic(fmt.Sprintf("pay_table length < %d", _wild))
	}
	if len(s.gameConfig.RealData) < 5 {
		panic("real_data length < 5")
	}
	if s.gameConfig.Free.FreeTimes <= 0 || s.gameConfig.Free.ScatterMin <= 0 {
		panic("invalid free config")
	}
	if len(s.gameConfig.RespinRate) != 2 {
		panic("respin_rate length != 2")
	}
	if len(s.gameConfig.WildExpandRate) != 2 {
		panic("wild_expand_rate length != 2")
	}

	s.calculateExpandWeight(&s.gameConfig.ExpandMultiConfig)

	s.calculateRollWeight(&s.gameConfig.RollCfg.Base)
	s.calculateRollWeight(&s.gameConfig.RollCfg.BaseRespin)
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free)
	s.calculateRollWeight(&s.gameConfig.RollCfg.FreeRespin)
	s.calculateRollWeight(&s.gameConfig.RollCfg.BuyBase)
	s.calculateRollWeight(&s.gameConfig.RollCfg.BuyFree)
	s.calculateRollWeight(&s.gameConfig.RollCfg.BuyFreeRespin)
}

func (s *betOrderService) calculateRollWeight(rollCfg *rollCfgType) {
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

func (s *betOrderService) calculateExpandWeight(c *expandMultiConf) {
	if len(c.Multi) == 0 || len(c.Multi) != len(c.Weight) {
		panic("invalid expand_multi_config")
	}
	totalMultiWeight := 0
	for _, w := range c.Weight {
		totalMultiWeight += w
	}
	if totalMultiWeight <= 0 {
		panic("invalid expand_multi_config total")
	}
	c.WTotal = totalMultiWeight
}

func (s *betOrderService) initSpinSymbol() {
	mode, cfg := s.deriveSpinExecMode()
	//cfg := s.getRollCfgByMode(mode)

	s.debug.mode = mode

	realIndex := cfg.UseKey[pickWeightIndex(cfg.Weight, cfg.WTotal)]
	s.scene.SymbolRoller = s.getSceneSymbol(realIndex)
}

// deriveSpinExecMode 将当前步上下文（主阶段+过程标志）映射为单一执行态。
func (s *betOrderService) deriveSpinExecMode() (spinExecMode, rollCfgType) {
	isPurchaseMode := s.isPurchaseActive()
	switch {
	case s.isFreeRound && isPurchaseMode && s.stepIsRespinMode:
		return spinExecBuyFreeRespin, s.gameConfig.RollCfg.BuyFreeRespin
	case s.isFreeRound && isPurchaseMode:
		return spinExecBuyFree, s.gameConfig.RollCfg.BuyFree
	case s.isFreeRound && s.stepIsRespinMode:
		return spinExecFreeRespin, s.gameConfig.RollCfg.FreeRespin
	case s.isFreeRound:
		return spinExecFree, s.gameConfig.RollCfg.Free
	case isPurchaseMode && s.stepIsRespinMode:
		// 兼容状态恢复场景：基础购买阶段若出现重转标记，兜底走购买基础滚轴，避免测试/线上直接中断。
		if s.scene.IsRespinMode { // 如果这里panic了，说明代码逻辑有问题
			panic(fmt.Sprintf("===== 代码逻辑有问题 scene.IsPurchase = %v scene.IsRespinMode=%v", s.scene.IsPurchase, s.scene.IsRespinMode))
		}
		return spinExecBuyBase, s.gameConfig.RollCfg.BuyBase
	case isPurchaseMode:
		return spinExecBuyBase, s.gameConfig.RollCfg.BuyBase
	case s.stepIsRespinMode:
		return spinExecBaseRespin, s.gameConfig.RollCfg.BaseRespin
	default:
		return spinExecBase, s.gameConfig.RollCfg.Base
	}
}

// getRollCfgByMode 执行态到滚轴配置的单点映射。
func (s *betOrderService) getRollCfgByMode(mode spinExecMode) rollCfgType {
	switch mode {
	case spinExecBase:
		return s.gameConfig.RollCfg.Base
	case spinExecBaseRespin:
		return s.gameConfig.RollCfg.BaseRespin
	case spinExecFree:
		return s.gameConfig.RollCfg.Free
	case spinExecFreeRespin:
		return s.gameConfig.RollCfg.FreeRespin
	case spinExecBuyBase:
		return s.gameConfig.RollCfg.BuyBase
	case spinExecBuyFree:
		return s.gameConfig.RollCfg.BuyFree
	case spinExecBuyFreeRespin:
		return s.gameConfig.RollCfg.BuyFreeRespin
	default:
		return s.gameConfig.RollCfg.Base
	}
}

func (s *betOrderService) getSceneSymbol(realIndex int) [_colCount]SymbolRoller {
	var symbols [_colCount]SymbolRoller
	realData := s.gameConfig.RealData[realIndex]

	for c := 0; c < _colCount; c++ {
		reel := realData[c]
		reelLen := len(reel)
		start := rand.IntN(reelLen)
		roller := SymbolRoller{Real: realIndex, Start: start, Fall: (start + _rowCount - 1) % reelLen, Col: c, Len: reelLen}

		for r := 0; r < _rowCount; r++ {
			roller.BoardSymbol[_rowCount-1-r] = reel[(start+r)%reelLen]
		}
		symbols[c] = roller
	}
	return symbols
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

func (s *betOrderService) weightWildMultiplier() int64 {
	idx := pickWeightIndex(s.gameConfig.ExpandMultiConfig.Weight, s.gameConfig.ExpandMultiConfig.WTotal)
	return s.gameConfig.ExpandMultiConfig.Multi[idx]
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

func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if scatterCount < s.gameConfig.Free.ScatterMin {
		return 0
	}
	return s.gameConfig.Free.FreeTimes + (scatterCount-s.gameConfig.Free.ScatterMin)*s.gameConfig.Free.PerScatterAddTimes
}

func (s *betOrderService) isHitRespinProb() bool {
	if s.isFreeRound {
		return isHit(s.gameConfig.RespinRate[1][0], s.gameConfig.RespinRate[1][1])
	}
	return isHit(s.gameConfig.RespinRate[0][0], s.gameConfig.RespinRate[0][1])
}

func (s *betOrderService) isHitWildExpandProb() bool {
	if s.isFreeRound {
		return isHit(s.gameConfig.WildExpandRate[1][0], s.gameConfig.WildExpandRate[1][1])
	}
	return isHit(s.gameConfig.WildExpandRate[0][0], s.gameConfig.WildExpandRate[0][1])
}

// isHit 按 num/den 概率命中：在 [0, den) 均匀取随机数，小于 num 则命中。
func isHit(num, den int64) bool {
	if num <= 0 || den <= 0 {
		return false
	}
	return rand.Int64N(den) < num
}
