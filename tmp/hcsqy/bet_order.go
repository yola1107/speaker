package hcsqy

import (
	"errors"
	"fmt"

	"egame-grpc/game/common"
	"egame-grpc/game/hcsqy/pb"
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
	respinWildCol  int32                // 重转至赢模式长条百搭列(0-2)，每次重转重新随机
	wildExpandCol  int32                // 长条百搭变大列(0-2)
	scatterCount   int64                // 夺宝符个数
	addFreeTime    int64                // 增加的免费次数
	lineMultiplier int64                // 线赔率合计（未乘长条百搭），回包给前端
	wildMultiplier int64                // 长条百搭倍数
	stepMultiplier int64                // Step倍数
	winInfos       []WinInfo            // 中奖信息
	symbolGrid     int64Grid            // 符号网格（3行3列）
	winGrid        int64Grid            // 中奖网格（3行3列）
	debug          rtpDebugData         // 是否为RTP测试流程

	stepIsPurchase   bool // 当前步是否处于购买态（用于回包/日志口径）
	stepIsRespinMode bool // 当前步是否处于Respin态（用于回包/日志口径）
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
	result := &pb.Hcsqy_BetOrderResponse{
		OrderSN:          proto.String(s.orderSn.OrderSN),
		Balance:          proto.Float64(s.gameOrder.CurBalance),
		BetAmount:        proto.Float64(s.betAmount.Round(2).InexactFloat64()),
		CurrentWin:       proto.Float64(s.bonusAmount.Round(2).InexactFloat64()),
		FreeWin:          proto.Float64(s.client.ClientOfFreeGame.GetFreeTotalMoney()),
		TotalWin:         proto.Float64(s.client.ClientOfFreeGame.GetGeneralWinTotal()),
		Free:             proto.Bool(s.isFreeRound),
		Review:           proto.Int64(s.req.Review),
		WinInfo:          s.buildWinInfo(),
		Cards:            s.int64GridToArray(s.symbolGrid),
		ScatterCount:     proto.Int64(s.scatterCount),
		IsRoundOver:      proto.Bool(s.isRoundOver),
		State:            proto.Int64(int64(s.scene.Stage)),
		FreeNum:          proto.Int64(int64(s.client.ClientOfFreeGame.GetFreeNum())),
		FreeTime:         proto.Int64(int64(s.client.ClientOfFreeGame.GetFreeTimes())),
		WinGrid:          s.int64GridToArray(s.winGrid),
		IsGameOver:       proto.Bool(s.isFreeRound && s.isRoundOver && s.scene.FreeNum <= 0),
		Next:             proto.Bool(s.next),
		IsRespinUntilWin: proto.Bool(s.stepIsRespinMode),
		RespinWildCol:    proto.Int32(s.respinWildCol),
		WildMultiplier:   proto.Int64(s.wildMultiplier),
		LineMultiplier:   proto.Int64(s.lineMultiplier),
		IsPurchase:       proto.Bool(s.stepIsPurchase),
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

func (s *betOrderService) buildWinInfo() *pb.Hcsqy_WinInfo {
	winArr := make([]*pb.Hcsqy_WinArr, len(s.winInfos))
	for i, elem := range s.winInfos {
		winArr[i] = &pb.Hcsqy_WinArr{
			RoadNum: proto.Int64(elem.LineCount),
			Odds:    proto.Int64(elem.Odds),
		}
	}
	return &pb.Hcsqy_WinInfo{
		WinArr:           winArr,
		AddFreeNum:       proto.Int64(s.addFreeTime),
		FreeGameMultiple: proto.Int64(s.stepMultiplier),
	}
}

/*func (s *betOrderService) calcRoundWin() float64 {
	if s.stepMultiplier == 0 {
		return 0
	}
	return s.bonusAmount.Round(2).InexactFloat64()
}
*/
