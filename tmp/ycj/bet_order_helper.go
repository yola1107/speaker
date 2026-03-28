package ycj

import (
	"fmt"
	"strconv"
	"strings"

	"egame-grpc/game/common"
	"egame-grpc/gamelogic"
	"egame-grpc/global"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

func (s *betOrderService) getRequestContext() error {
	mer, mem, ga, err := common.GetRequestContext(s.req)
	if err != nil {
		global.GVA_LOG.Error("getRequestContext error.")
		return err
	}
	s.merchant, s.member, s.game = mer, mem, ga
	return nil
}

func (s *betOrderService) updateBetAmount() bool {
	s.betAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_baseMultiplier))
	s.amount = s.betAmount

	if s.betAmount.LessThanOrEqual(decimal.Zero) || s.amount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
		return false
	}
	return true
}

func (s *betOrderService) checkBalance() bool {
	f, _ := s.amount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

func (s *betOrderService) symbolGridToString(symbolGrid int64Grid) string {
	var b strings.Builder
	b.Grow(512)
	cellIndex := 0
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			b.WriteString(strconv.Itoa(cellIndex + 1))
			b.WriteString(":")
			b.WriteString(strconv.FormatInt(symbolGrid[r][c], 10))
			b.WriteString("; ")
			cellIndex++
		}
	}
	return b.String()
}

func (s *betOrderService) int64GridToArray(grid int64Grid) []int64 {
	elements := make([]int64, _rowCount*_colCount)
	for c := 0; c < _colCount; c++ {
		elements[c] = grid[0][c]
	}
	return elements
}

func (s *betOrderService) updateBonusAmount(stepMultiplier float64) {
	if s.debug.open || stepMultiplier == 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	bonusAmount := s.betAmount.Mul(decimal.NewFromFloat(stepMultiplier))
	s.bonusAmount = bonusAmount

	if s.bonusAmount.GreaterThan(decimal.Zero) {
		rounded := bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(rounded)
		s.client.ClientOfFreeGame.IncRoundBonus(rounded)
		if s.isFreeRound {
			s.client.ClientOfFreeGame.IncrFreeTotalMoney(rounded)
		}
	}
}

// handleSymbolGrid 将场景数据填充到符号网格
func (s *betOrderService) handleSymbolGrid() {
	for c := 0; c < _colCount; c++ {
		s.symbolGrid[0][c] = int64(s.scene.SymbolRoller[c].BoardSymbol[0])
	}
}

// findWinInfos 判奖核心逻辑
func (s *betOrderService) findWinInfos() WinResult {
	left := s.symbolGrid[0][0]
	mid := s.symbolGrid[0][1]
	right := s.symbolGrid[0][2]

	result := WinResult{}

	// 中间为空：检查推展模式
	if mid == _blank {
		if isNumberSymbol(left) && isNumberSymbol(right) && left == right {
			result.TriggerExtend = true
		}
		return result
	}

	// 左或右非数字：检查重转模式（仅免费模式）
	if !isNumberSymbol(left) || !isNumberSymbol(right) {
		// 与策划口径保持一致：左右非数字（含空）时不触发重转。
		return result
	}

	// 走到这里时，左右已经都被判定为数字（即不含空符号）。
	// 保留等价显式判断，降低后续维护误读风险。
	if left != _blank && left == right {
		result.Win = true
		result.Multiplier = s.calculateMultiplier(left, mid, right)
		if isFreeSpinSymbol(mid) {
			result.TriggerFree = true
			result.FreeSpinNum = int64(s.gameConfig.SymbolMul[mid])
		}
	} else {
		// 左右不同：重转模式（仅免费模式）
		if s.isFreeRound {
			result.TriggerRespin = true
		}
	}

	return result
}

// calculateMultiplier 计算返奖倍数
// 银行+0.5+0.5 → 0.5倍；X5+5+5 → 25倍
// 仅在中奖前提（左右数字相等）下调用，公式为：数字值 × 中间倍率（只乘一次）
func (s *betOrderService) calculateMultiplier(left, mid, right int64) float64 {
	symbolMul := s.gameConfig.SymbolMul
	leftVal := symbolMul[left]

	// 中间符号系数：翻倍符号用配置值，银行/夺宝符号为1
	midVal := float64(1)
	if isMultiplierSymbol(mid) {
		midVal = symbolMul[mid]
	}

	// 倍数 = 数字值 × 中间倍率（中奖时 left == right，只乘一次）
	multiplier := leftVal * midVal

	if multiplier > float64(_maxMultiplier) {
		multiplier = float64(_maxMultiplier)
	}
	return multiplier
}

func btoi(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
