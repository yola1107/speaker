package ycj

import (
	"fmt"

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

	if s.betAmount.LessThanOrEqual(decimal.Zero) {
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

func (s *betOrderService) updateBonusAmount(stepMultiplier float64) {
	if s.debug.open || stepMultiplier == 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	s.bonusAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromFloat(stepMultiplier))
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		rounded := s.bonusAmount.Round(2).InexactFloat64()
		s.scene.TotalWin += rounded
		s.scene.RoundWin += rounded
		if s.isFreeRound {
			s.scene.FreeWin += rounded
		}
	}
}
func int64GridToArray(grid int64Grid) []int64 {
	elements := make([]int64, _rowCount*_colCount)
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			elements[r*_colCount+c] = grid[r][c]
		}
	}
	return elements
}

// handleSymbolGrid 将场景数据填充到符号网格
func (s *betOrderService) handleSymbolGrid() {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.symbolGrid[r][c] = int64(s.scene.SymbolRoller[c].BoardSymbol[r])
		}
	}
}

// findWinInfos 判奖核心逻辑
func (s *betOrderService) findWinInfos() WinResult {
	left := s.symbolGrid[_midRow][0]
	mid := s.symbolGrid[_midRow][1]
	right := s.symbolGrid[_midRow][2]

	isNumberLeft := isNumberSymbol(left)
	isNumberRight := isNumberSymbol(right)

	result := WinResult{}

	// 中间为空：检查推展模式（同一回合仅允许一次下落补判）
	if mid == _blank {
		if isNumberLeft && isNumberRight && left == right && (s.scene.Done&_doneExtend == 0) {
			result.TriggerExtend = true
		}
		return result
	}

	// 左或右非数字：检查重转模式（仅免费模式）
	if !isNumberLeft || !isNumberRight {
		// 左右非数字（含空）时不触发重转。
		return result
	}

	//【推展模式】当左侧卷轴和右侧卷轴上的数字相同，中间卷轴是空符号时，中间卷轴会向下滚动一个符号的位置，并根据滚动的结果判奖。
	//【重转模式】重转模式只会在免费旋转中生效，当中间卷轴是非空符号，且左侧卷轴和右侧卷轴都为非空符号且数字不相同时，左侧卷轴和右侧卷轴重新旋转一次，并根据旋转结果判奖。（竞品游戏是在普通模式和免费模式都是生效）
	//【夺宝模式】当中间卷轴是免费旋转符号，左侧卷轴和右侧上的数字相同时，除正常返奖外，还进入夺宝模式。在夺宝模式中可以触发夺宝模式，此时累积免费旋转次数。

	// 左右已经都被判定为数字（即不含空符号）。
	if left != _blank && left == right {
		result.Win = true
		result.Multiplier = s.getNumberPay(left) * s.getMiddleMul(mid) * _baseMultiplier
		if isFreeSymbol(mid) {
			result.TriggerFree = true
			result.FreeSpinNum = s.getFreeSpinCount(mid)
		}
	} else {
		// 左右不同：重转模式（仅免费模式；同一回合仅允许一次左右重转）
		if s.isFreeRound && s.scene.Done&_doneRespin == 0 {
			result.TriggerRespin = true
		}
	}

	return result
}

func isNumberSymbol(symbol int64) bool {
	return symbol >= _num01 && symbol <= _num10
}

func isFreeSymbol(symbol int64) bool {
	return symbol >= _free5 && symbol <= _free20
}
