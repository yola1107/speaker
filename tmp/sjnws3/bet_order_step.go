package sjnws3

import (
	"strconv"
	"time"

	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/pool"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

func (s *betOrderService) getCurrentBalance() float64 {
	currBalance := decimal.NewFromFloat(s.member.Balance).
		Sub(s.amount).
		Add(s.bonusAmount).
		Round(2).
		InexactFloat64()
	return currBalance
}

// 结算step
func (s *betOrderService) settleStep() error { // 94
	poolRecord := pool.GamePoolRecord{
		OrderId:      s.gameOrder.OrderSn,
		MemberId:     s.gameOrder.MemberID,
		GameType:     1,
		GameId:       s.game.ID,
		GameName:     s.game.GameName,
		MerchantID:   s.merchant.ID,
		Merchant:     s.merchant.Merchant,
		Amount:       0,
		BeforeAmount: 0,
		AfterAmount:  0,
		EventType:    1,
		EventName:    "自然蓄水",
		EventDesc:    "",
		CreatedBy:    "SYSTEM",
	}
	s.gameOrder.CreatedAt = time.Now().Unix()
	poolRecord.CreatedAt = time.Now().Unix()
	saveParam := &gamelogic.SaveTransferParam{
		Client:      s.client,
		GameOrder:   s.gameOrder,
		MerchantOne: s.merchant,
		MemberOne:   s.member,
		Ip:          s.req.Ip,
	}
	res := gamelogic.SaveTransfer(saveParam)
	//res.CurBalance 当前余额，已兼容转账、单一
	if res.Err != nil {
		return res.Err
	}
	return nil
}

func (s *betOrderService) getWinDetailsMap(result *BaseSpinResult) map[string]any {
	//global.GVA_LOG.Info("s.scene.TotalWin", zap.Any("s.scene.TotalWin:", s.scene.TotalWin))
	return map[string]any{
		"betMoney":       s.req.BaseMoney,        // 下注金额 - 玩家本次下注的金额
		"bonusState":     s.BonusState,           // 免费状态 - 当前奖金游戏的状态标识
		"balance":        result.Balance,         // 玩家余额 - 下注后的剩余金额
		"free":           result.CurrentRespin,   // 是否在免费游戏中 - true表示正在进行免费游戏
		"review":         0,                      // 特殊字段 - 预留字段，当前固定为0
		"freeNum":        s.scene.FreeTimes,      // 剩余免费次数 - 玩家还剩余的免费游戏次数
		"totalWin":       s.scene.TotalWin,       // 总中奖金额 - 玩家当前spin累计中奖金额
		"win":            result.CurrentWin,      // 当前轮次赢取金额 - 本次旋转赢得的金额
		"freeTotalMoney": result.FreeTotalAmount, // 免费游戏总赢取金额 - 免费游戏期间累计赢取金额
		"treasureNum":    s.ScatterNum,           // 夺宝符号数量 - 当前轮次出现的夺宝符号个数
		"cards":          s.grid,                 // 游戏网格 - 4x5的游戏符号网格
		"winDetails":     result.WinDetails,      // 中奖详情 - 详细的中奖信息数组
		"wincards":       result.WinCards,        // 中奖符号网格 - 标记哪些位置中奖的网格
	}
}

func (s *betOrderService) getBonusDetail(req *request.BetBonusReq, freeTimes int) map[string]any {
	return map[string]any{
		"free":           req.BonusNum, // 奖金游戏类型 - 请求的奖金游戏类型编号
		"freeNum":        freeTimes,    // 剩余免费次数 - 玩家还剩余的免费游戏次数
		"freeTotalMoney": 0,            // 免费游戏总赢取金额 - 当前固定为0，表示未开始免费游戏
	}
}

func (s *betOrderService) updateGameOrder(result *BaseSpinResult) (bool, error) {
	var addFreeTimes int
	if s.IsFreeSpin && s.ScatterNum >= 3 { //免费中中了免费，
		addFreeTimes = s.scene.AddFreeTimes
	}
	var bonusTime int64
	if s.scene.BonusTimes > 0 { //免费的第一次
		bonusTime = int64(s.scene.BonusTimes)
		s.scene.BonusTimes = 0
	}
	gameOrder := game.GameOrder{
		MerchantID:        s.merchant.ID,
		Merchant:          s.merchant.Merchant,
		MemberID:          s.member.ID,
		Member:            s.member.MemberName,
		GameID:            s.game.ID,
		GameName:          s.game.GameName,
		BaseMultiple:      _baseMultiplier,
		Multiple:          s.req.Multiple,
		LineMultiple:      s.StepMulTy,
		BonusHeadMultiple: 1,
		BonusMultiple:     result.StepMultiplier,
		BaseAmount:        s.req.BaseMoney,
		Amount:            s.amount.Round(2).InexactFloat64(),
		ValidAmount:       s.amount.Round(2).InexactFloat64(),
		BonusAmount:       s.bonusAmount.Round(2).InexactFloat64(),
		CurBalance:        s.getCurrentBalance(),
		OrderSn:           result.Osn,
		ParentOrderSn:     s.parentOrderSN,
		FreeOrderSn:       s.freeOrderSN,
		State:             1,
		BonusTimes:        bonusTime,
		IsFree:            int64(s.scene.BonusNum),
		HuNum:             int64(s.ScatterNum),
		FreeNum:           int64(addFreeTimes),
		FreeTimes:         int64(s.client.ClientOfFreeGame.GetFreeTimes()),
	}
	s.gameOrder = &gameOrder

	return s.fillInGameOrderDetails(result)
}

// 填充订单细节
func (s *betOrderService) fillInGameOrderDetails(result *BaseSpinResult) (bool, error) {

	betRawDetail, err := json.CJSON.MarshalToString(result.Cards)

	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return false, err
	}
	s.gameOrder.BetDetail = setCardDetailsMap(result.CardDetails)
	s.gameOrder.BonusDetail = setWinDetailsMap(result.BonusDetails)
	s.gameOrder.BetRawDetail = betRawDetail
	s.gameOrder.BonusHeadMultiple = s.PreMul
	s.gameOrder.BonusMultiple = s.PreMul
	winRawDetail, err := json.CJSON.MarshalToString(s.winGrid)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return false, err
	}
	s.gameOrder.BonusRawDetail = winRawDetail
	if len(s.winResult) == 0 {
		if s.ScatterNum >= 3 {
			winDetails, _ := json.CJSON.MarshalToString([]CardType{{Type: _scatter,
				Way: s.scene.AddFreeTimes, Multiple: 0, Route: s.ScatterNum}})
			s.gameOrder.WinDetails = winDetails
			return true, nil
		}
		s.gameOrder.WinDetails = ""
		return true, nil
	}

	winDetails, err := json.CJSON.MarshalToString(s.SetResultList())
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return false, err
	}
	s.gameOrder.WinDetails = winDetails
	return true, err
}

func setCardDetailsMap(List [20]int) string {
	Str := ""
	for i := 1; i <= len(List); i++ {
		Str = Str + strconv.Itoa(i) + ":"
		if i < len(List) {
			Str = Str + strconv.Itoa(List[i-1]) + "; "
			continue
		}
		Str = Str + strconv.Itoa(List[i-1]) + ";"
	}
	return Str
}

func setWinDetailsMap(List [15]int) string {
	Str := ""
	for i := 1; i <= len(List); i++ {
		Str = Str + strconv.Itoa(i) + ":"
		if i < len(List) {
			Str = Str + strconv.Itoa(List[i-1]) + "; "
			continue
		}
		Str = Str + strconv.Itoa(List[i-1]) + ";"
	}
	return Str
}

type CardType struct {
	Type     int `json:"Type"`     // 牌型
	Way      int `json:"Way"`      // 路数
	Multiple int `json:"Multiple"` // 倍数
	Route    int `json:"Route"`    // 几路
}

func (s *betOrderService) SetResultList() []CardType {
	List := make([]CardType, 0)
	for _, detail := range s.winResult {
		Line := CardType{}
		Line.Type = detail.Symbol
		Line.Way = detail.LineCount
		Line.Multiple = detail.BaseLineMultiplier
		Line.Route = detail.SymbolCount
		List = append(List, Line)
	}
	return List
}
