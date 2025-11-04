package xslm2

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

// betOrderService 下注服务（管理单次下注的完整流程）
type betOrderService struct {
	req           *request.BetOrderReq // 用户请求
	merchant      *merchant.Merchant   // 商户信息
	member        *member.Member       // 用户信息
	game          *game.Game           // 游戏信息
	client        *client.Client       // 用户上下文
	lastOrder     *game.GameOrder      // 用户上一个订单
	gameRedis     *redis.Client        // 游戏Redis
	scene         *SpinSceneData       // 场景数据（持久化状态）
	spin          spin                 // Spin数据（符号网格、中奖信息等）
	gameOrder     *game.GameOrder      // 当前订单
	bonusAmount   decimal.Decimal      // 本Step奖金
	betAmount     decimal.Decimal      // Spin下注金额（回合第一局扣费）
	amount        decimal.Decimal      // Step扣费金额（首局=betAmount，连消=0）
	currBalance   decimal.Decimal      // 当前余额
	orderSN       string               // 订单号
	parentOrderSN string               // 父订单号（回合第一个step此字段为空）
	freeOrderSN   string               // 触发免费的回合父订单号（基础step为空）
	isFreeRound   bool                 // 是否免费回合
	isFirst       bool                 // 是否首次spin（回合第一局）

	debug rtpDebugData // RTP压测调试
}

func newBetOrderService(forRtpBench bool) *betOrderService {
	s := &betOrderService{
		debug: rtpDebugData{open: forRtpBench},
	}
	s.selectGameRedis()
	return s
}

// betOrder 下注主入口函数
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
	switch {
	case err != nil:
		global.GVA_LOG.Error("betOrder", zap.Error(err))
		return nil, InternalServerError
	}
	s.lastOrder = lastOrder

	// 判断是否首次spin
	switch {
	case s.lastOrder == nil:
		s.isFirst = true
		s.cleanScene() // 首次spin清除场景
	case s.client.IsRoundOver:
		s.isFirst = true
	}

	// 加载场景数据
	if err := s.reloadScene(); err != nil {
		global.GVA_LOG.Error("betOrder", zap.Error(err))
		return nil, InternalServerError
	}

	return s.doBetOrder()
}

// doBetOrder 执行下注流程
func (s *betOrderService) doBetOrder() (map[string]any, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}

	// 调用 baseSpin 执行核心逻辑
	s.spin.baseSpin(s.isFreeRound)

	// 更新状态
	s.updateStepResult()

	// 更新订单
	if !s.updateGameOrder() {
		return nil, InternalServerError
	}

	// 结算
	if !s.settleStep() {
		return nil, InternalServerError
	}

	// 保存场景数据
	if err := s.saveScene(); err != nil {
		global.GVA_LOG.Error("doBetOrder", zap.Error(err))
		return nil, InternalServerError
	}

	return s.getBetResultMap(), nil
}
