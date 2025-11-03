package xxg2

import (
	"fmt"
	"strconv"
	"strings"

	"egame-grpc/gamelogic"
	"egame-grpc/global"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// getRequestContext 获取请求上下文
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

// selectGameRedis 初始化游戏redis
func (s *betOrderService) selectGameRedis() {
	if s.debug.open {
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
		Mul(decimal.NewFromInt(s.gameConfig.BaseBat))

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

// gridToString 网格转换为字符串（通用函数）
func gridToString(grid *int64Grid) string {
	if grid == nil {
		return ""
	}

	var builder strings.Builder
	builder.Grow(int(_rowCount * _colCount * gridStringCapacity))

	sn := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			builder.WriteString(strconv.Itoa(sn))
			builder.WriteByte(':')
			builder.WriteString(strconv.FormatInt(grid[row][col], 10))
			builder.WriteString("; ")
			sn++
		}
	}
	return builder.String()
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

// reverseBats 交换bat的X/Y坐标（服务器X=行/Y=列 → 客户端x=列/y=行）
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
