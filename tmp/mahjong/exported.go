package mahjong

import (
	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game/request"
	"egame-grpc/model/pb"
	"errors"
	"runtime/debug"

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
			global.GVA_LOG.Error("BetOrder", zap.Any("r", r))
			debug.PrintStack()
			result, err = nil, InternalServerError
			return
		}
	}()

	var betService = newBetOrderService(false)

	betService.initGameConfigs()

	resMap, err := betService.betOrder(req)

	if err != nil {
		return nil, err
	}

	return map[string]any{

		"win":            resMap.CurrentWin, // 当前Step赢
		"cards":          resMap.Cards,
		"wincards":       resMap.WinInfo.WinGrid,
		"betMoney":       resMap.BetAmount, // 下注额
		"balance":        resMap.Balance,   // 余额
		"free":           resMap.Free,      // 是否在免费
		"freeNum":        resMap.WinInfo.FreeNum,
		"freeTotalMoney": resMap.AccWin, // 当前round赢
		"totalWin":       resMap.TotalWin,
	}, nil
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
