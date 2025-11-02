package xxg2

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
	req              *request.BetOrderReq // 用户请求
	merchant         *merchant.Merchant   // 商户信息
	member           *member.Member       // 用户信息
	game             *game.Game           // 游戏信息
	client           *client.Client       // 用户上下文
	lastOrder        *game.GameOrder      // 用户上一个订单
	gameRedis        *redis.Client        // 游戏 redis
	scene            *SpinSceneData       // 场景数据
	gameOrder        *game.GameOrder      // 订单
	bonusAmount      decimal.Decimal      // 奖金金额
	betAmount        decimal.Decimal      // spin 下注金额
	amount           decimal.Decimal      // step 扣费金额
	orderSN          string               // 订单号
	parentOrderSN    string               // 父订单号，回合第一个 step 此字段为空
	freeOrderSN      string               // 触发免费的回合的父订单号，基础 step 此字段为空
	stepMultiplier   int64                // Step倍数
	isRoundFirstStep bool                 // 是否是 spin 的第一个 step
	isSpinFirstRound bool                 // 是否为 Spin 的第一回合
	forRtpBench      bool                 // 是否为RTP测试流程
	gameConfig       *gameConfigJson      // 配置数据
	winInfos         []*winInfo           // 中奖信息
	originalGrid     *int64Grid           // 初始符号网格（转换前）
	symbolGrid       *int64Grid           // 符号网格（填wind后）
	winGrid          *int64Grid           // 中奖网格
	// xxg2 特有字段
	stepMap        *stepMap     // step 预设数据
	winResults     []*winResult // 中奖结果
	lineMultiplier int64        // 中奖线倍数
	newFreeCount   int64        // step 新增免费次数

	debug statDebug // 调试信息
}

// RTP 调试信息
type statDebug struct {
	col [_colCount]statColInfo // 转轮起始位置（用于调试）
}
type statColInfo struct {
	startIdx int
	len      int
}

// 生成下注服务实例
func newBetOrderService(forRtpBench bool) *betOrderService {
	s := &betOrderService{
		forRtpBench: forRtpBench,
	}
	s.selectGameRedis()
	return s
}

// 统一下注请求接口，无论是免费还是普通
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

	// 判断是否首次 spin
	if s.lastOrder == nil {
		s.isSpinFirstRound = true
		s.isRoundFirstStep = true
		s.cleanScene()
	}

	// 加载场景数据
	s.reloadScene()

	// spin的第一个round（参考 zcm2）
	if s.scene.SpinFirstRound == 0 {
		s.isSpinFirstRound = true
	}
	// round的第一个step
	if s.scene.RoundFirstStep == 0 {
		s.isRoundFirstStep = true
	}

	// 执行主要的 spin 逻辑
	_, err = s.baseSpin()
	if err != nil {
		return nil, err
	}

	// 更新游戏订单
	if !s.updateGameOrder() {
		return nil, InternalServerError
	}

	// 结算 step
	if !s.settleStep() {
		return nil, InternalServerError
	}

	// 保存场景数据
	if err = s.saveScene(); err != nil {
		return nil, err
	}

	// 直接构建返回结果
	ret := map[string]any{
		"orderSN":            s.gameOrder.OrderSn,
		"symbolGrid":         s.symbolGrid,         // Grid经过上下对称处理后的Gird
		"treasureCount":      s.stepMap.TreatCount, // 本轮treasure(11号符号)数量
		"winGrid":            s.winGrid,            //
		"winResults":         s.winResults,         //
		"baseBet":            s.req.BaseMoney,
		"multiplier":         s.req.Multiple,
		"betAmount":          s.betAmount.Round(2).InexactFloat64(),
		"bonusAmount":        s.bonusAmount.Round(2).InexactFloat64(),
		"spinBonusAmount":    s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		"freeBonusAmount":    s.client.ClientOfFreeGame.GetFreeTotalMoney(),
		"roundBonus":         s.client.ClientOfFreeGame.RoundBonus,
		"currentBalance":     s.gameOrder.CurBalance,
		"isFree":             s.isFreeRound(),
		"step":               s.stepMap.ID,
		"newFreeCount":       s.stepMap.New,
		"totalFreeCount":     s.client.GetLastMaxFreeNum(),
		"remainingFreeCount": s.stepMap.FreeNum,
		"lineMultiplier":     s.lineMultiplier,
		"stepMultiplier":     s.stepMultiplier,
		"bat":                s.stepMap.Bat, // 也是通过Grid上下对称处理后的Gird对应的坐标
	}
	return ret, nil
}
