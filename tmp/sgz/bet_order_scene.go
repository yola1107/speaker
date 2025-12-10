package sgz

import (
	"context"
	"fmt"
	"time"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type SpinSceneData struct {
	Steps           uint64                  `json:"steps"`     // step步数
	Stage           int8                    `json:"stage"`     // 运行阶段
	NextStage       int8                    `json:"nStage"`    // 下一阶段
	FreeNum         int64                   `json:"freeNum"`   // 剩余免费次数（独立统计，不依赖client）
	HeroID          int64                   `json:"heroID"`    // 免费游戏类型（英雄ID 1/2/3/4/5/6/7/8）
	CityValue       int64                   `json:"CityValue"` // 打赢新城池的战斗力累计值 用于解锁英雄ID
	ContinueNum     int64                   `json:"cNum"`      // 连续消除次数
	RoundMultiplier int64                   `json:"rMul"`      // 回合倍数
	SymbolRoller    [_colCount]SymbolRoller `json:"sRoller"`   // 滚轮符号表
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
	if err := s.loadCacheSceneData(); err != nil {
		global.GVA_LOG.Error("reloadScene", zap.Error(err))
		s.cleanScene()
		return nil
	}

	// 加载场景后立即进行状态切换
	s.syncGameStage()

	return nil
}

func (s *betOrderService) loadCacheSceneData() error {
	v := global.GVA_REDIS.Get(context.Background(), s.sceneKey()).Val()
	if len(v) > 0 {
		tmpSc := new(SpinSceneData)
		if err := jsoniter.UnmarshalFromString(v, tmpSc); err != nil {
			return err
		}
		s.scene = tmpSc
	}
	return nil
}

func (s *betOrderService) syncGameStage() {
	if s.scene.Stage == 0 {
		s.scene.Stage = _spinTypeBase
	}

	if s.scene.NextStage > 0 {
		if s.scene.NextStage != s.scene.Stage {
			s.scene.Stage = s.scene.NextStage
		}
		s.scene.NextStage = 0
	}

	s.isFreeRound = s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli

	if s.scene.Steps == 0 {
		s.scene.RoundMultiplier = 0

		// 每局开始判断是否解锁英雄id
		if ok, id := s.cityUnlockHero(s.scene.CityValue); ok && id >= _heroID1 && id <= _heroID8 {
			s.scene.HeroID = id
		}
	}
}
