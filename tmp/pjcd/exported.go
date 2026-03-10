package pjcd

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

// Game 游戏实例
type Game struct{}

// NewGame 创建游戏实例
func NewGame() *Game {
	return &Game{}
}

// NewBetOrder 下注接口，返回 protobuf 和 json 格式
func (g *Game) NewBetOrder(req *request.BetOrderReq) (pbData []byte, result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			global.GVA_LOG.Error("BetOrder", zap.Any("r", r))
			debug.PrintStack()
			pbData, result, err = nil, "", InternalServerError
			return
		}
	}()

	betService := newBetOrderService()
	return betService.betOrder(req)
}

// MemberLogin 用户登录
func (g *Game) MemberLogin(req *pb.LoginStreamReq, c *client.Client) (result string, err error) {
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
