package hbtr2

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
	isRoundOver    bool                 // 回合是否结束
	isFreeRound    bool                 // 是否为免费回合
	scatterCount   int64                // 夺宝符个数
	addFreeTime    int64                // 增加的免费次数
	gameMultiple   int64                // 连续消除倍数，初始1倍
	lineMultiplier int64                // 线倍数
	stepMultiplier int64                // Step倍数
	winInfos       []WinInfo            // 中奖信息（统一格式，避免冗余转换）
	nextSymbolGrid *int64Grid           // 下一把 step 符号网格
	symbolGrid     int64Grid            // 符号网格
	winGrid        int64Grid            // 中奖网格
	debug          rtpDebugData         // 是否为RTP测试流程

	reversalSymbolGrid int64Grid // 反转符号网格
	reversalWinGrid    int64Grid // 反转中奖网格
}

func newBetOrderService() *betOrderService {
	s := &betOrderService{}
	s.selectGameRedis()
	s.initGameConfigs()
	s.gameMultiple = 1
	return s
}

func (s *betOrderService) betOrder(req *request.BetOrderReq) (map[string]any, error) {
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
	if err = s.reloadScene(); err != nil {
		global.GVA_LOG.Error("betOrder: reloadScene failed", zap.Error(err))
		return nil, InternalServerError
	}
	if err = s.baseSpin(); err != nil {
		return nil, err
	}

	// 上下对称下网格 用于填充 客户端和GameOrder订单信息
	s.reversalSymbolGrid = reverseGridRows(&s.symbolGrid)
	s.reversalWinGrid = reverseGridRows(&s.winGrid)

	if err = s.updateGameOrder(); err != nil {
		return nil, err
	}
	if err = s.settleStep(); err != nil {
		return nil, err
	}
	if err = s.saveScene(); err != nil {
		return nil, err
	}

	return s.getBetResultMap(), nil
}

func (s *betOrderService) getBetResultMap() map[string]any {
	global.GVA_LOG.Debug("betOrder: getBetResultMap", zap.Any("currentWin ", s.bonusAmount.Round(2).InexactFloat64()))
	return map[string]any{
		"sn":         s.orderSN,
		"balance":    s.gameOrder.CurBalance,
		"free":       btoi(s.isFreeRound),                            // 是否免费 0：否，1:是
		"cards":      s.reversalSymbolGrid,                           // 当前牌型（上下翻转对称）
		"winInfo":    s.buildWinInfoDetail(),                         // 中奖路线信息（对齐 hbtr，字符串 JSON）
		"currentWin": s.bonusAmount.Round(2).InexactFloat64(),        // 当前回合赢得
		"totalWin":   s.client.ClientOfFreeGame.GetGeneralWinTotal(), //
		"accWin":     s.client.ClientOfFreeGame.GetFreeTotalMoney(),  // 当前轮共赢得
		"sumFreeWin": s.client.ClientOfFreeGame.GetRoundBonus(),      // 免费共赢得（暂无则为0）
	}
}
