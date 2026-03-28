package ycj

import (
	"errors"
	"fmt"

	"egame-grpc/game/common"
	"egame-grpc/game/ycj/pb"
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
	scene          *SpinSceneData       // 场景中间态数据
	gameConfig     *gameConfigJson      // 配置数据
	gameOrder      *game.GameOrder      // 订单
	bonusAmount    decimal.Decimal      // 奖金金额
	betAmount      decimal.Decimal      // spin 下注金额
	amount         decimal.Decimal      // step 扣费金额
	orderSn        *common.OrderSN      // 订单号
	isRoundOver    bool                 // 回合是否结束
	isFreeRound    bool                 // 是否为免费回合
	next           bool                 // 是否需要继续请求（必赢重转）
	addFreeTime    int64                // 增加的免费次数
	stepMultiplier float64              // 返奖倍数
	winResult      WinResult            // 判奖结果
	symbolGrid     int64Grid            // 符号网格（1行3列）
	winGrid        int64Grid            // 中奖网格（1行3列）
	debug          rtpDebugData         // RTP调试数据
}

func newBetOrderService() *betOrderService {
	s := &betOrderService{}
	s.initGameConfigs()
	return s
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
		return nil, "", InternalServerError
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
		global.GVA_LOG.Error("betOrder: reloadScene failed", zap.Error(err))
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
	result := &pb.Ycj_BetOrderResponse{
		OrderSN:      proto.String(s.orderSn.OrderSN),
		Balance:      proto.Float64(s.gameOrder.CurBalance),
		BetAmount:    proto.Float64(s.betAmount.Round(2).InexactFloat64()),
		CurrentWin:   proto.Float64(s.bonusAmount.Round(2).InexactFloat64()),
		FreeWin:      proto.Float64(s.client.ClientOfFreeGame.GetFreeTotalMoney()),
		TotalWin:     proto.Float64(s.client.ClientOfFreeGame.GetGeneralWinTotal()),
		Free:         proto.Bool(s.isFreeRound),
		Review:       proto.Int64(s.req.Review),
		WinInfo:      s.buildWinInfo(),
		Cards:        s.int64GridToArray(s.symbolGrid),
		Multiplier:   proto.Float64(s.stepMultiplier),
		IsRoundOver:  proto.Bool(s.isRoundOver),
		State:        proto.Int64(int64(s.scene.Stage)),
		FreeNum:      proto.Int64(s.scene.FreeNum),
		FreeTime:     proto.Int64(int64(s.client.ClientOfFreeGame.GetFreeTimes())),
		WinGrid:      s.int64GridToArray(s.winGrid),
		IsGameOver:   proto.Bool(s.isFreeRound && s.isRoundOver && s.scene.FreeNum <= 0),
		Next:         proto.Bool(s.next),
		IsExtendMode: proto.Bool(s.scene.IsExtendMode),
		IsRespinMode: proto.Bool(s.scene.IsRespinMode),
		AddFreeNum:   proto.Int64(s.addFreeTime),
	}
	pbData, err := proto.Marshal(result)
	if err != nil {
		return nil, "", err
	}
	jsonData, err := json.CJSON.MarshalToString(result)
	if err != nil {
		return nil, "", err
	}
	return pbData, jsonData, nil
}

func (s *betOrderService) buildWinInfo() *pb.Ycj_WinInfo {
	return &pb.Ycj_WinInfo{
		LeftSymbol:  proto.Int64(s.symbolGrid[0][0]),
		MidSymbol:   proto.Int64(s.symbolGrid[0][1]),
		RightSymbol: proto.Int64(s.symbolGrid[0][2]),
		Multiplier:  proto.Float64(s.stepMultiplier),
		IsExtend:    proto.Bool(s.scene.IsExtendMode),
		IsRespin:    proto.Bool(s.scene.IsRespinMode),
		IsFree:      proto.Bool(s.addFreeTime > 0),
		FreeSpinNum: proto.Int64(s.addFreeTime),
	}
}
