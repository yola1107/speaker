package clzw

import (
	"context"
	"time"

	"go.uber.org/zap"

	"egame-grpc/game/common"
	"egame-grpc/global"
	"egame-grpc/utils/jsonx"
)

type SpinSceneData struct {
	SceneFreeGame
	Steps        int64                   `json:"steps"`   // 连消 Step 步数
	Stage        int8                    `json:"stage"`   // 运行阶段
	NextStage    int8                    `json:"nStage"`  // 下一阶段
	SymbolRoller [_colCount]SymbolRoller `json:"sRoller"` // 滚轮符号表
}

type SceneFreeGame struct {
	FreeNum        int64   `json:"fn"` // 免费剩余次数
	FreeTimes      int64   `json:"ft"` // 免费次数
	RoundWin       float64 `json:"rw"` // 回合奖金
	FreeWin        float64 `json:"fw"` // 免费累计总金额
	TotalWin       float64 `json:"tw"` // 中奖total
	PurchaseAmount int64   `json:"pa"` // 本次购买扣费额，与 s.req.Purchase 一致
}

func (s *SceneFreeGame) Reset() {
	s.FreeNum = 0
	s.FreeTimes = 0
	s.RoundWin = 0
	s.FreeWin = 0
	s.TotalWin = 0
	s.PurchaseAmount = 0
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
		global.GVA_LOG.Error("saveScene", zap.Error(err))
		return err
	}
	if err = global.GVA_REDIS.Set(context.Background(), s.sceneKey(), sceneStr, time.Hour*24*90).Err(); err != nil {
		global.GVA_LOG.Error("saveScene", zap.Error(err))
		return err
	}
	return nil
}

func (s *betOrderService) reloadScene() error {
	s.scene = new(SpinSceneData)
	if v := global.GVA_REDIS.Get(context.Background(), s.sceneKey()).Val(); len(v) > 0 {
		if err := jsonx.UnmarshalString(v, s.scene); err != nil {
			global.GVA_LOG.Error("reloadScene", zap.Error(err))
			s.cleanScene()
			return err
		}
	}
	return nil
}

func (s *betOrderService) syncGameStage() error {
	if s.scene.Stage == 0 {
		s.scene.Stage = _spinTypeBase
		s.scene.Steps = 0
	}
	if s.scene.NextStage > 0 {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}
	s.isFreeRound = s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.scene.RoundWin = 0
	}
	return nil
}
