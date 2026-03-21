package sbyymx2

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

// NewBetOrder 下注接口，返回 protobuf 与 JSON 字符串（与 hcsqy / NewGameBetMap 一致）
func (g *Game) NewBetOrder(req *request.BetOrderReq) (pbData []byte, result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			global.GVA_LOG.Error("NewBetOrder", zap.Any("r", r))
			debug.PrintStack()
			pbData, result, err = nil, "", InternalServerError
		}
	}()
	return newBetOrderService().betOrder(req)
}

func (g *Game) MemberLogin(req *pb.LoginStreamReq, c *client.Client) (result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			global.GVA_LOG.Error("MemberLogin", zap.Any("r", r))
			debug.PrintStack()
			result, err = "", InternalServerError
		}
	}()
	return newMemberLoginService().memberLogin(req, c)
}
