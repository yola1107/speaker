package sbyymx2

import (
	"fmt"
	"math/rand/v2"
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

	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
		return false
	}
	return true
}

func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
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

func (s *betOrderService) updateBonusAmount(stepMultiplier int64) {
	// RTP 测试模式或无倍数时直接返回
	if s.debug.open || stepMultiplier == 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	bonusAmount := s.betAmount.
		Mul(decimal.NewFromInt(stepMultiplier)).
		Div(decimal.NewFromInt(_baseMultiplier))
	s.bonusAmount = bonusAmount

	if s.bonusAmount.GreaterThan(decimal.Zero) {
		rounded := bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(rounded)
		s.client.ClientOfFreeGame.IncRoundBonus(rounded)
	}
}

// findWinInfos 仅中间行；左右为 1–7 且相等；中格为百搭或与左右相同
func (s *betOrderService) findWinInfos() {
	s.winInfos = nil
	left := s.symbolGrid[1][0]
	mid := s.symbolGrid[1][1]
	right := s.symbolGrid[1][2]
	if left == _blank || left == _blankCell {
		return
	}
	if right == _blank || right == _blankCell {
		return
	}
	if left < 1 || left > 7 || right < 1 || right > 7 {
		return
	}
	if left != right {
		return
	}
	if left != mid && mid < _wild {
		return
	}
	s.winInfos = []*winInfo{{Symbol: left}}
}

// processWinInfos 线倍数 = pay(左符号) × (中格倍率百搭时 × (mid-100))
func (s *betOrderService) processWinInfos() {
	var winGrid int64Grid
	lineMul := int64(0)
	var winResults []*winResult
	mid := s.symbolGrid[1][1]

	for _, info := range s.winInfos {
		mul := s.getPayMultiplier(info.Symbol)
		if mul == 0 {
			continue
		}
		var extra int64 = 1
		switch {
		case mid == _wild:
			extra = s.pickPlainWildMultiplier()
		case mid > _wild:
			extra = mid - _wild
		}
		if extra <= 0 {
			continue
		}
		if extra > 1 {
			if max := int64(^uint64(0) >> 1); mul > max/extra {
				global.GVA_LOG.Error("processWinInfos: multiplier overflow risk",
					zap.Int64("mul", mul), zap.Int64("extra", extra))
				continue
			}
			mul *= extra
		}
		winResults = append(winResults, &winResult{Symbol: info.Symbol, Multiplier: mul})
		winGrid[1][0] = s.symbolGrid[1][0]
		winGrid[1][1] = s.symbolGrid[1][1]
		winGrid[1][2] = s.symbolGrid[1][2]
		lineMul += mul
	}
	s.lineMultiplier = lineMul
	s.stepMultiplier = lineMul
	s.winResults = winResults
	s.winGrid = winGrid
}

func (s *betOrderService) calcTease() bool {
	if s.debug.open {
		return false
	}
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		return false
	}
	return rand.IntN(1000) < 85
}
