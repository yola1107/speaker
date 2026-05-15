package gcd

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
	Stage      int64                   `json:"s"`
	NextStage  int64                   `json:"ns"`
	Real       int64                   `json:"re"`
	Roller     [_colCount]SymbolRoller `json:"ro"`
	NormalStep int64                   `json:"nsm"`

	FreeStep   int64   `json:"fsm"`
	FreeType   int64   `json:"fty"`
	FreeNum    int64   `json:"fn"`
	FreeTimes  int64   `json:"ft"`
	BonusState int64   `json:"bs"`
	RoundWin   float64 `json:"rw"`
	FreeWin    float64 `json:"fw"`
	TotalWin   float64 `json:"tw"`
	ScatterNum int64   `json:"sn"` // 触发免费时的夺宝数
}

func (s *SpinSceneData) Reset() {
	s.FreeType = 0
	s.FreeNum = 0
	s.FreeTimes = 0
	s.RoundWin = 0
	s.FreeWin = 0
	s.TotalWin = 0
	s.NormalStep = 0
	s.FreeStep = 0
	s.BonusState = 0
	s.Real = 0
	s.Roller = [_colCount]SymbolRoller{}
	s.ScatterNum = 0
}

func (s *betOrderService) loadScene() error {
	s.scene = new(SpinSceneData)
	scene, err := loadScene(s.member.ID)
	if err != nil {
		s.cleanScene()
		return err
	}
	if scene != nil {
		s.scene = scene
	}
	return nil
}

func (s *betOrderService) stepScene() error {
	s.scene.Stage = _normalMode
	if s.scene.NextStage > _normalMode && s.scene.NextStage != s.scene.Stage {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = _normalMode
	}

	if s.scene.Stage == _normalMode {
		s.refreshData()
	} else if s.scene.Stage == _freeMode {
		s.scene.FreeStep = 0
		s.scene.FreeTimes += 1
		if s.scene.FreeNum > 0 {
			s.scene.FreeNum -= 1
		}
	}

	if s.isFreeMode() {
		s.freeType = s.scene.FreeType
	}
	return nil
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

func (s *betOrderService) isNormalMode() bool {
	return s.scene.Stage == _normalMode || s.scene.Stage == _normalModeEli
}

func (s *betOrderService) isFreeMode() bool {
	return s.scene.Stage == _freeMode || s.scene.Stage == _freeModeEli
}

func (s *betOrderService) refreshData() {
	s.scene.NormalStep = 0
	s.scene.FreeStep = 0
	s.scene.Real = 0
	s.scene.Roller = [_colCount]SymbolRoller{}
}
