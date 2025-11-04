package xxg2

import (
	"strconv"
	"strings"

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
	if s.debug.open {
		return
	}
	if len(global.GVA_GAME_REDIS) == 0 {
		return
	}
	index := GameID % int64(len(global.GVA_GAME_REDIS))
	s.gameRedis = global.GVA_GAME_REDIS[index]
}

// updateBetAmount 更新下注金额
func (s *betOrderService) updateBetAmount() bool {
	// 校验参数
	if !_cnf.validateBetSize(s.req.BaseMoney) {
		global.GVA_LOG.Warn("invalid baseMoney", zap.Float64("value", s.req.BaseMoney))
		return false
	}
	if !_cnf.validateBetLevel(s.req.Multiple) {
		global.GVA_LOG.Warn("invalid multiple", zap.Int64("value", s.req.Multiple))
		return false
	}

	// 计算下注金额
	s.betAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_cnf.BaseBat))

	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("invalid betAmount", zap.String("amount", s.betAmount.String()))
		return false
	}
	return true
}

// contains 检查值是否在切片中
func contains[T comparable](slice []T, val T) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
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

// gridToString 网格转字符串
func gridToString(grid *int64Grid) string {
	if grid == nil {
		return ""
	}

	var b strings.Builder
	b.Grow(int(_rowCount * _colCount * gridStringCapacity))

	sn := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			b.WriteString(strconv.Itoa(sn))
			b.WriteByte(':')
			b.WriteString(strconv.FormatInt(grid[row][col], 10))
			b.WriteString("; ")
			sn++
		}
	}
	return b.String()
}

// symbolGridToString 符号网格转字符串
func (s *betOrderService) symbolGridToString() string {
	return gridToString(s.symbolGrid)
}

// winGridToString 中奖网格转字符串
func (s *betOrderService) winGridToString() string {
	return gridToString(s.winGrid)
}

// reverseGridRows 网格行序反转
func reverseGridRows(grid *int64Grid) int64Grid {
	if grid == nil {
		return int64Grid{}
	}
	var reversed int64Grid
	for i := int64(0); i < _rowCount; i++ {
		reversed[i] = grid[_rowCount-1-i]
	}
	return reversed
}

// reverseBats 交换bat的X/Y坐标(服务器行列→客户端列行)
func reverseBats(bats []*Bat) []*Bat {
	if len(bats) == 0 {
		return bats
	}
	reversed := make([]*Bat, len(bats))
	for i, bat := range bats {
		reversed[i] = &Bat{
			X:      bat.Y,
			Y:      bat.X,
			TransX: bat.TransY,
			TransY: bat.TransX,
			Syb:    bat.Syb,
			Sybn:   bat.Sybn,
		}
	}
	return reversed
}

// reverseWinResults 反转WinPositions的行序
func reverseWinResults(winResults []*winResult) []*winResult {
	if len(winResults) == 0 {
		return winResults
	}
	reversed := make([]*winResult, len(winResults))
	for i, wr := range winResults {
		reversed[i] = &winResult{
			Symbol:             wr.Symbol,
			SymbolCount:        wr.SymbolCount,
			LineCount:          wr.LineCount,
			BaseLineMultiplier: wr.BaseLineMultiplier,
			TotalMultiplier:    wr.TotalMultiplier,
			WinPositions:       reverseGridRows(&wr.WinPositions),
		}
	}
	return reversed
}
