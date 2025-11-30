package sjnws3

import (
	"context"
	"time"

	"egame-grpc/global"
	"egame-grpc/global/client"

	jsoniter "github.com/json-iterator/go"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type Scene struct {
	spinBonusAmount  float64     `json:"-"`
	roundBonusAmount float64     `json:"-"`
	freeTotalMoney   float64     `json:"-"`
	FreeTimes        int         `json:"free_times"`
	AddFreeTimes     int         `json:"add_free_times"`
	BonusTimes       int         `json:"bonus_times"`
	MaxFreeTimes     int         `json:"max_free_times"`
	IsRespin         bool        `json:"get_free"`
	IsRestart        bool        `json:"is_restart"`
	SceneColList     []*SceneCol `json:"scene_col_list"`
	NextGrid         int64GridY  `json:"next_grid"`
	BonusState       int         `json:"bonus_state"`
	BonusNum         int         `json:"bonus_num"`
	ScatterNum       int         `json:"scatter_num"`
	FreeAmount       float64     `json:"free_amount"`
	ContinueNum      int         `json:"continue_num"`
	ContinueMulti    int         `json:"continue_multi"`
	Win              float64     `json:"win"`
	TotalWin         float64     `json:"total_win"`
	TotalFreeWin     float64     `json:"total_free_win"`
	TotalBaseWin     float64     `json:"total_base_win"`
	/*
		一下部分专门为重构历史数据添加
	*/
	LastRepin         int        `json:"last_repin"`
	LastGrid          int64GridY `json:"last_grid"`
	LastWinGrid       HisGridY   `json:"last_win_grid"`
	BetAmount         float64    `json:"bet_amount"`
	BetMoney          float64    `json:"bet_money"`
	LastBonusState    int        `json:"last_bonus_amount"`
	LastFreeTimes     int        `json:"last_free_times"`
	LastBonusNum      int        `json:"last_bonus_num"`
	LastMaxFreeTimes  int        `json:"last_max_free_times"`
	LastContinueMulti int        `json:"last_continue_multi"`
	LastWin           float64    `json:"last_win"`
	LastTotalWin      float64    `json:"last_total_win"`
	LastFreeAmount    float64    `json:"last_free_amount"`
	LastTreasureNum   int        `json:"last_treasure_num"`
}

// 加载场景数据
func (s *betOrderService) reloadScene() bool {
	err := s.loadCacheSceneData()
	if s.scene.BonusState == _freeGame && s.scene.BonusNum > 0 {
		s.scene.BonusState = _normalGame
		global.GVA_LOG.Error("本游戏中scene,出现了错误的选择中奖状态,修复:", zap.Any("s.scene.BonusState", s.scene.BonusState))
	}
	if err != nil {
		return false
	}
	return true
}

func (s *betOrderService) cleanScene() {
	global.GVA_REDIS.Del(context.Background(), getSceneKey(s.member.ID))
}

func (m *memberLoginService) getSceneInfo() *Scene {
	key := getSceneKey(m.req.MemberId)
	v := global.GVA_REDIS.Get(context.Background(), key).Val()
	scene := &Scene{}
	if len(v) > 0 {
		err := jsoniter.UnmarshalFromString(v, scene)
		if err != nil {
			return scene
		}
		return scene
	}
	return scene
}

// 保存场景
func (s *betOrderService) saveScene() error {
	s.client.ClientOfFreeGame.SetFreeNum(uint64(s.scene.LastFreeTimes))
	s.client.SetTreasureNum(uint64(s.ScatterNum))
	s.client.ClientOfFreeGame.FreeTotalMoney = s.scene.freeTotalMoney
	s.client.ClientOfFreeGame.SetFreeType(uint64(s.scene.LastBonusNum))
	s.client.SetMaxFreeNum(uint64(s.scene.MaxFreeTimes))
	if s.scene.FreeTimes <= 0 {
		s.client.ClientOfFreeGame.SetFreeType(0)
	}

	// 保存grpc客户端
	client.GVA_CLIENT_BUCKET.SaveClient(s.client)
	s.client.ClientGameCache.SaveScenes(s.client)
	/*
		以下才是本游戏中的处理场景过度数据的逻辑
	*/
	s.scene.BetMoney = s.req.BaseMoney
	s.scene.BetAmount = s.betAmount.Round(2).InexactFloat64()
	sceneStr, err := jsoniter.MarshalToString(s.scene)
	if err != nil {
		return err
	}
	global.GVA_REDIS.Set(context.Background(), getSceneKey(s.member.ID), sceneStr, 90*24*time.Hour)
	return nil
}

func (s *betOrderService) loadCacheSceneData() error {
	v := global.GVA_REDIS.Get(context.Background(), getSceneKey(s.member.ID)).Val()
	if len(v) > 0 {
		err := jsoniter.UnmarshalFromString(v, &s.scene)
		if err != nil {
			return err
		}
		s.BonusState = s.scene.BonusState
		s.ContinuNum = s.scene.ContinueNum
		s.grid = s.scene.NextGrid
		s.toTolFreeAmount = s.scene.FreeAmount
		s.loadSceneColList()
		if s.scene.BonusState == 0 {
			s.scene.ContinueMulti = 1
		}
		s.scene.BonusState = _normalGame
		s.BonusState = _normalGame
		if s.scene.ContinueNum > 0 {
			s.IsRestart = false
			if s.scene.IsRespin {
				s.IsFreeSpin = true
			}
			return nil
		}
		if s.scene.IsRespin {
			s.IsFreeSpin = true
			s.IsRestart = false
			return nil
		}
		return nil
	}
	s.reSetCleanData()
	s.BonusState = _normalGame
	s.BonusState = s.scene.BonusState
	return nil
}
func (s *betOrderService) updateHistoryFreeDate() {
	s.scene.LastMaxFreeTimes = s.scene.MaxFreeTimes
	s.scene.LastBonusState = s.scene.BonusState
	s.scene.LastBonusNum = s.scene.BonusNum
	s.scene.LastTreasureNum = s.ScatterNum
	s.scene.LastGrid = s.grid
	s.scene.LastWin = s.scene.Win
	s.scene.LastTotalWin = decimal.NewFromFloat(s.scene.Win).
		Add(decimal.NewFromFloat(s.scene.LastTotalWin)).Round(2).InexactFloat64()
	s.scene.TotalWin += s.scene.Win
	s.scene.LastWinGrid = s.winCards
	if s.IsFreeSpin {
		s.scene.LastRepin = 1
	} else {
		s.scene.LastRepin = 0
	}
	s.scene.LastFreeAmount = s.scene.FreeAmount
	s.scene.LastFreeTimes = s.scene.FreeTimes
	s.scene.LastContinueMulti = s.scene.ContinueMulti

}

func (s *betOrderService) cleanFreeData() {
	s.scene.ContinueNum = 0
	s.scene.FreeAmount = 0
	s.scene.IsRespin = false
	s.scene.BonusState = _normalGame
	s.scene.BonusNum = 0
	s.scene.MaxFreeTimes = 0
	s.scene.FreeTimes = 0
	s.scene.AddFreeTimes = 0
}

func (s *betOrderService) reSetCleanData() {
	s.scene.ContinueNum = 0
	s.scene.FreeAmount = 0
	s.scene.IsRespin = false
	s.scene.BonusState = _normalGame
	s.scene.BonusNum = 0
	s.scene.IsRestart = true
	s.scene.MaxFreeTimes = 0
	s.scene.FreeTimes = 0
	s.scene.AddFreeTimes = 0
	s.scene.Win = 0
	s.scene.SceneColList = make([]*SceneCol, 0)
	s.scene.NextGrid = getInitCardsFor8890()
	s.scene.LastGrid = getInitCardsFor8890()
	s.scene.LastWin = 0
	s.scene.LastWinGrid = HisGridY{}
	s.scene.LastRepin = 0
	s.scene.LastFreeAmount = 0
	s.scene.LastFreeTimes = 0
	s.scene.LastContinueMulti = 1
	s.scene.LastBonusState = _normalGame
	s.scene.LastBonusNum = 0
	s.scene.LastMaxFreeTimes = 0
}

func (s *betOrderService) normalGameStopContinue() {
	s.scene.ContinueNum = 0
	s.scene.IsRestart = false
}
func (s *betOrderService) freeGameStopContinue() {
	s.scene.ContinueNum = 0
	s.scene.IsRestart = false
}

func (s *betOrderService) continueFreeGame() {
	s.scene.IsRestart = false
}

func (s *betOrderService) continueNormalGame() {
	s.scene.IsRestart = false
}

func (s *betOrderService) checkCleanFreeData() {
	if s.IsFreeSpin && s.scene.FreeTimes <= 0 && s.scene.ContinueNum <= 0 {
		s.cleanFreeData()
	}
}
