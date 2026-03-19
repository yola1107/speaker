package sjxj

import (
	"math/rand/v2"

	"egame-grpc/game/common"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable             [][]int64 `json:"pay_table"`                      // 赔付表
	Lines                [][]int64 `json:"lines"`                          // 中奖线定义
	FreeGameScatterMin   int64     `json:"free_game_scatter_min"`          // 触发免费最少 Scatter（未中线奖时）
	FreeGameTimes        int64     `json:"free_game_times"`                // 免费游戏基础次数
	FreeUnlockThresholds []int64   `json:"free_unlock_thresholds"`         // 解锁阈值（按 UnlockedRows 索引；推荐长度8）
	FreeUnlockResetSpins int64     `json:"free_unlock_reset_spins"`        // 免费游戏解锁新行，重置免费次数
	FreeScatterMulByRow  [][]int64 `json:"free_scatter_multiplier_by_row"` // 夺宝随机倍数范围（按“初始4行、第5~8行”共5列配置）
	RealData             []Reals   `json:"real_data"`                      // 真实数据
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

	if s.gameConfig.FreeGameTimes <= 0 {
		panic("s.gameConfig.FreeGameTimes <= 0 ")
	}
	if s.gameConfig.FreeUnlockResetSpins <= 0 {
		panic("s.gameConfig.FreeUnlockResetSpins <= 0 ")
	}
	if len(s.gameConfig.FreeUnlockThresholds) != _rowCount {
		panic("len(s.gameConfig.FreeUnlockThresholds) != 8")
	}
	if len(s.gameConfig.FreeScatterMulByRow) < 8 {
		panic("len(s.gameConfig.FreeScatterMulByRow) < 8")
	}
	// 固定索引：base 用 real_data[0]，free 用 real_data[1]
	if len(s.gameConfig.RealData) < 2 {
		panic("len(s.gameConfig.RealData) < 2")
	}
}

func (s *betOrderService) initSpinSymbol() [_colCount]SymbolRoller {
	if s.isFreeRound {
		return s.getSceneSymbolFree()
	}
	return s.getSceneSymbolBase()
}

// 基础模式：纯随机填充（8×5 权威盘面）
func (s *betOrderService) getSceneSymbolBase() [_colCount]SymbolRoller {
	realIndex := 0
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
func (s *betOrderService) getSceneSymbolFree() [_colCount]SymbolRoller {
	realIndex := 1
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
// 规则：夺宝数 >= free_game_scatter_min 时触发，免费次数 = free_game_times
func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if scatterCount < s.gameConfig.FreeGameScatterMin {
		return 0
	}
	return s.gameConfig.FreeGameTimes
}
