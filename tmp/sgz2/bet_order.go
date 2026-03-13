package sgz

import (
	"errors"
	"fmt"

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
	req                   *request.BetOrderReq // 用户请求
	merchant              *merchant.Merchant   // 商户信息
	member                *member.Member       // 用户信息
	game                  *game.Game           // 游戏信息
	client                *client.Client       // 用户上下文
	lastOrder             *game.GameOrder      // 用户上一个订单
	scene                 *SpinSceneData       // 场景中间态数据
	gameConfig            *gameConfigJson      // 配置数据
	gameOrder             *game.GameOrder      // 订单
	bonusAmount           decimal.Decimal      // 奖金金额
	betAmount             decimal.Decimal      // spin 下注金额
	amount                decimal.Decimal      // step 扣费金额
	orderSn               *common.OrderSN      // 订单号
	isRoundOver           bool                 // 回合是否结束
	isFreeRound           bool                 // 是否为免费回合
	scatterCount          int64                // 夺宝符个数
	addFreeTime           int64                // 增加的免费次数
	gameMultiple          int64                // 连续消除倍数，初始1倍（从 scene.ContinueNum 计算得出）
	freeGameMultiples     []int64              // 免费模式: 当前英雄对应的倍数列表
	freeGameMultipleIndex int64                // 免费模式: 当前连击倍数在配置列表中的索引
	lineMultiplier        int64                // 线倍数
	stepMultiplier        int64                // Step倍数
	winInfos              []WinInfo            // 中奖信息
	nextSymbolGrid        int64Grid            // 下一把 step 符号网格
	symbolGrid            int64Grid            // 符号网格（4行5列）
	winGrid               int64Grid            // 中奖网格（4行5列，只包含参与中奖的行）
	winGridReward         int64GridW           // 奖励网格（3行5列，只包含参与中奖的行）
	debug                 rtpDebugData         // 是否为RTP测试流程
}

func newBetOrderService() *betOrderService {
	s := &betOrderService{}
	s.initGameConfigs()
	return s
}

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

	return s.getBetResultMap()
}

func (s *betOrderService) getBetResultMap() ([]byte, string, error) {
	result := &pb.Sgz_BetOrderResponse{
		OrderSN:      proto.String(s.orderSn.OrderSN),
		Balance:      proto.Float64(s.gameOrder.CurBalance),
		BetAmount:    proto.Float64(s.betAmount.Round(2).InexactFloat64()),
		CurrentWin:   proto.Float64(s.bonusAmount.Round(2).InexactFloat64()),
		FreeWin:      proto.Float64(s.client.ClientOfFreeGame.GetFreeTotalMoney()),
		TotalWin:     proto.Float64(s.client.ClientOfFreeGame.GetGeneralWinTotal()),
		Free:         proto.Bool(s.isFreeRound),
		Review:       proto.Int64(s.req.Review),
		WinInfo:      s.buildWinInfo(),
		Cards:        s.int64GridToPbBoard(s.symbolGrid),
		ScatterCount: proto.Int64(s.scatterCount),
	}
	return s.MarshalData(result)
}

// buildWinInfo 构建 Sgz_WinInfo，参考 jqt 结构
func (s *betOrderService) buildWinInfo() *pb.Sgz_WinInfo {
	state := int64(0)
	if s.isFreeRound {
		state = 1
	}
	winArr := make([]*pb.Sgz_WinArr, len(s.winInfos))
	for i, elem := range s.winInfos {
		winArr[i] = &pb.Sgz_WinArr{
			Val:     proto.Int64(elem.Symbol),
			RoadNum: proto.Int64(elem.LineCount),
			StarNum: proto.Int64(elem.SymbolCount),
			Odds:    proto.Int64(elem.Odds),
			Mul:     proto.Int64(elem.Odds),
			Grid:    s.int64GridToPbBoard(elem.WinGrid),
		}
	}
	heroID, freeMulIndex, freeMulList := s.calcMulData()
	return &pb.Sgz_WinInfo{
		IsRoundOver:           proto.Bool(s.isRoundOver),
		Multi:                 proto.Int64(s.stepMultiplier),
		State:                 proto.Int64(state),
		FreeNum:               proto.Int64(int64(s.client.ClientOfFreeGame.GetFreeNum())),
		FreeTime:              proto.Int64(int64(s.client.ClientOfFreeGame.GetFreeTimes())),
		WinArr:                winArr,
		WinGrid:               s.int64GridToPbBoard(s.winGrid),
		IsGameOver:            proto.Bool(s.isFreeRound && s.isRoundOver && s.scene.FreeNum <= 0), // ++
		AddFreeNum:            proto.Int64(s.addFreeTime),
		FreeGameMultiple:      freeMulList,
		FreeGameMultipleIndex: proto.Int64(freeMulIndex),
		HeroId:                proto.Int64(heroID),
		CityValue:             proto.Int64(s.scene.CityValue),
		RoundWin:              proto.Float64(s.calcRoundWin()),
		KingDom:               s.GetKingdomMap(s.scene.CurrMaxUnlockHeroID),
		IsNewHeroUnlock:       proto.Bool(s.isFreeRound && s.scene.CurrMaxUnlockHeroID > s.scene.LastMaxUnlockHero), // 本次spin是否解锁了新英雄
	}
}

func (s *betOrderService) calcMulData() (int64, int64, []int64) {
	heroID := s.scene.FreeHeroID
	freeMulList := s.freeGameMultiples
	freeMulIndex := s.freeGameMultipleIndex
	// 特殊情况：基础模式进入免费时，提前计算免费游戏相关数据供
	if !s.isFreeRound && s.scene.FreeNum > 0 && s.addFreeTime > 0 {

		freeMulList = s.gameConfig.FreeMultipleMap[heroID]
		freeMulIndex = 0
	}
	if freeMulList == nil {
		freeMulList = []int64{}
	}
	return heroID, freeMulIndex, freeMulList
}

// int64GridToPbBoard 将 int64Grid 转为 common.Board，行优先 4行×5列
func (s *betOrderService) int64GridToPbBoard(grid int64Grid) *pb.Board {
	elements := make([]int64, _rowCount*_colCount)
	for row := 0; row < _rowCount; row++ {
		for col := 0; col < _colCount; col++ {
			elements[row*_colCount+col] = grid[row][col]
		}
	}
	return &pb.Board{Elements: elements}
}

func (s *betOrderService) MarshalData(data proto.Message) ([]byte, string, error) {
	pbData, err := proto.Marshal(data)
	if err != nil {
		return nil, "", err
	}
	jsonData, err := json.CJSON.MarshalToString(data)
	if err != nil {
		return nil, "", err
	}
	return pbData, jsonData, nil
}

func (s *betOrderService) calcRoundWin() float64 {
	return decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(s.scene.RoundMultiplier)).
		Round(2).InexactFloat64()
}
