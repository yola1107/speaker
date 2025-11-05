package xslm2

import (
	"context"
	"fmt"
	"time"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

// SpinSceneData 场景数据（需要持久化的状态）
type SpinSceneData struct {
	FemaleCountsForFree [_femaleC - _femaleA + 1]int64 `json:"femaleCounts"` // 女性符号计数
}

// getSceneKey 获取场景数据的Redis key
func (s *betOrderService) getSceneKey() string {
	return fmt.Sprintf("%s:scene-%d:%d", global.GVA_CONFIG.System.Site, _gameID, s.member.ID)
}

// cleanScene 清除场景数据
func (s *betOrderService) cleanScene() {
	if s.debug.open {
		return
	}
	global.GVA_REDIS.Del(context.Background(), s.getSceneKey())
}

// reloadScene 加载场景数据
func (s *betOrderService) reloadScene() error {
	s.scene = &SpinSceneData{}

	if s.debug.open {
		return nil
	}

	if err := s.loadCacheSceneData(); err != nil {
		global.GVA_LOG.Error("reloadScene", zap.Error(err))
		s.cleanScene()
		return err
	}

	// 恢复女性符号计数到 spin 结构
	s.restoreFemaleCountsToSpin()
	return nil
}

// restoreFemaleCountsToSpin 恢复女性符号计数到spin结构
func (s *betOrderService) restoreFemaleCountsToSpin() {
	s.spin.femaleCountsForFree = s.scene.FemaleCountsForFree
	s.spin.nextFemaleCountsForFree = s.scene.FemaleCountsForFree
}

// loadCacheSceneData 从Redis加载场景数据
func (s *betOrderService) loadCacheSceneData() error {
	v := global.GVA_REDIS.Get(context.Background(), s.getSceneKey()).Val()
	if len(v) == 0 {
		return nil
	}

	tmpScene := SpinSceneData{}
	if err := jsoniter.UnmarshalFromString(v, &tmpScene); err != nil {
		return err
	}
	s.scene = &tmpScene
	return nil
}

// saveScene 保存场景数据
func (s *betOrderService) saveScene() error {
	if s.debug.open {
		return nil
	}

	// 更新场景数据
	s.scene.FemaleCountsForFree = s.spin.nextFemaleCountsForFree

	return s.saveCacheSceneData()
}

// saveCacheSceneData 保存场景数据到Redis
func (s *betOrderService) saveCacheSceneData() error {
	sceneStr, err := jsoniter.MarshalToString(s.scene)
	if err != nil {
		global.GVA_LOG.Error("saveCacheSceneData", zap.Error(err))
		return err
	}

	err = global.GVA_REDIS.Set(context.Background(), s.getSceneKey(), sceneStr, 90*24*time.Hour).Err()
	if err != nil {
		global.GVA_LOG.Error("saveCacheSceneData", zap.Error(err))
		return err
	}

	return nil
}
