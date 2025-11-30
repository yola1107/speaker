package mahjong

import (
	"fmt"
	"math/rand/v2"
	"strconv"

	jsoniter "github.com/json-iterator/go"
)

type gameConfigJson struct {
	PayTable                    [][]int64         `json:"pay_table"`                        // 赔付表
	WinLines                    [][]int64         `json:"lines"`                            // 中奖线定义（20条支付线）
	RollCfg                     RollConf          `json:"roll_cfg"`                         // 滚轴配置
	RealData                    []Reals           `json:"real_data"`                        // 真实数据
	BaseStreakMulti             []int             `json:"base_streak_multi"`                // 普通游戏连续消除倍数
	FreeGameScatterMin          int               `json:"free_game_scatter_min"`            // 触发免费游戏最小夺宝符号数
	FreeGame1Multi              []int             `json:"free_game1_multi"`                 // 免费游戏1连续消除倍数
	FreeGame1Times              int               `json:"free_game1_times"`                 // 免费游戏1基础次数
	FreeGame1AddTimesPerScatter int               `json:"free_game1_add_times_per_scatter"` // 免费游戏1每个额外夺宝符号增加次数
	FreeGame2Multi              []int             `json:"free_game2_multi"`                 // 免费游戏2连续消除倍数
	FreeGame2Times              int               `json:"free_game2_times"`                 // 免费游戏2基础次数
	FreeGame2AddTimesPerScatter int               `json:"free_game2_add_times_per_scatter"` // 免费游戏2每个额外夺宝符号增加次数
	FreeGame3Multi              []int             `json:"free_game3_multi"`                 // 免费游戏3连续消除倍数
	FreeGame3Times              int               `json:"free_game3_times"`                 // 免费游戏3基础次数
	FreeGame3AddTimesPerScatter int               `json:"free_game3_add_times_per_scatter"` // 免费游戏3每个额外夺宝符号增加次数
	FreeBonusMap                map[int]BonusItem `json:"-"`                                // 免费游戏配置映射（1/2/3 → BonusItem），解析后自动填充
}

type RollConf struct {
	Base  RollCfgType `json:"base"`  // 普通游戏滚轴配置
	Free1 RollCfgType `json:"free1"` // 免费游戏1滚轴配置
	Free2 RollCfgType `json:"free2"` // 免费游戏2滚轴配置
	Free3 RollCfgType `json:"free3"` // 免费游戏3滚轴配置
}

type RollCfgType struct {
	UseKey []int `json:"use_key"` // 滚轴数据索引
	Weight []int `json:"weight"`  // 权重
	WTotal int   `json:"-"`       // 总权重（计算得出）
}

type Reals [][]int64

type SymbolRoller struct {
	Real        int              `json:"real"`  // 选择的第几个轮盘
	Start       int              `json:"start"` // 开始索引
	Fall        int              `json:"fall"`  // 开始索引
	Col         int              `json:"col"`   // 第几列
	BoardSymbol [_rowCount]int64 `json:"board"` // 盘面符号
}

type BonusItem struct {
	Multi    []int `json:"multi"`     // 连续消除倍数
	Times    int   `json:"times"`     // 基础次数
	AddTimes int   `json:"add_times"` // 每个额外夺宝符号增加次数
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
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free1)
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free2)
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free3)

	// 初始化 FreeBonusMap（优化配置访问）
	s.gameConfig.FreeBonusMap = map[int]BonusItem{
		_bonusNum1: {
			Multi:    s.gameConfig.FreeGame1Multi,
			Times:    s.gameConfig.FreeGame1Times,
			AddTimes: s.gameConfig.FreeGame1AddTimesPerScatter,
		},
		_bonusNum2: {
			Multi:    s.gameConfig.FreeGame2Multi,
			Times:    s.gameConfig.FreeGame2Times,
			AddTimes: s.gameConfig.FreeGame2AddTimesPerScatter,
		},
		_bonusNum3: {
			Multi:    s.gameConfig.FreeGame3Multi,
			Times:    s.gameConfig.FreeGame3Times,
			AddTimes: s.gameConfig.FreeGame3AddTimesPerScatter,
		},
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
	var rollCfg RollCfgType
	if s.isFreeRound {
		switch s.scene.BonusNum {
		case _bonusNum1:
			rollCfg = s.gameConfig.RollCfg.Free1
		case _bonusNum2:
			rollCfg = s.gameConfig.RollCfg.Free2
		case _bonusNum3:
			rollCfg = s.gameConfig.RollCfg.Free3
		default:
			panic(fmt.Sprintf("invalid BonusNum: %d (must be 1, 2, or 3) when isFreeRound=true", s.scene.BonusNum))
		}
	} else {
		rollCfg = s.gameConfig.RollCfg.Base
	}
	return s.getSceneSymbol(rollCfg)
}

// getSceneSymbol 生成符号滚轴
// BoardSymbol 从下往上存储：索引0=最下面，索引3=最上面
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

	var symbols [_colCount]SymbolRoller
	for col := 0; col < _colCount; col++ {
		data := realData[col]
		if len(data) == 0 {
			panic("real data column is empty")
		}

		start := rand.IntN(len(data))
		end := (start + _rowCount - 1) % len(data)
		roller := SymbolRoller{Real: realIndex, Start: start, Fall: end, Col: col}

		// 填充符号：从下往上存储（索引0=最下面）
		dataIndex := 0
		for row := 0; row < _rowCount; row++ {
			symbol := data[(start+dataIndex)%len(data)]
			// BoardSymbol 从下往上存储：索引0=最下面，索引3=最上面
			roller.BoardSymbol[int(_rowCount)-1-row] = symbol
			dataIndex++
		}
		symbols[col] = roller
	}

	return symbols
}

// ringSymbol 补充掉下来导致的空缺位置
// 注意：BoardSymbol 从下往上存储（索引0=最下面，索引3=最上面）
// 填充时应该从最上面（索引3）开始，按照索引从高到低（3→2→1→0）的顺序填充
func (rs *SymbolRoller) ringSymbol(gameConfig *gameConfigJson) {
	var newBoard [_rowCount]int64
	// 先复制非空白位置（保持原位置）
	for i, s := range rs.BoardSymbol {
		if s != 0 {
			newBoard[i] = s
		}
	}
	// 从高索引到低索引（3→2→1→0）遍历，遇到空白就填充
	for i := int(_rowCount) - 1; i >= 0; i-- {
		if newBoard[i] == 0 {
			newBoard[i] = rs.getFallSymbol(gameConfig)
		}
	}
	rs.BoardSymbol = newBoard
}

// getFallSymbol 从滚轴获取下一个符号：Fall 指向当前最后一个符号，下一个是 (Fall+1) % len
func (rs *SymbolRoller) getFallSymbol(gameConfig *gameConfigJson) int64 {
	data := gameConfig.RealData[rs.Real][rs.Col]
	rs.Fall = (rs.Fall + 1) % len(data)
	return data[rs.Fall]
}

// 读取符号的赔率
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
