package xslm

import (
	"math/rand/v2"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable      [][]int64 `json:"pay_table"`       // 赔付表（符号倍率表）
	FreeSpinCount []int64   `json:"free_spin_count"` // 免费次数配置   对应scatter1~5个时，奖励的免费游戏数量。 [0,0,7,10,15],
	RollCfg       RollConf  `json:"roll_cfg"`        // 滚轴配置
	RealData      []Reals   `json:"real_data"`       // 真实数据（预设滚轴数据）
}
type RollConf struct {
	Base RollCfgType            `json:"base"`
	Free map[string]RollCfgType `json:"free"` // 免费游戏配置（key为女性符号收集状态，如"000", "001"等）
}
type RollCfgType struct {
	UseKey []int `json:"use_key"`
	Weight []int `json:"weight"`
	WTotal int   `json:"-"`
}
type Reals [][]int64

type SymbolRoller struct {
	Real        int              `json:"real"`  //选择的第几个轮盘
	Start       int              `json:"start"` //开始索引
	Fall        int              `json:"fall"`  //开始索引
	Col         int              `json:"col"`   //第几列
	BoardSymbol [_rowCount]int64 `json:"board"`
}

func (s *betOrderService) initGameConfigs() {
	if s.gameConfig != nil {
		return
	}

	s.gameConfig = &gameConfigJson{}
	if err := jsoniter.UnmarshalFromString(_gameJsonConfigsRaw, s.gameConfig); err != nil {
		panic(err)
	}

	// 预计算基础模式权重总和
	s.gameConfig.RollCfg.Base.WTotal = 0
	for _, w := range s.gameConfig.RollCfg.Base.Weight {
		s.gameConfig.RollCfg.Base.WTotal += w
	}
	if s.gameConfig.RollCfg.Base.WTotal <= 0 {
		panic("base roll weight sum <= 0")
	}

	// 预计算免费模式权重总和
	for key, cfg := range s.gameConfig.RollCfg.Free {
		cfg.WTotal = 0
		for _, w := range cfg.Weight {
			cfg.WTotal += w
		}
		if cfg.WTotal <= 0 {
			panic("free roll weight sum <= 0 for key: " + key)
		}
		s.gameConfig.RollCfg.Free[key] = cfg
	}
}

func (s *betOrderService) getSceneSymbol() [_colCount]SymbolRoller {
	rollCfg := s.gameConfig.RollCfg.Base
	if s.isFreeRound {
		key := ""
		for i := 0; i < 3; i++ {
			if s.femaleCountsForFree[i] >= _femaleFullCount {
				key += "1"
			} else {
				key += "0"
			}
		}
		if freeCfg, ok := s.gameConfig.RollCfg.Free[key]; ok {
			rollCfg = freeCfg
		} else {
			panic("free roll weight sum <= 0 for key: " + key)
		}
	}

	realIndex := 0
	r := rand.IntN(rollCfg.WTotal)
	for i, w := range rollCfg.Weight {
		if r < w {
			realIndex = rollCfg.UseKey[i]
			break
		}
		r -= w
	}
	if realIndex >= len(s.gameConfig.RealData) {
		panic("real data index out of range")
	}

	// 生成符号网格和滚轴
	var symbolGrid int64Grid
	var rollers [_colCount]SymbolRoller

	for col := 0; col < int(_colCount); col++ {
		data := s.gameConfig.RealData[realIndex][col]
		if len(data) == 0 {
			panic("real data column is empty")
		}

		start := rand.IntN(len(data))
		end := (start + int(_rowCount) - 1) % len(data)
		roller := SymbolRoller{Real: realIndex, Start: start, Fall: end, Col: col}

		for row := 0; row < int(_rowCount); row++ {
			symbol := data[(start+row)%len(data)]
			// 左上角和右上角填充下block [0][0] [0][4]
			// (*grid)[0][0], (*grid)[0][_colCount-1] = _blocked, _blocked
			if row == 0 && (col == 0 || col == int(_colCount-1)) {
				symbol = _blocked
			}

			if s.isFreeRound {
				// 免费模式下，根据 ABC 计数转换符号（A->wildA, B->wildB, C->wildC）
				// 检查是否是女性符号（A/B/C），且对应的计数 >= 10，转换为对应的 女性百搭
				if symbol >= _femaleA && symbol <= _femaleC {
					idx := symbol - _femaleA
					if s.femaleCountsForFree[idx] >= _femaleFullCount {
						symbol = _wildFemaleA + idx
					}
				}
			}

			symbolGrid[row][col] = symbol
			roller.BoardSymbol[int(_rowCount)-1-row] = symbol
		}
		rollers[col] = roller
	}

	return rollers
}

// ringSymbol 补充掉下来导致的空缺位置
func (r *SymbolRoller) ringSymbol(c *gameConfigJson) {
	var newBoard [_rowCount]int64
	var zeroIndex []int
	for i, s := range r.BoardSymbol {
		if s != 0 {
			newBoard[i] = s
		} else {
			zeroIndex = append(zeroIndex, i)
		}
	}
	for _, index := range zeroIndex {
		newSymbol := r.getFallSymbol(c)
		newBoard[index] = newSymbol
	}
	r.BoardSymbol = newBoard
}

// getFallSymbol 从滚轴获取下一个符号
// 逻辑：Fall 指向当前最后一个符号的位置，下一个符号是 (Fall + 1) % len
func (r *SymbolRoller) getFallSymbol(c *gameConfigJson) int64 {
	data := c.RealData[r.Real][r.Col]
	nextPos := (r.Fall + 1) % len(data)
	r.Fall = nextPos
	return data[nextPos]
}
