package ys2

import (
	"context"
	"errors"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/global"
	"egame-grpc/utils/jsonx"

	redisv8 "github.com/go-redis/redis/v8"
	redisv9 "github.com/redis/go-redis/v9"
)

type SpinSceneData struct {
	SceneFreeGame
	Steps        uint64                  `json:"steps"`   // 连消 Step 步数
	Stage        int8                    `json:"stage"`   // 运行阶段
	NextStage    int8                    `json:"nStage"`  // 下一阶段
	SymbolRoller [_colCount]SymbolRoller `json:"sRoller"` // 滚轮符号表
}

type SceneFreeGame struct {
	FreeNum   int64   `json:"fn"` // 免费剩余次数
	FreeTimes int64   `json:"ft"` // 免费次数
	RoundWin  float64 `json:"rw"` // 回合奖金
	FreeWin   float64 `json:"fw"` // 免费累计总金额
	TotalWin  float64 `json:"tw"` // 中奖total

	// bonus 相关
	BonusState int64 `json:"bs"` // 免费选档状态
	BonusNum   int64 `json:"bn"` // 免费档位
	ScatterNum int64 `json:"sn"` // 触发免费时的夺宝数
}

func (s *SceneFreeGame) Reset() {
	*s = SceneFreeGame{}
}

func (s *betOrderService) cleanScene() {
	common.RetCleanScene(s.client, s.member.ID, GameID)
}

func (s *SpinSceneData) save(memberID int64) error {
	sceneStr, err := jsonx.MarshalString(s)
	if err != nil {
		return err
	}
	return global.GVA_REDIS.Set(context.Background(), common.GetSceneKey(memberID, GameID), sceneStr, time.Hour*24*90).Err()
}

func (s *betOrderService) reloadScene() error {
	s.scene = new(SpinSceneData)
	scene, err := loadScene(s.member.ID)
	if err != nil {
		s.cleanScene()
		return err
	}
	if scene != nil {
		s.scene = scene
	}
	s.syncGameStage()
	return nil
}

func loadScene(memberID int64) (*SpinSceneData, error) {
	raw, err := global.GVA_REDIS.Get(context.Background(), common.GetSceneKey(memberID, GameID)).Result()
	if err != nil {
		if errors.Is(err, redisv8.Nil) || errors.Is(err, redisv9.Nil) {
			return nil, nil
		}
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	scene := new(SpinSceneData)
	if err = jsonx.UnmarshalString(raw, scene); err != nil {
		return nil, err
	}
	return scene, nil
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
}
