package sjxj

import (
	"context"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type SpinSceneData struct {
	SceneFreeGame
	Stage                  int8                    `json:"stage"`                  // 运行阶段
	NextStage              int8                    `json:"nStage"`                 // 下一阶段
	SymbolRoller           [_colCount]SymbolRoller `json:"sRoller"`                // 滚轮符号表
	ScatterLock            int64Grid               `json:"scatterLock"`            // Free 专用：ScatterLock 8×5（0=未锁，>0=锁定为夺宝且值即夺宝倍数）
	UnlockedRows           int                     `json:"unlockedRows"`           // 当前已解锁行数（范围 4-8，初始4行）
	PrevUnlockedRows       int                     `json:"prevUnlockedRows"`       // 上一局结束时的解锁行数
	BaseEnterFreeFirstStep bool                    `json:"baseEnterFreeFirstStep"` // 基础进入免费后，免费首局只做一次阶段1解锁（按锁定夺宝）
}

type SceneFreeGame struct {
	FreeNum   int64   `json:"fn"` // 免费剩余次数
	FreeTimes int64   `json:"ft"` // 免费次数
	RoundWin  float64 `json:"rw"` // 回合奖金
	FreeWin   float64 `json:"fw"` // 免费累计总金额
	TotalWin  float64 `json:"tw"` // 中奖total
}

func (s *SceneFreeGame) Reset() {
	s.FreeNum = 0
	s.FreeTimes = 0
	s.RoundWin = 0
	s.FreeWin = 0
	s.TotalWin = 0
}

func (s *betOrderService) sceneKey() string {
	return common.GetSceneKey(s.member.ID, GameID)
}

func (s *betOrderService) cleanScene() {
	common.RetCleanScene(s.client, s.member.ID, GameID)
}

func (s *betOrderService) saveScene() error {
	sceneStr, err := jsoniter.MarshalToString(s.scene)
	if err != nil {
		global.GVA_LOG.Error("saveScene: marshal failed", zap.Error(err))
		return err
	}
	if err = global.GVA_REDIS.Set(context.Background(), s.sceneKey(), sceneStr, time.Hour*24*90).Err(); err != nil {
		global.GVA_LOG.Error("saveScene: redis set failed", zap.Error(err))
		return err
	}
	return nil
}

func (s *betOrderService) reloadScene() error {
	s.scene = new(SpinSceneData)
	if v := global.GVA_REDIS.Get(context.Background(), s.sceneKey()).Val(); len(v) > 0 {
		if err := jsoniter.UnmarshalFromString(v, s.scene); err != nil {
			global.GVA_LOG.Error("betOrder: reloadScene failed", zap.Error(err))
			s.cleanScene()
			return err
		}
	}
	s.syncGameStage()
	return nil
}

func (s *betOrderService) syncGameStage() {
	if s.scene.Stage == 0 {
		s.scene.Stage = _spinTypeBase
	}
	if s.scene.NextStage > 0 {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}
	s.isFreeRound = s.scene.Stage == _spinTypeFree

	if !s.isFreeRound {
		s.scene.ScatterLock = int64Grid{}
		s.scene.UnlockedRows = _rowCountReward
		s.scene.PrevUnlockedRows = _rowCountReward
		s.scene.BaseEnterFreeFirstStep = false
	}
}
