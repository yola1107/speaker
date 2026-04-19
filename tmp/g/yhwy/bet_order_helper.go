package yhwy

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

func (s *betOrderService) updateBonusAmount(stepMultiplier int64) {
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

func (s *betOrderService) handleSymbolGrid() {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.symbolGrid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}

	s.handleMystery()
}

func (s *betOrderService) handleMystery() {
	s.debug.origin = s.symbolGrid
	s.mysteryGrid = int64Grid{}

	sakuraEndCol := -1
	if !s.isFreeRound && s.isHitSakuraTriggerRate() {
		sakuraEndCol = s.pickSakuraReels()
	}

	hit := false
	x := s.pickMMysterySymbol()
	for c := 0; c < _colCount; c++ {
		for r := 0; r < _rowCount; r++ {
			sym := s.symbolGrid[r][c]
			if sym != _mystery && c >= sakuraEndCol && (!s.isFreeRound || s.scene.Lock[r][c] == 0) {
				continue
			}
			if sakuraEndCol == -1 {
				s.scene.CollectCount++
			}
			s.mysteryGrid[r][c] = x
			s.symbolGrid[r][c] = x
			hit = true
		}
	}

	// 免费游戏保存所有揭开的符号位置
	if s.isFreeRound {
		s.scene.Lock = s.mysteryGrid
	}
	if sakuraEndCol != -1 {
		s.scene.CollectCount = 0 // 触发樱吹雪 清理统计个数
	}

	if !hit {
		s.debug.mysSymbol = -1
	} else {
		s.debug.mysSymbol = x
	}
	s.debug.sakuraCol = sakuraEndCol
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

func (s *betOrderService) checkSymbolGridWin() {
	var winInfos []WinInfo
	var totalWinGrid int64Grid

	for i, line := range s.gameConfig.Lines {
		firstP := line[0]
		firstR := firstP / _colCount
		firstC := firstP % _colCount
		firstSymbol := s.symbolGrid[firstR][firstC]

		var (
			symbolCandidates [_wild + 1]int64
			candCount        int
		)

		if firstSymbol == _wild {
			for symbol := _blank + 1; symbol <= _wild; symbol++ {
				symbolCandidates[candCount] = symbol
				candCount++
			}
		} else if firstSymbol >= _blank+1 && firstSymbol <= _wild {
			symbolCandidates[0] = firstSymbol
			candCount = 1
		} else {
			continue
		}

		var (
			bestWin     *WinInfo
			bestWinGrid int64Grid
		)
		for idx := 0; idx < candCount; idx++ {
			symbol := symbolCandidates[idx]
			var count int64
			var winGrid int64Grid
			var exist bool

			for _, p := range line {
				r, c := p/_colCount, p%_colCount
				currSymbol := s.symbolGrid[r][c]
				if currSymbol == symbol || currSymbol == _wild {
					if currSymbol == symbol {
						exist = true
					}
					winGrid[r][c] = currSymbol
					count++
				} else {
					break
				}
			}

			if count >= _minMatchCount && exist {
				if odds := s.getSymbolBaseMultiplier(symbol, int(count)); odds > 0 {
					curr := WinInfo{
						Symbol:      symbol,
						SymbolCount: count,
						LineCount:   int64(i),
						Odds:        odds,
						WinGrid:     winGrid,
					}
					// 同一支付线命中多个候选符号时，仅保留赔率最高的结果。
					if bestWin == nil || curr.Odds > bestWin.Odds {
						tmp := curr
						bestWin = &tmp
						bestWinGrid = winGrid
					}
				}
			}
		}
		if bestWin != nil {
			winInfos = append(winInfos, *bestWin)
			for _, p := range line {
				r, c := p/_colCount, p%_colCount
				if bestWinGrid[r][c] > 0 {
					totalWinGrid[r][c] = 1
				}
			}
		}
	}

	s.winInfos = winInfos
	s.winGrid = totalWinGrid
}
