package mahjong

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game/request"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

// BetBonus 客户端选择免费游戏类型
func (g *Game) BetBonus(req *request.BetBonusReq) (result map[string]any, err error) {
	defer func() {
		if r := recover(); r != nil {
			global.GVA_LOG.Error("BetBonus", zap.Any("r", r))
			debug.PrintStack()
			result, err = nil, InternalServerError
			return
		}
	}()

	// 验证 BonusNum 范围
	if !common.Containers([]int{1, 2, 3}, int(req.BonusNum)) {
		return nil, InvalidRequestParams
	}

	// 获取客户端
	player, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("玩家对象不存在")
		return nil, fmt.Errorf("player not exist")
	}

	c, err := player.ClientGameCache.GetScenes(player)
	if err != nil {
		global.GVA_LOG.Error("获取场景数据出错", zap.Any("err", err))
		return nil, fmt.Errorf("client not exist")
	}

	// 创建服务实例
	s := newBetOrderService()
	s.client = c
	s.req = &request.BetOrderReq{
		MemberId: req.MemberId,
		GameId:   req.GameId,
	}

	// 获取请求上下文（商户、用户、游戏信息）
	if !s.getRequestContext() {
		return nil, InternalServerError
	}

	// 加载场景数据
	key := s.sceneKey()
	v := global.GVA_REDIS.Get(context.Background(), key).Val()
	if len(v) == 0 {
		global.GVA_LOG.Error("没有查到缓存")
		return nil, fmt.Errorf("scene not found")
	}
	if err = jsoniter.UnmarshalFromString(v, &s.scene); err != nil {
		global.GVA_LOG.Error("解析场景数据失败", zap.Error(err))
		return nil, err
	}

	// 检查状态是否允许选择
	if s.scene.BonusState != _bonusStatePending {
		global.GVA_LOG.Error("BonusState 不允许选择",
			zap.Int("bonusState", s.scene.BonusState),
			zap.Int("bonusNum", s.scene.BonusNum),
			zap.Int64("scatterNum", s.scene.ScatterNum))
		return nil, fmt.Errorf("bonus state invalid: %d (expected %d)", s.scene.BonusState, _bonusStatePending)
	}

	// 检查是否可以选择 BonusNum（触发免费游戏但未选择类型）
	if s.scene.BonusNum > 0 {
		global.GVA_LOG.Error("BonusNum 已设置，无需重复选择", zap.Int("bonusNum", s.scene.BonusNum))
		return nil, fmt.Errorf("bonus already selected. bonusNum=%d", s.scene.BonusNum)
	}

	// 从场景数据获取夺宝符数量
	scatterCount := s.scene.ScatterNum
	if scatterCount < int64(s.gameConfig.FreeGameScatterMin) {
		global.GVA_LOG.Error("夺宝符数量不足，无法触发免费游戏",
			zap.Int64("scatterCount", scatterCount),
			zap.Int64("minRequired", int64(s.gameConfig.FreeGameScatterMin)))
		return nil, fmt.Errorf("insufficient scatter count")
	}

	// 设置 BonusNum 并计算免费次数
	freeTimes := s.setupBonusNumAndFreeTimes(scatterCount, int(req.BonusNum))

	// 保存场景数据
	sceneStr, err := jsoniter.MarshalToString(s.scene)
	if err != nil {
		global.GVA_LOG.Error("序列化场景数据失败", zap.Error(err))
		return nil, fmt.Errorf("marshal scene data failed: %w", err)
	}
	if err := global.GVA_REDIS.Set(context.Background(), key, sceneStr, time.Hour*24*90).Err(); err != nil {
		global.GVA_LOG.Error("保存场景数据失败", zap.Error(err))
		return nil, fmt.Errorf("save scene data failed: %w", err)
	}

	// 设置客户端状态
	c.BetLock.Lock()
	defer c.BetLock.Unlock()

	c.ClientOfFreeGame.SetFreeType(uint64(req.BonusNum))
	c.ClientOfFreeGame.SetFreeNum(uint64(freeTimes))
	c.ClientOfFreeGame.ResetBonusState()
	c.ClientOfFreeGame.IncrBonusState()

	// 保存客户端
	client.GVA_CLIENT_BUCKET.SaveClient(c)
	c.ClientGameCache.SaveScenes(c)

	return map[string]any{
		"free":    req.BonusNum,
		"freeNum": freeTimes,
	}, nil
}
