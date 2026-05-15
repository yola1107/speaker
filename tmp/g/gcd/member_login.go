package gcd

import (
	"egame-grpc/game/common"
	gcdpb "egame-grpc/game/gcd/pb"
	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/pb"
	"egame-grpc/utils/jsonx"

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
	case isNilFromCache(err):
		return "", nil
	case scenes == nil:
		return "", nil
	default:
		global.GVA_LOG.Error("memberLogin", zap.Error(err))
		return "", internalServerError
	}
	lastOrder, _, err := scenes.GetLastOrder()
	switch {
	case err != nil:
		global.GVA_LOG.Error("memberLogin", zap.Error(err))
		return "", internalServerError
	case lastOrder == nil:
		common.RetCleanScene(s.client, req.MemberId, GameID)
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
		return "", internalServerError
	}
	if scene, err := loadScene(s.req.MemberId); err != nil {
		global.GVA_LOG.Warn("doMemberLogin: load scene failed", zap.Error(err))
	} else if scene != nil {
		applySceneToLoginResponse(resp, scene)
	}
	respByte, _ := jsonx.Marshal(resp)
	var orderMap map[string]any
	if err = jsonx.Unmarshal(respByte, &orderMap); err != nil {
		global.GVA_LOG.Error("doMemberLogin", zap.Error(err))
		return "", internalServerError
	}
	orderMap["lastOrder"] = s.lastOrder
	delete(orderMap, "balance")
	data, err := jsonx.MarshalString(orderMap)
	if err != nil {
		global.GVA_LOG.Error("doMemberLogin", zap.Error(err))
		return "", internalServerError
	}
	return data, nil
}

func applySceneToLoginResponse(resp *gcdpb.Gcd_BetOrderResponse, scene *SpinSceneData) {
	state := scene.Stage
	if scene.NextStage > 0 {
		state = scene.NextStage
	}

	resp.IsFree = proto.Bool(state == _freeMode || state == _freeModeEli)
	resp.RemainingFreeTimes = proto.Int64(scene.FreeNum)
	resp.TotalFreeTimes = proto.Int64(scene.FreeNum)
	resp.BonusState = proto.Int64(scene.BonusState)
	resp.FreeType = proto.Int64(scene.FreeType)
}
