package ajtm

import (
	"context"
	"fmt"
	"time"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type SpinSceneData struct {
	Steps              uint64                  `json:"steps"`     // 连消 step 数
	Stage              int8                    `json:"stage"`     // 当前阶段
	NextStage          int8                    `json:"nStage"`    // 下一阶段
	RoundMultiplier    int64                   `json:"rMul"`      // 当前 round 累计倍数
	FreeNum            int64                   `json:"freeNum"`   // 剩余免费次数
	SymbolRoller       [_colCount]SymbolRoller `json:"sRoller"`   // 当前滚轴窗口
	MysMultiplierTotal int64                   `json:"mysMul"`    // 神秘符号累计倍数
	LongCount          [_colCount]int          `json:"longCount"` // 各列当前长符号数量，仅免费模式继承使用
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
		s.scene.Stage = _spinTypeBase
		s.scene.Steps = 0
	}

	if s.scene.NextStage > 0 {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}

	s.isFreeRound = s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli

	if s.scene.Steps == 0 {
		s.scene.RoundMultiplier = 0
	}
}
