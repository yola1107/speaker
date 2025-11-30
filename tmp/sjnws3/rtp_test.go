package sjnws3

import (
	"fmt"
	"slices"

	"egame-grpc/game/common"
	"egame-grpc/global"
	"egame-grpc/model/game/request"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

const (
	MaxNum   = 100000 // RTP测试最大次数
	FreeType = 1      // 免费类型
)

type rtpInfo struct {
	GameNum            int     `json:"game_num"`               // 游戏局数
	Rtp                float64 `json:"rtp"`                    // 游戏rtp
	Bonus              float64 `json:"bonus"`                  // 游戏总奖金
	FreeType           int     `json:"free_type"`              //免费模式
	BaseGameNoBonusNum int     `json:"base_game_no_bonus_num"` //普通模式没有中奖次数
	BaseGameBonusNum   int     `json:"base_game_bonus_num"`    //普通模式有中奖次数
	BaseGameBonus      float64 `json:"base_game_bonus"`        //普通模式总中奖金额
	BaseGameRpt        float64 `json:"base_game_rpt"`          //普通模式rtp
	GetFreeNum         int     `json:"get_free_num"`           //进入免费的次数
	FreeGameBonus      float64 `json:"free_game_bonus"`        //免费模式总奖金
	BetAmount          float64 `json:"bet_amount"`
}

type Rpt struct {
	s        *betOrderService
	scene    Scene
	Ballance float64
	rtpInfo  rtpInfo
}

func RtpService(req *request.BetOrderReq) map[string]any {
	global.GVA_LOG.Debug("开始rtp")
	r := NewRtp(FreeType)
	r.RunRtp(req)
	return r.Debug()
}

func (r *Rpt) RunRtp(req *request.BetOrderReq) {
	//global.GVA_LOG.Debug("实际游戏次数:")
	//var num int
	for {
		//num++
		//fmt.Printf("\r完成:%d局游戏", num)
		//if num > 200000 {
		//	return
		//}
		s := newBetOrderServiceRTP()
		s.req = req
		s.scene = r.scene
		r.s = s
		baseRet, _ := r.s.RtpOrder()
		/*
			处理免费
		*/
		if r.s.scene.BonusState == _freeGame {
			r.rtpInfo.GetFreeNum++
			r.s.scene.BonusState = _normalGame
			r.s.scene.BonusNum = r.rtpInfo.FreeType
			value, _ := r.s.bonusMap[r.s.scene.BonusNum]
			r.s.scene.FreeTimes += value.Times + (r.s.ScatterNum-_minScatterNum)*value.AddTimes
			r.s.scene.MaxFreeTimes += value.Times + (r.s.ScatterNum-_minScatterNum)*value.AddTimes
		}
		/*
			处理普通
		*/
		if baseRet.CurrentWin > 0 {
			r.rtpInfo.Bonus = decimal.NewFromFloat(baseRet.CurrentWin).
				Add(decimal.NewFromFloat(r.rtpInfo.Bonus)).
				Round(2).InexactFloat64()
		}

		if r.s.IsFreeSpin && baseRet.CurrentWin > 0 {
			r.rtpInfo.FreeGameBonus = decimal.NewFromFloat(baseRet.CurrentWin).
				Add(decimal.NewFromFloat(r.rtpInfo.FreeGameBonus)).
				Round(2).InexactFloat64()
			//global.GVA_LOG.Debug("免费游戏",
			//	zap.Any("新增奖金:", baseRet.CurrentWin),
			//	zap.Any("累计免费奖金", r.rtpInfo.FreeGameBonus))
		}
		if !r.s.IsFreeSpin {
			if baseRet.CurrentWin == 0 {
				r.rtpInfo.BaseGameNoBonusNum++
			} else {
				r.rtpInfo.BaseGameBonusNum++
			}
			if baseRet.CurrentWin > 0 {
				r.rtpInfo.BaseGameBonus = decimal.NewFromFloat(baseRet.CurrentWin).
					Add(decimal.NewFromFloat(r.rtpInfo.BaseGameBonus)).
					Round(2).InexactFloat64()
			}
		}
		if r.s.amount.GreaterThan(decimal.Zero) {
			r.rtpInfo.GameNum++
			fmt.Printf("\r完成:%d局投注游戏,"+
				"---总奖金:%f,---普通游戏奖金:%f,"+
				"---免费游戏奖金:%f,触发免费的次数:%d", r.rtpInfo.GameNum,
				r.rtpInfo.Bonus, r.rtpInfo.BaseGameBonus,
				r.rtpInfo.FreeGameBonus, r.rtpInfo.GetFreeNum)
			r.rtpInfo.BetAmount = decimal.NewFromFloat(r.rtpInfo.BetAmount).
				Add(decimal.NewFromFloat(r.s.amount.Round(2).InexactFloat64())).
				Round(2).InexactFloat64()
		}
		r.scene = r.s.scene
		//fmt.Printf("\r完成:%d局投注游戏,", r.rtpInfo.GameNum)
		if r.rtpInfo.GameNum >= MaxNum {
			r.rtpInfo.Rtp = decimal.NewFromFloat(r.rtpInfo.Bonus).
				Div(decimal.NewFromFloat(r.rtpInfo.BetAmount)).
				Round(4).InexactFloat64()
			r.rtpInfo.BaseGameRpt = decimal.NewFromFloat(r.rtpInfo.BaseGameBonus).
				Div(decimal.NewFromFloat(r.rtpInfo.BetAmount)).
				Round(4).InexactFloat64()
			return
		}
	}
}

func NewRtp(bonusNum int) *Rpt {
	r := &Rpt{}
	r.NewScene()
	r.rtpInfo = rtpInfo{FreeType: bonusNum}
	return r
}
func (r *Rpt) Debug() map[string]any {
	ret, err := common.StructToMap(r.rtpInfo)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	global.GVA_LOG.Debug("rtp", zap.Any("rtpInfo", ret))
	return ret
}

func (r *Rpt) NewScene() {
	scene := Scene{IsRestart: true, SceneColList: make([]*SceneCol, 0), BonusState: _normalGame}
	r.scene = scene
}
func newBetOrderServiceRTP() *betOrderService {
	s := &betOrderService{grid: int64GridY{}, winGrid: int64GridY{},
		winCards: HisGridY{}, bonusMap: make(map[int]*betFreeGame),
		ColInfoMap: make(map[int]*ColInfo), midgrid: int64GridY{}, winDetails: [20]int{},
		nextGrid: int64GridY{}, winCol: make([]int, 0), colList: make(map[int][]int, 0),
		winResult: make([]*winResult, 0), BonusState: 2, bonusLine: make([][]*Pos, 0),
		symbolMulMap: make(map[int]map[int]int), symbolList: slices.Clone(symbolList),
		symbolCol: make(map[string]*Pos)}
	s.loadConfig()
	s.loadSymbolCol()
	s.SetFreeBonusCfg()
	return s
}

// 加载场景数据
func (s *betOrderService) reloadSceneRTP() bool {
	err := s.loadCacheSceneDataRTP()
	if err != nil {
		return false
	}
	return true
}

func (s *betOrderService) loadCacheSceneDataRTP() error {
	s.BonusState = s.scene.BonusState
	if s.scene.ContinueNum > 0 {
		s.ContinuNum = s.scene.ContinueNum
		s.IsRestart = false
		s.loadSceneColList()
		s.grid = s.scene.NextGrid
		if s.scene.IsRespin {
			s.IsFreeSpin = true
			//zap.L().Debug("loadCacheSceneData:", zap.Any("s.scene.freeTimes", s.scene.FreeTimes))
			s.toTolFreeAmount = s.scene.FreeAmount
		}
		return nil
	}
	if s.scene.IsRespin {
		//zap.L().Debug("loadCacheSceneData:", zap.Any("s.scene.freeTimes", s.scene.FreeTimes))
		s.IsFreeSpin = true
		s.toTolFreeAmount = s.scene.FreeAmount
		s.IsRestart = false
		s.loadSceneColList()
		s.grid = s.scene.NextGrid
		return nil
	}
	s.scene.BonusState = _normalGame
	s.BonusState = _normalGame
	s.scene.IsRestart = true
	return nil
}

func (s *betOrderService) checkInitRpt() {
	if !s.IsFreeSpin {
		s.updateBetAmount()
		s.scene.BetAmount = s.betAmount.Round(2).InexactFloat64()
	} else {
		s.betAmount = decimal.NewFromFloat(s.scene.BetAmount)
		s.amount = decimal.Zero
	}
}

func (s *betOrderService) RtpOrder() (*BaseSpinResult, error) {
	//加载场景数据
	s.reloadSceneRTP()
	s.checkInitRpt()
	s.setPretraMul()
	var baseRes *BaseSpinResult
	var err error
	if s.IsFreeSpin {
		//zap.L().Debug("免费游戏")
		baseRes, err = s.reSpinBase()
	} else {
		//zap.L().Debug("付费游戏")
		baseRes, err = s.baseSpin()
	}
	s.checkCleanFreeData()
	return baseRes, err
}
