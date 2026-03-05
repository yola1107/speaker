package xxg2

import (
	"errors"
	"runtime/debug"

	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game/request"
	"egame-grpc/model/pb"

	"go.uber.org/zap"
)

var (
	InternalServerError  = errors.New("internal server error")
	InvalidRequestParams = errors.New("invalid request params")
	InsufficientBalance  = errors.New("insufficient balance")
)

type Game struct{}

func NewGame() *Game {
	return &Game{}
}

// BetOrder 用户下注
func (g *Game) BetOrder(req *request.BetOrderReq) (result map[string]any, err error) {
	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			global.GVA_LOG.Error("BetOrder", zap.Any("r", r), zap.Stack("stack"))
			result, err = nil, InternalServerError
			return
		}
	}()

	svc := newBetOrderService(false)
	ret, err := svc.betOrder(req)
	if err != nil {
		global.GVA_LOG.Error("BetOrder", zap.Any("req", req), zap.Error(err))
	}
	return ret, err
}

// MemberLogin 用户登录
func (g *Game) MemberLogin(req *pb.LoginStreamReq, c *client.Client) (result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			global.GVA_LOG.Error("MemberLogin", zap.Any("r", r), zap.Stack("stack"))
			result, err = "", InternalServerError
			return
		}
	}()
	return newMemberLoginService().memberLogin(req, c)
}
