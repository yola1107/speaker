// game/bxkh2/bet_order_configs.go
package bxkh2

import (
	"fmt"

	"egame-grpc/game/common"
	"egame-grpc/game/common/rand"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable         [][]int64  `json:"pay_table"`
	FreeTimes        int64      `json:"free_game_times"`
	AddFreeTimes     int64      `json:"extra_add_free_times"`
	FreeGameScatter  int64      `json:"trigger_free_game_need_scatter"`
	BaseProbability  int64      `json:"base_silver_symbol_probability"`
	FreeProbability  int64      `json:"free_silver_symbol_probability"`
	BaseBigSyNums    []int64    `json:"base_big_symbol_nums"`
	BaseBigSyWeights []int64    `json:"base_big_symbol_weights"`
	FreeBigSyNums    []int64    `json:"free_big_symbol_nums"`
	FreeBigSyWeights []int64    `json:"free_big_symbol_weights"`
	BigSyMultiples   []Location `json:"big_symbol_multiples"`
	RollCfg          RollConf   `json:"roll_cfg"`
	RealData         []Reals    `json:"real_data"`
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
	Real        int              `json:"real"`
	Start       int              `json:"start"`
	Fall        int              `json:"fall"`
	Col         int              `json:"col"`
	BoardSymbol [_rowCount]int64 `json:"board"`
}

var _gameJsonConfig *gameConfigJson
var sceneDataKeyPrefix = fmt.Sprintf("scene-%d", GameID_18965)

func (s *betOrderService) initGameConfigs() {
	if s.gameConfig != nil {
		return
	}
	s.parseGameConfigs()
}

func (s *betOrderService) parseGameConfigs() {
	raw := _gameJsonConfigsRaw
	if !s.debug.open {
		cacheText, _ := common.GetRedisGameJson(GameID_18965)
		if len(cacheText) > 0 {
			raw = cacheText
		}
	}
	s.gameConfig = &gameConfigJson{}
	if err := jsoniter.UnmarshalFromString(raw, s.gameConfig); err != nil {
		panic(err)
	}

	// 计算权重总和
	s.gameConfig.RollCfg.Base.WTotal = 0
	for _, w := range s.gameConfig.RollCfg.Base.Weight {
		s.gameConfig.RollCfg.Base.WTotal += w
	}
	s.gameConfig.RollCfg.Free.WTotal = 0
	for _, w := range s.gameConfig.RollCfg.Free.Weight {
		s.gameConfig.RollCfg.Free.WTotal += w
	}

	// 验证配置
	if len(s.gameConfig.PayTable) == 0 {
		panic("no pay table conf exists")
	}
	if s.gameConfig.RollCfg.Base.WTotal <= 0 {
		panic("real data base roll weight <= 0")
	}
	if s.gameConfig.RollCfg.Free.WTotal <= 0 {
		panic("real data free roll weight <= 0")
	}
}

// createMatrix 创建符号矩阵
func (s *betOrderService) createMatrix(stage int8) {
	s.addSymbolTails(stage)
	s.fillEmptyPositions(stage)
	s.changeTailNumbers(stage)
	s.symbolGrid = s.buildSymbolGrid()
	s.buildTailArrays()
}

// addSymbolTails 生成长符号尾巴
func (s *betOrderService) addSymbolTails(stage int8) {
	s.scene.SymbolRoller = [_colCount]SymbolRoller{}

	var weights int64
	var bigSyWeights []int64
	if stage == _spinTypeBase {
		bigSyWeights = s.gameConfig.BaseBigSyWeights
	} else {
		bigSyWeights = s.gameConfig.FreeBigSyWeights
	}
	for _, w := range bigSyWeights {
		weights += w
	}

	// 只处理第2-5列（索引1-4）
	for col := 1; col < _colCount-1; col++ {
		r := rand.Int64N(weights)
		var realIndex int64
		for i, w := range bigSyWeights {
			if r < w {
				realIndex = int64(i)
				break
			}
			r -= w
		}

		if realIndex > 0 {
			tailWeight := len(s.gameConfig.BigSyMultiples[realIndex-1])
			randomIndex := rand.IntN(tailWeight)
			tailValues := s.gameConfig.BigSyMultiples[realIndex-1][randomIndex]

			currentRow := 1
			for _, value := range tailValues {
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

// fillEmptyPositions 填充空白位置
func (s *betOrderService) fillEmptyPositions(stage int8) {
	var rollCfg RollCfgType
	if stage == _spinTypeBase {
		rollCfg = s.gameConfig.RollCfg.Base
	} else {
		rollCfg = s.gameConfig.RollCfg.Free
	}

	r := rand.IntN(rollCfg.WTotal)
	realIndex := 0
	for i, w := range rollCfg.Weight {
		if r < w {
			realIndex = rollCfg.UseKey[i]
			break
		}
		r -= w
	}

	realData := s.gameConfig.RealData[realIndex]
	for i := 0; i < _colCount; i++ {
		reelLen := len(realData[i])
		start := rand.IntN(reelLen)
		s.scene.SymbolRoller[i].Col = i
		s.scene.SymbolRoller[i].Start = start
		s.scene.SymbolRoller[i].Real = realIndex

		add := 0
		for j := 0; j < _rowCount; j++ {
			if s.scene.SymbolRoller[i].BoardSymbol[j] == 0 {
				index := (start + add) % reelLen
				s.scene.SymbolRoller[i].BoardSymbol[j] = realData[i][index]
				add++
				s.scene.SymbolRoller[i].Fall = index
			}
		}
	}
}

// changeTailNumbers 尾巴变真实符号
func (s *betOrderService) changeTailNumbers(stage int8) {
	for col := 1; col < _colCount-1; col++ {
		for row := 1; row < _rowCount; row++ {
			if s.scene.SymbolRoller[col].BoardSymbol[row] == _longSymbol {
				targetRow := 0
				for r := row - 1; r >= 0; r-- {
					if s.scene.SymbolRoller[col].BoardSymbol[r] < _longSymbol {
						targetRow = r
						break
					}
				}

				randomTail := rand.IntN(100)
				tailWeight := s.gameConfig.BaseProbability
				if stage == _spinTypeFree {
					tailWeight = s.gameConfig.FreeProbability
				}
				if randomTail < int(tailWeight) && s.scene.SymbolRoller[col].BoardSymbol[targetRow] < _treasure {
					s.scene.SymbolRoller[col].BoardSymbol[targetRow] += _silverSymbol
				}

				colTmp := 0
				for i := row; i < _rowCount; i++ {
					if s.scene.SymbolRoller[col].BoardSymbol[i] == _longSymbol {
						s.scene.SymbolRoller[col].BoardSymbol[i] += s.scene.SymbolRoller[col].BoardSymbol[targetRow]
						colTmp++
					} else {
						break
					}
				}
				row += colTmp
			}
		}
	}
}

// buildSymbolGrid 构建符号网格
func (s *betOrderService) buildSymbolGrid() int64Grid {
	var grid int64Grid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			grid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
	return grid
}

// buildTailArrays 构建尾巴数组
func (s *betOrderService) buildTailArrays() {
	// 保持原逻辑不变，用于前端显示
}

// getSymbolBaseMultiplier 获取符号赔率
func (s *betOrderService) getSymbolBaseMultiplier(symbol int64, starN int) int64 {
	if len(s.gameConfig.PayTable) < int(symbol-1) {
		return 0
	}
	table := s.gameConfig.PayTable[symbol-1]
	if len(table) < starN {
		starN = len(table)
	}
	return table[starN-1]
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

// ringSymbol 补充掉落符号
func (rs *SymbolRoller) ringSymbol(gameConfig *gameConfigJson) {
	var newBoard [_rowCount]int64
	index := 0
	for i, s := range rs.BoardSymbol {
		if s != 0 {
			newBoard[i] = s
			index++
		}
	}
	needNewSymbol := _rowCount - index
	for k := needNewSymbol - 1; k >= 0; k-- {
		newBoard[k] = rs.getFallSymbol(gameConfig)
	}
	rs.BoardSymbol = newBoard
}
