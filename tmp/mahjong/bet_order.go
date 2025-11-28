package mahjong

import (
	"errors"
	"fmt"

	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"

	"github.com/go-redis/redis/v8"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type betOrderService struct {
	req                *request.BetOrderReq // 用户请求
	merchant           *merchant.Merchant   // 商户信息
	member             *member.Member       // 用户信息
	game               *game.Game           // 游戏信息
	client             *client.Client       // 用户上下文
	lastOrder          *game.GameOrder      // 用户上一个订单
	gameRedis          *redis.Client        // 游戏 redis
	scene              *SpinSceneData       // 场景中间态数据
	gameOrder          *game.GameOrder      // 订单
	bonusAmount        decimal.Decimal      // 奖金金额
	betAmount          decimal.Decimal      // spin 下注金额
	amount             decimal.Decimal      // step 扣费金额
	orderSN            string               // 订单号
	parentOrderSN      string               // 父订单号，回合第一个 step 此字段为空
	freeOrderSN        string               // 触发免费的回合的父订单号，基础 step 此字段为空
	gameType           int64                // 游戏type
	stepMultiplier     int64                // Step倍数
	isRoundFirstStep   bool                 // 是否为第一step
	isSpinFirstRound   bool                 // 是否为Spin的第一回合
	forRtpBench        bool                 // 是否为RTP测试流程
	isRoundOver        bool                 // 一轮是否结束
	gameConfig         *gameConfigJson      // 配置数据
	removeNum          int64                // 免费游戏中奖消除次数
	gameMultiple       int64                // 免费倍数，初始1倍
	winInfos           []*winInfo           // 中奖信息
	nextSymbolGrid     int64Grid            // 下一把 step 符号网格
	symbolGrid         int64Grid            // 符号网格
	winGrid            int64Grid            // 中奖网格
	reversalSymbolGrid int64Grid            // 反转符号网格
	reversalWinGrid    int64GridW           // 反转中奖网格
}

func newBetOrderService() *betOrderService {
	s := &betOrderService{}
	s.selectGameRedis()
	s.gameMultiple = 1
	return s
}

// betOrder 统一下注请求接口，无论是免费还是普通
func (s *betOrderService) betOrder(req *request.BetOrderReq) (*SpinResultC, error) {
	s.req = req
	if !s.getRequestContext() {
		return nil, InternalServerError
	}
	c, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("betOrder", zap.Error(errors.New("user not exists")))
		return nil, fmt.Errorf("client not exist")
	}
	s.client = c
	c.BetLock.Lock()
	defer c.BetLock.Unlock()

	lastOrder, _, err := c.GetLastOrder()
	if err != nil {
		return nil, InternalServerError
	}
	s.lastOrder = lastOrder

	if s.lastOrder == nil {
		s.isSpinFirstRound = true
		s.isRoundFirstStep = true
		s.cleanScene()
	}
	s.reloadScene()

	baseRes, err := s.baseSpin()
	if err != nil {
		return nil, err
	}

	var accWin float64
	if s.isFreeRound() {
		accWin = s.client.ClientOfFreeGame.GetFreeTotalMoney()
	}
	isFreeInt := 0
	if s.scene.IsFreeRound {
		isFreeInt = 1
	}
	spinResultC := &SpinResultC{
		Balance:    s.getCurrentBalance(),
		BetAmount:  s.betAmount.Round(2).InexactFloat64(),
		CurrentWin: s.bonusAmount.Round(2).InexactFloat64(),
		AccWin:     accWin,
		TotalWin:   s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		Free:       isFreeInt,
		Review:     0,
		Sn:         s.orderSN,
		LastWinId:  s.client.ClientOfFreeGame.GetLastWinId(),
		MapId:      s.client.ClientOfFreeGame.GetLastMapId(),
		WinInfo:    baseRes.winInfo,
		Cards:      baseRes.cards,
		RoundBonus: s.client.ClientOfFreeGame.RoundBonus,
	}

	ok, err = s.updateGameOrder(baseRes)
	if !ok || err != nil {
		return nil, err
	}
	if err = s.settleStep(); err != nil {
		return nil, err
	}
	if err = s.saveScene(); err != nil {
		return nil, err
	}
	spinResultC.Balance = s.gameOrder.CurBalance
	return spinResultC, nil
}
