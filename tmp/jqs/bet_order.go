package jqs

import (
	"egame-grpc/game/common"
	"egame-grpc/game/common/pb"
	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"
	"errors"
	"fmt"

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
	betAmount        decimal.Decimal      // spin 下注金额
	amount           decimal.Decimal      // step 扣费金额
	gameConfig       *gameConfigJson      // 游戏配置
	gameOrder        *game.GameOrder      // 订单
	bonusAmount      decimal.Decimal      // 奖金金额
	scene            *SpinSceneData       // 场景数据
	symbolGrid       int64Grid            // 符号网格
	winInfos         []winInfo            // 中奖信息
	winResults       []*winResult         // 中奖结果
	winArr           []*pb.Jqs_WinArr     // 中奖结果-客户端
	winGrid          int64Grid            // 中奖网格
	lineMultiplier   int64                // 中奖线倍数
	stepMultiplier   int64                // step 倍数
	state            int64                // 0常规 ，1免费，2免费幸运兔，3炸胡
	orderSn          *common.OrderSN
	isFreeRound      bool
	isRtp            bool
	isRoundCompleted bool // 回合是否完成标记
}

// 生成下注服务实例
func newBetOrderService() *betOrderService {
	s := new(betOrderService)
	s.initGameConfigs()
	return s
}

// 下注逻辑
func (s *betOrderService) betOrder(req *request.BetOrderReq) ([]byte, string, error) {
	s.req = req
	if !s.getRequestContext() {
		return nil, "", InternalServerError
	}
	c, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("betOrder", zap.Error(errors.New("user not exists")))
		return nil, "", fmt.Errorf("client not exist")
	}
	s.client = c
	c.BetLock.Lock()
	defer c.BetLock.Unlock()

	lastOrder, _, err := c.GetLastOrder()
	if err != nil {
		return nil, "", InternalServerError
	}
	s.lastOrder = lastOrder
	if s.lastOrder == nil {
		s.cleanScene()
	}

	if err = s.reloadScene(); err != nil {
		return nil, "", err
	}
	if err = s.baseSpin(); err != nil {
		return nil, "", err
	}

	s.updateBonusAmount()
	if _, err = s.updateGameOrder(); err != nil {
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

// 下注控制逻辑
func (s *betOrderService) baseSpin() error {
	if s.isRtp {
		s.syncGameStage() // RTP测试模式需要手动状态转换
	}

	if err := s.initialize(); err != nil {
		return err
	}
	if err := s.initSymbolGrid(); err != nil {
		return InternalServerError
	}
	// initSymbolGrid可能修改了NextStage，需要同步到Stage
	if s.isRtp && s.scene.NextStage > 0 {
		s.syncGameStage()
	}
	/*	if err := s.initMoneySymbol(); err != nil {
		return InternalServerError
	}*/
	if err := s.updateStepResult(); err != nil {
		return InternalServerError
	}
	return nil
}

// 获取下注结果
func (s *betOrderService) getBetResultMap() ([]byte, string, error) {
	freeNum := int64(s.client.ClientOfFreeGame.GetFreeNum())
	freeTimes := int64(s.client.ClientOfFreeGame.GetFreeTimes())
	betAmount := s.betAmount.Round(2).InexactFloat64()
	currentWin := s.bonusAmount.Round(2).InexactFloat64()
	freeWin := s.client.ClientOfFreeGame.GetFreeTotalMoney()
	totalWin := s.client.ClientOfFreeGame.GetGeneralWinTotal()

	// 判断是否为免费模式（考虑NextStage）
	isFreeRound := s.scene.Stage == _spinTypeFree || s.scene.NextStage == _spinTypeFree
	return s.MarshalData(&pb.Jqs_BetOrderResponse{
		OrderSN:    &s.gameOrder.OrderSn,
		Balance:    &s.gameOrder.CurBalance,
		BetAmount:  &betAmount,
		CurrentWin: &currentWin,
		FreeWin:    &freeWin,
		TotalWin:   &totalWin,
		Free:       isFreeRound,
		Review:     &s.req.Review,
		WinInfo: &pb.Jqs_WinInfo{
			Next:     s.scene.NextStage == _spinTypeFree,
			Multi:    &s.stepMultiplier,
			State:    &s.state,
			FreeNum:  &freeNum,
			FreeTime: &freeTimes,
			WinArr:   s.winArr,
			WinGird:  s.int64GridToPbBoard(s.winGrid),
		},
		Cards: s.int64GridToPbBoard(s.symbolGrid),
	})
}
