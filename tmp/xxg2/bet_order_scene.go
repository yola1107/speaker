package xxg2

import (
	"context"
	"fmt"
	"time"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

// scene 场景数据结构（参考 mahjong 的 SpinSceneData，保持 xxg2 所需字段）
type scene struct {
	LastPresetID     uint64      `json:"lastPresetID"`     // 上一条预设数据的 id，预设数据最后一个 step 设置为0（已废弃）
	LastStepID       uint64      `json:"lastStepID"`       // 上一个 step 的 id，预设数据最后一个 step 设置为0（已废弃）
	SpinBonusAmount  float64     `json:"spinBonusAmount"`  // spin 奖金金额
	RoundBonusAmount float64     `json:"roundBonusAmount"` // round 奖金金额
	FreeNum          uint64      `json:"freeNum"`          // 免费次数
	FreeTimes        uint64      `json:"freeTimes"`        // 免费回合编号
	FreeTotalMoney   float64     `json:"freeTotalMoney"`   // 免费总金额
	LastMaxFreeNum   uint64      `json:"lastMaxFreeNum"`   // 上一次最大免费次数
	BatPositions     []*position `json:"batPositions"`     // 蝙蝠当前位置（免费模式持续追踪）
}

// cleanScene 清理场景数据
func (s *betOrderService) cleanScene() {
	key := fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
	global.GVA_REDIS.Del(context.Background(), key)
}

// reloadScene 加载场景数据
func (s *betOrderService) reloadScene() bool {
	s.scene = scene{}

	err := s.loadCacheSceneData()
	if err != nil {
		global.GVA_LOG.Error("reloadScene", zap.Error(err))
		s.cleanScene()
		return false
	}

	// 从 scene 恢复到 client（xxg2 特有逻辑）
	if s.scene.LastPresetID > 0 || s.scene.LastStepID > 0 {
		s.client.ClientOfFreeGame.SetLastWinId(s.scene.LastPresetID)
		s.client.ClientOfFreeGame.SetLastMapId(s.scene.LastStepID)
	}
	if s.scene.SpinBonusAmount > 0 {
		s.client.ClientOfFreeGame.GeneralWinTotal = s.scene.SpinBonusAmount
	}
	if s.scene.RoundBonusAmount > 0 {
		s.client.ClientOfFreeGame.RoundBonus = s.scene.RoundBonusAmount
	}
	if s.scene.FreeNum > 0 {
		s.client.ClientOfFreeGame.SetFreeNum(s.scene.FreeNum)
	}
	if s.scene.FreeTotalMoney > 0 {
		s.client.ClientOfFreeGame.FreeTotalMoney = s.scene.FreeTotalMoney
	}
	if s.scene.LastMaxFreeNum > 0 {
		s.client.SetLastMaxFreeNum(s.scene.LastMaxFreeNum)
	}
	if s.scene.FreeTimes > 0 {
		s.client.ClientOfFreeGame.SetFreeTimes(s.scene.FreeTimes)
	}

	return true
}

// saveScene 保存场景数据
func (s *betOrderService) saveScene() error {
	// 从 client 保存数据到 scene
	s.scene.LastPresetID = 0 // 使用配置 RealData，不使用预设数据
	s.scene.LastStepID = 0   // 使用配置 RealData，不使用预设step
	s.scene.SpinBonusAmount = s.client.ClientOfFreeGame.GetGeneralWinTotal()
	s.scene.RoundBonusAmount = s.client.ClientOfFreeGame.RoundBonus
	s.scene.FreeNum = s.client.ClientOfFreeGame.GetFreeNum()
	s.scene.FreeTotalMoney = s.client.ClientOfFreeGame.GetFreeTotalMoney()
	s.scene.LastMaxFreeNum = s.client.GetLastMaxFreeNum()
	s.scene.FreeTimes = s.client.ClientOfFreeGame.GetFreeTimes()

	// 保存蝙蝠位置（免费模式持续追踪）
	s.scene.BatPositions = s.getBatPositionsFromBats()

	// 序列化并保存到 Redis
	sceneStr, _ := jsoniter.MarshalToString(s.scene)

	key := fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)

	if err := global.GVA_REDIS.Set(context.Background(), key, sceneStr, time.Hour*24*90).Err(); err != nil {
		global.GVA_LOG.Error("saveScene", zap.Error(err))
		return err
	}
	return nil
}

// getBatPositionsFromBats 从Bat数据中提取蝙蝠的新位置
func (s *betOrderService) getBatPositionsFromBats() []*position {
	if s.stepMap == nil || len(s.stepMap.Bat) == 0 {
		return nil
	}

	var positions []*position
	for _, bat := range s.stepMap.Bat {
		// 使用移动后的位置（TransX, TransY）
		positions = append(positions, &position{
			Row: bat.TransX,
			Col: bat.TransY,
		})
	}

	return positions
}

// loadCacheSceneData 从 Redis 加载场景数据
func (s *betOrderService) loadCacheSceneData() error {
	key := fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
	v := global.GVA_REDIS.Get(context.Background(), key).Val()

	if len(v) > 0 {
		tmpSc := &scene{}
		err := jsoniter.UnmarshalFromString(v, &tmpSc)
		if err != nil {
			return err
		}
		s.scene = *tmpSc
	}
	return nil
}
