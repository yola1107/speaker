package yhwy

import (
	"errors"

	"egame-grpc/game/common"
	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/pb"
	"egame-grpc/utils/json"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type memberLoginService struct {
	req       *pb.LoginStreamReq
	client    *client.Client
	lastOrder *game.GameOrder
}

func newMemberLoginService() *memberLoginService {
	return &memberLoginService{}
}

func (s *memberLoginService) memberLogin(req *pb.LoginStreamReq, c *client.Client) (string, error) {
	s.req = req
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
		common.RetCleanScene(scenes, req.MemberId, GameID)
		return "", nil
	}
	lastOrder, _, err := scenes.GetLastOrder()
	switch {
	case err != nil:
		global.GVA_LOG.Error("memberLogin", zap.Error(err))
		return "", InternalServerError
	case lastOrder == nil:
		common.RetCleanScene(scenes, req.MemberId, GameID)
		return "", nil
	default:
		s.lastOrder = lastOrder
	}

	return s.doMemberLogin()
}

func (s *memberLoginService) doMemberLogin() (string, error) {
	if s.lastOrder == nil {
		return "", nil
	}
	resp, err := gameOrderToResponse(s.lastOrder)
	if err != nil {
		global.GVA_LOG.Error("doMemberLogin", zap.Error(err))
		return "", InternalServerError
	}
	respByte, _ := json.CJSON.Marshal(resp)

	var orderMap map[string]any
	if err := json.CJSON.Unmarshal(respByte, &orderMap); err != nil {
		global.GVA_LOG.Error("doMemberLogin", zap.Error(err))
		return "", InternalServerError
	}
	orderMap["lastOrder"] = s.lastOrder
	delete(orderMap, "balance")
	data, err := json.CJSON.MarshalToString(orderMap)
	if err != nil {
		global.GVA_LOG.Error("doMemberLogin", zap.Error(err))
		return "", InternalServerError
	}
	return data, nil
}
