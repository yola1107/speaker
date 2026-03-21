package sbyymx2

import (
	"errors"

	"egame-grpc/game/common"
	"egame-grpc/game/common/pb"
	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type betOrderService struct {
	req            *request.BetOrderReq // 用户请求
	merchant       *merchant.Merchant   // 商户信息
	member         *member.Member       // 用户信息
	game           *game.Game           // 游戏信息
	client         *client.Client       // 用户上下文
	lastOrder      *game.GameOrder      // 用户上一个订单
	scene          *SpinSceneData       // 场景中间态数据
	gameConfig     *gameConfigJson      // 配置数据
	gameOrder      *game.GameOrder      // 订单
	bonusAmount    decimal.Decimal      // 奖金金额
	betAmount      decimal.Decimal      // spin 下注金额
	amount         decimal.Decimal      // step 扣费金额
	orderSn        *common.OrderSN      // 订单号
	lineMultiplier int64                // 线倍数合计（回包）
	stepMultiplier int64                // 步倍数（与 line 同值）
	winInfos       []*winInfo           // 中奖候选
	winResults     []*winResult         // 中奖摘要
	symbolGrid     int64Grid            // 符号网格（3 行 3 列）
	winGrid        int64Grid            // 中奖网格（3 行 3 列）
	debug          rtpDebugData         // 是否为 RTP 测试流程
}

func newBetOrderService() *betOrderService {
	s := &betOrderService{}
	s.initGameConfigs()
	return s
}

func (s *betOrderService) betOrder(req *request.BetOrderReq) ([]byte, string, error) {
	s.req = req
	if err := s.getRequestContext(); err != nil {
		return nil, "", InternalServerError
	}
	c, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("betOrder", zap.Error(errors.New("user not exists")))
		return nil, "", InternalServerError
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
		global.GVA_LOG.Error("betOrder: reloadScene failed", zap.Error(err))
		return nil, "", err
	}

	if err = s.baseSpin(); err != nil {
		return nil, "", err
	}
	if err = s.updateGameOrder(); err != nil {
		return nil, "", err
	}
	if err = s.settleStep(); err != nil {
		return nil, "", err
	}
	if err = s.saveScene(); err != nil {
		return nil, "", err
	}

	return s.getBetResult()
}

func (s *betOrderService) getBetResult() ([]byte, string, error) {
	tease := s.calcTease()
	result := &pb.Sbyymx2_BetOrderResponse{
		OrderSN:        s.gameOrder.OrderSn,
		Balance:        s.gameOrder.CurBalance,
		BetAmount:      s.betAmount.Round(2).InexactFloat64(),
		CurrentWin:     s.bonusAmount.Round(2).InexactFloat64(),
		TotalWin:       s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		Cards:          s.int64GridToPbBoard(s.symbolGrid),
		WinGrid:        s.int64GridToPbBoard(s.winGrid),
		Special:        false,
		Tease:          tease,
		Review:         s.req.Review,
		LineMultiplier: s.lineMultiplier,
		StepMultiplier: s.stepMultiplier,
	}
	pbData, err := proto.Marshal(result)
	if err != nil {
		return nil, "", err
	}
	jsonData, err := json.CJSON.MarshalToString(result)
	if err != nil {
		return nil, "", err
	}
	return pbData, jsonData, nil
}

func (s *betOrderService) int64GridToPbBoard(grid int64Grid) *pb.Board {
	elements := make([]int64, _rowCount*_colCount)
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			elements[r*_colCount+c] = grid[r][c]
		}
	}
	return &pb.Board{Elements: elements}
}
