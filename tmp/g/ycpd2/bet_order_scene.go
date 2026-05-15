package ycpd

import (
	"context"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/global"
	"egame-grpc/utils/jsonx"

	"go.uber.org/zap"
)

// SpinSceneData 持久化场景
type SpinSceneData struct {
	SceneFreeGame
	Steps          uint64                  `json:"steps"`   // 连消 step 计数
	Stage          int64                   `json:"stage"`   // 运行阶段
	NextStage      int64                   `json:"nStage"`  // 下一阶段
	GameMultiple   int64                   `json:"GameMul"` // 列消除倍数累计
	RemoveMultiple [_colCount]int64        `json:"RemMul"`  // 各列消除计数
	SymbolRoller   [_colCount]SymbolRoller `json:"sRoller"` // 滚轮
}

type SceneFreeGame struct {
	FreeNum   int64   `json:"fn"` // 免费剩余次数
	FreeTimes int64   `json:"ft"` // 免费次数
	RoundWin  float64 `json:"rw"` // 回合奖金
	FreeWin   float64 `json:"fw"` // 免费累计金额
	TotalWin  float64 `json:"tw"` // 总中奖金额
	TotalFree int64   `json:"tf"` // 累计免费次数
}

func (s *SceneFreeGame) Reset() {
	*s = SceneFreeGame{}
}

func (s *SceneFreeGame) Decr() bool {
	if s.FreeNum <= 0 {
		s.FreeNum = 0
		s.FreeWin = 0
		return false
	}
	s.FreeNum--
	return true
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
}
