package mahjong

import (
	"context"
	"egame-grpc/global"
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type SpinSceneData struct {
	Steps           uint64                  `json:"steps"`   // step步数，也是连赢次数
	Stage           int8                    `json:"stage"`   //运行阶段
	RoundMultiplier int64                   `json:"rMul"`    // 回合倍数
	SpinMultiplier  int64                   `json:"sMul"`    // Spin倍数
	FreeMultiplier  int64                   `json:"fMul"`    // 免费倍数
	NextStage       int8                    `json:"nStage"`  // 下一阶段
	RoundOver       bool                    `json:"rOver"`   // 回合是否已结束
	IsFreeRound     bool                    `json:"free"`    // 是否为免费回合
	GameWinMultiple int64                   `json:"fWMul"`   // 免费倍数，初始1倍
	RemoveNum       int64                   `json:"rNum"`    //免费游戏中奖消除次数
	SymbolRoller    [_colCount]SymbolRoller `json:"sRoller"` // 滚轮符号表
}

func (s *betOrderService) cleanScene() {
	key := fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
	global.GVA_REDIS.Del(context.Background(), key)

}

// 加载场景数据
func (s *betOrderService) reloadScene() bool {
	s.scene = &SpinSceneData{}

	err := s.loadCacheSceneData()
	if err != nil {
		global.GVA_LOG.Error("reloadScene", zap.Error(err))
		s.cleanScene()
		return false
	}

	s.scene.Stage = _spinTypeBase
	if s.scene.IsFreeRound {
		s.scene.Stage = _spinTypeFree
	}

	//新免费回合开始
	if s.scene.NextStage > 0 && s.scene.NextStage != s.scene.Stage {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}

	//免费回合开始
	if s.scene.Stage == _spinTypeFree {
		s.scene.IsFreeRound = true
	}
	s.removeNum = s.scene.RemoveNum
	s.gameMultiple = s.scene.GameWinMultiple
	if s.scene.Stage == _spinTypeBase {
		s.scene.IsFreeRound = false
	}

	return true
}

// 保存场景
func (s *betOrderService) saveScene() error {

	sceneStr, _ := jsoniter.MarshalToString(s.scene)

	key := fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)

	if err := global.GVA_REDIS.Set(context.Background(), key, sceneStr, time.Hour*24*90).Err(); err != nil {
		global.GVA_LOG.Error("saveScene", zap.Error(err))
		return err
	}
	return nil

}

func (s *betOrderService) loadCacheSceneData() error {
	key := fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
	v := global.GVA_REDIS.Get(context.Background(), key).Val()

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
func (s *betOrderService) isFreeRound() bool {
	return s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli
}
