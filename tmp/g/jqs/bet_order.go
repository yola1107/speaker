package jqs

import (
	"errors"
	"fmt"

	"egame-grpc/game/jqs/pb"
	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type betOrderService struct {
	req            *request.BetOrderReq // 用户请求
	merchant       *merchant.Merchant   // 商户信息
	member         *member.Member       // 用户信息
	game           *game.Game           // 游戏信息
	client         *client.Client       // 用户上下文
	lastOrder      *game.GameOrder      // 用户上一个订单
	betAmount      decimal.Decimal      // spin 下注金额
	amount         decimal.Decimal      // step 扣费金额
	gameConfig     *gameConfigJson      // 游戏配置
	gameOrder      *game.GameOrder      // 订单
	bonusAmount    decimal.Decimal      // 奖金金额
	scene          *SpinSceneData       // 场景数据
	symbolGrid     int64Grid            // 符号网格
	winInfos       []WinInfo            // 中奖信息
	winGrid        int64Grid            // 中奖网格
	stepMultiplier int64                // 本步线倍合计
	state          int64                // 0常规 ，1免费，2免费幸运兔，3炸胡
	isFreeRound    bool                 // 是否为免费模式
	debug          rtpDebugData         // RTP测试调试模式
}

func newBetOrderService() *betOrderService {
	s := new(betOrderService)
	s.initGameConfigs()
	return s
}

// 下注逻辑
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
		return nil, "", InternalServerError
	}
	s.lastOrder = lastOrder
	if s.lastOrder == nil {
		s.cleanScene()
	}

	if err = s.reloadScene(); err != nil {
		return nil, "", err
	}
	if err = s.baseSpin(); err != nil {
		return nil, "", err
	}
	if err = s.updateGameOrder(); err != nil {
		return nil, "", err
	}
	if err = s.settleStep(); err != nil {
		return nil, "", err
	}
	if err = s.saveScene(); err != nil {
		return nil, "", err
	}
	return s.getBetResultMap()
}

func (s *betOrderService) getBetResultMap() ([]byte, string, error) {
	result, err := gameOrderToResponse(s.gameOrder)
	if err != nil {
		return nil, "", err
	}
	return marshalProtoMessage(result)
}

func gameOrderToResponse(gameOrder *game.GameOrder) (*pb.Jqs_BetOrderResponse, error) {
	var winDetail WinDetails
	if err := json.CJSON.UnmarshalFromString(gameOrder.WinDetails, &winDetail); err != nil {
		return nil, err
	}
	var symbolGrid int64Grid
	if err := json.CJSON.UnmarshalFromString(gameOrder.BetRawDetail, &symbolGrid); err != nil {
		return nil, err
	}
	var winGrid int64Grid
	if err := json.CJSON.UnmarshalFromString(gameOrder.BonusRawDetail, &winGrid); err != nil {
		return nil, err
	}

	bet := gameOrder.BaseAmount * float64(gameOrder.BaseMultiple) * float64(gameOrder.Multiple)
	review := int64(0)
	return &pb.Jqs_BetOrderResponse{
		Sn:             &gameOrder.OrderSn,
		Balance:        &gameOrder.CurBalance,
		BaseBet:        &gameOrder.BaseAmount,
		Multiplier:     &gameOrder.Multiple,
		BetAmount:      &bet,
		CurWin:         &gameOrder.BonusAmount,
		FreeWin:        &winDetail.FreeWin,
		TotalWin:       &winDetail.TotalWin,
		IsFree:         proto.Bool(gameOrder.IsFree == 1),
		Review:         &review,
		Next:           &winDetail.Next,
		State:          &gameOrder.State,
		LineMultiplier: &gameOrder.BonusMultiple,
		SymGrid:        int64GridToArray(symbolGrid),
		WinGrid:        int64GridToArray(winGrid),
		WinArr:         winDetail.WinArr,
	}, nil
}
