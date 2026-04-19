package sjxj

import (
	"fmt"

	"egame-grpc/game/common"
	"egame-grpc/game/common/rand"

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
		if cacheText, _ := common.GetRedisGameJson(GameID); len(cacheText) > 0 {
			raw = cacheText
		}
	}
	s.gameConfig = &gameConfigJson{}
	if err := jsoniter.UnmarshalFromString(raw, s.gameConfig); err != nil {
		panic(err)
	}

	if s.gameConfig.FreeGameTimes <= 0 {
		panic("s.gameConfig.FreeGameTimes <= 0")
	}
	if s.gameConfig.FreeUnlockResetSpins <= 0 {
		panic("s.gameConfig.FreeUnlockResetSpins <= 0")
	}
	if len(s.gameConfig.FreeUnlockThresholds) < _rowCount {
		panic(fmt.Sprintf("len(FreeUnlockThresholds) < %d", _rowCount))
	}
	if len(s.gameConfig.FreeScatterMulByRow) < _rowCount {
		panic(fmt.Sprintf("len(FreeScatterMulByRow) < %d", _rowCount))
	}
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

func (s *betOrderService) getSceneSymbolBase() [_colCount]SymbolRoller {
	realIndex := 0
	realData := s.gameConfig.RealData[realIndex]
	var symbols [_colCount]SymbolRoller

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

func (s *betOrderService) getSceneSymbolFree() [_colCount]SymbolRoller {
	realIndex := 1
	realData := s.gameConfig.RealData[realIndex]
	var symbols [_colCount]SymbolRoller

	for c := 0; c < _colCount; c++ {
		reel := realData[c]
		dataLen := len(reel)

		needRows := make([]int, 0, _rowCount)
		roller := SymbolRoller{Real: realIndex, Col: c, Len: dataLen}

		for r := _rowCount - 1; r >= 0; r-- {
			if s.scene.ScatterLock[r][c] != 0 {
				roller.BoardSymbol[r] = _treasure
			} else {
				needRows = append(needRows, r)
			}
		}

		if x := len(needRows); x > 0 {
			start := rand.IntN(dataLen)
			roller.Start = start
			roller.Fall = (start + x - 1) % dataLen
			for i := 0; i < x; i++ {
				roller.BoardSymbol[needRows[i]] = reel[(start+i)%dataLen]
			}
		}
		symbols[c] = roller
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

func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if scatterCount < s.gameConfig.FreeGameScatterMin {
		return 0
	}
	return s.gameConfig.FreeGameTimes
}
