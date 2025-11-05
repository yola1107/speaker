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
	scene          *SpinSceneData       // 场景数据
	gameOrder      *game.GameOrder      // 订单
	bonusAmount    decimal.Decimal      // 奖金金额
	betAmount      decimal.Decimal      // spin 下注金额
	amount         decimal.Decimal      // step 扣费金额
	orderSN        string               // 订单号
	parentOrderSN  string               // 父订单号，回合第一个 step 此字段为空
	freeOrderSN    string               // 触发免费的回合的父订单号，基础 step 此字段为空
	stepMultiplier int64                // Step倍数
	lineMultiplier int64                // 线倍数
	winInfos       []*winInfo           // 中奖信息
	symbolGrid     *int64Grid           // 符号网格（填wind后）
	winGrid        *int64Grid           // 中奖网格
	stepMap        *stepMap             // step 预设数据
	winResults     []*winResult         // 中奖结果
	newFreeCount   int64                // step 新增免费次数
	debug          rtpDebugData         // RTP压测调试
}

// 生成下注服务实例
func newBetOrderService(forRtpBench bool) *betOrderService {
	return &betOrderService{
		debug: rtpDebugData{open: forRtpBench},
	}
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
		s.cleanScene()
	}

	s.reloadScene()

	if _, err = s.baseSpin(); err != nil {
		return nil, err
	}

	if !s.updateGameOrder() {
		return nil, InternalServerError
	}

	if !s.settleStep() {
		return nil, InternalServerError
	}

	// 保存当前 isFree 状态（必须在 saveScene() 之前，因为 saveScene 会更新 Stage）
	currentIsFree := s.isFreeRound()

	if err = s.saveScene(); err != nil {
		return nil, err
	}

	// 构建返回结果（bat和winResults需要坐标转换）
	return map[string]any{
		"orderSN":            s.gameOrder.OrderSn,
		"symbolGrid":         s.symbolGrid,
		"treasureCount":      s.stepMap.TreatCount,
		"winGrid":            reverseGridRows(s.winGrid),      // winGrid行序反转
		"winResults":         reverseWinResults(s.winResults), // WinPositions行序反转
		"baseBet":            s.req.BaseMoney,
		"multiplier":         s.req.Multiple,
		"betAmount":          s.betAmount.Round(2).InexactFloat64(),
		"bonusAmount":        s.bonusAmount.Round(2).InexactFloat64(),
		"spinBonusAmount":    s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		"freeBonusAmount":    s.client.ClientOfFreeGame.GetFreeTotalMoney(),
		"roundBonus":         s.client.ClientOfFreeGame.RoundBonus,
		"currentBalance":     s.gameOrder.CurBalance,
		"isFree":             currentIsFree, // 使用保存的状态，不受 saveScene 影响
		"step":               s.stepMap.ID,
		"newFreeCount":       s.stepMap.New,
		"totalFreeCount":     s.client.GetLastMaxFreeNum(),
		"remainingFreeCount": s.stepMap.FreeNum,
		"lineMultiplier":     s.lineMultiplier,
		"stepMultiplier":     s.stepMultiplier,
		"bat":                reverseBats(s.stepMap.Bat), // X/Y坐标交换
	}, nil
}
