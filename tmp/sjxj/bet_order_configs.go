package sjxj

import (
	"math/rand/v2"
	"strconv"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable             [][]int64 `json:"pay_table"`              // 赔付表
	Lines                [][]int64 `json:"lines"`                  // 中奖线定义
	FreeGameScatterMin   int64     `json:"free_game_scatter_min"`  // 触发免费最少 Scatter（未中线奖时）
	FreeGameTimes        int64     `json:"free_game_times"`        // 免费游戏基础次数
	FreeUnlockThresholds []int64   `json:"free_unlock_thresholds"` // 解锁第5-8行所需的夺宝数：[8, 12, 16, 20]
	FreeUnlockAddSpins   int64     `json:"free_unlock_add_spins"`  // 每新解锁一行增加的免费次数
	RollCfg              RollConf  `json:"roll_cfg"`               // 滚轴配置
	RealData             []Reals   `json:"real_data"`              // 真实数据
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

	if len(s.gameConfig.FreeUnlockThresholds) != 4 {
		panic("len(s.gameConfig.FreeUnlockThresholds) != 4")
	}
	if s.gameConfig.FreeUnlockAddSpins == 0 {
		panic("s.gameConfig.FreeUnlockAddSpins == 0 ")
	}

	x := int64(_rowCount * _colCount)
	for _, line := range s.gameConfig.Lines {
		for _, p := range line {
			if p < 0 || p >= x {
				panic("line position out of range")
			}
		}
	}
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
	if s.isFreeRound {
		return s.getSceneSymbolFree(s.gameConfig.RollCfg.Free)
	}
	return s.getSceneSymbolBase(s.gameConfig.RollCfg.Base)
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

// 基础模式：纯随机填充（8×5 权威盘面）
func (s *betOrderService) getSceneSymbolBase(rollCfg RollCfgType) [_colCount]SymbolRoller {
	realIndex := s.selectRealIndex(rollCfg)
	if realIndex < 0 || realIndex >= len(s.gameConfig.RealData) {
		panic("real data index out of range: " + strconv.Itoa(realIndex))
	}
	realData := s.gameConfig.RealData[realIndex]

	var symbols [_colCount]SymbolRoller
	for col := 0; col < _colCount; col++ {
		reel := realData[col]
		reelLen := len(reel)
		if reelLen == 0 {
			panic("real data column is empty")
		}

		start := rand.IntN(reelLen)
		end := (start + _rowCount - 1) % reelLen
		roller := SymbolRoller{Real: realIndex, Start: start, Fall: end, Col: col, Len: reelLen}

		for row := 0; row < _rowCount; row++ {
			symbol := reel[(start+row)%reelLen]
			roller.BoardSymbol[int(_rowCount)-1-row] = symbol
		}
		symbols[col] = roller
	}

	return symbols
}

// 免费模式 8×5：ScatterLock 固定夺宝占位，其余格由滚轴填充；ScatterLock 在本局结束统一重建。
func (s *betOrderService) getSceneSymbolFree(rollCfg RollCfgType) [_colCount]SymbolRoller {
	realIndex := s.selectRealIndex(rollCfg)
	if realIndex < 0 || realIndex >= len(s.gameConfig.RealData) {
		panic("real data index out of range: " + strconv.Itoa(realIndex))
	}
	realData := s.gameConfig.RealData[realIndex]

	var symbols [_colCount]SymbolRoller
	for col := 0; col < _colCount; col++ {
		reel := realData[col]
		dataLen := len(reel)
		if dataLen == 0 {
			panic("real data column is empty")
		}

		needRows := make([]int, 0, _rowCount)
		roller := SymbolRoller{Real: realIndex, Col: col, Len: dataLen}

		for row := _rowCount - 1; row >= 0; row-- {
			if s.scene.ScatterLock[row][col] != 0 {
				roller.BoardSymbol[row] = _treasure
			} else {
				roller.BoardSymbol[row] = 0
				needRows = append(needRows, row)
			}
			if row == 0 {
				break
			}
		}

		x := len(needRows)
		if x > 0 {
			start := rand.IntN(dataLen)
			roller.Start = start
			roller.Fall = (start + x - 1) % dataLen
			for i := 0; i < x; i++ {
				roller.BoardSymbol[needRows[i]] = reel[(start+i)%dataLen]
			}
		}

		symbols[col] = roller
	}

	return symbols
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
// 规则：3个夺宝触发10次免费，每多1个夺宝增加2次免费 -> 10 + (scatterCount-3)*2
func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if scatterCount < s.gameConfig.FreeGameScatterMin {
		return 0
	}
	return s.gameConfig.FreeGameTimes
}
