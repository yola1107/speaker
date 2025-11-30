package sjnws3

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

	"github.com/go-redis/redis/v8"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type SJNWSelector struct {
}

func NewGameSJNWSelector() *SJNWSelector {
	return &SJNWSelector{}
}

func (game *SJNWSelector) BetBonus(req *request.BetBonusReq) (result map[string]any, err error) {
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
	key := getSceneKey(memberId)
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
		if s.scene.BonusState == _freeGame && s.scene.BonusNum <= 0 {
			s.scene.BonusNum = int(req.BonusNum)
			s.scene.LastBonusNum = int(req.BonusNum)
			s.scene.BonusState = _normalGame
			s.scene.LastBonusState = _normalGame
			s.scene.ContinueMulti = 1
			s.scene.LastContinueMulti = 1
			s.scene.LastRepin = 1
			mulkey := int(req.BonusNum)
			value, ok := s.bonusMap[mulkey]
			if !ok {
				global.GVA_LOG.Error("值不到对应选择的倍数", zap.Error(err))
				return nil, err
			}
			s.scene.AddFreeTimes = value.Times + (s.scene.ScatterNum-_minScatterNum)*value.AddTimes
			s.scene.FreeTimes = value.Times + (s.scene.ScatterNum-_minScatterNum)*value.AddTimes
			s.scene.AddFreeTimes = s.scene.FreeTimes
			s.scene.MaxFreeTimes = s.scene.FreeTimes
			s.scene.LastFreeTimes = s.scene.FreeTimes
			s.scene.LastMaxFreeTimes = s.scene.MaxFreeTimes

			sceneStr, _ := jsoniter.MarshalToString(s.scene)
			global.GVA_REDIS.Set(context.Background(), key, sceneStr, 90*24*time.Hour)
			/*
				下面是老版本代码中中的逻辑
			*/
			// 投注冥等性，同一时间，只能一个投注
			s.client.BetLock.Lock()
			defer s.client.BetLock.Unlock()

			//上一次的注单结果
			lastOrder, ok, err := s.client.GetLastOrder()
			//redis出错问题
			if !ok && err != nil {
				global.GVA_LOG.Error("投注失败", zap.Error(err))
				return nil, fmt.Errorf("internal error")
			}
			s.client.ClientOfFreeGame.SetFreeType(uint64(req.BonusNum))
			s.client.ClientOfFreeGame.SetFreeTimes(0)
			s.client.ClientOfFreeGame.SetFreeNum(uint64(s.scene.MaxFreeTimes))

			s.client.ClientOfFreeGame.ResetBonusState()
			s.client.ClientOfFreeGame.IncrBonusState()
			// 保存grpc客户端
			client.GVA_CLIENT_BUCKET.SaveClient(s.client)
			s.client.ClientGameCache.SaveScenes(s.client)
			UpdateExtraCahe(req, lastOrder, s.scene.FreeTimes)
			return s.getBonusDetail(req, s.scene.FreeTimes), nil
		}
		global.GVA_LOG.Error("没有查到缓存")
		return nil, fmt.Errorf("没有查到缓存")
	}
	return nil, InternalServerError
}

func UpdateExtraCahe(req *request.BetBonusReq,
	lastOrder *game.GameOrder, huCount int) {

	// 复盘数据逻辑调整
	key := fmt.Sprintf("%s:%d:%d:%d:lastBetRecord", global.GVA_CONFIG.System.Site,
		lastOrder.MerchantID, lastOrder.MemberID, req.GameId)
	redisTemp := SelectRedis(req.GameId)
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
	err = json.CJSON.Unmarshal([]byte(orderBytes), &orderTemp)
	if err != nil {
		global.GVA_LOG.Error("序列化玩家最后一次下注记录失败", zap.Error(err))
		return
	}
	orderTemp["free"] = 1
	orderTemp["freeNum"] = huCount
	orderTemp["bonusState"] = 2
	orderTemp["cards"] = getDefaultCardsFor8890()
	orderBytesNew, err := json.CJSON.Marshal(orderTemp)
	// 保存复盘数据到REDIS中
	err = redisTemp.Set(context.Background(), key, string(orderBytesNew), time.Duration(90*3600*24*int64(time.Second))).Err()
	if err != nil {
		global.GVA_LOG.Error("存储玩家最后一次下注记录失败", zap.Error(err))
	}
}

// 获取默认牌型
func getDefaultCardsFor8890() []int {
	return []int{
		6, 2, 5, 2, 6,
		1, 8, 9, 8, 1,
		4, 7, 3, 7, 4,
		6, 9, 8, 9, 6,
	}
}
func getInitCardsFor8890() [4][5]int {
	return [4][5]int{
		{6, 2, 5, 2, 6},
		{1, 8, 9, 8, 1},
		{4, 7, 3, 7, 4},
		{6, 9, 8, 9, 6},
	}
}

// 根据gameId,选择不同的Redis-db
func SelectRedis(gameId int64) *redis.Client {
	index := gameId % int64(len(global.GVA_ORDER_LIST))
	return global.GVA_ORDER_LIST[index]
}
