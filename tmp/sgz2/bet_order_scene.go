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
	Steps            uint64                  `json:"steps"`       // step步数
	Stage            int8                    `json:"stage"`       // 运行阶段
	NextStage        int8                    `json:"nStage"`      // 下一阶段
	FreeNum          int64                   `json:"freeNum"`     // 剩余免费次数（独立统计，不依赖client）
	CityValue        int64                   `json:"CityValue"`   // 打赢新城池的战斗力累计值 用于解锁英雄ID
	ContinueNum      int64                   `json:"cNum"`        // 连续消除次数
	RoundMultiplier  int64                   `json:"rMul"`        // 回合倍数
	SymbolRoller     [_colCount]SymbolRoller `json:"sRoller"`     // 滚轮符号表
	IsRoundFirstStep bool                    `json:"isFirstStep"` // 是否为 Round 首 Step（免费次数扣减标志）

	FreeHeroID          int64 `json:"hID"`             // 免费游戏类型（英雄ID 1/2/3/4/5/6/7/8）
	LastMaxUnlockHero   int64 `json:"LastHeroID"`      // 上一次的英雄ID，用于判断是否解锁了新英雄
	CurrMaxUnlockHeroID int64 `json:"MaxUnlockHeroID"` // 当前最大解锁的英雄ID
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

	s.isFreeRound = s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli

	if s.scene.Steps == 0 {
		s.scene.RoundMultiplier = 0
	}

	if s.scene.CurrMaxUnlockHeroID <= 0 {
		s.scene.FreeHeroID = _heroID1
		s.scene.LastMaxUnlockHero = _heroID1
		s.scene.CurrMaxUnlockHeroID = _heroID1
	}
}
