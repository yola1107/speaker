package ajtm

import (
	"math/rand/v2"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable         [][]int64  `json:"pay_table"`                            // 赔率表 [符号ID-1][列数-1]
	FreeTimes        int64      `json:"free_game_times"`                      // 基础免费次数
	AddFreeTimes     int64      `json:"extra_add_free_times"`                 // 每多1个夺宝增加的免费次数
	FreeGameScatter  int64      `json:"trigger_free_game_need_scatter"`       // 触发免费需要的夺宝数
	BaseBigSyNums    []int64    `json:"base_big_symbol_nums"`                 // 大符号数量选项 [0,1,2,3]
	BaseBigSyWeights []int64    `json:"base_big_symbol_weights"`              // 对应权重，索引=数量
	FreeBigSyNums    []int64    `json:"free_big_symbol_nums"`                 //
	FreeBigSyWeights []int64    `json:"free_big_symbol_weights"`              //
	BigSyMultiples   []Location `json:"big_symbol_multiples"`                 // 布局配置 [数量-1][布局选项][位置]
	MysSyMultipliers []int64    `json:"mysterious_symbol_multipliers"`        //
	MysSyMulWeights  []int64    `json:"mysterious_symbol_multiplier_weights"` //
	RollCfg          RollConf   `json:"roll_cfg"`                             // 滚轴配置
	RealData         []Reals    `json:"real_data"`                            // 滚轴数据 [模式][列][符号序列]
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

// createMatrix 创建符号矩阵（三阶段生成）
func (s *betOrderService) createMatrix() {
	s.scene.SymbolRoller = [_colCount]SymbolRoller{}
	s.mysMultipliers = int64Grid{}
	s.addMysSymbolTails()
	s.fillEmptyPositions()
	s.changeMysTailNumbers()
	s.handleSymbolGrid()
	s.assignMysMultipliers()
}

// addMysSymbolTails 阶段1：生成神秘符号占位
func (s *betOrderService) addMysSymbolTails() {
	bigSyWeights := s.gameConfig.BaseBigSyWeights
	if s.isFreeRound {
		bigSyWeights = s.gameConfig.FreeBigSyWeights
	}

	for col := 1; col < _colCount-1; col++ {
		realIndex := pickWeightIndex(bigSyWeights)

		if realIndex > 0 && realIndex < len(s.gameConfig.BigSyMultiples) {
			layouts := s.gameConfig.BigSyMultiples[realIndex-1]
			if len(layouts) == 0 {
				continue
			}
			layoutIndex := rand.IntN(len(layouts))
			layout := layouts[layoutIndex]

			currentRow := 1
			for _, value := range layout {
				if value > 1 {
					for i := int64(1); i < value; i++ {
						if currentRow < _rowCount {
							s.scene.SymbolRoller[col].BoardSymbol[currentRow] = _longSymbol
							currentRow++
						}
					}
				}
				currentRow++
			}
		}
	}
}

// fillEmptyPositions 阶段2：填充空位
func (s *betOrderService) fillEmptyPositions() {
	rollCfg := s.gameConfig.RollCfg.Base
	if s.isFreeRound {
		rollCfg = s.gameConfig.RollCfg.Free
	}

	realIndex := rollCfg.UseKey[pickWeightIndex(rollCfg.Weight)]
	realData := s.gameConfig.RealData[realIndex]

	for c := 0; c < _colCount; c++ {
		reelLen := len(realData[c])
		start := rand.IntN(reelLen)
		s.scene.SymbolRoller[c].Col = c
		s.scene.SymbolRoller[c].Start = start
		s.scene.SymbolRoller[c].Real = realIndex
		s.scene.SymbolRoller[c].Len = reelLen

		add := 0
		for r := 0; r < _rowCount; r++ {
			// 边缘列的顶角和底角保持为0
			if (c == 0 || c == _colCount-1) && (r == 0 || r == _rowCount-1) {
				continue
			}

			// 只填充空位（0），占位标记（_longSymbol）不覆盖
			if s.scene.SymbolRoller[c].BoardSymbol[r] == 0 {
				s.scene.SymbolRoller[c].BoardSymbol[r] = realData[c][(start+add)%reelLen]
				add++
			}
		}
	}
}

// changeMysTailNumbers 阶段3：神秘符号尾巴变真实符号
// 头部保持原符号值，尾巴变为 _longSymbol + symbol
func (s *betOrderService) changeMysTailNumbers() {
	for col := 1; col < _colCount-1; col++ {
		for row := 1; row < _rowCount; row++ {
			if s.scene.SymbolRoller[col].BoardSymbol[row] != _longSymbol {
				continue
			}

			headRow := -1
			var headSymbol int64
			for r := row - 1; r >= 0; r-- {
				sym := s.scene.SymbolRoller[col].BoardSymbol[r]
				if sym > 0 && sym < _longSymbol {
					headRow = r
					headSymbol = sym
					break
				}
			}

			if headRow < 0 {
				continue
			}

			for i := row; i < _rowCount; i++ {
				if s.scene.SymbolRoller[col].BoardSymbol[i] == _longSymbol {
					s.scene.SymbolRoller[col].BoardSymbol[i] = _longSymbol + headSymbol
					row = i
				} else {
					break
				}
			}
		}
	}
}

// assignMysMultipliers 为每个神秘符号分配倍数
func (s *betOrderService) assignMysMultipliers() {
	for col := 1; col < _colCount-1; col++ {
		for row := 0; row < _rowCount; row++ {
			if s.symbolGrid[row][col] > _longSymbol {
				s.mysMultipliers[row-1][col] = s.randomMysMultiplier()
			}
		}
	}
}

// randomMysMultiplier 随机生成神秘符号倍数
func (s *betOrderService) randomMysMultiplier() int64 {
	idx := pickWeightIndex(s.gameConfig.MysSyMulWeights)
	return s.gameConfig.MysSyMultipliers[idx]
}

// processFreeModeNoWin 免费模式无中奖后的处理
func (s *betOrderService) processFreeModeNoWin() {
	mysCount := 0
	for col := 1; col < _colCount-1; col++ {
		for row := 0; row < _rowCount; row++ {
			if s.symbolGrid[row][col] > _longSymbol {
				mysCount++
				headSymbol := s.symbolGrid[row][col] - _longSymbol
				var newSymbol int64
				for {
					newSymbol = int64(rand.IntN(12) + 1)
					if newSymbol != headSymbol && newSymbol != _treasure {
						break
					}
				}
				s.symbolGrid[row][col] = _longSymbol + newSymbol
				s.mysMultipliers[row-1][col] = s.randomMysMultiplier()
			}
		}
	}

	if mysCount < 9 {
		s.generateOneMysSymbol()
	}
	s.syncSymbolGridToRoller()
}

// generateOneMysSymbol 生成一个新神秘符号
func (s *betOrderService) generateOneMysSymbol() {
	col := 1 + rand.IntN(3)
	for row := 0; row < _rowCount-1; row++ {
		if s.symbolGrid[row][col] >= 0 && s.symbolGrid[row][col] < _longSymbol &&
			s.symbolGrid[row+1][col] >= 0 && s.symbolGrid[row+1][col] < _longSymbol {
			var symbol int64
			for {
				symbol = int64(rand.IntN(12) + 1)
				if symbol != _treasure {
					break
				}
			}
			s.symbolGrid[row][col] = symbol
			s.symbolGrid[row+1][col] = _longSymbol + symbol
			s.mysMultipliers[row][col] = s.randomMysMultiplier()
			return
		}
	}
}

// syncSymbolGridToRoller 将 symbolGrid 同步到 SymbolRoller
func (s *betOrderService) syncSymbolGridToRoller() {
	for c := 0; c < _colCount; c++ {
		for r := 0; r < _rowCount; r++ {
			s.scene.SymbolRoller[c].BoardSymbol[r] = s.symbolGrid[r][c]
		}
	}
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

// 当3 个夺宝符号出现有界面上将触发免费模式，同时获得 3 倍的投注金额的奖金 同时获得 10 次免费旋转 ，每多一个夺宝符号将额外获得 2 倍的投注金额
// 免费模式下不会有夺宝符号
func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if s.isFreeRound || scatterCount < s.gameConfig.FreeGameScatter {
		return 0
	}
	return s.gameConfig.FreeTimes + (scatterCount-s.gameConfig.FreeGameScatter)*s.gameConfig.AddFreeTimes
}
