package sbyymx2

import (
	"errors"
	"fmt"

	"egame-grpc/game/common"
	"egame-grpc/game/sbyymx2/pb"
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
	req              *request.BetOrderReq // 用户请求
	merchant         *merchant.Merchant   // 商户信息
	member           *member.Member       // 用户信息
	game             *game.Game           // 游戏信息
	client           *client.Client       // 用户上下文
	lastOrder        *game.GameOrder      // 用户上一个订单
	scene            *SpinSceneData       // 场景中间态数据
	gameConfig       *gameConfigJson      // 配置数据
	gameOrder        *game.GameOrder      // 订单
	bonusAmount      decimal.Decimal      // 奖金金额
	betAmount        decimal.Decimal      // spin 下注金额
	amount           decimal.Decimal      // step 扣费金额
	orderSn          *common.OrderSN      // 订单号
	isRoundOver      bool                 // 回合是否结束
	next             bool                 // 是否需要继续请求（必赢重转）
	stepIsRespinMode bool                 // 当前spin是否是重转至赢模式
	isInstrumentWin  bool                 // 是否乐器符号中奖（吉他/鼓）
	lineMultiplier   int64                // 线赔率合计（未乘长条百搭），回包给前端
	wildMultiplier   int64                // 长条百搭倍数
	stepMultiplier   int64                // Step倍数
	winInfos         []WinInfo            // 中奖信息
	symbolGrid       int64Grid            // 符号网格（3行3列）
	winGrid          int64Grid            // 中奖网格（3行3列）
	debug            rtpDebugData         // 是否为RTP测试流程

	isWildExpandCol bool // 是否发生百搭变大列
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
	result := &pb.Sbyymx_BetOrderResponse{
		OrderSN:          proto.String(s.orderSn.OrderSN),
		Balance:          proto.Float64(s.gameOrder.CurBalance),
		BetAmount:        proto.Float64(s.betAmount.Round(2).InexactFloat64()),
		CurrentWin:       proto.Float64(s.bonusAmount.Round(2).InexactFloat64()),
		TotalWin:         proto.Float64(s.client.ClientOfFreeGame.GetGeneralWinTotal()),
		Review:           proto.Int64(s.req.Review),
		WinInfo:          s.buildWinInfo(),
		Cards:            s.int64GridToArray(s.symbolGrid),
		IsRoundOver:      proto.Bool(s.isRoundOver),
		WinGrid:          s.int64GridToArray(s.winGrid),
		Next:             proto.Bool(s.next),
		IsRespinUntilWin: proto.Bool(s.stepIsRespinMode),
		WildMultiplier:   proto.Int64(s.wildMultiplier),
		LineMultiplier:   proto.Int64(s.lineMultiplier),
		StepMultiple:     proto.Int64(s.stepMultiplier),
		IsInstrumentWin:  proto.Bool(s.isInstrumentWin),
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

func (s *betOrderService) buildWinInfo() *pb.Sbyymx_WinInfo {
	winArr := make([]*pb.Sbyymx_WinArr, len(s.winInfos))
	for i, elem := range s.winInfos {
		winArr[i] = &pb.Sbyymx_WinArr{
			RoadNum: proto.Int64(elem.LineCount),
			Odds:    proto.Int64(elem.Odds),
		}
	}
	return &pb.Sbyymx_WinInfo{
		WinArr: winArr,
	}
}
