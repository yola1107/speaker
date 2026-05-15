package bxkh2

import (
	"context"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/global"
	"egame-grpc/utils/jsonx"

	"go.uber.org/zap"
)

type SpinSceneData struct {
	SceneFreeGame
	Steps           uint64                  `json:"steps"`
	Stage           int8                    `json:"stage"`
	NextStage       int8                    `json:"nStage"`
	FreeWinMultiple int64                   `json:"fWMul"`
	SymbolRoller    [_colCount]SymbolRoller `json:"sRoller"`
}

type SceneFreeGame struct {
	FreeNum   uint64  `json:"fn"` // 免费剩余次数
	FreeTimes uint64  `json:"ft"` // 已使用免费次数
	RoundWin  float64 `json:"rw"` // 当前 round 累计赢分
	FreeWin   float64 `json:"fw"` // 免费阶段累计总赢
	TotalWin  float64 `json:"tw"` // 当前 spin/本轮累计总赢
}

func (s *SpinSceneData) Reset() {
	s.FreeNum = 0
	s.FreeTimes = 0
	s.RoundWin = 0
	s.FreeWin = 0
	s.TotalWin = 0

	s.Steps = 0
	s.Stage = _spinTypeBase
	s.NextStage = 0
	s.FreeWinMultiple = 1
	s.SymbolRoller = [_colCount]SymbolRoller{}
}

func (s *betOrderService) sceneKey() string {
	return common.GetSceneKey(s.member.ID, GameID)
}

func (s *betOrderService) cleanScene() {
	common.RetCleanScene(s.client, s.member.ID, GameID)
}

func (s *betOrderService) saveScene() error {
	sceneStr, err := jsonx.MarshalString(s.scene)
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
		if err := jsonx.UnmarshalString(v, s.scene); err != nil {
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
		s.scene.Steps = 0
	}
	if s.scene.NextStage > 0 {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}
	s.isFreeRound = s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli
	if s.scene.Steps == 0 {
		s.scene.RoundWin = 0
	}
}
