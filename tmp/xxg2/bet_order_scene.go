package xxg2

import (
	"context"
	"fmt"
	"time"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

// SpinSceneData 场景数据结构（xxg2 特有的游戏状态，client 数据通过 ClientGameCache 自动持久化）
type SpinSceneData struct {
	Stage             int8        `json:"stage"`             // 当前阶段（1:base, 2:free）
	NextStage         int8        `json:"nStage"`            // 下一阶段
	IsFirstFree       bool        `json:"isFirstFree"`       // 第一次免费游戏
	IsFreeRound       bool        `json:"free"`              // 是否为免费回合
	BatPositions      []*position `json:"batPositions"`      // 蝙蝠当前位置（免费模式持续追踪）
	InitialBatCount   int64       `json:"initialBatCount"`   // 初始蝙蝠数量（触发免费游戏时的夺宝数量）
	AccumulatedNewBat int64       `json:"accumulatedNewBat"` // 免费游戏中累计新增的蝙蝠数量
	SpinFirstRound    int         `json:"spinFirstRound"`    // spin首个round（0:首次, >0:非首次）
	RoundFirstStep    int         `json:"roundFirstStep"`    // round首次step（0:首次, >0:非首次）
}

// isFreeRound 是否为免费回合（参考 mahjong）
func (s *betOrderService) isFreeRound() bool {
	return s.scene.Stage == _spinTypeFree
}

// isBaseRound 是否为基础回合（参考 mahjong）
func (s *betOrderService) isBaseRound() bool {
	return s.scene.Stage == _spinTypeBase
}

// sceneDataKeyPrefix 场景数据 key 前缀
var sceneDataKeyPrefix = fmt.Sprintf("scene-%d", _gameID)

// cleanScene 清理场景数据
func (s *betOrderService) cleanScene() {
	key := fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
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

	// 设置当前阶段
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

	// 同步IsFreeRound标志
	s.scene.IsFreeRound = (s.scene.Stage == _spinTypeFree)

	return true
}

// saveScene 保存场景数据到Redis
func (s *betOrderService) saveScene() error {
	if s.forRtpBench {
		return nil
	}

	// 设置阶段（蝙蝠位置已在updateFreeStepResult中保存）
	if s.client.ClientOfFreeGame.GetFreeNum() > 0 {
		s.scene.Stage = _spinTypeFree
		s.scene.IsFreeRound = true
	} else {
		s.scene.Stage = _spinTypeBase
		s.scene.IsFreeRound = false
	}

	sceneStr, _ := jsoniter.MarshalToString(s.scene)
	key := fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)

	if err := global.GVA_REDIS.Set(context.Background(), key, sceneStr, time.Hour*24*90).Err(); err != nil {
		global.GVA_LOG.Error("saveScene", zap.Error(err))
		return err
	}
	return nil
}

// loadCacheSceneData 从Redis加载场景数据
func (s *betOrderService) loadCacheSceneData() error {
	key := fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
	v := global.GVA_REDIS.Get(context.Background(), key).Val()

	if len(v) > 0 {
		return jsoniter.UnmarshalFromString(v, s.scene)
	}
	return nil
}
