// game/bxkh2/exported.go
package bxkh2

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

// NewBetOrder 下注接口，返回 protobuf 和 json 格式
func (g *Game) NewBetOrder(req *request.BetOrderReq) (pbData []byte, jsonData string, err error) {
	defer func() {
		if r := recover(); r != nil {
			global.GVA_LOG.Error("BetOrder", zap.Any("r", r))
			debug.PrintStack()
			pbData, jsonData, err = nil, "", InternalServerError
			return
		}
	}()

	betService := newBetOrderService()
	return betService.betOrder(req)
}

// BetOrder 兼容旧接口
func (g *Game) BetOrder(req *request.BetOrderReq) (result map[string]any, err error) {
	_, jsonData, err := g.NewBetOrder(req)
	if err != nil {
		return nil, err
	}
	// 解析 JSON 返回 map（兼容旧调用）
	return map[string]any{"data": jsonData}, nil
}

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
