package sjnws3

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
	req             *request.BetOrderReq // 用户请求
	merchant        *merchant.Merchant   // 商户信息
	member          *member.Member       // 用户信息
	game            *game.Game           // 游戏信息
	client          *client.Client       // 用户上下文
	lastOrder       *game.GameOrder      // 用户上一个订单
	gameRedis       *redis.Client        // 游戏 redis
	scene           Scene                // 场景中间态数据
	gameOrder       *game.GameOrder      // 订单
	bonusAmount     decimal.Decimal      // 奖金金额
	betAmount       decimal.Decimal      // spin 下注金额
	amount          decimal.Decimal      // step 扣费金额
	orderSN         string               // 订单号
	parentOrderSN   string               // 父订单号，回合第一个 step 此字段为空
	freeOrderSN     string               // 触发免费的回合的父订单号，基础 step 此字段为空
	gameType        int64                // 游戏type
	toTolFreeAmount float64
	IsFreeSpin      bool // 当前spin是不是免费
	StepMulTy       int64
	grid            int64GridY
	midgrid         int64GridY
	winGrid         int64GridY
	winCards        HisGridY
	winList         []int
	setNewList      []int
	cutList         []int
	EndSetList      []int
	winDetails      [_rowCount * _colCount]int
	colList         map[int][]int
	nextGrid        int64GridY
	winResult       []*winResult
	ScatterNum      int
	BonusState      int
	IsRestart       bool //如果上一次已经中奖了
	cfg             *gameConfigJson
	bonusMap        map[int]*betFreeGame
	ColInfoMap      map[int]*ColInfo //某一列的数据，过度到下一局游戏，滚轴新增符号，淘汰中奖的符号用
	winCol          []int            //中奖列
	ExMul           int64
	PreMul          int64
	ContinuNum      int
	result          *BaseSpinResult
	SymbolColMap    map[string]*Pos
	bonusLine       [][]*Pos
	symbolMulMap    map[int]map[int]int
	symbolList      []int
	symbolCol       map[string]*Pos
}

// 统一下注请求接口，无论是免费还是普通
func (s *betOrderService) betOrder(req *request.BetOrderReq) (*BaseSpinResult, error) {
	//zap.L().Debug("开始下注")
	s.req = req
	//global.GVA_LOG.Debug("BetOrder",
	//	zap.Any("req.BaseMoney", req.BaseMoney),
	//	zap.Any("req.Multiple", req.Multiple),
	//	zap.Any("req.Purchase", req.Purchase))

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
	//加载场景数据
	s.reloadScene()

	if s.scene.IsRespin {
		if s.scene.BonusState == _freeGame && s.scene.BonusNum <= 0 {
			return nil, ActError
		}
		if s.scene.BonusState == _normalGame && s.scene.BonusNum <= 0 {

			return nil, ActError
		}

	}
	if check, err := s.checkInit(); !check {
		return nil, err
	}
	s.setPretraMul()
	var baseRes *BaseSpinResult
	if s.IsFreeSpin {
		baseRes, err = s.reSpinBase()
	} else {
		baseRes, err = s.baseSpin()
	}
	if err != nil {
		return nil, err
	}
	//baseRes.Balance = s.getCurrentBalance()
	if ok, err = s.updateGameOrder(baseRes); !ok {
		return nil, err
	}
	s.checkCleanFreeData()
	if err = s.settleStep(); err != nil {
		return nil, err
	}
	err = s.saveScene()
	if err != nil {
		return nil, err
	}
	baseRes.Balance = s.gameOrder.CurBalance
	return baseRes, nil
}
