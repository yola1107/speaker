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
	FemaleCountsForFree [_femaleC - _femaleA + 1]int64 `json:"femaleCounts"`       // 女性符号计数
	NextSymbolGrid      *int64Grid                     `json:"nextGrid"`           // 下一step的符号网格（已消除下落填充）
	SymbolRollers       *[_colCount]SymbolRoller       `json:"rollers"`            // 滚轴状态（保存Start位置）
	RollerKey           string                         `json:"rollerKey"`          // 滚轴配置key（基础=base / 免费=收集状态）
	RoundStartTreasure  int64                          `json:"roundStartTreasure"` // 回合开始时的夺宝数量（免费模式使用）
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

	return nil
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

	// 免费游戏完全结束时，清空所有场景数据
	if s.isFreeRound && s.client.ClientOfFreeGame.GetFreeNum() == 0 {
		s.cleanScene()
		return nil
	}

	// spin > scene
	s.syncSceneFromSpin()

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

// prepareSpinFromScene 加载场景数据到spin
func (s *betOrderService) prepareSpinFromScene() (*int64Grid, *[_colCount]SymbolRoller) {
	if s.scene == nil {
		s.scene = &SpinSceneData{}
	}

	scene := s.scene

	s.spin.femaleCountsForFree = scene.FemaleCountsForFree
	s.spin.nextFemaleCountsForFree = scene.FemaleCountsForFree
	s.spin.rollerKey = scene.RollerKey
	s.spin.roundStartTreasure = scene.RoundStartTreasure

	pending := scene.NextSymbolGrid != nil && scene.SymbolRollers != nil
	if pending {
		s.spin.isRoundOver = false
		s.isFirst = false
		s.isFreeRound = s.lastOrder != nil && s.lastOrder.IsFree > 0
		return scene.NextSymbolGrid, scene.SymbolRollers
	}

	s.spin.isRoundOver = true
	s.isFirst = true
	s.isFreeRound = s.client != nil && s.client.ClientOfFreeGame.GetFreeNum() > 0
	return nil, nil
}

// syncSceneFromSpin spin数据写入到scene
func (s *betOrderService) syncSceneFromSpin() {
	if s.scene == nil {
		s.scene = &SpinSceneData{}
	}

	scene := s.scene
	scene.FemaleCountsForFree = s.spin.nextFemaleCountsForFree
	scene.RoundStartTreasure = s.spin.roundStartTreasure
	if s.spin.isRoundOver {
		scene.NextSymbolGrid = nil
		scene.SymbolRollers = nil
		scene.RollerKey = ""
		return
	}

	if s.spin.nextSymbolGrid != nil {
		copyGrid := *s.spin.nextSymbolGrid
		scene.NextSymbolGrid = &copyGrid
	} else {
		scene.NextSymbolGrid = nil
	}

	if scene.SymbolRollers == nil {
		scene.SymbolRollers = new([_colCount]SymbolRoller)
	}
	*scene.SymbolRollers = s.spin.rollers
	scene.RollerKey = s.spin.rollerKey
}
