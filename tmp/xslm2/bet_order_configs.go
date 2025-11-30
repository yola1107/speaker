package xslm2

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
		key := s.getFemaleCountKey()
		if freeCfg, ok := s.gameConfig.RollCfg.Free[key]; ok {
			rollCfg = freeCfg
		} else {
			panic("free roll weight sum <= 0 for key: " + key)
		}
	}

	// 根据权重随机选择滚轴数据
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

	// 生成符号滚轴
	var rollers [_colCount]SymbolRoller
	for col := 0; col < int(_colCount); col++ {
		data := s.gameConfig.RealData[realIndex][col]
		if len(data) == 0 {
			panic("real data column is empty")
		}

		// 判断是否是block列（第0列或最后一列）
		isBlockCol := col == 0 || col == int(_colCount-1)
		actualRowCount := int(_rowCount)
		if isBlockCol {
			actualRowCount--
		}

		start := rand.IntN(len(data))
		end := (start + actualRowCount - 1) % len(data)
		roller := SymbolRoller{Real: realIndex, Start: start, Fall: end, Col: col}

		// 填充符号
		dataIndex := 0
		for row := 0; row < int(_rowCount); row++ {
			var symbol int64
			// 左上角和右上角固定为 block [0][0] [0][4]
			if row == 0 && isBlockCol {
				symbol = _blocked
			} else {
				// 非 block 位置从 data 中按顺序取
				symbol = data[(start+dataIndex)%len(data)]
				dataIndex++

				// 免费模式下：根据 ABC 计数转换符号（A->wildA, B->wildB, C->wildC）
				if s.isFreeRound {
					symbol = s.convertFemaleSymbol(symbol)
				}
			}
			roller.BoardSymbol[int(_rowCount)-1-row] = symbol
		}
		rollers[col] = roller
	}
	return rollers
}

// ringSymbol 补充掉下来导致的空缺位置
// 注意：BoardSymbol 从下往上存储（索引0=最下面，索引3=最上面）
// 填充时应该从最上面（索引3）开始，按照索引从高到低（3→2→1→0）的顺序填充
func (r *SymbolRoller) ringSymbol(c *gameConfigJson) {
	var newBoard [_rowCount]int64
	// 先复制非空白位置
	for i, s := range r.BoardSymbol {
		if s != 0 {
			newBoard[i] = s
		}
	}
	// 从高索引到低索引（3→2→1→0）遍历，遇到空白就填充
	for i := int(_rowCount) - 1; i >= 0; i-- {
		if newBoard[i] == 0 {
			newBoard[i] = r.getFallSymbol(c)
		}
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

// getFreeRoundCountFromTreasure 基础模式根据夺宝数量从配置获取免费次数
func (s *betOrderService) getFreeRoundCountFromTreasure() int64 {
	if s.treasureCount < _triggerTreasureCount {
		return 0
	}
	idx := int(s.treasureCount - 1)
	if idx >= len(s.gameConfig.FreeSpinCount) {
		idx = len(s.gameConfig.FreeSpinCount) - 1
	}
	return s.gameConfig.FreeSpinCount[idx]
}

// getFemaleCountKey 生成女性符号收集状态的key
func (s *betOrderService) getFemaleCountKey() string {
	key := ""
	for i := 0; i < 3; i++ {
		if s.scene.RoundFemaleCountsForFree[i] >= _femaleFullCount {
			key += "1"
		} else {
			key += "0"
		}
	}
	return key
}

// convertFemaleSymbol 免费模式下转换女性符号为女性百搭
func (s *betOrderService) convertFemaleSymbol(symbol int64) int64 {
	if symbol >= _femaleA && symbol <= _femaleC {
		idx := symbol - _femaleA
		if s.scene.RoundFemaleCountsForFree[idx] >= _femaleFullCount {
			return _wildFemaleA + idx
		}
	}
	return symbol
}
