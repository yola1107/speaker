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
	bats           []Bat                // Wild移动记录
	debug          rtpDebugData         // 是否为RTP测试流程
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
	var freeTotalMoney float64
	var isFreeInt int
	if s.isFreeRound {
		freeTotalMoney = s.client.ClientOfFreeGame.GetFreeTotalMoney()
		isFreeInt = 1
	}

	currentWin := s.bonusAmount.Round(2).InexactFloat64()
	accWin := s.client.ClientOfFreeGame.GetFreeTotalMoney()
	sumFreeWin := s.client.ClientOfFreeGame.GetRoundBonus()
	winInfo := s.buildWinInfoDetail()

	return map[string]any{
		// 对齐 hbtr BetOrder 返回字段
		"sn":         s.orderSN,
		"balance":    s.gameOrder.CurBalance,
		"free":       isFreeInt,                                      // 是否免费 0：否，1:是
		"cards":      reverseGridRows(&s.symbolGrid),                 // 当前牌型（上下翻转对称）
		"winInfo":    winInfo,                                        // 中奖路线信息（对齐 hbtr，map 结构）
		"currentWin": currentWin,                                     // 当前回合赢得
		"totalWin":   s.client.ClientOfFreeGame.GetGeneralWinTotal(), //
		"accWin":     accWin,                                         // 当前轮共赢得
		"sumFreeWin": sumFreeWin,                                     // 免费共赢得（暂无则为0）

		// 新增字段（hbtr2扩展，前端可选用）
		"betMoney":       s.betAmount.Round(2).InexactFloat64(), // 新增
		"review":         0,                                     // 新增
		"freeNum":        uint64(s.scene.FreeNum),               // 新增
		"win":            currentWin,                            // 新增：等同 currentWin
		"freeTotalMoney": freeTotalMoney,                        // 新增：等同 accWin
		"wincards":       s.winGrid,                             // 新增
		"scatterCount":   s.scatterCount,                        // 新增
	}
}
