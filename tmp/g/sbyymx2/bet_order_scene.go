package sbyymx2

import (
	"context"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type SpinSceneData struct {
	IsRespinMode bool                    `json:"isRespinMode"` // 重转至赢模式
	TotalWin     float64                 `json:"tw"`           // 本回合（含重转链）累计赢取
	SymbolRoller [_colCount]SymbolRoller `json:"sRoller"`      // 滚轮符号表
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
	return nil
}
