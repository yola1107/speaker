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
	gameOrder     *game.GameOrder      // 当前订单
	orderSN       string               // 订单号
	parentOrderSN string               // 父订单号（回合第一个step此字段为空）
	freeOrderSN   string               // 触发免费的回合父订单号（基础step为空）
	bonusAmount   decimal.Decimal      // 本Step奖金
	betAmount     decimal.Decimal      // Spin下注金额（回合第一局扣费）
	amount        decimal.Decimal      // Step扣费金额（首局=betAmount，连消=0）
	currBalance   decimal.Decimal      // 当前余额
	scene         *SpinSceneData       // 场景数据（持久化状态）
	spin          spin                 // Spin数据（符号网格、中奖信息等）
	isFreeRound   bool                 // 是否免费回合
	isFirst       bool                 // 是否首次spin（回合第一局）
	debug         rtpDebugData         // RTP压测调试
}

func newBetOrderService(forRtpBench bool) *betOrderService {
	return &betOrderService{
		debug: rtpDebugData{open: forRtpBench},
	}
}

// betOrder 下注主入口函数
func (s *betOrderService) betOrder(req *request.BetOrderReq) (map[string]any, error) {
	s.req = req
	if !s.getRequestContext() {
		return nil, InternalServerError
	}

	// 获取客户端并加锁
	c, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("betOrder", zap.Error(errors.New("user not exists")))
		return nil, fmt.Errorf("client not exist")
	}
	s.client = c
	c.BetLock.Lock()
	defer c.BetLock.Unlock()

	// 获取上一个订单
	lastOrder, _, err := c.GetLastOrder()
	if err != nil {
		global.GVA_LOG.Error("betOrder", zap.Error(err))
		return nil, InternalServerError
	}
	s.lastOrder = lastOrder

	// 判断是否首次spin
	s.isFirst = s.lastOrder == nil || s.client.IsRoundOver
	if s.lastOrder == nil {
		s.cleanScene()
	}

	// 加载场景数据
	if err = s.reloadScene(); err != nil {
		global.GVA_LOG.Error("betOrder", zap.Error(err))
		return nil, InternalServerError
	}

	// scene -> spin
	nextGrid, rollers := s.prepareSpinFromScene()

	return s.doBetOrder(nextGrid, rollers)
}

// doBetOrder 执行下注流程
func (s *betOrderService) doBetOrder(nextGrid *int64Grid, rollers *[_colCount]SymbolRoller) (map[string]any, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}

	// 调用 baseSpin 执行核心逻辑（传递网格和滚轴）
	s.spin.baseSpin(s.isFreeRound, s.isFirst, nextGrid, rollers)

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

	// 保存场景数据（包括当前网格）
	if err := s.saveScene(); err != nil {
		global.GVA_LOG.Error("doBetOrder", zap.Error(err))
		return nil, InternalServerError
	}

	return s.buildResultMap(), nil
}

// buildResultMap 构建下注结果（返回给前端，复用于订单详情）
func (s *betOrderService) buildResultMap() map[string]any {
	ret := map[string]any{
		"orderSN":                 s.gameOrder.OrderSn,
		"currentBalance":          s.gameOrder.CurBalance,
		"baseBet":                 s.req.BaseMoney,
		"multiplier":              s.req.Multiple,
		"betAmount":               s.betAmount.Round(2).InexactFloat64(),
		"symbolGrid":              s.spin.symbolGrid,
		"winGrid":                 s.spin.winGrid,
		"winResults":              s.spin.winResults,
		"bonusAmount":             s.bonusAmount.Round(2).InexactFloat64(),
		"stepMultiplier":          s.spin.stepMultiplier,
		"lineMultiplier":          s.spin.stepMultiplier, // 兼容字段
		"isRoundOver":             s.spin.isRoundOver,
		"hasFemaleWin":            s.spin.hasFemaleWin,
		"isFreeRound":             s.isFreeRound,
		"newFreeRoundCount":       s.spin.newFreeRoundCount,
		"totalFreeRoundCount":     s.client.GetLastMaxFreeNum(),
		"remainingFreeRoundCount": s.client.ClientOfFreeGame.GetFreeNum(),
		"femaleCountsForFree":     s.spin.femaleCountsForFree,
		"nextFemaleCountsForFree": s.spin.nextFemaleCountsForFree,
		"enableFullElimination":   s.spin.enableFullElimination,
		"treasureCount":           s.spin.treasureCount,
		"spinBonusAmount":         s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		"freeBonusAmount":         s.client.ClientOfFreeGame.GetFreeTotalMoney(),
		"roundBonus":              s.client.ClientOfFreeGame.RoundBonus,
	}
	if true {
		s.printResultLog(ret)
	}
	return ret
}

// printResultLog 调试日志输出
func (s *betOrderService) printResultLog(ret map[string]any) {
	//WGrid := func(grid *int64Grid) string {
	//	if grid == nil {
	//		return ""
	//	}
	//	b := strings.Builder{}
	//	b.WriteString("\n")
	//	for r := int64(0); r < _rowCount; r++ {
	//		for c := int64(0); c < _colCount; c++ {
	//			sym := grid[r][c]
	//			b.WriteString(fmt.Sprintf("%3d", sym))
	//			if c < _colCount-1 {
	//				b.WriteString("| ")
	//			}
	//		}
	//		b.WriteString("\n")
	//	}
	//	return b.String()
	//}

	global.GVA_LOG.Sugar().Debugf(
		"\nisRoundOver=%v, IsFreeRound=%v, freeCountLeft=%d, isFullEls=%v, hasFemaleWin=%v, hasWildFemaleWin=%v, ABC=%v, nextABC=%v, newFree=%d, treasure=%d"+
			"\nsymbolGrid:%v\nwinGrid:%v\nwinResults:%v\nnextGrid:%v\n",
		s.spin.isRoundOver,
		s.isFreeRound,
		s.client.ClientOfFreeGame.GetFreeNum(),
		s.spin.enableFullElimination,
		s.spin.hasFemaleWin,
		s.spin.hasFemaleWildWin,
		s.spin.femaleCountsForFree,
		s.spin.nextFemaleCountsForFree,
		s.spin.newFreeRoundCount,
		s.spin.treasureCount,
		s.spin.symbolGrid,
		s.spin.winGrid,
		ToJSON(s.spin.winResults),
		s.spin.nextSymbolGrid,
	)

	//global.GVA_LOG.Sugar().Debugf(
	//	"\n"+
	//		"========== Step结算信息 ==========\n"+
	//		"本步奖金: %.2f\n"+
	//		"累计总奖金(spinBonusAmount): %.2f\n"+
	//		"累计免费奖金(freeBonusAmount): %.2f\n"+
	//		"累计回合奖金(roundBonus): %.2f\n"+
	//		"是否回合结束: %v\n"+
	//		"是否免费回合: %v\n"+
	//		"========== 网格信息 ==========\n"+
	//		"srollers=%v\n"+
	//		"ABC=%v\n"+
	//		"nextABC=%v\n"+
	//		"symbol=%v\n"+
	//		"winGrid=%v\n"+
	//		"nextSymbol=%v\n"+
	//		"========== 完整结果 ==========\n"+
	//		"ret=%v\n",
	//	stepBonus,
	//	spinBonusTotal,
	//	freeBonusTotal,
	//	roundBonusTotal,
	//	s.spin.isRoundOver,
	//	s.isFreeRound,
	//	ToJSON(s.spin.rollers),
	//	ToJSON(s.spin.femaleCountsForFree),
	//	ToJSON(s.spin.nextFemaleCountsForFree),
	//	WGrid(s.spin.symbolGrid),
	//	WGrid(s.spin.winGrid),
	//	WGrid(s.spin.nextSymbolGrid),
	//	ToJSON(ret),
	//)
}
