package sjnws3

import (
	"maps"
	"slices"

	"egame-grpc/global"
	"egame-grpc/utils/json"

	"go.uber.org/zap"
)

type betFreeGame struct {
	Multi    []int `json:"multi"`     // 连续消除倍数
	Times    int   `json:"times"`     // 基础次数
	AddTimes int   `json:"add_times"` // 每个额外夺宝符号增加次数
}

type rollConfig struct {
	UseKey []int `json:"use_key"` // 滚轴数据索引
	Weight []int `json:"weight"`  // 权重
}

type gameConfigJson struct {
	PayTable                    [][]int   `json:"pay_table"`                        // 赔付表
	BetSize                     []float64 `json:"bet_size"`                         // 下注金额列表
	BetLevel                    []int     `json:"bet_level"`                        // 下注等级列表
	LinesNum                    int       `json:"linesNum"`                         // 中奖线数量
	Lines                       [][]int   `json:"lines"`                            // 中奖线定义
	BaseStreakMulti             []int     `json:"base_streak_multi"`                // 普通游戏连续消除倍数
	FreeGameScatterMin          int       `json:"free_game_scatter_min"`            // 触发免费游戏最小夺宝符号数
	FreeGame1Multi              []int     `json:"free_game1_multi"`                 // 免费游戏1连续消除倍数
	FreeGame1Times              int       `json:"free_game1_times"`                 // 免费游戏1基础次数
	FreeGame1AddTimesPerScatter int       `json:"free_game1_add_times_per_scatter"` // 免费游戏1每个额外夺宝符号增加次数
	FreeGame2Multi              []int     `json:"free_game2_multi"`                 // 免费游戏2连续消除倍数
	FreeGame2Times              int       `json:"free_game2_times"`                 // 免费游戏2基础次数
	FreeGame2AddTimesPerScatter int       `json:"free_game2_add_times_per_scatter"` // 免费游戏2每个额外夺宝符号增加次数
	FreeGame3Multi              []int     `json:"free_game3_multi"`                 // 免费游戏3连续消除倍数
	FreeGame3Times              int       `json:"free_game3_times"`                 // 免费游戏3基础次数
	FreeGame3AddTimesPerScatter int       `json:"free_game3_add_times_per_scatter"` // 免费游戏3每个额外夺宝符号增加次数
	RollCfg                     struct {
		Base     rollConfig `json:"base"`      // 普通游戏滚轴配置
		Free1    rollConfig `json:"free1"`     // 免费游戏1滚轴配置
		Free2    rollConfig `json:"free2"`     // 免费游戏2滚轴配置
		Free3    rollConfig `json:"free3"`     // 免费游戏3滚轴配置
		RealData [][][]int  `json:"real_data"` // 滚轴符号数据
	} `json:"roll_cfg"`
}

func newBetOrderService() *betOrderService {
	s := &betOrderService{grid: int64GridY{}, winGrid: int64GridY{},
		winCards: HisGridY{}, bonusMap: make(map[int]*betFreeGame),
		ColInfoMap: make(map[int]*ColInfo), midgrid: int64GridY{}, winDetails: [20]int{},
		nextGrid: int64GridY{}, winCol: make([]int, 0), colList: make(map[int][]int, 0),
		winResult: make([]*winResult, 0), BonusState: 2, bonusLine: make([][]*Pos, 0),
		symbolMulMap: make(map[int]map[int]int), symbolList: slices.Clone(symbolList),
		symbolCol: make(map[string]*Pos)}
	s.selectGameRedis()
	s.loadConfig()
	s.loadSymbolCol()
	s.SetFreeBonusCfg()
	return s
}

func (s *betOrderService) loadSymbolCol() {
	for i := 0; i < _colCount; i++ {
		StrList := mapColList[i]
		for _, str := range StrList {
			pos, _ := StringToPos(str)
			s.symbolCol[str] = pos
		}
	}
	s.SymbolColMap = maps.Clone(s.symbolCol)
}

func (s *betOrderService) loadConfig() {
	config, err := loadConfigs()
	if err != nil {
		global.GVA_LOG.Error("loadConfig", zap.Error(err))
		return
	}
	s.cfg = config
	for i, Symbol := range s.symbolList {
		MulList := config.PayTable[i]
		s.SetSymbolMul(Symbol, threeLine, key1, MulList)
		s.SetSymbolMul(Symbol, fourLine, key2, MulList)
		s.SetSymbolMul(Symbol, fiveLine, key3, MulList)
	}
	for _, line := range s.cfg.Lines {
		newLine := make([]*Pos, 0)
		for _, Value := range line {
			x, y := GetXYByValue(Value)
			newPos := &Pos{x, y}
			newLine = append(newLine, newPos)
		}
		s.bonusLine = append(s.bonusLine, newLine)
	}
}

func (s *betOrderService) SetSymbolMul(Symbol, mapKey, key int, MulList []int) {
	if _, ok := s.symbolMulMap[Symbol]; ok {
		s.symbolMulMap[Symbol][mapKey] = MulList[key]
	} else {
		s.symbolMulMap[Symbol] = map[int]int{mapKey: MulList[key]}
	}
}

func (s *betOrderService) SetFreeBonusCfg() {
	Fr := s.cfg
	s.bonusMap[_bonusNum1] = &betFreeGame{
		Multi:    Fr.FreeGame1Multi,
		Times:    Fr.FreeGame1Times,
		AddTimes: Fr.FreeGame1AddTimesPerScatter,
	}

	s.bonusMap[_bonusNum2] = &betFreeGame{
		Multi:    Fr.FreeGame2Multi,
		Times:    Fr.FreeGame2Times,
		AddTimes: Fr.FreeGame2AddTimesPerScatter,
	}
	s.bonusMap[_bonusNum3] = &betFreeGame{
		Multi:    Fr.FreeGame3Multi,
		Times:    Fr.FreeGame3Times,
		AddTimes: Fr.FreeGame3AddTimesPerScatter,
	}
}

// ==================== 配置加载 ====================

func loadConfigs() (*gameConfigJson, error) {
	Config := &gameConfigJson{}
	err := json.CJSON.UnmarshalFromString(_gameJsonConfigsRaw, Config)
	if err != nil {
		global.GVA_LOG.Error("loadConfigs", zap.Any("err", err.Error()))
		return nil, err
	}
	return Config, nil
}
