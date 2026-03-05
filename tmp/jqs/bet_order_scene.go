package jqs

import (
	"context"
	"egame-grpc/global"
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type SpinSceneData struct {
	Stage           int64 `json:"stage"`  // 当前阶段
	NextStage       int64 `json:"nStage"` // 下一阶段
	RoundMultiplier int64 `json:"rMul"`   // 回合倍数
}

// 加载场景数据
func (s *betOrderService) reloadScene() error {
	s.scene = new(SpinSceneData)
	if err := s.loadCacheSceneData(); err != nil {
		global.GVA_LOG.Error("reloadScene", zap.Error(err))
		s.cleanScene()
		return err
	}
	////新免费回合开始
	//s.scene.Stage = _spinTypeBase
	//if s.scene.NextStage > _spinTypeBase && s.scene.NextStage != s.scene.Stage {
	//	s.scene.Stage = s.scene.NextStage
	//	s.scene.NextStage = _spinTypeBase
	//}
	//
	//if s.scene.Stage == _spinTypeBase {
	//	s.isFirstRound = true
	//}

	s.syncGameStage()
	return nil
}

var sceneDataKeyPrefix = fmt.Sprintf("scene-%d", _gameID)

func (s *betOrderService) sceneKey() string {
	return fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
}

func (s *betOrderService) cleanScene() {
	global.GVA_REDIS.Del(context.Background(), s.sceneKey())
}

// 保存场景
func (s *betOrderService) saveScene() error {
	sceneStr, _ := jsoniter.MarshalToString(s.scene)
	if err := global.GVA_REDIS.Set(context.Background(), s.sceneKey(), sceneStr, time.Hour*24*90).Err(); err != nil {
		global.GVA_LOG.Error("saveScene", zap.Error(err))
		return err
	}
	return nil

}

func (s *betOrderService) loadCacheSceneData() error {
	v := global.GVA_REDIS.Get(context.Background(), s.sceneKey()).Val()
	if len(v) > 0 {
		tmpSc := new(SpinSceneData)
		if err := jsoniter.UnmarshalFromString(v, tmpSc); err != nil {
			return err
		}
		s.scene = tmpSc
	}
	return nil
}

func (s *betOrderService) syncGameStage() {
	// 正常游戏模式逻辑
	if s.scene.Stage == 0 {
		s.scene.Stage = _spinTypeBase
		s.scene.RoundMultiplier = 0
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
}
