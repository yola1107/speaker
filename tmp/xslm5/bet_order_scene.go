package xslm3

import (
	"context"
	"fmt"
	"time"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

// SpinSceneData 场景数据（需要持久化的状态，与 xslm3 保持一致）
type SpinSceneData struct {
	// 新增字段
	Steps               uint64                  `json:"steps"`        // step步数，也是连赢次数
	Stage               int8                    `json:"stage"`        // 运行阶段
	NextStage           int8                    `json:"nStage"`       // 下一阶段
	FreeNum             int64                   `json:"freeNum"`      // 剩余免费次数（独立统计，不依赖client）
	TreasureNum         int64                   `json:"treasureNum"`  // 夺宝符号数量 (每局结束写入)
	FemaleCountsForFree [3]int64                `json:"femaleCounts"` // 女性符号计数
	SymbolRoller        [_colCount]SymbolRoller `json:"sRoller"`      // 滚轮符号表
}

var sceneDataKeyPrefix = fmt.Sprintf("scene-%d", _gameID)

func (s *betOrderService) sceneKey() string {
	return fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
}

// 加载场景数据
func (s *betOrderService) reloadScene() error {
	s.scene = new(SpinSceneData)

	if err := s.loadCacheSceneData(); err != nil {
		global.GVA_LOG.Error("reloadScene", zap.Error(err))
		s.cleanScene()
		return nil
	}

	return nil
}

func (s *betOrderService) cleanScene() {
	global.GVA_REDIS.Del(context.Background(), s.sceneKey())
}

// 保存场景
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
		tmpSc := new(SpinSceneData)
		if err := jsoniter.UnmarshalFromString(v, tmpSc); err != nil {
			return err
		}
		s.scene = tmpSc
	}
	return nil
}

// handleStageTransition 处理状态跳转
func (s *betOrderService) handleStageTransition() {
	/*
		s.scene.Stage = _spinTypeBase
		//新免费回合开始
		if s.scene.NextStage > 0 && s.scene.NextStage != s.scene.Stage {
			s.scene.Stage = s.scene.NextStage
			s.scene.NextStage = 0
		}
	*/

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

	// 根据当前 Stage 设置 isFreeRound（在状态切换后更新）
	s.isFreeRound = s.isFreeRoundStage()
}

// isBaseRoundStage 判断是否为基础模式阶段
func (s *betOrderService) isBaseRoundStage() bool {
	return s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeBaseEli
}

// isFreeRoundStage 判断是否为免费模式阶段
func (s *betOrderService) isFreeRoundStage() bool {
	return s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli
}
