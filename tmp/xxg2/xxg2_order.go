package xxg2

import (
	"errors"
	"fmt"
	"strings"

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
	req            *request.BetOrderReq // 用户请求
	merchant       *merchant.Merchant   // 商户信息
	member         *member.Member       // 用户信息
	game           *game.Game           // 游戏信息
	client         *client.Client       // 用户上下文
	lastOrder      *game.GameOrder      // 用户上一个订单
	gameOrder      *game.GameOrder      // 当前订单
	orderSN        string               // 订单号
	parentOrderSN  string               // 父订单号（回合第一个step此字段为空）
	freeOrderSN    string               // 触发免费的回合父订单号（基础step为空）
	bonusAmount    decimal.Decimal      // 本Step奖金
	betAmount      decimal.Decimal      // Spin下注金额（回合第一局扣费）
	amount         decimal.Decimal      // Step扣费金额（首局=betAmount，后续=0）
	scene          *SpinSceneData       // 场景数据（持久化状态）
	stepMap        *stepMap             // Step数据（符号网格、蝙蝠、中奖等）
	stepMultiplier int64                // Step总倍数
	lineMultiplier int64                // 线倍数
	winInfos       []*winInfo           // 中奖信息
	symbolGrid     *int64Grid           // 符号网格（填wind后）
	winGrid        *int64Grid           // 中奖网格
	winResults     []*winResult         // 中奖结果
	newFreeCount   int64                // 新增免费次数
	debug          rtpDebugData         // RTP压测调试
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
	if s.lastOrder == nil {
		s.cleanScene()
	}

	// 加载场景数据
	if !s.reloadScene() {
		global.GVA_LOG.Error("betOrder: reloadScene failed")
		return nil, InternalServerError
	}

	return s.doBetOrder()
}

// doBetOrder 执行下注流程
func (s *betOrderService) doBetOrder() (map[string]any, error) {
	// 执行核心spin逻辑
	if _, err := s.baseSpin(); err != nil {
		return nil, err
	}

	// 更新订单
	if !s.updateGameOrder() {
		return nil, InternalServerError
	}

	// 结算
	if !s.settleStep() {
		return nil, InternalServerError
	}

	// 保存当前 isFree 状态（必须在 saveScene() 之前，因为 saveScene 会更新 Stage）
	currentIsFree := s.isFreeRound()

	// 保存场景数据
	if err := s.saveScene(); err != nil {
		global.GVA_LOG.Error("doBetOrder", zap.Error(err))
		return nil, InternalServerError
	}

	return s.buildResultMap(currentIsFree), nil
}

// buildResultMap 构建下注结果（返回给前端）
func (s *betOrderService) buildResultMap(currentIsFree bool) map[string]any {
	ret := map[string]any{
		"orderSN":            s.gameOrder.OrderSn,
		"symbolGrid":         reverseGridRows(s.symbolGrid), // symbolGrid行序反转
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
	}

	if true {
		s.printResultLog(ret)
	}

	return ret
}

// printResultLog 调试日志输出
func (s *betOrderService) printResultLog(ret map[string]any) {
	WGrid := func(grid *int64Grid) string {
		b := strings.Builder{}
		b.WriteString("\n")
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				sym := grid[r][c]
				b.WriteString(fmt.Sprintf("%3d", sym))
				if c < _colCount-1 {
					b.WriteString("| ")
				}
			}
			b.WriteString("\n")
		}
		return b.String()
	}

	global.GVA_LOG.Sugar().Debugf("\norigin=%v symbol=%v winGrid=%vbat=%v"+
		"\nret['bat']=%v\nret['symbolGrid']=%v\nret['winResults']=%v\n",
		WGrid(s.debug.originalGrid),
		WGrid(s.symbolGrid),
		WGrid(s.winGrid),
		ToJSON(s.stepMap.Bat),
		// ret结算结果
		ToJSON(ret["bat"]),
		ToJSON(ret["symbolGrid"]),
		ToJSON(ret["winResults"]),
	)
}
