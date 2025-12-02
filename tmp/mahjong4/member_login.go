package mahjong4

import (
	"context"
	"errors"
	"fmt"

	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/pb"
	"egame-grpc/utils/json"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type memberLoginService struct {
	req        *pb.LoginStreamReq
	orderRedis *redis.Client
	client     *client.Client
	lastOrder  *game.GameOrder
}

func newMemberLoginService() *memberLoginService {
	return &memberLoginService{}
}

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
	if scenes.ClientOfFreeGame.GetFreeClean() == 1 {
		return "", nil
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

func (s *memberLoginService) selectOrderRedis() {
	index := _gameID % int64(len(global.GVA_ORDER_LIST))
	s.orderRedis = global.GVA_ORDER_LIST[index]
}

func (s *memberLoginService) doMemberLogin() (string, error) {
	site := global.GVA_CONFIG.System.Site
	merchantID, memberID := s.req.MerchantId, s.req.MemberId
	key := fmt.Sprintf("%v:%v:%v:%v:lastBetRecord", site, merchantID, memberID, _gameID)

	orderBytes, err := s.orderRedis.Get(context.Background(), key).Result()
	if err != nil {
		global.GVA_LOG.Error("doMemberLogin", zap.Error(err))
		return "", InternalServerError
	}
	if len(orderBytes) == 0 {
		return "", nil
	}

	var orderMap map[string]any
	if err = json.CJSON.Unmarshal([]byte(orderBytes), &orderMap); err != nil {
		global.GVA_LOG.Error("doMemberLogin", zap.Error(err))
		return "", InternalServerError
	}

	orderMap["lastOrder"] = s.lastOrder
	data, err := json.CJSON.MarshalToString(orderMap)
	if err != nil {
		global.GVA_LOG.Error("doMemberLogin", zap.Error(err))
		return "", InternalServerError
	}
	return data, nil
}
