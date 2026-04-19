package hcsqy

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
	s.amount = s.betAmount

	if s.req.Purchase > 0 {
		if maxBet := s.gameConfig.Buy.MaxBuyBetAmount; maxBet > 0 && s.betAmount.GreaterThan(decimal.NewFromInt(maxBet)) {
			global.GVA_LOG.Error("updateBetAmount", zap.Error(fmt.Errorf("invalid request params: Purchase [%v,%v,%v]", s.req.BaseMoney, s.req.Multiple, s.req.Purchase)))
			return false
		}
		s.amount = s.betAmount.Mul(decimal.NewFromInt(s.gameConfig.Buy.Price))
	}

	if s.betAmount.LessThanOrEqual(decimal.Zero) || s.amount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v,%v]", s.req.BaseMoney, s.req.Multiple, s.req.Purchase)))
		return false
	}
	return true
}

func (s *betOrderService) checkBalance() bool {
	f, _ := s.amount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

func (s *betOrderService) updateBonusAmount(stepMultiplier int64) {
	// RTP测试模式或无倍数时直接返回
	if s.debug.open || stepMultiplier == 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	s.bonusAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(stepMultiplier))

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

func (s *betOrderService) getScatterCount() int64 {
	var count int64
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.symbolGrid[r][c] == _treasure {
				count++
			}
		}
	}
	return count
}

func (s *betOrderService) handleSymbolGrid() {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.symbolGrid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
}

func (s *betOrderService) checkSymbolGridWin() {
	var winInfos []WinInfo
	var totalWinGrid int64Grid

	for i, line := range s.gameConfig.Lines {
		// 百搭可单独中奖。整条支付线全为百搭时，只按百搭赔付结一次（pay_table 中 ID=8），
		// 不再用 1..7 各枚举一遍以免同线多倍叠加。
		allWild := true
		for _, p := range line {
			r := p / _colCount
			c := p % _colCount
			if s.symbolGrid[r][c] != _wild {
				allWild = false
				break
			}
		}
		if allWild && len(line) >= _minMatchCount {
			if odds := s.getSymbolBaseMultiplier(_wild, 3); odds > 0 {
				var winGrid int64Grid
				for _, p := range line {
					r := p / _colCount
					c := p % _colCount
					winGrid[r][c] = _wild
				}
				winInfos = append(winInfos, WinInfo{
					Symbol:      _wild,
					SymbolCount: int64(len(line)),
					LineCount:   int64(i),
					Odds:        odds,
					WinGrid:     winGrid,
				})
				markLineHitsOnTotal(line, &winGrid, &totalWinGrid)
			}
			continue
		}

		firstP := line[0]
		firstR := firstP / _colCount
		firstC := firstP % _colCount
		firstSymbol := s.symbolGrid[firstR][firstC]

		var symbolCandidates [8]int64
		var candCount int

		if firstSymbol == _wild {
			for symbol := _blank + 1; symbol < _wild; symbol++ {
				symbolCandidates[candCount] = symbol
				candCount++
			}
		} else if firstSymbol >= _blank+1 && firstSymbol < _wild {
			symbolCandidates[0] = firstSymbol
			candCount = 1
		} else {
			continue
		}

		for idx := 0; idx < candCount; idx++ {
			symbol := symbolCandidates[idx]
			var count int64
			var winGrid int64Grid

			for _, p := range line {
				r, c := p/_colCount, p%_colCount
				currSymbol := s.symbolGrid[r][c]
				if currSymbol == symbol || currSymbol == _wild {
					winGrid[r][c] = currSymbol
					count++
				} else {
					break
				}
			}

			if count >= _minMatchCount {
				if odds := s.getSymbolBaseMultiplier(symbol, 3); odds > 0 {
					winInfos = append(winInfos, WinInfo{
						Symbol:      symbol,
						SymbolCount: count,
						LineCount:   int64(i),
						Odds:        odds,
						WinGrid:     winGrid,
					})
					markLineHitsOnTotal(line, &winGrid, &totalWinGrid)
				}
			}
		}
	}

	var totalOdds int64
	for _, w := range winInfos {
		totalOdds += w.Odds
	}
	s.lineMultiplier = totalOdds
	s.winInfos = winInfos
	s.winGrid = totalWinGrid
}

// markLineHitsOnTotal 将本支付线在 winGrid 上命中的格子合并进 total（用于最终 WinGrid 展示）。
func markLineHitsOnTotal(line []int, winGrid *int64Grid, total *int64Grid) {
	for _, p := range line {
		r, c := p/_colCount, p%_colCount
		if (*winGrid)[r][c] > 0 {
			(*total)[r][c] = 1
		}
	}
}

func (s *betOrderService) isPurchaseActive() bool {
	return s.scene.IsPurchase || s.scene.PurchaseAmount > 0
}
