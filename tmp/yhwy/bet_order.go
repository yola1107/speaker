package yhwy

import (
	"errors"
	"fmt"

	"egame-grpc/game/common"
	"egame-grpc/game/yhwy/pb"
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

// betOrderService 封装 yhwy 单次下注请求的完整执行上下文。
type betOrderService struct {
	req            *request.BetOrderReq // 原始下注请求
	merchant       *merchant.Merchant   // 商户信息
	member         *member.Member       // 玩家信息
	game           *game.Game           // 游戏基础信息
	client         *client.Client       // 玩家运行时客户端缓存
	lastOrder      *game.GameOrder      // 上一笔订单，用于续局/生成订单号
	scene          *SpinSceneData       // 当前持久化场景
	gameConfig     *gameConfigJson      // 游戏配置
	gameOrder      *game.GameOrder      // 本次生成的订单
	bonusAmount    decimal.Decimal      // 本次 step 派彩金额
	betAmount      decimal.Decimal      // 本轮标准投注金额
	amount         decimal.Decimal      // 本次实际扣费金额
	orderSn        *common.OrderSN      // 当前订单号信息
	isRoundOver    bool                 // 当前 round 是否结束
	isFreeRound    bool                 // 当前是否处于免费游戏阶段
	scatterCount   int64                // 最终盘面 Scatter 数量
	addFreeTime    int64                // 本次新增的免费次数
	lineMultiplier int64                // 全部中奖线赔率之和
	stepMultiplier int64                // 本 step 最终结算倍数
	winInfos       []WinInfo            // 本次命中的所有中奖线明细
	originGrid     int64Grid            // 原始停轮盘面
	mysteryGrid    int64Grid            // 复位/扩散后、揭示前盘面
	finalGrid      int64Grid            // 揭示完成后的最终判奖盘面
	winGrid        int64Grid            // 所有中奖位置的掩码盘面
	revealSymbol   int64                // 百变樱花本次统一揭示出的目标符号
	spreadToReel   int64                // 樱花扩散最远到第几列，未扩散时为 1
	isSakuraReset  bool                 // 第 1 列是否触发樱花复位
	resetDirection int64                // 复位方向：无 / 上移 / 下移

	debug rtpDebugData // 本地测试控制开关
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
	result := &pb.Yhwy_BetOrderResponse{
		OrderSN:        proto.String(s.orderSn.OrderSN),
		Balance:        proto.Float64(s.gameOrder.CurBalance),
		BetAmount:      proto.Float64(s.betAmount.Round(2).InexactFloat64()),
		CurrentWin:     proto.Float64(s.bonusAmount.Round(2).InexactFloat64()),
		FreeWin:        proto.Float64(s.client.ClientOfFreeGame.GetFreeTotalMoney()),
		TotalWin:       proto.Float64(s.client.ClientOfFreeGame.GetGeneralWinTotal()),
		Free:           proto.Bool(s.isFreeRound),
		Review:         proto.Int64(s.req.Review),
		WinInfo:        s.buildWinInfo(),
		Cards:          s.int64GridToArray(s.originGrid),
		ScatterCount:   proto.Int64(s.scatterCount),
		IsRoundOver:    proto.Bool(s.isRoundOver),
		State:          proto.Int64(int64(s.scene.Stage)),
		FreeNum:        proto.Int64(int64(s.client.ClientOfFreeGame.GetFreeNum())),
		FreeTime:       proto.Int64(int64(s.client.ClientOfFreeGame.GetFreeTimes())),
		WinGrid:        s.int64GridToArray(s.winGrid),
		IsGameOver:     proto.Bool(s.isFreeRound && s.isRoundOver && s.scene.FreeNum <= 0),
		EffectCards:    s.int64GridToArray(s.mysteryGrid),
		FinalCards:     s.int64GridToArray(s.finalGrid),
		RevealSymbol:   proto.Int64(s.revealSymbol),
		SpreadToReel:   proto.Int64(s.spreadToReel),
		IsSakuraReset:  proto.Bool(s.isSakuraReset),
		ResetDirection: proto.Int64(s.resetDirection),
		LineMultiplier: proto.Int64(s.lineMultiplier),
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

func (s *betOrderService) buildWinInfo() *pb.Yhwy_WinInfo {
	winArr := make([]*pb.Yhwy_WinArr, len(s.winInfos))
	for i, elem := range s.winInfos {
		winArr[i] = &pb.Yhwy_WinArr{
			RoadNum: proto.Int64(elem.LineCount),
			Odds:    proto.Int64(elem.Odds),
			Symbol:  proto.Int64(elem.Symbol),
			Count:   proto.Int64(elem.SymbolCount),
		}
	}
	return &pb.Yhwy_WinInfo{
		WinArr:         winArr,
		AddFreeNum:     proto.Int64(s.addFreeTime),
		LineMultiplier: proto.Int64(s.lineMultiplier),
	}
}
