package sgz

import (
	"fmt"
	"math/rand/v2"
	"strconv"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type gameConfigJson struct {
	PayTable                   [][]int64 `json:"pay_table"`                       // 赔付表
	LinesNum                   int       `json:"linesNum"`                        // 中奖线数量
	Lines                      [][]int64 `json:"lines"`                           // 中奖线定义（20条支付线）
	FreeGameScatterMin         int64     `json:"free_game_scatter_min"`           // 触发免费游戏最小夺宝符号数
	FreeGameTimes              int64     `json:"free_game_times"`                 // 免费游戏基础次数
	FreeGameAddTimesPerScatter int64     `json:"free_game_add_times_per_scatter"` // 免费游戏每个额外夺宝符号增加次数
	FreeGameMultiLiuBei        []int64   `json:"free_game_multi_liubei"`          // 免费游戏连续消除倍数配置 刘备(1)
	FreeGameMultiZhangFei      []int64   `json:"free_game_multi_zhangfei"`        // 免费游戏连续消除倍数配置 张飞(2)
	FreeGameMultiGuanYu        []int64   `json:"free_game_multi_guanyu"`          // 免费游戏连续消除倍数配置 关羽(3)
	FreeGameMultiZhaoYun       []int64   `json:"free_game_multi_zhaoyun"`         // 免费游戏连续消除倍数配置 赵云(4)
	FreeGameMultiZhuGeLiang    []int64   `json:"free_game_multi_zhugeliang"`      // 免费游戏连续消除倍数配置 诸葛亮(5)
	FreeGameMultiHuangZhong    []int64   `json:"free_game_multi_huangzhong"`      // 免费游戏连续消除倍数配置 黄忠(6)
	FreeGameMultiMaChao        []int64   `json:"free_game_multi_machao"`          // 免费游戏连续消除倍数配置 马超(7)
	FreeGameMultiKing          []int64   `json:"free_game_multi_king"`            // 免费游戏连续消除倍数配置 君主(8)
	CityUnlockHero             []int64   `json:"city_unlock_hero"`                // 城市解锁英雄
	CityChangeNumber           []int64   `json:"city_change_number"`              // 城市变更数字
	KingData                   [][]int   `json:"king_data"`                       // 君主数据
	RollCfg                    RollConf  `json:"roll_cfg"`                        // 滚轴配置
	RealData                   []Reals   `json:"real_data"`                       // 真实数据

	FreeMultipleMap [][]int64 `json:"-"` // 免费游戏倍数配置映射（英雄ID → Multiples），解析后自动填充
}

type RollConf struct {
	Base  RollCfgType `json:"base"`  // 普通游戏滚轴配置
	Free1 RollCfgType `json:"free1"` // 免费游戏滚轴配置 刘备(1)
	Free2 RollCfgType `json:"free2"` // 免费游戏滚轴配置 张飞(2)
	Free3 RollCfgType `json:"free3"` // 免费游戏滚轴配置 关羽(3)
	Free4 RollCfgType `json:"free4"` // 免费游戏滚轴配置 赵云(4)
	Free5 RollCfgType `json:"free5"` // 免费游戏滚轴配置 诸葛亮(5)
	Free6 RollCfgType `json:"free6"` // 免费游戏滚轴配置 黄忠(6)
	Free7 RollCfgType `json:"free7"` // 免费游戏滚轴配置 马超(7)
	Free8 RollCfgType `json:"free8"` // 免费游戏滚轴配置 君主(8)
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
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free4)
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free5)
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free6)
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free7)
	s.calculateRollWeight(&s.gameConfig.RollCfg.Free8)

	s.gameConfig.FreeMultipleMap = [][]int64{
		_heroID1: s.gameConfig.FreeGameMultiLiuBei,
		_heroID2: s.gameConfig.FreeGameMultiZhangFei,
		_heroID3: s.gameConfig.FreeGameMultiGuanYu,
		_heroID4: s.gameConfig.FreeGameMultiZhaoYun,
		_heroID5: s.gameConfig.FreeGameMultiZhuGeLiang,
		_heroID6: s.gameConfig.FreeGameMultiHuangZhong,
		_heroID7: s.gameConfig.FreeGameMultiMaChao,
		_heroID8: s.gameConfig.FreeGameMultiKing,
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
		switch s.scene.HeroID {
		case _heroID1:
			rollCfg = s.gameConfig.RollCfg.Free1
		case _heroID2:
			rollCfg = s.gameConfig.RollCfg.Free2
		case _heroID3:
			rollCfg = s.gameConfig.RollCfg.Free3
		case _heroID4:
			rollCfg = s.gameConfig.RollCfg.Free4
		case _heroID5:
			rollCfg = s.gameConfig.RollCfg.Free5
		case _heroID6:
			rollCfg = s.gameConfig.RollCfg.Free6
		case _heroID7:
			rollCfg = s.gameConfig.RollCfg.Free7
		case _heroID8:
			rollCfg = s.gameConfig.RollCfg.Free8
		default:
			panic(fmt.Sprintf("invalid HeroID: %d (must be 1-8) when isFreeRound=true", s.scene.HeroID))
		}
	} else {
		rollCfg = s.gameConfig.RollCfg.Base
	}
	return s.getSceneSymbol(rollCfg)
}

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

		dataIndex := 0
		for row := 0; row < _rowCount; row++ {
			symbol := data[(start+dataIndex)%len(data)]
			roller.BoardSymbol[int(_rowCount)-1-row] = symbol
			dataIndex++
		}
		symbols[col] = roller
	}

	return symbols
}

func (rs *SymbolRoller) ringSymbol(gameConfig *gameConfigJson) {
	var newBoard [_rowCount]int64
	for i, s := range rs.BoardSymbol {
		if s != 0 {
			newBoard[i] = s
		}
	}
	for i := int(_rowCount) - 1; i >= 0; i-- {
		if newBoard[i] == 0 {
			newBoard[i] = rs.getFallSymbol(gameConfig)
		}
	}
	rs.BoardSymbol = newBoard
}

func (rs *SymbolRoller) getFallSymbol(gameConfig *gameConfigJson) int64 {
	data := gameConfig.RealData[rs.Real][rs.Col]
	rs.Fall = (rs.Fall + 1) % len(data)
	return data[rs.Fall]
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

func (s *betOrderService) getStreakMultiplier() int64 {
	if !s.isFreeRound {
		return 1
	}
	if s.scene.HeroID < 1 && s.scene.HeroID >= int64(len(s.gameConfig.FreeMultipleMap)) { // 1~8
		global.GVA_LOG.Error("getStreakMultiplier: HeroID not found", zap.Any("scene", s.scene.HeroID))
		return 1
	}
	multiples := s.gameConfig.FreeMultipleMap[s.scene.HeroID]
	index := min(s.scene.ContinueNum, int64(len(multiples)-1))
	return multiples[index]
}

// calcNewFreeGameNum 计算触发免费游戏的次数
func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if scatterCount < s.gameConfig.FreeGameScatterMin {
		return 0
	}
	// 规则：3个夺宝触发10次免费，每多1个夺宝增加2次免费
	//return 10 + (scatterCount-3)*2
	return s.gameConfig.FreeGameTimes + (scatterCount-s.gameConfig.FreeGameScatterMin)*s.gameConfig.FreeGameAddTimesPerScatter
}

// cityUnlockHero 根据城市变更数字查找对应的解锁英雄ID
// cNumber 是累计值，通过 getCityMapIndex 找到对应下标，再从 city_unlock_hero 获取英雄ID
// 示例：
// cNumber=50,  city_change_number=[0,100,200,...] -> 返回 0 (对应区间 [0,100))
// cNumber=150, city_change_number=[0,100,200,...] -> 返回 1 (对应区间 [100,200))
func (s *betOrderService) cityUnlockHero(cNumber int64) (int, int64) {
	//"city_unlock_hero":  [1, 2,  3,  4,  0,  0,  5,  0,  0,  6,  0,   0,   7,   0,   0,   8],
	//"city_change_number":[0,100,200,300,400,500,600,700,800,900,1000,1100,1200,1300,1400,1500],
	cityNumbers := s.gameConfig.CityChangeNumber
	unlockHeroes := s.gameConfig.CityUnlockHero

	// 找到第一个 >= cNumber 的位置，然后取前一个
	idx := 0
	for idx < len(cityNumbers) && cNumber >= cityNumbers[idx] {
		idx++
	}
	idx-- // 回退到正确的区间

	if idx >= 0 && idx < len(unlockHeroes) {
		return idx, unlockHeroes[idx]
	}
	return -1, -1
}
