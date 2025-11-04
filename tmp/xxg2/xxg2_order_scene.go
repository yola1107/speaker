package xxg2

import (
	"context"
	"fmt"
	"time"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

// SpinSceneData 场景数据结构（client 数据通过 ClientGameCache 自动持久化）
type SpinSceneData struct {
	Stage             int8        `json:"stage"`             // 当前阶段（1:base, 2:free）
	NextStage         int8        `json:"nStage"`            // 下一阶段
	BatPositions      []*position `json:"batPositions"`      // 蝙蝠当前位置（免费模式持续追踪）
	InitialBatCount   int64       `json:"initialBatCount"`   // 初始蝙蝠数量（触发免费游戏时的夺宝数量）
	AccumulatedNewBat int64       `json:"accumulatedNewBat"` // 免费游戏中累计新增的蝙蝠数量
}

// isFreeRound 是否为免费回合
func (s *betOrderService) isFreeRound() bool {
	return s.scene.Stage == _spinTypeFree
}

func (s *betOrderService) getSceneKey() string {
	return fmt.Sprintf("%s:scene-%d:%d", global.GVA_CONFIG.System.Site, GameID, s.member.ID)
}

// cleanScene 清理场景数据
func (s *betOrderService) cleanScene() {
	key := s.getSceneKey()
	global.GVA_REDIS.Del(context.Background(), key)
}

// reloadScene 加载场景数据
func (s *betOrderService) reloadScene() bool {
	s.scene = &SpinSceneData{}

	if err := s.loadCacheSceneData(); err != nil {
		global.GVA_LOG.Error("reloadScene", zap.Error(err))
		s.cleanScene()
		return false
	}

	s.updateSceneStage()
	return true
}

// saveScene 保存场景数据到Redis
func (s *betOrderService) saveScene() error {
	if s.debug.open {
		return nil
	}

	s.updateSceneStage()

	sceneStr, _ := jsoniter.MarshalToString(s.scene)
	key := s.getSceneKey()

	if err := global.GVA_REDIS.Set(context.Background(), key, sceneStr, time.Hour*24*90).Err(); err != nil {
		global.GVA_LOG.Error("saveScene", zap.Error(err))
		return err
	}
	return nil
}

// updateSceneStage 更新场景阶段
func (s *betOrderService) updateSceneStage() {
	if s.client.ClientOfFreeGame.GetFreeNum() > 0 {
		s.scene.Stage = _spinTypeFree
	} else {
		s.scene.Stage = _spinTypeBase
	}

	// 处理阶段切换
	if s.scene.NextStage > 0 && s.scene.NextStage != s.scene.Stage {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}
}

// loadCacheSceneData 从Redis加载场景数据
func (s *betOrderService) loadCacheSceneData() error {
	key := s.getSceneKey()
	v := global.GVA_REDIS.Get(context.Background(), key).Val()

	if len(v) > 0 {
		return jsoniter.UnmarshalFromString(v, s.scene)
	}
	return nil
}
