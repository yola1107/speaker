package sgz

import (
	"fmt"
	"math/rand/v2"
	"strconv"

	"google.golang.org/protobuf/proto"

	"egame-grpc/game/common"
	"egame-grpc/game/common/pb"
	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type gameConfigJson struct {
	PayTable                   [][]int64    `json:"pay_table"`                       // 赔付表
	Lines                      [][]int64    `json:"lines"`                           // 中奖线定义（20条支付线）
	FreeGameScatterMin         int64        `json:"free_game_scatter_min"`           // 触发免费游戏最小夺宝符号数
	FreeGameTimes              int64        `json:"free_game_times"`                 // 免费游戏基础次数
	FreeGameAddTimesPerScatter int64        `json:"free_game_add_times_per_scatter"` // 免费游戏每个额外夺宝符号增加次数
	FreeGameMultiLiuBei        []int64      `json:"free_game_multi_liubei"`          // 免费游戏连续消除倍数配置 刘备(1)
	FreeGameMultiZhangFei      []int64      `json:"free_game_multi_zhangfei"`        // 免费游戏连续消除倍数配置 张飞(2)
	FreeGameMultiGuanYu        []int64      `json:"free_game_multi_guanyu"`          // 免费游戏连续消除倍数配置 关羽(3)
	FreeGameMultiZhaoYun       []int64      `json:"free_game_multi_zhaoyun"`         // 免费游戏连续消除倍数配置 赵云(4)
	FreeGameMultiZhuGeLiang    []int64      `json:"free_game_multi_zhugeliang"`      // 免费游戏连续消除倍数配置 诸葛亮(5)
	FreeGameMultiHuangZhong    []int64      `json:"free_game_multi_huangzhong"`      // 免费游戏连续消除倍数配置 黄忠(6)
	FreeGameMultiMaChao        []int64      `json:"free_game_multi_machao"`          // 免费游戏连续消除倍数配置 马超(7)
	FreeGameMultiKing          []int64      `json:"free_game_multi_king"`            // 免费游戏连续消除倍数配置 君主(8)
	RollCfg                    RollConf     `json:"roll_cfg"`                        // 滚轴配置
	RealData                   []Reals      `json:"real_data"`                       // 真实数据
	KingdomMapArray            []KingdomMap `json:"kingdom_map_array"`               // 三国志游戏王国地图数组

	FreeMultipleMap [][]int64                `json:"-"` // 免费游戏倍数配置映射（英雄ID → Multiples），解析后自动填充
	KingdomMap      map[int64]*pb.KingdomMap `json:"-"` // 王国地图配置映射（英雄ID → KingdomMap），解析后自动填充
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

type KingdomMap struct {
	CityOwnerIndexArray    [16]int32 `json:"city_owner_index_array"`    // 16个城镇主公的索引
	CurrentCityIndex       int32     `json:"current_city_index"`        // 当前城镇的索引
	CurrentCommanderIndex  int32     `json:"current_commander_index"`   // 当前武将的索引
	CurrentCityForce       int32     `json:"current_city_force"`        // 当前城镇的战斗力值
	NextCityForce          int32     `json:"next_city_force"`           // 下一个城镇的战斗力值
	NextCommanderIndex     int32     `json:"next_commander_index"`      // 下一个武将索引
	NextCommanderCityIndex int32     `json:"next_commander_city_index"` // 下一个武将的城镇的索引
}

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

	s.gameConfig.KingdomMap = map[int64]*pb.KingdomMap{}
	for i, v := range s.gameConfig.KingdomMapArray {
		if _, exist := s.gameConfig.KingdomMap[int64(v.CurrentCommanderIndex)]; exist {
			panic(fmt.Sprintf("kingdom map config error, index: %d", i))
		}
		s.gameConfig.KingdomMap[int64(v.CurrentCommanderIndex)] = &pb.KingdomMap{
			CityOwnerIndexArray:    v.CityOwnerIndexArray[:],
			CurrentCityIndex:       proto.Int32(v.CurrentCityIndex),
			CurrentCommanderIndex:  proto.Int32(v.CurrentCommanderIndex),
			CurrentCityForce:       proto.Int32(v.CurrentCityForce),
			NextCityForce:          proto.Int32(v.NextCityForce),
			NextCommanderIndex:     proto.Int32(v.NextCommanderIndex),
			NextCommanderCityIndex: proto.Int32(v.NextCommanderCityIndex),
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
	if !s.isFreeRound {
		return s.getSceneSymbol(s.gameConfig.RollCfg.Base)
	}

	var rollCfg RollCfgType
	switch s.scene.FreeHeroID {
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
		panic(fmt.Sprintf("invalid HeroID: %d (must be 1-8) when isFreeRound=true", s.scene.FreeHeroID))
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
		dataLen := len(data)
		if dataLen == 0 {
			panic("real data column is empty")
		}

		start := rand.IntN(dataLen)
		end := (start + _rowCount - 1) % dataLen
		roller := SymbolRoller{Real: realIndex, Start: start, Fall: end, Col: col, Len: dataLen}

		for row := 0; row < _rowCount; row++ {
			symbol := data[(start+row)%dataLen]
			roller.BoardSymbol[int(_rowCount)-1-row] = symbol
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

func (s *betOrderService) getStreakMultiplier() (int64, []int64, int64) {
	if !s.isFreeRound {
		return 1, []int64{}, 0
	}
	if s.scene.FreeHeroID < _heroID1 || s.scene.FreeHeroID > _heroID8 {
		global.GVA_LOG.Error("getStreakMultiplier: HeroID must in [1,8]", zap.Any("scene", s.scene.FreeHeroID))
		return 1, []int64{}, 0
	}
	multiples := s.gameConfig.FreeMultipleMap[s.scene.FreeHeroID]
	//index := min(s.scene.ContinueNum, int64(len(multiples)-1))
	index := s.scene.ContinueNum
	if index < 0 {
		index = 0
	}
	if index >= int64(len(multiples)) {
		index = int64(len(multiples)) - 1
	}
	return multiples[index], multiples, index
}

// calcNewFreeGameNum 计算触发免费游戏的次数
// 规则：3个夺宝触发10次免费，每多1个夺宝增加2次免费 -> 10 + (scatterCount-3)*2
func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if scatterCount < s.gameConfig.FreeGameScatterMin {
		return 0
	}
	return s.gameConfig.FreeGameTimes + (scatterCount-s.gameConfig.FreeGameScatterMin)*s.gameConfig.FreeGameAddTimesPerScatter
}

// GetKingdomMap 根据英雄ID获取对应的地图数据
func (s *betOrderService) GetKingdomMap(heroID int64) *pb.KingdomMap {
	if heroID < _heroID1 {
		heroID = _heroID1
	}
	v, ok := s.gameConfig.KingdomMap[heroID]
	if !ok {
		global.GVA_LOG.Error("GetKingdomMap", zap.Any("can not find heroID config", heroID))
		return &pb.KingdomMap{
			CityOwnerIndexArray:    []int32{},
			CurrentCityIndex:       proto.Int32(0),
			CurrentCommanderIndex:  proto.Int32(0),
			CurrentCityForce:       proto.Int32(0),
			NextCityForce:          proto.Int32(0),
			NextCommanderIndex:     proto.Int32(0),
			NextCommanderCityIndex: proto.Int32(0),
		}
	}
	return v
}
