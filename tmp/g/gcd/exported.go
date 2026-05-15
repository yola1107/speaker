package gcd

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
	internalServerError   = errors.New("internal server error")
	invalidRequestParams  = errors.New("invalid request params")
	insufficientBalance   = errors.New("insufficient balance")
	ErrBonusNumMustSelect = errors.New("bonusNum must be select")
)

type Game struct{}

func NewGame() *Game {
	return &Game{}
}

// 用户下注
func (g *Game) NewBetOrder(req *request.BetOrderReq) (pbData []byte, result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			pbData, result, err = nil, "", internalServerError
			return
		}
	}()
	return newBetOrderService().betOrder(req)
}

func (g *Game) BetBonus(req *request.BetBonusReq) (result map[string]any, err error) {
	defer func() {
		if r := recover(); r != nil {
			global.GVA_LOG.Error("betOrder", zap.Any("r", r))
			debug.PrintStack()
			result, err = nil, internalServerError
			return
		}
	}()
	return newBetOrderService().betBonus(req)
}

// 用户登录
func (g *Game) MemberLogin(req *pb.LoginStreamReq, c *client.Client) (result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			global.GVA_LOG.Error("MemberLogin", zap.Any("r", r), zap.Stack("stack"))
			result, err = "", internalServerError
			return
		}
	}()
	return newMemberLoginService().memberLogin(req, c)
}

func (g *Game) BetReplay(req *request.BetOrderReq, gameOrder *game.GameOrder) (result *game_replay.InternalResponse, err error) {
	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			result, err = nil, internalServerError
			return
		}
	}()
	return newBetOrderService().replayByOrder(req, gameOrder)
}
