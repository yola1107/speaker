package pjcd

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
	f, _ := s.betAmount.Float64()
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
		Mul(decimal.NewFromInt(s.stepMultiplier))
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
	var symbolGrid int64Grid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			symbolGrid[_rowCount-1-r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
	s.symbolGrid = symbolGrid
}

/*
func (s *betOrderService) checkSymbolGridWin() {
	var winInfos []WinInfo
	var totalWinGrid int64Grid

	for i, line := range s.gameConfig.Lines {
		for symbol := _blank + 1; symbol < _wild; symbol++ {
			var count int64
			var winGrid int64Grid

			for _, p := range line {
				r := p / _colCount
				c := p % _colCount
				currSymbol := s.symbolGrid[r][c]
				if currSymbol == symbol || isWild(currSymbol) {
					winGrid[r][c] = currSymbol
					count++
				} else {
					break
				}
			}

			if count >= _minMatchCount {
				odds := s.getSymbolBaseMultiplier(symbol, int(count))
				if odds > 0 {
					winInfos = append(winInfos, WinInfo{
						Symbol:      symbol,
						SymbolCount: count,
						LineCount:   int64(i),
						Odds:        odds,
						WinGrid:     winGrid,
					})
					for r := 0; r < _rowCount; r++ {
						for c := 0; c < _colCount; c++ {
							if winGrid[r][c] > 0 {
								totalWinGrid[r][c] = 1
							}
						}
					}
				}
			}
		}
	}

	s.winInfos = winInfos
	s.winGrid = totalWinGrid
}*/

func (s *betOrderService) checkSymbolGridWin() {
	var winInfos []WinInfo
	var totalWinGrid int64Grid

	for i, line := range s.gameConfig.Lines {
		firstP := line[0]
		firstR := firstP / _colCount
		firstC := firstP % _colCount
		firstSymbol := s.symbolGrid[firstR][firstC]

		var (
			symbolCandidates [8]int64
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
				if currSymbol == symbol || isWild(currSymbol) {
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

func (s *betOrderService) calcWildForm() int64Grid {
	s.addWildEliCount = 0
	s.wildMultiplier = 0
	var wildForm int64Grid

	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.winGrid[r][c] > 0 && isWild(s.symbolGrid[r][c]) {
				wildForm[r][c] = s.symbolGrid[r][c] + _mask
				if isEmiWild(wildForm[r][c]) {
					s.addWildEliCount++
				}
			}
		}
	}
	s.scene.TotalWildEliCount += s.addWildEliCount
	return wildForm
}

func (s *betOrderService) moveSymbols(wildForm int64Grid) int64Grid {
	nextSymbolGrid := s.symbolGrid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.winGrid[r][c] > 0 {
				if wildForm[r][c] > 0 && !isEmiWild(wildForm[r][c]) {
					nextSymbolGrid[r][c] = wildForm[r][c]
				} else {
					nextSymbolGrid[r][c] = 0
				}
			}
		}
	}

	s.dropSymbols(&nextSymbolGrid)
	return nextSymbolGrid
}

func (s *betOrderService) dropSymbols(grid *int64Grid) {
	for c := 0; c < _colCount; c++ {
		writePos := 0
		for r := 0; r < _rowCount; r++ {
			if val := (*grid)[r][c]; val != 0 {
				if r != writePos {
					(*grid)[writePos][c] = val
					(*grid)[r][c] = 0
				}
				writePos++
			}
		}
	}
}

func (s *betOrderService) fallingWinSymbols(nextSymbolGrid int64Grid) {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.scene.SymbolRoller[c].BoardSymbol[r] = nextSymbolGrid[_rowCount-1-r][c]
		}
	}
	for i := range s.scene.SymbolRoller {
		s.scene.SymbolRoller[i].ringSymbol(s.getReelForCol(i))
	}
}

// isWild 检查符号是否为wild符号（可替代、不可消除、下落时占位）
func isWild(symbol int64) bool {
	return symbol == 8 || symbol == 18 || symbol == 28 || symbol == 38
}

// isEmiWild 是否是可消除百搭 > 蝴蝶百搭 (毛虫→蝶茧→蝴蝶)
func isEmiWild(symbol int64) bool {
	return symbol == 38
}
