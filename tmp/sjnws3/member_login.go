package sjnws3

import (
	"context"
	"errors"
	"fmt"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/pb"
	"egame-grpc/utils/json"

	"github.com/go-redis/redis/v8"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type memberLoginService struct {
	req        *pb.LoginStreamReq
	orderRedis *redis.Client
	client     *client.Client
	lastOrder  *game.GameOrder
}

// 生成登录服务实例
func newMemberLoginService() *memberLoginService {
	return &memberLoginService{}
}

// 登录逻辑
func (s *memberLoginService) memberLogin(req *pb.LoginStreamReq, c *client.Client) (string, error) {

	s.req = req
	s.selectOrderRedis()

	c.BetLock.Lock()
	defer c.BetLock.Unlock()
	scenes, err := c.ClientGameCache.GetScenes(c)

	switch {
	case err == nil:
		s.client = scenes
	case errors.Is(err, redis.Nil):
		return "", nil
	case scenes == nil:
		return "", nil
	default:
		global.GVA_LOG.Error("memberLogin", zap.Error(err))
		return "", InternalServerError
	}
	lastOrder, _, err := scenes.GetLastOrder()
	switch {
	case err != nil:
		global.GVA_LOG.Error("memberLogin", zap.Error(err))
		return "", InternalServerError
	case lastOrder == nil:
		return "", nil
	default:
		s.lastOrder = lastOrder
	}
	return s.doMemberLogin()
}

// 初始化订单redis
func (s *memberLoginService) selectOrderRedis() {
	index := _gameID % int64(len(global.GVA_ORDER_LIST))
	s.orderRedis = global.GVA_ORDER_LIST[index]
}

// 登录核心逻辑
func (s *memberLoginService) doMemberLogin() (string, error) {
	//global.GVA_LOG.Info("登陆")
	s.LoginCheck(s.req)
	site := global.GVA_CONFIG.System.Site
	merchantID, memberID := s.req.MerchantId, s.req.MemberId
	key := fmt.Sprintf("%v:%v:%v:%v:lastBetRecord", site, merchantID, memberID, _gameID)
	orderBytes, err := s.orderRedis.Get(context.Background(), key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		global.GVA_LOG.Error("doMemberLogin", zap.Error(err))
		return "", InternalServerError
	}
	if len(orderBytes) == 0 {
		return "", nil
	}
	var freeData map[string]any
	if err = json.CJSON.Unmarshal([]byte(orderBytes), &freeData); err != nil {
		global.GVA_LOG.Error("doMemberLogin", zap.Error(err))
		return "", InternalServerError
	}

	//freeData["lastOrder"] = s.lastOrder
	freeData = s.SetFreeData(freeData, s.lastOrder)
	data, err := json.CJSON.MarshalToString(freeData)
	if err != nil {
		global.GVA_LOG.Error("doMemberLogin", zap.Error(err))
		return "", InternalServerError
	}
	return data, nil
}

// 登录核心逻辑
func (m *memberLoginService) LoginCheck(req *pb.LoginStreamReq) {
	s := newBetOrderService()
	memberId := req.MemberId
	player, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("玩家对象不存在")
		return
	}
	var c *client.Client
	c, err := player.ClientGameCache.GetScenes(player)
	if err != nil {
		global.GVA_LOG.Error("获取场景数据出错", zap.Any("err:", err))
		return
	}
	s.client = c
	key := getSceneKey(memberId)
	v := global.GVA_REDIS.Get(context.Background(), key).Val()
	if len(v) > 0 {
		err = jsoniter.UnmarshalFromString(v, &s.scene)
		if err != nil {
			global.GVA_LOG.Error("jsoniter.UnmarshalFromString", zap.Error(err))
		}
		if s.scene.BonusState == _freeGame && s.scene.BonusNum > 0 {
			s.scene.BonusNum = 0
			sceneStr, _ := jsoniter.MarshalToString(s.scene)
			global.GVA_REDIS.Set(context.Background(), key, sceneStr, 90*24*time.Hour)
		}
		if s.scene.IsRespin && s.scene.BonusState == _normalGame &&
			!common.Containers([]int{1, 2, 3}, s.scene.BonusNum) {
			s.reSetCleanData()
		}
	}
	//global.GVA_LOG.Info("没有查到缓存")
}

func (s *memberLoginService) SetFreeData(freeData map[string]any, GameOrder *game.GameOrder) map[string]any {
	scene := s.getSceneInfo()
	freeData = map[string]any{
		"cards":          scene.LastGrid,
		"free":           scene.LastRepin,
		"freeNum":        scene.LastFreeTimes,
		"freeTotalMoney": scene.LastFreeAmount,
		"win":            scene.LastWin,
		"wincards":       scene.LastWinGrid,
		"freeClean":      0,
		"betMoney":       scene.BetAmount,
		"bonusState":     scene.LastBonusState, // 得到奖金选择状态 1 触发奖金档位选择页面
		"betAmount":      scene.BetAmount,
		"bonusMultiple":  scene.LastContinueMulti,
		"treasureNum":    scene.LastTreasureNum,
		"freeTimes":      scene.LastFreeTimes,
		"freeType":       scene.LastBonusNum,
		"headBonusLine":  0,
		"maxFreeNum":     scene.LastMaxFreeTimes,
		"totalWin":       scene.LastTotalWin,
		"lastOrder":      GameOrder,
		//"randBrand":      ParseGridToSlice(scene.LastGrid),
	}
	zap.L().Info("登陆获取数据", zap.Any("freeData", freeData))
	return freeData

}
func ParseGridToSlice(gird [4][5]int) []int {
	list := make([]int, 0)
	for y := 0; y < _rowCount; y++ {
		for x := 0; x < _colCount; x++ {
			list = append(list, gird[y][x])
		}
	}
	global.GVA_LOG.Debug("转切片:", zap.Any("list", list))
	return list
}
