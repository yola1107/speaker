package mahjong

import (
	"context"
	"fmt"
	"time"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type SpinSceneData struct {
	Steps           uint64                  `json:"steps"`   // step步数，也是连赢次数
	Stage           int8                    `json:"stage"`   // 运行阶段
	NextStage       int8                    `json:"nStage"`  // 下一阶段
	RoundMultiplier int64                   `json:"rMul"`    // 回合倍数（必须持久化：Round进行中断线时需要保留累加值）
	GameWinMultiple int64                   `json:"fWMul"`   // 游戏中奖倍数，初始1倍（必须持久化：避免动态计算出错）
	RemoveNum       int64                   `json:"rNum"`    // 免费游戏中奖消除次数
	SymbolRoller    [_colCount]SymbolRoller `json:"sRoller"` // 滚轮符号表
}

var sceneDataKeyPrefix = fmt.Sprintf("scene-%d", _gameID)

func (s *betOrderService) sceneKey() string {
	return fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
}

func (s *betOrderService) cleanScene() {
	global.GVA_REDIS.Del(context.Background(), s.sceneKey())
}

func (s *betOrderService) reloadScene() bool {
	s.scene = &SpinSceneData{}

	if err := s.loadCacheSceneData(); err != nil {
		global.GVA_LOG.Error("reloadScene", zap.Error(err))
		s.cleanScene()
		return false
	}

	if s.scene.Stage == 0 {
		s.scene.Stage = _spinTypeBase
	}

	if s.scene.NextStage > 0 && s.scene.NextStage != s.scene.Stage {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}

	s.removeNum = s.scene.RemoveNum
	if s.scene.GameWinMultiple == 0 {
		s.gameMultiple = 1
		s.scene.GameWinMultiple = 1
	} else {
		s.gameMultiple = s.scene.GameWinMultiple
	}

	s.isRoundOver = s.scene.Steps == 0
	// isFreeRound：根据 Stage 判断
	s.isFreeRound = s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli

	return true
}

func (s *betOrderService) saveScene() error {
	sceneStr, _ := jsoniter.MarshalToString(s.scene)
	if err := global.GVA_REDIS.Set(context.Background(), s.sceneKey(), sceneStr, time.Hour*24*90).Err(); err != nil {
		global.GVA_LOG.Error("saveScene", zap.Error(err))
		return err
	}
	return nil
}

func (s *betOrderService) loadCacheSceneData() error {
	v := global.GVA_REDIS.Get(context.Background(), s.sceneKey()).Val()
	if len(v) > 0 {
		tmpSc := &SpinSceneData{}
		err := jsoniter.UnmarshalFromString(v, &tmpSc)
		if err != nil {
			return err
		}
		s.scene = tmpSc
	}
	return nil
}

func (s *betOrderService) isBaseRound() bool {
	return s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeBaseEli
}
