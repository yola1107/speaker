package sjxj

import (
	"context"
	"fmt"
	"time"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type SpinSceneData struct {
	Stage            int8                    `json:"stage"`            // 运行阶段
	NextStage        int8                    `json:"nStage"`           // 下一阶段
	FreeNum          int64                   `json:"freeNum"`          // 剩余免费次数（独立统计，不依赖client）
	SymbolRoller     [_colCount]SymbolRoller `json:"sRoller"`          // 滚轮符号表
	ScatterLock      int64Grid               `json:"scatterLock"`      // Free 专用：ScatterLock 8×5（0=未锁，>0=锁定为夺宝且值即夺宝倍数）
	UnlockedRows     int                     `json:"unlockedRows"`     // 当前已解锁行数（范围 4-8，初始4行）
	PrevUnlockedRows int                     `json:"prevUnlockedRows"` // 上一局结束时的解锁行数
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
			s.cleanScene()
			return err
		}
	}

	// 加载场景后立即进行状态切换
	s.syncGameStage()
	return nil
}

func (s *betOrderService) syncGameStage() {
	if s.scene.Stage == 0 {
		s.scene.Stage = _spinTypeBase
	}

	if s.scene.NextStage > 0 {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}

	s.isFreeRound = s.scene.Stage == _spinTypeFree

	// 只要不是免费态，就清理 free 专用状态，避免旧 scatterLock 影响下一次 lockScatter。
	if !s.isFreeRound {
		s.scene.ScatterLock = int64Grid{}
		s.scene.UnlockedRows = _rowCountReward
		s.scene.PrevUnlockedRows = _rowCountReward
	}
}
