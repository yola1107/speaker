// game/bxkh2/bet_order_scene.go
package bxkh2

import (
	"context"
	"fmt"
	"time"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
)

type SpinSceneData struct {
	Steps           uint64                  `json:"steps"`
	Stage           int8                    `json:"stage"`
	RoundMultiplier int64                   `json:"rMul"`
	SpinMultiplier  int64                   `json:"sMul"`
	FreeMultiplier  int64                   `json:"fMul"`
	NextStage       int8                    `json:"nStage"`
	RoundOver       bool                    `json:"rOver"`
	IsFreeRound     bool                    `json:"free"`
	FreeWinMultiple int64                   `json:"fWMul"`
	RemoveNum       int64                   `json:"rNum"`
	SymbolRoller    [_colCount]SymbolRoller `json:"sRoller"`
}

func (s *betOrderService) sceneKey() string {
	return fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
}

func (s *betOrderService) cleanScene() {
	global.GVA_REDIS.Del(context.Background(), s.sceneKey())
}

func (s *betOrderService) saveScene() error {
	sceneStr, err := jsoniter.MarshalToString(s.scene)
	if err != nil {
		return err
	}
	return global.GVA_REDIS.Set(context.Background(), s.sceneKey(), sceneStr, time.Hour*24*90).Err()
}

func (s *betOrderService) reloadScene() error {
	s.scene = &SpinSceneData{}
	v := global.GVA_REDIS.Get(context.Background(), s.sceneKey()).Val()
	if len(v) > 0 {
		if err := jsoniter.UnmarshalFromString(v, s.scene); err != nil {
			s.cleanScene()
			return err
		}
	}

	// 同步阶段
	if s.scene.Stage == 0 {
		s.scene.Stage = _spinTypeBase
	}
	if s.scene.NextStage > 0 {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}

	s.removeNum = s.scene.RemoveNum
	s.freeMultiple = s.scene.FreeWinMultiple

	// 根据阶段设置 IsFreeRound
	if s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli {
		s.scene.IsFreeRound = true
	} else {
		s.scene.IsFreeRound = false
	}
	return nil
}
