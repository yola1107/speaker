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
	req            *request.BetOrderReq // 用户请求
	merchant       *merchant.Merchant   // 商户信息
	member         *member.Member       // 用户信息
	game           *game.Game           // 游戏信息
	client         *client.Client       // 用户上下文
	lastOrder      *game.GameOrder      // 用户上一个订单
	gameRedis      *redis.Client        // 游戏 redis
	scene          *SpinSceneData       // 场景中间态数据
	gameConfig     *gameConfigJson      // 配置数据
	gameOrder      *game.GameOrder      // 订单
	bonusAmount    decimal.Decimal      // 奖金金额
	betAmount      decimal.Decimal      // spin 下注金额
	amount         decimal.Decimal      // step 扣费金额
	orderSN        string               // 订单号
	parentOrderSN  string               // 父订单号，回合第一个 step 此字段为空
	freeOrderSN    string               // 触发免费的回合的父订单号，基础 step 此字段为空
	stepMultiplier int64                // Step倍数
	isRoundOver    bool                 // 回合是否结束
	isFreeRound    bool                 // 是否为免费回合
	gameMultiple   int64                // 连续消除倍数，初始1倍（从 scene.ContinueNum 计算得出）
	winInfos       []*winInfo           // 中奖信息
	nextSymbolGrid int64Grid            // 下一把 step 符号网格
	symbolGrid     int64Grid            // 符号网格
	winGrid        int64Grid            // 中奖网格
	debug          rtpDebugData         // 是否为RTP测试流程
}

func newBetOrderService() *betOrderService {
	s := &betOrderService{}
	s.selectGameRedis()
	s.initGameConfigs()
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
		s.cleanScene()
	}

	// 加载场景数据
	if err := s.reloadScene(); err != nil {
		global.GVA_LOG.Error("betOrder: reloadScene failed", zap.Error(err))
		return nil, InternalServerError
	}

	// 检查是否等待客户端选择免费游戏类型
	if s.scene.BonusState == _bonusStatePending {
		msg := fmt.Sprintf("scatterNum=%d bonusNum=%d,bonusState=%d", s.scene.ScatterNum, s.scene.BonusNum, s.scene.BonusState)
		global.GVA_LOG.Warn("betOrder", zap.String("waiting for client to select bonus type.", msg))
		return nil, fmt.Errorf("waiting for client to select bonus type: %s", msg)
	}

	baseRes, err := s.baseSpin()
	if err != nil {
		return nil, err
	}

	var accWin float64
	var isFreeInt int
	if s.isFreeRound {
		accWin = s.client.ClientOfFreeGame.GetFreeTotalMoney()
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
		BonusState: s.scene.BonusState,
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
