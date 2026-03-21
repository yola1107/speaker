package sjxj

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
	// RTP测试模式或无倍数时直接返回
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
		if s.isFreeRound {
			s.client.ClientOfFreeGame.IncrFreeTotalMoney(rounded)
		}
	}
}

func (s *betOrderService) getScatterCount() int64 {
	var count int64
	startRow := _rowCountReward
	if s.isFreeRound {
		startRow = _rowCount - s.scene.UnlockedRows
	}
	for r := startRow; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.symbolGrid[r][c] == _treasure {
				count++
			}
		}
	}
	return count
}

func (s *betOrderService) handleSymbolGrid() {
	var symbolGrid int64Grid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			symbolGrid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
	s.symbolGrid = symbolGrid
}

func (s *betOrderService) calcCurrentFreeGameMul() (bool, int64, int64) {
	cfg := s.gameConfig.FreeScatterMulByRow
	startRow := _rowCount - s.scene.UnlockedRows

	isFullScatter := true
	var mul int64
	var newScatterCount int64
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.symbolGrid[r][c] != _treasure {
				if r >= startRow {
					isFullScatter = false
				}
				continue
			}

			if s.scene.ScatterLock[r][c] == 0 {
				s.scene.ScatterLock[r][c] = cfg[r][rand.IntN(len(cfg[r]))]
			}

			if r >= startRow {
				mul += s.scene.ScatterLock[r][c]
				newScatterCount++
			}
		}
	}
	return isFullScatter, mul, newScatterCount
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
			symbolCandidates [9]int64
			candCount        int
		)

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
				if odds := s.getSymbolBaseMultiplier(symbol, int(count)); odds > 0 {
					winInfos = append(winInfos, WinInfo{
						Symbol:      symbol,
						SymbolCount: count,
						LineCount:   int64(i),
						Odds:        odds,
						WinGrid:     winGrid,
					})
					for _, p := range line {
						r, c := p/_colCount, p%_colCount
						if winGrid[r][c] > 0 {
							totalWinGrid[r][c] = 1
						}
					}
				}
			}
		}
	}

	s.winInfos = winInfos
	s.winGrid = totalWinGrid
}

func btoi(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
