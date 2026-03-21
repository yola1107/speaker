package sbyymx2

import (
	"context"
	"fmt"
	"time"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

// SpinSceneData 场景中间态（存 Redis）
type SpinSceneData struct {
	Stage int8 `json:"stage"` // 运行阶段（预留）：默认 1=基础单步

	// 以下为「狂欢/必赢」等特殊模式预留，与策划 5.3 节对应；当前服务端未驱动多步重转，由客户端表现或二期逻辑接入
	SpecialMode       bool `json:"specialMode,omitempty"`       // 是否处于必赢/狂欢流程
	SpecialSpinRemain int8 `json:"specialSpinRemain,omitempty"` // 剩余无奖重转次数上限估算（预留）
	SpecialTotalSpins int8 `json:"specialTotalSpins,omitempty"` // 已执行子步数（预留）
}

var sceneDataKeyPrefix = fmt.Sprintf("scene-%d", _gameID)

func (s *betOrderService) sceneKey() string {
	return fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
}

func (s *betOrderService) cleanScene() {
	global.GVA_REDIS.Del(context.Background(), s.sceneKey())
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
			global.GVA_LOG.Warn("reloadScene: corrupt scene json, reset",
				zap.String("key", s.sceneKey()),
				zap.Error(err))
			s.cleanScene()
			s.scene = new(SpinSceneData)
		}
	}
	s.syncGameStage()
	return nil
}

func (s *betOrderService) syncGameStage() {
	if s.scene.Stage == 0 {
		s.scene.Stage = 1
	}
}
