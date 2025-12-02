package mahjong4

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/utils/json"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type Selector struct{}

func NewSelector() *Selector {
	return &Selector{}
}

// BetBonus 客户端选择免费游戏类型
func (g *Selector) BetBonus(req *request.BetBonusReq) (result map[string]any, err error) {
	defer func() {
		if r := recover(); r != nil {
			global.GVA_LOG.Error("BetBonus", zap.Any("r", r))
			debug.PrintStack()
			result, err = nil, InternalServerError
			return
		}
	}()
	s := newBetOrderService()
	memberId := req.MemberId
	player, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("玩家对象不存在")
		return nil, fmt.Errorf("player not exist")
	}
	var c *client.Client
	c, err = player.ClientGameCache.GetScenes(player)
	if err != nil {
		global.GVA_LOG.Error("获取场景数据出错", zap.Any("err:", err))
		return nil, fmt.Errorf("client not exist")
	}
	s.client = c
	key := fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, memberId)
	v := global.GVA_REDIS.Get(context.Background(), key).Val()
	if !common.Containers([]int{1, 2, 3}, int(req.BonusNum)) {
		return nil, InvalidRequestParams
	}
	if len(v) > 0 {
		err := jsoniter.UnmarshalFromString(v, &s.scene)
		if err != nil {
			global.GVA_LOG.Error("jsoniter.UnmarshalFromString", zap.Error(err))
			return nil, err
		}
		if s.scene.BonusState == _bonusStatePending && s.scene.BonusNum <= 0 {
			// 设置 BonusNum 并计算免费次数
			freeTimes := s.setupBonusNumAndFreeTimes(s.scene.ScatterNum, int(req.BonusNum))
			sceneStr, _ := jsoniter.MarshalToString(s.scene)
			global.GVA_REDIS.Set(context.Background(), key, sceneStr, 90*24*time.Hour)

			s.client.BetLock.Lock()
			defer s.client.BetLock.Unlock()

			lastOrder, ok, err := s.client.GetLastOrder()
			if !ok && err != nil {
				global.GVA_LOG.Error("投注失败", zap.Error(err))
				return nil, fmt.Errorf("internal error")
			}
			s.client.ClientOfFreeGame.SetFreeType(uint64(req.BonusNum))
			s.client.ClientOfFreeGame.SetFreeTimes(0)
			s.client.ClientOfFreeGame.SetFreeNum(uint64(freeTimes))

			s.client.ClientOfFreeGame.ResetBonusState()
			s.client.ClientOfFreeGame.IncrBonusState()
			client.GVA_CLIENT_BUCKET.SaveClient(s.client)
			s.client.ClientGameCache.SaveScenes(s.client)
			UpdateExtraCache(req, lastOrder, freeTimes)
			ret := map[string]any{
				"free":           req.BonusNum, // 奖金游戏类型 - 请求的奖金游戏类型编号
				"freeNum":        freeTimes,    // 剩余免费次数 - 玩家还剩余的免费游戏次数
				"freeTotalMoney": 0,            // 免费游戏总赢取金额 - 当前固定为0，表示未开始免费游戏
			}
			return ret, nil
		}
		global.GVA_LOG.Error("没有查到缓存")
		return nil, fmt.Errorf("没有查到缓存")
	}
	return nil, InternalServerError
}

func UpdateExtraCache(req *request.BetBonusReq, lastOrder *game.GameOrder, huCount int) {
	// 复盘数据逻辑调整
	key := fmt.Sprintf("%s:%d:%d:%d:lastBetRecord", global.GVA_CONFIG.System.Site,
		lastOrder.MerchantID, lastOrder.MemberID, req.GameId)

	index := req.GameId % int64(len(global.GVA_ORDER_LIST))
	redisTemp := global.GVA_ORDER_LIST[index]
	if redisTemp == nil {
		global.GVA_LOG.Error("获取redis游戏对象失败")
		return
	}

	orderBytes, err := redisTemp.Get(context.Background(), key).Result()
	if err != nil {
		global.GVA_LOG.Error(fmt.Sprintf("读取玩家最后一次下注记录失败,游戏ID:%d，商户ID:%d,玩家ID: %d",
			req.GameId, req.MerchantId, req.MemberId), zap.Error(err))
		return
	}

	if len(orderBytes) == 0 {
		global.GVA_LOG.Error("读取玩家最后一次下注记录失败", zap.String("error", "lastBetRecord not exist"))
		return
	}

	var orderTemp map[string]any
	if err = json.CJSON.Unmarshal([]byte(orderBytes), &orderTemp); err != nil {
		global.GVA_LOG.Error("序列化玩家最后一次下注记录失败", zap.Error(err))
		return
	}
	orderTemp["free"] = 1
	orderTemp["freeNum"] = huCount
	orderTemp["bonusState"] = 2
	orderBytesNew, err := json.CJSON.Marshal(orderTemp)
	err = redisTemp.Set(context.Background(), key, string(orderBytesNew), time.Duration(90*3600*24*int64(time.Second))).Err()
	if err != nil {
		global.GVA_LOG.Error("存储玩家最后一次下注记录失败", zap.Error(err))
	}
}
