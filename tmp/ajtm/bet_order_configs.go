package ajtm

import (
	"math/rand/v2"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable          [][]int64  `json:"pay_table"`                      // 赔率表 [符号ID-1][列数-1]
	FreeGameTimes     int64      `json:"free_game_times"`                // 基础免费次数
	FreeGameBonus     int64      `json:"free_game_bonus"`                // 每多1个夺宝增加的免费倍数
	ExtraAddFreeBonus int64      `json:"extra_add_free_bonus"`           // 每多1个夺宝增加的免费倍数
	FreeGameScatter   int64      `json:"trigger_free_game_need_scatter"` // 触发免费需要的夺宝数
	BaseBigSyNums     []int64    `json:"base_mysterious_symbol_nums"`    // 神秘符号数量选项 [0,1,2]
	BaseBigSyWeights  []int64    `json:"base_mysterious_symbol_weights"` // 神秘符号权重，对应数量
	FreeBigSyNums     []int64    `json:"free_mysterious_symbol_nums"`    // 免费模式神秘符号数量选项 [0,1,2]
	FreeBigSyWeights  []int64    `json:"free_mysterious_symbol_weights"` // 免费模式神秘符号权重，对应数量
	BigSyMultiples    []Location `json:"big_symbol_multiples"`           // 布局配置 [数量-1][布局选项][位置]
	RollCfg           RollConf   `json:"roll_cfg"`                       // 滚轴配置
	RealData          []Reals    `json:"real_data"`                      // 滚轴数据 [模式][列][符号序列]
}

type RollConf struct {
	Base RollCfgType `json:"base"` // 基础模式滚轴配置
	Free RollCfgType `json:"free"` // 免费模式滚轴配置
}

type RollCfgType struct {
	UseKey []int `json:"use_key"` // 使用的滚轴索引列表
	Weight []int `json:"weight"`  // 对应权重
}

type Reals [][]int64    // 滚轴数据 [列][符号序列]
type Location [][]int64 // 神秘符号布局 [位置] = 占格数(1=普通,2=神秘)

type SymbolRoller struct {
	Real        int              `json:"real"`  // 选择的第几个轮盘
	Col         int              `json:"col"`   // 第几列
	Len         int              `json:"len"`   // 长度
	Start       int              `json:"start"` // 开始索引
	Fall        int              `json:"fall"`  // 结束索引
	BoardSymbol [_rowCount]int64 `json:"board"` // 盘面符号
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

	s.calculateRollWeight(&s.gameConfig.RollCfg.Base)
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free)
}

func (s *betOrderService) calculateRollWeight(rollCfg *RollCfgType) {
	if len(rollCfg.Weight) != len(rollCfg.UseKey) {
		panic("roll weight and use_key length not match")
	}
}

// ringSymbol 消除后填充：从底行向上填补空位
// 第0列和第4列的顶角/底角永远保持为0，不填充
// rs.Start 递减保证连续下落的符号顺序（滚轴从上往下读取）
func (rs *SymbolRoller) ringSymbol(gameConfig *gameConfigJson) {
	isEdgeCol := rs.Col == 0 || rs.Col == _colCount-1
	for r := _rowCount - 1; r >= 0; r-- {
		// 第0列和第4列的顶角(行0)和底角(行5)不填充
		if isEdgeCol && (r == 0 || r == _rowCount-1) {
			continue
		}
		if rs.BoardSymbol[r] == 0 {
			rs.BoardSymbol[r] = rs.getFallSymbol(gameConfig)
		}
	}
}

// getFallSymbol 获取掉落符号
func (rs *SymbolRoller) getFallSymbol(gameConfig *gameConfigJson) int64 {
	rs.Start--
	if rs.Start < 0 {
		reelLen := len(gameConfig.RealData[rs.Real][rs.Col])
		rs.Start = reelLen - 1
	}
	return gameConfig.RealData[rs.Real][rs.Col][rs.Start]
}

// pickWeightIndex 按权重随机选择索引
func pickWeightIndex[T int | int64](weights []T) int {
	if len(weights) <= 1 {
		return 0
	}
	var total T
	for _, w := range weights {
		total += w
	}
	if total <= 0 {
		return 0
	}
	var r T
	switch any(total).(type) {
	case int:
		r = T(rand.IntN(int(total)))
	case int64:
		r = T(rand.Int64N(int64(total)))
	}
	var curr T
	for i, w := range weights {
		curr += w
		if r < curr {
			return i
		}
	}
	return 0
}

//// createMatrix 创建符号矩阵（两阶段）
//func (s *betOrderService) createMatrix() {
//	s.scene.SymbolRoller = [_colCount]SymbolRoller{}
//	//s.mysMultipliers = int64Grid{}
//	s.fillEmptyPositions() // 阶段1：填充普通符号
//	s.generateMysSymbols() // 阶段2：生成神秘符号（带冲突检测）
//	s.handleSymbolGrid()
//	//s.assignMysMultipliers()
//}

///*
//	判断位置：在 generateMysterySymbols 里，对每个中间轴（现在是 2~4 轴）会：
//		先按权重抽出本轴要生成的神秘符号数量 mysterySymbolCount。
//		再从 MYSTERY_SYMBOL_TYPE_CONFIGS[mysterySymbolCount] 里挑一个纵向布局 pattern。
//	防止 S 变成神秘块：
//		对候选 pattern 做一次“模拟填充”：按照该布局，计算每个 2 格块对应的原始符号 symbolId。
//		如果某个 2 格块的首格符号是 夺宝符号（内部 ID INTERNAL_SYMBOL_S），就认为这个布局“冲突”。
//		会在当前轴上轮询所有可用布局，随机选起点依次尝试，直到找到一个“不让 S 变神秘”的布局为止。
//	找不到安全布局时的处理：
//		如果该轴所有布局都会让 S 位置变成神秘块（例如这一段刚好都是 S），则：
//			把当前轴的 mysterySymbolCount 置为 0；
//			使用全 1 的布局（即当前轴本局不生成神秘符号）。
//	写入阶段：
//		通过上述筛选得到的安全 pattern，再走原来的写入逻辑（首格原符号 + 尾格 1000 + 前端ID），从而保证不会出现“夺宝符号生成神秘符号”的情况。
//*/
//
//// fillEmptyPositions 填充空位
//func (s *betOrderService) fillEmptyPositions() {
//	rollCfg := s.gameConfig.RollCfg.Base
//	if s.isFreeRound {
//		rollCfg = s.gameConfig.RollCfg.Free
//	}
//
//	realIndex := rollCfg.UseKey[pickWeightIndex(rollCfg.Weight)]
//	realData := s.gameConfig.RealData[realIndex]
//
//	for c := 0; c < _colCount; c++ {
//		reelLen := len(realData[c])
//		start := rand.IntN(reelLen)
//		rs := &s.scene.SymbolRoller[c]
//		rs.Col = c
//		rs.Start = start
//		rs.Real = realIndex
//		rs.Len = reelLen
//
//		add := 0
//		for r := 0; r < _rowCount; r++ {
//			// 边缘列的顶角和底角保持为0
//			if (c == 0 || c == _colCount-1) && (r == 0 || r == _rowCount-1) {
//				continue
//			}
//			rs.BoardSymbol[r] = realData[c][(start+add)%reelLen]
//			add++
//		}
//		rs.Fall = (start + add - 1) % reelLen // 结束索引
//	}
//}
//
//// generateMysSymbols 生成神秘符号（带冲突检测，避开夺宝）
//func (s *betOrderService) generateMysSymbols() {
//	weights := s.gameConfig.BaseBigSyWeights
//	if s.isFreeRound {
//		weights = s.gameConfig.FreeBigSyWeights
//	}
//
//	for col := 1; col < _colCount-1; col++ {
//		mysCount := pickWeightIndex(weights)
//		if mysCount == 0 || mysCount >= len(s.gameConfig.BigSyMultiples) {
//			continue
//		}
//
//		patterns := s.gameConfig.BigSyMultiples[mysCount-1]
//		if len(patterns) == 0 {
//			continue
//		}
//
//		// 轮询找安全布局（避开夺宝）
//		startIdx := rand.IntN(len(patterns))
//		var safePattern []int64
//		for i := 0; i < len(patterns); i++ {
//			pattern := patterns[(startIdx+i)%len(patterns)]
//			if s.isPatternSafe(col, pattern) {
//				safePattern = pattern
//				break
//			}
//		}
//
//		if safePattern == nil {
//			continue // 全冲突，放弃神秘符号
//		}
//
//		s.writeMysPattern(col, safePattern)
//	}
//}
//
//// isPatternSafe 检测布局是否安全（头部不是夺宝）
//func (s *betOrderService) isPatternSafe(col int, pattern []int64) bool {
//	row := 1
//	for _, val := range pattern {
//		if val > 1 {
//			// 头部在 row-1 位置（尾巴在 row）
//			if s.scene.SymbolRoller[col].BoardSymbol[row-1] == _treasure {
//				return false
//			}
//			row++ // 跳过尾巴
//		}
//		row++ // 跳到下一个位置
//	}
//	return true
//}
//
//// writeMysPattern 写入神秘符号布局（头部保持原符号，尾巴标记）
//func (s *betOrderService) writeMysPattern(col int, pattern []int64) {
//	row := 1
//	for _, val := range pattern {
//		if val > 1 {
//			// 头部在 row-1 位置
//			headSymbol := s.scene.SymbolRoller[col].BoardSymbol[row-1]
//			// 尾巴在 row 位置
//			s.scene.SymbolRoller[col].BoardSymbol[row] = _longSymbol + headSymbol
//			row++ // 跳过尾巴
//		}
//		row++ // 跳到下一个位置
//	}
//}
//
////// assignMysMultipliers 为每个神秘符号分配倍数
////func (s *betOrderService) assignMysMultipliers() {
////	for col := 1; col < _colCount-1; col++ {
////		for row := 0; row < _rowCount; row++ {
////			if s.symbolGrid[row][col] > _longSymbol {
////				s.mysMultipliers[row-1][col] = s.randomMysMultiplier()
////			}
////		}
////	}
////}
//
////// randomMysMultiplier 随机生成神秘符号倍数
////func (s *betOrderService) randomMysMultiplier() int64 {
////	//idx := pickWeightIndex(s.gameConfig.MysSyMulWeights)
////	//return s.gameConfig.MysSyMultipliers[idx]
////	return 2 // 固定2倍，不需要权重配置
////}
//
//// processFreeModeNoWin 免费模式无中奖后的处理
//func (s *betOrderService) processFreeModeNoWin() {
//	mysCount := 0
//	for col := 1; col < _colCount-1; col++ {
//		for row := 0; row < _rowCount; row++ {
//			if s.symbolGrid[row][col] > _longSymbol {
//				mysCount++
//				headSymbol := s.symbolGrid[row][col] - _longSymbol
//				var newSymbol int64
//				for {
//					newSymbol = int64(rand.IntN(12) + 1)
//					if newSymbol != headSymbol && newSymbol != _treasure {
//						break
//					}
//				}
//				s.symbolGrid[row][col] = _longSymbol + newSymbol
//				//s.mysMultipliers[row-1][col] = s.randomMysMultiplier()
//			}
//		}
//	}
//
//	if mysCount < 9 {
//		s.generateOneMysSymbol()
//	}
//	s.syncSymbolGridToRoller()
//}
//
//// transformMysSymbols 中奖后神秘符号变身（变身为新符号，排除自身和夺宝）
//func (s *betOrderService) transformMysSymbols() {
//	for col := 1; col < _colCount-1; col++ {
//		for row := 1; row < _rowCount; row++ {
//			if s.scene.SymbolRoller[col].BoardSymbol[row] > _longSymbol {
//				headSymbol := s.scene.SymbolRoller[col].BoardSymbol[row] - _longSymbol
//				var newSymbol int64
//				for {
//					newSymbol = int64(rand.IntN(12) + 1)
//					if newSymbol != headSymbol && newSymbol != _treasure {
//						break
//					}
//				}
//				// 变身：头部新符号，尾巴新标记
//				s.scene.SymbolRoller[col].BoardSymbol[row-1] = newSymbol
//				s.scene.SymbolRoller[col].BoardSymbol[row] = _longSymbol + newSymbol
//				//s.mysMultipliers[row-1][col] = s.randomMysMultiplier()
//			}
//		}
//	}
//}
//
//// generateOneMysSymbol 生成一个新神秘符号
//func (s *betOrderService) generateOneMysSymbol() {
//	col := 1 + rand.IntN(3)
//	for row := 0; row < _rowCount-1; row++ {
//		if s.symbolGrid[row][col] >= 0 && s.symbolGrid[row][col] < _longSymbol &&
//			s.symbolGrid[row+1][col] >= 0 && s.symbolGrid[row+1][col] < _longSymbol {
//			var symbol int64
//			for {
//				symbol = int64(rand.IntN(12) + 1)
//				if symbol != _treasure {
//					break
//				}
//			}
//			s.symbolGrid[row][col] = symbol
//			s.symbolGrid[row+1][col] = _longSymbol + symbol
//			//s.mysMultipliers[row][col] = s.randomMysMultiplier()
//			return
//		}
//	}
//}
//
//// syncSymbolGridToRoller 将 symbolGrid 同步到 SymbolRoller
//func (s *betOrderService) syncSymbolGridToRoller() {
//	for c := 0; c < _colCount; c++ {
//		for r := 0; r < _rowCount; r++ {
//			s.scene.SymbolRoller[c].BoardSymbol[r] = s.symbolGrid[r][c]
//		}
//	}
//}

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

// 当3 个夺宝符号出现有界面上将触发免费模式，同时获得 3 倍的投注金额的奖金 同时获得 10 次免费旋转 ，每多一个夺宝符号将额外获得 2 倍的投注金额
// 免费模式下不会有夺宝符号
func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) (int64, int64) {
	if s.isFreeRound || scatterCount < s.gameConfig.FreeGameScatter {
		return 0, 0
	}
	mul := s.gameConfig.FreeGameBonus + (scatterCount-s.gameConfig.FreeGameScatter)*s.gameConfig.ExtraAddFreeBonus
	return s.gameConfig.FreeGameTimes, mul
}

// ------------------------------------------------------------------------------------------------------------------------------------------
// ------------------------------------------------------------------------------------------------------------------------------------------
/*
  5. 数据流向图

  initSpinSymbol()
         │
         ▼
  getSceneSymbol(realIndex)
         │
         ├──► 处理边缘列(0,4) ──► symbols[0], symbols[4]
         │
         └──► 处理中间列(1,2,3)
                │
                ├── Step1: 恢复长符号
                │
                ├── Step2: 生成长符号
                │
                └── Step3: 填充普通符号
                        │
                        ▼
                symbols[1], symbols[2], symbols[3]
         │
         ▼
     返回 symbols

  ---
  总结

  你的思路非常清晰：
  1. 边缘列单独处理，顶角底角为空
  2. 中间列通过三步构建board，最后赋值给roller
*/

func (s *betOrderService) initSpinSymbol() [_colCount]SymbolRoller {
	var cfg RollCfgType
	switch {
	case s.isFreeRound:
		cfg = s.gameConfig.RollCfg.Free
	default:
		cfg = s.gameConfig.RollCfg.Base
	}

	realIndex := cfg.UseKey[pickWeightIndex(cfg.Weight)]
	return s.getSceneSymbol(realIndex)
}

func (s *betOrderService) getSceneSymbol(realIndex int) [_colCount]SymbolRoller {
	var symbols [_colCount]SymbolRoller
	realData := s.gameConfig.RealData[realIndex]

	// 处理第0列和第4列
	for c := 0; c < _colCount; c++ {
		// 第0列和第4列（索引0和4）：顶角/底角置空，只填充行1-4
		if !(c == 0 || c == _colCount-1) {
			continue
		}
		reel := realData[c]
		reelLen := len(reel)
		start := rand.IntN(reelLen)
		roller := SymbolRoller{Real: realIndex, Start: start, Col: c, Len: reelLen}

		roller.Fall = (start + 3) % reelLen // 4个符号：start ~ start+3
		roller.BoardSymbol[0] = 0           // 顶角空
		roller.BoardSymbol[_rowCount-1] = 0 // 底角空
		for r := 1; r < _rowCount-1; r++ {
			roller.BoardSymbol[r] = reel[(start+r-1)%reelLen]
		}
		symbols[c] = roller
	}

	// 处理中间列 （1，2，3列）
	for c := 0; c < _colCount; c++ {
		if c == 0 || c == _colCount-1 {
			continue
		}

		board := [_rowCount]int64{0, 0, 0, 0, 0, 0}
		// 获取中间列的board
		// 1.scene里被占用的长符号 标记
		// 2.中间列生成的长符号位置 标记
		// 3.填充中间列的board

		//symbols[c] = roller
	}

	return symbols
}

// 获取当前列的长符号占位情况
func (s *betOrderService) getMysBaseSymbolBoard(c int) []longBlock {
	long := make([]longBlock, 0, len(s.scene.long[c]))

	// 1. 恢复场景中的长符号
	for _, lb := range s.scene.long[c] {
		long = append(long, longBlock{StartRow: lb.StartRow, EndRow: lb.EndRow, Symbol: _longSymbol})
	}

	// 如果这一列已经有3个长符号了，则返回 长符号已经占满了列
	if len(long) >= 3 {
		return long
	}

	// 2. 生成长符号
	weights := s.gameConfig.BaseBigSyWeights
	if s.isFreeRound {
		weights = s.gameConfig.FreeBigSyWeights
	}

	longCount := pickWeightIndex(weights)
	if longCount == 0 || longCount > len(s.gameConfig.BigSyMultiples) {
		return long
	}

	patterns := s.gameConfig.BigSyMultiples[longCount-1]
	if len(patterns) == 0 {
		return long
	}

	// 轮询查找不冲突的布局
	startIdx := rand.IntN(len(patterns))
	for i := 0; i < len(patterns); i++ {
		if newBlocks, ok := s.tryPattern(patterns[(startIdx+i)%len(patterns)], long); ok {
			return append(long, newBlocks...)
		}
	}

	return long
}

// tryPattern 尝试布局，检测冲突
func (s *betOrderService) tryPattern(pattern []int64, existing []longBlock) ([]longBlock, bool) {
	var newBlocks []longBlock
	row := 1

	for _, val := range pattern {
		if val > 1 {
			headRow, tailRow := int64(row-1), int64(row)

			// 边界检查
			if tailRow >= _rowCount {
				return nil, false
			}

			// 检查冲突
			for _, lb := range existing {
				if lb.StartRow == headRow || lb.EndRow == headRow ||
					lb.StartRow == tailRow || lb.EndRow == tailRow {
					return nil, false
				}
			}

			newBlocks = append(newBlocks, longBlock{StartRow: headRow, EndRow: tailRow, Symbol: _longSymbol})
			row++
		}
		row++
	}

	return newBlocks, true
}
