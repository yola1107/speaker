package jqs

import (
	"context"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type SpinSceneData struct {
	Stage     int64 `json:"stage"`  // 当前阶段
	NextStage int64 `json:"nStage"` // 下一阶段
	SceneFreeGame
}

type SceneFreeGame struct {
	RoundWin float64 `json:"rw"` // 本回合累计奖金
	FreeWin  float64 `json:"fw"` // 免费段累计奖金
	TotalWin float64 `json:"tw"` // 总中奖（展示）
}

func (s *SceneFreeGame) Reset() {
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
	sceneStr, _ := jsoniter.MarshalToString(s.scene)
	if err := global.GVA_REDIS.Set(context.Background(), s.sceneKey(), sceneStr, time.Hour*24*90).Err(); err != nil {
		global.GVA_LOG.Error("saveScene", zap.Error(err))
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

	if s.scene.Stage == 0 {
		s.scene.Stage = _spinTypeBase
	}
	if s.scene.NextStage > 0 {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}
	// 设置游戏状态
	switch s.scene.Stage {
	case _spinTypeBase:
		s.state = 0
	case _spinTypeFree:
		s.state = 1
	}
	s.isFreeRound = s.scene.Stage == _spinTypeFree
	return nil
}
