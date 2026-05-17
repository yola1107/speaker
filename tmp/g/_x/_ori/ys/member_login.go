package ys

import (
	"errors"

	"egame-grpc/game/common"
	yspb "egame-grpc/game/ys/pb"
	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/pb"
	"egame-grpc/utils/jsonx"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
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
	if scene, err := loadScene(s.req.MemberId); err != nil {
		global.GVA_LOG.Warn("doMemberLogin: load scene failed", zap.Error(err))
	} else if scene != nil {
		applySceneToLoginResponse(resp, scene)
	}
	respByte, _ := jsonx.Marshal(resp)

	var orderMap map[string]any
	if err := jsonx.Unmarshal(respByte, &orderMap); err != nil {
		global.GVA_LOG.Error("doMemberLogin", zap.Error(err))
		return "", InternalServerError
	}
	orderMap["lastOrder"] = s.lastOrder
	delete(orderMap, "balance")
	data, err := jsonx.MarshalString(orderMap)
	if err != nil {
		global.GVA_LOG.Error("doMemberLogin", zap.Error(err))
		return "", InternalServerError
	}
	return data, nil
}

func applySceneToLoginResponse(resp *yspb.Ys_BetOrderResponse, scene *SpinSceneData) {
	state := int64(scene.Stage)
	if scene.NextStage > 0 {
		state = int64(scene.NextStage)
	}
	resp.IsFree = proto.Bool(state == _spinTypeFree || state == _spinTypeFreeEli)
	resp.RemainingFreeTimes = proto.Int64(scene.FreeNum)
	resp.TotalFreeTimes = proto.Int64(scene.FreeTimes + scene.FreeNum)
	resp.FreeTotalWin = proto.Float64(scene.FreeWin)
	resp.RoundWin = proto.Float64(scene.RoundWin)
	resp.BonusState = proto.Int64(scene.BonusState)
	resp.BonusNum = proto.Int64(scene.BonusNum)
}
