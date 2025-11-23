package mahjong

import (
	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/utils/json"
	"fmt"
	"strconv"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// 获取请求上下文
func (s *betOrderService) getRequestContext() bool {
	switch {
	case !s.mdbGetMerchant():
		return false
	case !s.mdbGetMember():
		return false
	case !s.mdbGetGame():
		return false
	default:
		return true
	}
}

// 初始化游戏redis
func (s *betOrderService) selectGameRedis() {
	index := _gameID % int64(len(global.GVA_GAME_REDIS))
	s.gameRedis = global.GVA_GAME_REDIS[index]
}

// 更新下注金额
func (s *betOrderService) updateBetAmount() bool {
	betAmount := decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_baseMultiplier))
	s.betAmount = betAmount
	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
		return false
	}
	return true
}

// 检查用户余额
func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

// 符号网格转换为字符串
func (s *betOrderService) symbolGridToString(symbolGrid int64Grid) string {

	typeCard := ""
	num := 0
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			typeCard += strconv.Itoa(num + 1)
			typeCard += ":"
			typeCard += strconv.Itoa(int(symbolGrid[i][j]))
			typeCard += "; "
			num++
		}
	}

	return typeCard
}

// 中奖网格转换为字符串
func (s *betOrderService) winGridToString(result *BaseSpinResult) string {

	winCard := ""
	numW := 0
	for i := 0; i < 4; i++ {
		for j := 0; j < 5; j++ {
			winCard += strconv.Itoa(numW + 1)
			winCard += ":"
			winCard += strconv.Itoa(int(result.winGrid[i][j]))
			winCard += "; "
			numW++
		}
	}

	return winCard
}

// 更新奖金金额
func (s *betOrderService) updateBonusAmount(stepMultiplier int64) decimal.Decimal {
	bonusAmount := decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(stepMultiplier))
	s.bonusAmount = bonusAmount
	return bonusAmount
}

// 获得中奖路数及详情
// 1中奖路数 2投注倍数 3连续中奖倍数（头部倍数）4连线倍数 5免费次数 6 免费局连中次数
func (g *betOrderService) getWinDetail(routeDetails []CardType, nwin int64, freeCount, freeNum, scatter int64) string {
	var returnRouteDetail []CardType
	if nwin > 0 {
		for _, v := range routeDetails {
			wd := &CardType{}
			wd.Type = v.Type
			wd.Route = v.Route
			wd.Multiple = v.Multiple
			wd.Way = v.Way
			returnRouteDetail = append(returnRouteDetail, *wd)
		}
	}

	// 存储胡的个数和次数
	if freeNum > 0 && freeCount >= scatter && len(returnRouteDetail) == 0 {
		var cardType CardType
		cardType.Type = int(_treasure)  // 牌型
		cardType.Route = int(freeCount) //夺宝符个数
		cardType.Multiple = 0
		cardType.Way = int(freeNum) //免费次数
		returnRouteDetail = append(returnRouteDetail, cardType)
	}

	if len(returnRouteDetail) == 0 {
		return ""
	}

	winDetailsBytes, _ := json.CJSON.Marshal(returnRouteDetail)

	return string(winDetailsBytes)
}
