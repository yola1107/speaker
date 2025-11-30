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
	Steps           uint64                  `json:"steps"`      // step步数
	Stage           int8                    `json:"stage"`      // 运行阶段
	NextStage       int8                    `json:"nStage"`     // 下一阶段
	FreeNum         int64                   `json:"freeNum"`    // 剩余免费次数（独立统计，不依赖client）
	BonusNum        int                     `json:"bonusNum"`   // 免费游戏类型（1/2/3）
	ScatterNum      int64                   `json:"scatterNum"` // 夺宝符数量（触发免费游戏时的数量）
	BonusState      int                     `json:"bonusState"` // 免费游戏选择状态：1-需要选择，2-已选择
	ContinueNum     int64                   `json:"cNum"`       // 连续消除次数
	RoundMultiplier int64                   `json:"rMul"`       // 回合倍数
	SymbolRoller    [_colCount]SymbolRoller `json:"sRoller"`    // 滚轮符号表
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

// handleStageTransition 处理状态跳转和验证
func (s *betOrderService) handleStageTransition() {
	// 初始化 Stage（如果是首次或未设置）
	if s.scene.Stage == 0 {
		s.scene.Stage = _spinTypeBase
	}

	// 处理状态切换：如果 NextStage 已设置且与当前 Stage 不同，则切换
	if s.scene.NextStage > 0 {
		if s.scene.NextStage != s.scene.Stage {
			s.scene.Stage = s.scene.NextStage
		}
		// 无论是否切换，都清零 NextStage（避免状态混淆）
		s.scene.NextStage = 0
	}

	// 根据当前 Stage 设置 isFreeRound
	s.isFreeRound = s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli

	if s.scene.Steps == 0 {
		s.scene.RoundMultiplier = 0
	}
}
