package sjnws3

import (
	"runtime/debug"

	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game/request"
	"egame-grpc/model/pb"

	"go.uber.org/zap"
)

type SJNW struct {
}

func NewGameSJNW() *SJNW {
	return &SJNW{}
}

func (g *SJNW) BetOrder(req *request.BetOrderReq) (result map[string]any, err error) {
	defer func() {
		if r := recover(); r != nil {
			global.GVA_LOG.Error("BetOrder", zap.Any("r", r))
			debug.PrintStack()
			result, err = nil, InternalServerError
			return
		}
	}()
	betService := newBetOrderService()
	resMap, err := betService.betOrder(req)
	if err != nil {
		return nil, err
	}
	Ret := betService.getWinDetailsMap(resMap)
	return Ret, nil
}

// MemberLogin 用户登录
func (g *SJNW) MemberLogin(req *pb.LoginStreamReq, c *client.Client) (result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			global.GVA_LOG.Error("MemberLogin", zap.Any("r", r))
			debug.PrintStack()
			result, err = "", InternalServerError
			return
		}
	}()
	return newMemberLoginService().memberLogin(req, c)
}
