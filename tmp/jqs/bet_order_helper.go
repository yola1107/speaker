package jqs

import (
	"egame-grpc/game/common"
	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"fmt"
	"strconv"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// 获取请求上下文
func (s *betOrderService) getRequestContext() bool {
	if s.isRtp {
		return true
	}
	mer, mem, ga, ok := common.GetRequestContext(s.req)
	if !ok {
		return false
	}
	s.merchant = mer
	s.member = mem
	s.game = ga
	return true
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
func (s *betOrderService) symbolGridToString() string {
	symbolStr := ""
	symbolSN := 1
	for row := 0; row < _rowCount; row++ {
		for col := 0; col < _colCount; col++ {
			symbolStr += strconv.Itoa(symbolSN)
			symbolStr += ":"
			symbolStr += strconv.FormatInt(s.symbolGrid[row][col], 10)
			symbolStr += "; "
			symbolSN++
		}
	}
	return symbolStr
}

// 中奖网格转换为字符串
func (s *betOrderService) winGridToString() string {
	winningStr := ""
	winningSN := 1
	for row := 0; row < _rowCount; row++ {
		for col := 0; col < _colCount; col++ {
			winningStr += strconv.Itoa(winningSN)
			winningStr += ":"
			winningStr += strconv.FormatInt(s.winGrid[row][col], 10)
			winningStr += "; "
			winningSN++
		}
	}
	return winningStr
}

// 更新奖金金额
func (s *betOrderService) updateBonusAmount() {
	if s.stepMultiplier == 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	s.bonusAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(s.stepMultiplier))
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		rounded := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(rounded)
		s.client.ClientOfFreeGame.IncRoundBonus(rounded)
		if s.scene.Stage == _spinTypeFree {
			s.client.ClientOfFreeGame.IncrFreeTotalMoney(rounded)
		}
	}
}
