package hbtr2

import (
	"math/rand/v2"
	"strconv"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable                   [][]int64 `json:"pay_table"`                      // 赔付表
	FreeGameTimes              int64     `json:"free_game_times"`                // 触发免费初始次数
	ExtraAddFreeTimes          int64     `json:"extra_add_free_times"`           // 额外赠送次数
	TriggerFreeGameNeedScatter int64     `json:"trigger_free_game_need_scatter"` // 触发免费所需 scatter 数
	RollCfg                    RollConf  `json:"roll_cfg"`                       // 滚轴配置
	RealData                   []Reals   `json:"real_data"`                      // 真实数据
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
	Real        int               `json:"real"`  // 选择的第几个轮盘
	Col         int               `json:"col"`   // 轮盘填充列
	Len         int               `json:"len"`   // 长度
	Start       int               `json:"start"` // 开始索引
	Fall        int               `json:"fall"`  // 开始索引
	BoardSymbol [_boardSize]int64 `json:"board"` // 盘面符号
}

func (s *betOrderService) initGameConfigs() {
	if s.gameConfig != nil {
		return
	}
	s.parseGameConfigs()
}

func (s *betOrderService) parseGameConfigs() {
	s.gameConfig = &gameConfigJson{}
	if err := jsoniter.UnmarshalFromString(_gameJsonConfigsRaw, s.gameConfig); err != nil {
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

func (s *betOrderService) initSpinSymbol() [_rollerColCount]SymbolRoller {
	var rollCfg RollCfgType
	if s.isFreeRound {
		rollCfg = s.gameConfig.RollCfg.Free
	} else {
		rollCfg = s.gameConfig.RollCfg.Base
	}
	return s.getSceneSymbol(rollCfg)
}

func (s *betOrderService) getSceneSymbol(rollCfg RollCfgType) [_rollerColCount]SymbolRoller {
	if s.isFreeRound {
		return s.getSceneSymbolFree(rollCfg)
	}
	return s.getSceneSymbolBase(rollCfg)
}

// selectRealIndex 按权重选择 realData 下标
func (s *betOrderService) selectRealIndex(rollCfg RollCfgType) int {
	r := rand.IntN(rollCfg.WTotal)
	for i, w := range rollCfg.Weight {
		if r < w {
			return rollCfg.UseKey[i]
		}
		r -= w
	}
	return 0
}

// 基础模式：纯随机填充
func (s *betOrderService) getSceneSymbolBase(rollCfg RollCfgType) [_rollerColCount]SymbolRoller {
	realIndex := s.selectRealIndex(rollCfg)
	if realIndex < 0 || realIndex >= len(s.gameConfig.RealData) {
		panic("real data index out of range: " + strconv.Itoa(realIndex))
	}
	realData := s.gameConfig.RealData[realIndex]

	// 检查 realData 是否有足够的数据
	if len(realData) < _rollerColCount {
		panic("real data must have at least 7 columns")
	}

	/*
		第 1 行从 realData[6]（逆序取 4）构成中间 4 列，两边填 0
		第 2～5 行按列从 realData[0]～realData[5] 各取 4 个符号垂直填充
	*/

	var symbols [_rollerColCount]SymbolRoller
	for col := 0; col < _rollerColCount; col++ {
		data := realData[col]
		dataLen := len(data)
		if len(data) == 0 {
			panic("real data column is empty")
		}

		start := rand.IntN(dataLen)
		// 往后取4个
		dataIndex := 0
		board := [_boardSize]int64{}
		for row := 0; row < _rowCount-1; row++ {
			symbol := data[(start+dataIndex)%dataLen]
			board[dataIndex] = symbol
			dataIndex++
		}

		end := (start + dataIndex - 1) % dataLen // 固定4个
		roller := SymbolRoller{Real: realIndex, Start: start, Fall: end, Col: col, Len: dataLen, BoardSymbol: board}
		symbols[col] = roller
	}
	return symbols
}

// 免费模式：先落位保留 wild，再填充剩余空位
func (s *betOrderService) getSceneSymbolFree(rollCfg RollCfgType) [_rollerColCount]SymbolRoller {
	// 预处理保留的 wild 坐标
	var topReserved [4]bool
	var colReserved [_colCount][4]bool
	if len(s.scene.LsatWildPos) > 0 {
		for _, p := range s.scene.LsatWildPos {
			rp, cp := p[0], p[1]
			if rp == 0 && cp > 0 && cp < _colCount-1 {
				topReserved[_boardSize-cp] = true
				continue
			}
			if rp >= 1 && rp < _rowCount && cp >= 0 && cp < _colCount {
				colReserved[cp][rp-1] = true
			}
		}
	}

	realIndex := s.selectRealIndex(rollCfg)
	if realIndex < 0 || realIndex >= len(s.gameConfig.RealData) {
		panic("real data index out of range: " + strconv.Itoa(realIndex))
	}
	realData := s.gameConfig.RealData[realIndex]
	if len(realData) < _rollerColCount {
		panic("real data must have at least 7 columns")
	}

	var symbols [_rollerColCount]SymbolRoller
	for col := 0; col < _rollerColCount; col++ {
		data := realData[col]
		dataLen := len(data)
		if len(data) == 0 {
			panic("real data column is empty")
		}

		start := rand.IntN(dataLen)
		dataIndex := 0
		board := [_boardSize]int64{}

		// 先落位保留 wild
		if col == _colCount { // 顶行中间4列
			for i := 0; i < _boardSize; i++ {
				if topReserved[i] {
					board[i] = _wild
				}
			}
		} else {
			for i := 0; i < _boardSize; i++ {
				if colReserved[col][i] {
					board[i] = _wild
				}
			}
		}

		// 再填充剩余空位
		for row := 0; row < _rowCount-1; row++ {
			if board[row] != 0 {
				continue
			}
			symbol := data[(start+dataIndex)%dataLen]
			board[row] = symbol
			dataIndex++
		}

		// end 与实际取出的符号数量对应
		end := start
		if dataIndex > 0 {
			end = (start + dataIndex - 1) % dataLen
		}

		roller := SymbolRoller{Real: realIndex, Start: start, Fall: end, Col: col, Len: dataLen, BoardSymbol: board}
		symbols[col] = roller
	}

	// 用完即清
	s.scene.LsatWildPos = nil
	return symbols
}

// getNextSymbol 从realData获取符号（使用Start索引，start--）  处理列消除 start在最上面，消除补齐的symbol应该用start--前移的符号
func (rs *SymbolRoller) getNextSymbol(gameConfig *gameConfigJson) int64 {
	data := gameConfig.RealData[rs.Real][rs.Col]
	rs.Start = (rs.Start - 1 + len(data)) % len(data)
	return data[rs.Start]
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

// calcNewFreeGameNum 计算触发免费游戏的次数
func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if scatterCount < s.gameConfig.TriggerFreeGameNeedScatter {
		return 0
	}
	// 规则：4个夺宝触发8次免费，每多1个夺宝增加2次免费
	//return 8 + (scatterCount-4)*2
	return s.gameConfig.FreeGameTimes + (scatterCount-s.gameConfig.TriggerFreeGameNeedScatter)*s.gameConfig.ExtraAddFreeTimes
}
