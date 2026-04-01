package yhwy

import (
	"context"
	"fmt"
	"time"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

// SpinSceneData 持久化场景
type SpinSceneData struct {
	Stage        int8                    `json:"stage"`        // 当前阶段：Base / Free
	NextStage    int8                    `json:"nextStage"`    // 下一手切换到的阶段
	FreeNum      int64                   `json:"freeNum"`      // 剩余免费次数
	SymbolRoller [_colCount]SymbolRoller `json:"symbolRoller"` // 上一次抽出的 5 列滚轴结果
}

var sceneDataKeyPrefix = fmt.Sprintf("scene-%d", GameID)

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
	s.syncGameStage()
	return nil
}

func (s *betOrderService) syncGameStage() {
	if s.scene.Stage == 0 {
		if s.scene.FreeNum > 0 {
			s.scene.Stage = _spinTypeFree
		} else {
			s.scene.Stage = _spinTypeBase
		}
	}
	if s.scene.NextStage > 0 {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}
	s.isFreeRound = s.scene.Stage == _spinTypeFree
}
