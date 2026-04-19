package jqs

import (
	"errors"
	"runtime/debug"

	"egame-grpc/gamelogic/game_replay"
	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
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
func (g *Game) NewBetOrder(req *request.BetOrderReq) (pbData []byte, result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			global.GVA_LOG.Error("BetOrder", zap.Any("r", r))
			debug.PrintStack()
			pbData, result, err = nil, "", InternalServerError
			return
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
			return
		}
	}()
	return newMemberLoginService().memberLogin(req, c)
}

func (g *Game) BetReplay(req *request.BetOrderReq, gameOrder *game.GameOrder) (result *game_replay.InternalResponse, err error) {
	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			result, err = nil, InternalServerError
			return
		}
	}()
	return newBetOrderService().replayByOrder(req, gameOrder)
}
