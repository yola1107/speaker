package gcd

import (
	"errors"
	"fmt"

	"egame-grpc/game/common"
	"egame-grpc/game/gcd/pb"
	"egame-grpc/gamelogic/game_replay"
	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type betOrderService struct {
	req              *request.BetOrderReq
	merchant         *merchant.Merchant
	member           *member.Member
	game             *game.Game
	client           *client.Client
	lastOrder        *game.GameOrder
	betAmount        decimal.Decimal
	amount           decimal.Decimal
	gameConfig       *GameConfig
	gameOrder        *game.GameOrder
	bonusAmount      decimal.Decimal
	scene            *SpinSceneData
	symbolGrid       Int64Grid
	clientSymbolGrid Int64Grid
	winInfos         []WinInfo
	winResults       []*WinResult
	winArr           []*pb.WinResult
	winGrid          Int64Grid
	lineMultiplier   int64
	stepMultiplier   int64
	roundMulti       int64
	treasureNum      int64
	newFreeTimes     int64
	freeType         int64
	isRoundOver      bool
	stepIndex        int64
}

func newBetOrderService() *betOrderService {
	gameJSON, _ := common.GetRedisGameJson(GameID)
	return &betOrderService{
		gameConfig: newGameConfig(gameJSON),
	}
}

func (s *betOrderService) betOrder(req *request.BetOrderReq) ([]byte, string, error) {
	s.req = req
	c, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("betOrder", zap.Error(errors.New("user not exists")))
		return nil, "", fmt.Errorf("client not exist")
	}
	s.client = c
	c.BetLock.Lock()
	defer c.BetLock.Unlock()
	if err := s.getRequestContext(); err != nil {
		return nil, "", err
	}
	lastOrder, _, err := c.GetLastOrder()
	if err != nil {
		return nil, "", internalServerError
	}
	s.lastOrder = lastOrder
	if s.lastOrder == nil {
		s.cleanScene()
	}
	if err = s.loadScene(); err != nil {
		return nil, "", err
	}
	if err = s.ensureBonusSelected(); err != nil {
		return nil, "", err
	}
	if err = s.stepScene(); err != nil {
		return nil, "", err
	}
	if err = s.betSpins(); err != nil {
		return nil, "", err
	}
	if err = s.updateBonusAmount(); err != nil {
		return nil, "", err
	}
	if err = s.updateGameOrder(); err != nil {
		return nil, "", err
	}
	if err = s.settleStep(); err != nil {
		return nil, "", err
	}
	if err = s.scene.save(s.member.ID); err != nil {
		return nil, "", err
	}
	return s.getBetResultMap()
}

func (s *betOrderService) getBetResultMap() ([]byte, string, error) {
	resp, err := gameOrderToResponse(s.gameOrder)
	if err != nil {
		return nil, "", err
	}
	return marshalData(resp)
}

func (s *betOrderService) replayByOrder(req *request.BetOrderReq, gameOrder *game.GameOrder) (*game_replay.InternalResponse, error) {
	resp, err := gameOrderToResponse(gameOrder)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("gcd replayByOrder err:%v", err))
	}
	pbData, jsonData, err := marshalData(resp)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("gcd replayByOrder marshalData err:%v", err))
	}
	return game_replay.NewPbInternalResponse(jsonData, pbData), nil
}

func (s *betOrderService) ensureBonusSelected() error {
	if s.scene == nil || s.scene.BonusState != _bonusStatePending {
		return nil
	}
	return ErrBonusNumMustSelect
}
