package xxg2

import (
	"fmt"
	"strconv"

	"egame-grpc/gamelogic"
	"egame-grpc/global"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// getRequestContext 获取请求上下文
func (s *betOrderService) getRequestContext() bool {
	return s.mdbGetMerchant() && s.mdbGetMember() && s.mdbGetGame()
}

// selectGameRedis 初始化游戏redis
func (s *betOrderService) selectGameRedis() {
	if s.forRtpBench {
		return
	}
	if len(global.GVA_GAME_REDIS) == 0 {
		return
	}
	index := _gameID % int64(len(global.GVA_GAME_REDIS))
	s.gameRedis = global.GVA_GAME_REDIS[index]
}

// updateBetAmount 更新下注金额
func (s *betOrderService) updateBetAmount() bool {
	s.betAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_baseMultiplier))

	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
		return false
	}
	return true
}

// checkBalance 检查用户余额
func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

// updateBonusAmount 更新奖金金额
func (s *betOrderService) updateBonusAmount() {
	s.bonusAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(s.stepMultiplier))
}

// symbolGridToString 符号网格转换为字符串
func (s *betOrderService) symbolGridToString() string {
	var result string
	sn := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			result += strconv.Itoa(sn) + ":" + strconv.FormatInt(s.symbolGrid[row][col], 10) + "; "
			sn++
		}
	}
	return result
}

// winGridToString 中奖网格转换为字符串
func (s *betOrderService) winGridToString() string {
	if s.winGrid == nil {
		return ""
	}
	var result string
	sn := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			result += strconv.Itoa(sn) + ":" + strconv.FormatInt(s.winGrid[row][col], 10) + "; "
			sn++
		}
	}
	return result
}

// showPostUpdateErrorLog 错误日志记录
func (s *betOrderService) showPostUpdateErrorLog() {
	global.GVA_LOG.Error(
		"showPostUpdateErrorLog",
		zap.Error(fmt.Errorf("step state mismatch")),
		zap.Int64("id", s.member.ID),
		zap.Bool("isFree", s.isFreeRound()),
		zap.Uint64("lastWinID", s.client.ClientOfFreeGame.GetLastWinId()),
		zap.Uint64("lastMapID", s.client.ClientOfFreeGame.GetLastMapId()),
		zap.Uint64("freeNum", s.client.ClientOfFreeGame.GetFreeNum()),
		zap.Uint64("freeTimes", s.client.ClientOfFreeGame.GetFreeTimes()),
		zap.String("orderSn", s.orderSN),
		zap.String("parentOrderSN", s.parentOrderSN),
		zap.String("freeOrderSN", s.freeOrderSN),
	)
}
