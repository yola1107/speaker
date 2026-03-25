package jwjzy

import (
	"math/rand/v2"
	"strconv"

	jsoniter "github.com/json-iterator/go"

	"egame-grpc/game/common"
)

type gameConfigJson struct {
	PayTable                   [][]int64 `json:"pay_table"`                       // 赔付表
	Lines                      [][]int64 `json:"lines"`                           // 中奖线定义（20条支付线）
	BaseSymbolWeights          []int     `json:"base_symbol_weights"`             // 基础模式符号权重（万分比）
	FreeSymbolWeights          []int     `json:"free_symbol_weights"`             // 免费模式符号权重（万分比）
	SymbolPermutationWeights   []int     `json:"symbol_permutation_weights"`      // 符号排列权重（单符号/二连/三连）
	BaseScatterProb            int       `json:"base_scatter_prob"`               // 基础模式夺宝符号替换概率（万分比）
	BaseWildProb               int       `json:"base_wild_prob"`                  // 基础模式百搭符号替换概率（万分比）
	FreeScatterProb            int       `json:"free_scatter_prob"`               // 免费模式夺宝符号替换概率（万分比）
	FreeWildProb               int       `json:"free_wild_prob"`                  // 免费模式百搭符号替换概率（万分比）
	BaseRoundMultipliers       []int64   `json:"base_round_multipliers"`          // 基础模式轮次倍数 [1,2,3,5]
	FreeRoundMultipliers       []int64   `json:"free_round_multipliers"`          // 免费模式轮次倍数 [3,6,9,15]
	WildAddFourthMultiple      int64     `json:"wild_add_fourth_multiple"`        // 蝴蝶百搭增加第4轮倍数值（配置里拼写为 multipier）
	BaseReelGenerateInterval   int       `json:"base_reel_generate_interval"`     // 基础轮轴重新生成间隔
	FreeGameSpins              int64     `json:"free_game_spins"`                 // 免费游戏基础次数
	FreeGameScatterMin         int64     `json:"free_game_scatter_min"`           // 触发免费游戏最小夺宝符号数
	FreeGameAddSpinsPerScatter int64     `json:"free_game_add_spins_per_scatter"` // 免费游戏每个额外夺宝符号增加次数
	FreeGameTwoScatterAddTimes int64     `json:"free_game_two_scatter_add_times"` // 免费模式 2 个夺宝时额外增加次数（再触发基础值）

	RollCfg  RollConf `json:"roll_cfg"`  // 滚轴配置
	RealData []Reals  `json:"real_data"` // 真实数据
}

type RollConf struct {
	Base RollCfgType `json:"base"` // 普通游戏滚轴配置
	Free RollCfgType `json:"free"` // 免费游戏滚轴配置
}

type RollCfgType struct {
	UseKey []int `json:"use_key"` // 滚轴数据索引
	Weight []int `json:"weight"`  // 权重
	WTotal int   `json:"-"`       // 总权重（计算得出）
}

type Reals [][]int64

type SymbolRoller struct {
	Real        int              `json:"real"`  // 选择的第几个轮盘
	Col         int              `json:"col"`   // 第几列
	Len         int              `json:"len"`   // 长度
	Start       int              `json:"start"` // 开始索引
	Fall        int              `json:"fall"`  // 开始索引
	BoardSymbol [_rowCount]int64 `json:"board"` // 盘面符号
	BoardGold   [_rowCount]bool  `json:"gold"`  // 金色背景属性（仅 2~4 格长符号占位）
	BoardLong   [_rowCount]bool  `json:"long"`  // 2~4 格长符号占位属性
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

	// 预计算基础/免费模式权重总和
	s.calculateRollWeight(&s.gameConfig.RollCfg.Base)
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free)
}

func (s *betOrderService) calculateRollWeight(rollCfg *RollCfgType) {
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

func (s *betOrderService) initSpinSymbol() [_colCount]SymbolRoller {
	rollers := s.generateInitialBoard(s.isFreeRound)

	// 购买免费：确保本次基础转轴满足 scatter>=min，从而在本回合结算时触发免费模式
	if !s.isFreeRound && s.req.Purchase > 0 {
		minScatter := s.gameConfig.FreeGameScatterMin
		if minScatter <= 0 {
			minScatter = 4
		}
		const maxAttempts = 50
		for i := 0; i < maxAttempts; i++ {
			// 统计当前 rollers 中的夺宝符数量
			var scatterCount int64
			for col := 0; col < _colCount; col++ {
				for r := 0; r < _rowCount; r++ {
					if rollers[col].BoardSymbol[r] == _treasure {
						scatterCount++
					}
				}
			}
			if scatterCount >= minScatter {
				return rollers
			}
			rollers = s.generateInitialBoard(false)
		}
	}

	return rollers
}

// generateInitialBoard 按策划的初始盘面逻辑生成（5x6，中间4列各有且仅有一个 2~4 格长符号；长符号可能有金色背景）
func (s *betOrderService) generateInitialBoard(isFree bool) [_colCount]SymbolRoller {
	// 基础符号池（策划：1~10 + 12）
	baseSymbols := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12}
	baseWeights := s.gameConfig.BaseSymbolWeights
	if isFree && len(s.gameConfig.FreeSymbolWeights) == len(baseWeights) && len(baseWeights) > 0 {
		// 如果你后续提供 free_symbol_weights，就可在此生效；当前策划未明确该字段，先复用 baseWeights
		// 为避免空值，这里仍以 baseWeights 为主。
	}
	if len(baseWeights) != len(baseSymbols) {
		// 使用策划文档默认权重兜底
		baseWeights = []int{1500, 1500, 1500, 1200, 1200, 800, 800, 500, 400, 300, 300}
	}

	drawWeightedSymbol := func() int64 {
		// random.IntN(r) 直接用权重总和
		total := int64(0)
		for _, w := range baseWeights {
			total += int64(w)
		}
		if total <= 0 {
			return baseSymbols[rand.IntN(len(baseSymbols))]
		}
		r := rand.IntN(int(total))
		for i, w := range baseWeights {
			if r < w {
				return baseSymbols[i]
			}
			r -= w
		}
		return baseSymbols[len(baseSymbols)-1]
	}

	// 长符号长度权重（策划：2/3/4）
	longLenWeights := []int64{8300, 1400, 300}
	longLenTotal := int64(0)
	for _, w := range longLenWeights {
		longLenTotal += w
	}
	drawLongLen := func() int {
		r := int64(rand.IntN(int(longLenTotal)))
		for i, w := range longLenWeights {
			if r < w {
				return int(2 + i) // 2,3,4
			}
			r -= w
		}
		return 2
	}

	// 散布夺宝概率（策划：scatter_symbol_prob = 250/10000）
	scatterProb := s.gameConfig.BaseScatterProb
	if isFree && s.gameConfig.FreeScatterProb > 0 {
		scatterProb = s.gameConfig.FreeScatterProb
	}
	if scatterProb <= 0 {
		scatterProb = 250
	}

	// 金色背景概率（策划：base_gold_symbol_prob = 8000/10000）
	goldProb := s.gameConfig.BaseWildProb // 当前旧字段复用：把 base_wild_prob 当作 gold_prob
	if isFree {
		if s.gameConfig.FreeWildProb > 0 {
			goldProb = s.gameConfig.FreeWildProb
		}
	}
	if goldProb <= 0 {
		goldProb = 8000
	}

	// 先生成内部盘面（row=0 底部，row=_rowCount-1 顶部）
	var symbolGrid int64Grid
	var goldGrid boolGrid
	var longGrid boolGrid

	drawSingle := func() int64 {
		// singles 可出现夺宝（treasure）
		if rand.IntN(10000) < scatterProb {
			return _treasure
		}
		return drawWeightedSymbol()
	}

	for col := 0; col < _colCount; col++ {
		// 中间4列：col=1..4
		if col >= 1 && col <= _colCount-2 {
			longLen := drawLongLen() // 2~4
			longTopMax := _rowCount - longLen
			if longTopMax < 0 {
				longTopMax = 0
			}
			longTop := rand.IntN(longTopMax + 1)
			longSym := drawWeightedSymbol()
			// 初始化：长符号必不是夺宝
			for longSym == _treasure {
				longSym = drawWeightedSymbol()
			}
			isGold := rand.IntN(10000) < goldProb
			for r := 0; r < _rowCount; r++ {
				if r >= longTop && r < longTop+longLen {
					symbolGrid[r][col] = longSym
					longGrid[r][col] = true
					goldGrid[r][col] = isGold
				} else {
					symbolGrid[r][col] = drawSingle()
				}
			}
		} else {
			// 边列：全部 singles
			for r := 0; r < _rowCount; r++ {
				symbolGrid[r][col] = drawSingle()
			}
		}
	}

	// 写入 scene.SymbolRoller：BoardSymbol/BoardGold/BoardLong 需要反转行索引与 handleSymbolGrid 对齐
	var rollers [_colCount]SymbolRoller
	for col := 0; col < _colCount; col++ {
		rollers[col] = SymbolRoller{Real: 0, Col: col, Len: 0, Start: 0, Fall: 0}
		for rollerRow := 0; rollerRow < _rowCount; rollerRow++ {
			internalRow := _rowCount - 1 - rollerRow
			rollers[col].BoardSymbol[rollerRow] = symbolGrid[internalRow][col]
			rollers[col].BoardGold[rollerRow] = goldGrid[internalRow][col]
			rollers[col].BoardLong[rollerRow] = longGrid[internalRow][col]
		}
	}
	return rollers
}

func (s *betOrderService) getSceneSymbol(rollCfg RollCfgType) [_colCount]SymbolRoller {
	realIndex := 0
	r := rand.IntN(rollCfg.WTotal)
	for i, w := range rollCfg.Weight {
		if r < w {
			realIndex = rollCfg.UseKey[i]
			break
		}
		r -= w
	}
	if realIndex < 0 || realIndex >= len(s.gameConfig.RealData) {
		panic("real data index out of range: " + strconv.Itoa(realIndex))
	}
	realData := s.gameConfig.RealData[realIndex]
	realDataCols := len(realData)
	if realDataCols <= 0 {
		panic("real data has no columns")
	}

	var symbols [_colCount]SymbolRoller
	for col := 0; col < _colCount; col++ {
		// 兼容旧配置：若 realData 的列数不足，使用取模重复填充，避免越界崩溃。
		data := realData[col%realDataCols]
		dataLen := len(data)
		if dataLen == 0 {
			panic("real data column is empty")
		}

		start := rand.IntN(dataLen)
		end := (start + _rowCount - 1) % dataLen
		roller := SymbolRoller{Real: realIndex, Start: start, Fall: end, Col: col, Len: dataLen}

		for row := 0; row < _rowCount; row++ {
			symbol := data[(start+row)%dataLen]
			roller.BoardSymbol[int(_rowCount)-1-row] = symbol
		}
		symbols[col] = roller
	}

	return symbols
}

func (rs *SymbolRoller) ringSymbol(gameConfig *gameConfigJson) {
	var newBoard [_rowCount]int64
	for i, s := range rs.BoardSymbol {
		if s != 0 {
			newBoard[i] = s
		}
	}
	for i := _rowCount - 1; i >= 0; i-- {
		if newBoard[i] == 0 {
			newBoard[i] = rs.getFallSymbol(gameConfig)
		}
	}
	rs.BoardSymbol = newBoard
}

func (rs *SymbolRoller) getFallSymbol(gameConfig *gameConfigJson) int64 {
	cols := len(gameConfig.RealData[rs.Real])
	if cols <= 0 {
		panic("real data has no columns")
	}
	data := gameConfig.RealData[rs.Real][rs.Col%cols]
	rs.Fall = (rs.Fall + 1) % len(data)
	return data[rs.Fall]
}

func (s *betOrderService) getSymbolBaseMultiplier(symbol int64, starN int) int64 {
	// starN: 连续匹配列数（3~6）
	if starN < 3 {
		return 0
	}

	// pay_table 行与符号ID的映射（策划：pay_table[11][6] 对应 11 种基础符号：
	// 1~10 + 12；不包含 11(雪茄非必要)、不包含 13(wild)、不包含 14(treasure)）
	var row int
	switch symbol {
	case 1, 2, 3, 4, 5, 6, 7, 8, 9, 10:
		row = int(symbol - 1)
	case 12:
		row = 10
	default:
		return 0
	}

	if row < 0 || row >= len(s.gameConfig.PayTable) {
		return 0
	}

	// pay_table 列：idx = starCount - 1
	idx := starN - 1
	table := s.gameConfig.PayTable[row]
	if idx < 0 || idx >= len(table) {
		return 0
	}
	return table[idx]
}

// calcNewFreeGameNum 计算触发免费游戏的次数
// 基础模式：scatter >= free_game_scatter_min → free_game_spins + (scatter - min) * free_game_add_spins_per_scatter
// 免费模式再触发：scatter >= 2 → free_game_two_scatter_add_times + (scatter - 2) * free_game_add_spins_per_scatter
func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if s.isFreeRound {
		// 免费模式再触发：4 个以上夺宝起算
		if scatterCount < s.gameConfig.FreeGameScatterMin {
			return 0
		}
		// 策划口径：额外增加 scatterCount * 2 次免费
		return scatterCount * s.gameConfig.FreeGameAddSpinsPerScatter
	}

	// 基础模式触发
	if scatterCount < s.gameConfig.FreeGameScatterMin {
		return 0
	}
	return s.gameConfig.FreeGameSpins +
		(scatterCount-s.gameConfig.FreeGameScatterMin)*s.gameConfig.FreeGameAddSpinsPerScatter
}
