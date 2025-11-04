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

var sceneDataKeyPrefix = fmt.Sprintf("scene-%d", _gameID)

// cleanScene 清除场景数据
func (s *betOrderService) cleanScene() {
	if s.debug.open {
		return
	}
	key := s.getSceneKey()
	global.GVA_REDIS.Del(context.Background(), key)
}

// reloadScene 加载场景数据
func (s *betOrderService) reloadScene() error {
	if s.debug.open {
		// RTP测试模式使用空场景
		s.scene = &SpinSceneData{}
		return nil
	}

	s.scene = &SpinSceneData{}
	err := s.loadCacheSceneData()
	if err != nil {
		global.GVA_LOG.Error("reloadScene", zap.Error(err))
		s.cleanScene()
		return err
	}

	// 恢复女性符号计数到 spin 结构
	s.spin.femaleCountsForFree = s.scene.FemaleCountsForFree
	s.spin.nextFemaleCountsForFree = s.scene.FemaleCountsForFree

	return nil
}

// saveScene 保存场景数据
func (s *betOrderService) saveScene() error {
	if s.debug.open {
		// RTP测试模式不保存
		return nil
	}

	// 更新场景数据
	s.scene.FemaleCountsForFree = s.spin.nextFemaleCountsForFree

	sceneStr, err := jsoniter.MarshalToString(s.scene)
	if err != nil {
		global.GVA_LOG.Error("saveScene", zap.Error(err))
		return err
	}

	key := s.getSceneKey()
	// 缓存场景数据90天
	err = global.GVA_REDIS.Set(context.Background(), key, sceneStr, 90*24*time.Hour).Err()
	if err != nil {
		global.GVA_LOG.Error("saveScene", zap.Error(err))
		return err
	}

	return nil
}

// loadCacheSceneData 从Redis加载场景数据
func (s *betOrderService) loadCacheSceneData() error {
	key := s.getSceneKey()
	v := global.GVA_REDIS.Get(context.Background(), key).Val()

	if len(v) > 0 {
		tmpScene := &SpinSceneData{}
		err := jsoniter.UnmarshalFromString(v, tmpScene)
		if err != nil {
			return err
		}
		s.scene = tmpScene
	}

	return nil
}

// getSceneKey 获取场景数据的Redis key
func (s *betOrderService) getSceneKey() string {
	return fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
}
