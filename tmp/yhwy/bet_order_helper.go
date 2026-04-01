package yhwy

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
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			elements[r*_colCount+c] = grid[r][c]
		}
	}
	return elements
}

func cloneGrid(src int64Grid) int64Grid {
	var dst int64Grid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			dst[r][c] = src[r][c]
		}
	}
	return dst
}

func (s *betOrderService) updateBonusAmount(stepMultiplier int64) {
	if s.debug.open || stepMultiplier == 0 {
		s.bonusAmount = decimal.Zero
		return
	}

	s.bonusAmount = s.betAmount.
		Mul(decimal.NewFromInt(stepMultiplier)).
		Div(decimal.NewFromInt(_baseMultiplier))

	if s.bonusAmount.GreaterThan(decimal.Zero) {
		rounded := s.bonusAmount.Round(2).InexactFloat64()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(rounded)
		s.client.ClientOfFreeGame.IncRoundBonus(rounded)
		if s.isFreeRound {
			s.client.ClientOfFreeGame.IncrFreeTotalMoney(rounded)
		}
	}
}

func (s *betOrderService) handleSymbolGrid() {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.originGrid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
}

func (s *betOrderService) getScatterCount() int64 {
	var count int64
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.finalGrid[r][c] == _Scatter {
				count++
			}
		}
	}
	return count
}

func markLineHitsOnTotal(line []int, winGrid *int64Grid, total *int64Grid) {
	for _, p := range line {
		r, c := p/_colCount, p%_colCount
		if (*winGrid)[r][c] > 0 {
			(*total)[r][c] = 1
		}
	}
}

func isRegularSymbol(symbol int64) bool {
	return symbol >= _10 && symbol <= _Miko
}

func (s *betOrderService) checkSymbolGridWin() {
	var winInfos []WinInfo
	var totalWinGrid int64Grid

	for i, line := range s.gameConfig.Lines {
		allWild := true
		for _, p := range line {
			r := p / _colCount
			c := p % _colCount
			if s.finalGrid[r][c] != _Wild {
				allWild = false
				break
			}
		}
		if allWild {
			if odds := s.getSymbolBaseMultiplier(_Wild, len(line)); odds > 0 {
				var winGrid int64Grid
				for _, p := range line {
					r := p / _colCount
					c := p % _colCount
					winGrid[r][c] = 1
				}
				winInfos = append(winInfos, WinInfo{
					Symbol:      _Wild,
					SymbolCount: int64(len(line)),
					LineCount:   int64(i),
					Odds:        odds,
					WinGrid:     winGrid,
				})
				markLineHitsOnTotal(line, &winGrid, &totalWinGrid)
			}
			continue
		}

		firstPos := line[0]
		firstR, firstC := firstPos/_colCount, firstPos%_colCount
		firstSymbol := s.finalGrid[firstR][firstC]

		var candidates []int64
		switch {
		case firstSymbol == _Wild:
			candidates = []int64{_10, _J, _Q, _K, _A, _Geta, _Fan, _Bell, _Ninja, _Miko}
		case isRegularSymbol(firstSymbol):
			candidates = []int64{firstSymbol}
		default:
			continue
		}

		var best *WinInfo
		for _, symbol := range candidates {
			var count int
			var winGrid int64Grid
			for _, p := range line {
				r, c := p/_colCount, p%_colCount
				curr := s.finalGrid[r][c]
				if curr == symbol || curr == _Wild {
					winGrid[r][c] = 1
					count++
					continue
				}
				break
			}
			if count < _minMatchCount {
				continue
			}
			odds := s.getSymbolBaseMultiplier(symbol, count)
			if odds <= 0 {
				continue
			}
			cur := &WinInfo{
				Symbol:      symbol,
				SymbolCount: int64(count),
				LineCount:   int64(i),
				Odds:        odds,
				WinGrid:     winGrid,
			}
			if best == nil || cur.Odds > best.Odds || (cur.Odds == best.Odds && cur.Symbol > best.Symbol) {
				best = cur
			}
		}

		if best != nil {
			winInfos = append(winInfos, *best)
			markLineHitsOnTotal(line, &best.WinGrid, &totalWinGrid)
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
