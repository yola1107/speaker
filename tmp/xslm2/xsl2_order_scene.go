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
	FemaleCountsForFree [_femaleC - _femaleA + 1]int64 `json:"femaleCounts"`   // 女性符号计数
	NextSymbolGrid      *int64Grid                     `json:"nextGrid"`       // 下一step的符号网格（已消除下落填充）
	SymbolRollers       *[_colCount]SymbolRoller       `json:"rollers"`        // 滚轴状态（保存Start位置）
	RoundFirstStep      int                            `json:"roundFirstStep"` // round首次step标志（0=首次，>0=非首次）
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

	// 免费游戏完全结束时，清空所有场景数据
	if s.isFreeRound && s.client.ClientOfFreeGame.GetFreeNum() == 0 {
		s.cleanScene()
		return nil
	}

	// 更新场景数据
	s.scene.FemaleCountsForFree = s.spin.nextFemaleCountsForFree

	// 保存下一step的网格和滚轴（已经消除下落填充完成）
	if !s.spin.isRoundOver {
		// 回合未结束，保存下落后的网格和滚轴状态供下一step使用
		s.scene.NextSymbolGrid = s.spin.nextSymbolGrid
		s.scene.SymbolRollers = &s.spin.rollers // 保存滚轴状态
		s.scene.RoundFirstStep = 1              // 标记非首次
	} else {
		// 回合结束，清空网格和滚轴数据
		s.scene.NextSymbolGrid = nil
		s.scene.SymbolRollers = nil // 清空滚轴
		s.scene.RoundFirstStep = 0  // 重置为首次
	}

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
