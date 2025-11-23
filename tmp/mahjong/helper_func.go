package mahjong

import (
	"github.com/shopspring/decimal"
)

// 获取夺宝符号数量
func (s *betOrderService) getScatterCount() int64 {
	var treasure int64
	for r := int64(1); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.symbolGrid[r][c] == _treasure {
				treasure++
			}
		}
	}
	return treasure
}

func (s *betOrderService) checkSymbolGridWin() []*winInfo {

	var winInfos []*winInfo
	s.winGrid = int64Grid{}

	for symbol := _blank + 1; symbol < _treasure; symbol++ {
		if info, ok := s.findSymbolWinInfo(symbol); ok {
			winInfos = append(winInfos, info)
		}
	}
	s.winInfos = winInfos
	return winInfos

}

// 查找 step 符号中奖信息
func (s *betOrderService) findSymbolWinInfo(symbol int64) (*winInfo, bool) {
	lineCount := int64(1)
	var winGrid int64Grid

	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(1); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || currSymbol == _wild || currSymbol == (symbol+_goldSymbol) {
				count++
				winGrid[r][c] = currSymbol
			}
		}
		if count == 0 {
			if c >= _minMatchCount {
				symbolMul := s.getSymbolBaseMultiplier(symbol, int(c))
				info := winInfo{Symbol: symbol, SymbolCount: c, LineCount: lineCount, Odds: symbolMul, Multiplier: lineCount * symbolMul, WinGrid: winGrid}
				for r := int64(1); r < _rowCount; r++ {
					for c := int64(0); c < _colCount; c++ {
						if winGrid[r][c] > 0 {
							s.winGrid[r][c] = winGrid[r][c]
						}
					}
				}
				return &info, true
			}
			break
		}
		lineCount *= count
		if c == _colCount-1 {
			symbolMul := s.getSymbolBaseMultiplier(symbol, int(_colCount))
			info := winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, Odds: symbolMul, Multiplier: lineCount * symbolMul, WinGrid: winGrid}
			for r := int64(1); r < _rowCount; r++ {
				for c := int64(0); c < _colCount; c++ {
					if winGrid[r][c] > 0 {
						s.winGrid[r][c] = winGrid[r][c]
					}
				}
			}
			return &info, true
		}
	}
	return nil, false
}

// 处理中奖列表，顺便把中奖的位置置为0，以便下一步处理掉落
func (s *betOrderService) handleWinInfosMultiplier(infos []*winInfo) int64 {

	var winResults []winResult
	var stepMultiplier int64

	for _, info := range infos {
		wRes := s.symbolWinMultiplier(*info)
		winResults = append(winResults, wRes)
		stepMultiplier += wRes.TotalMultiplier
	}

	return stepMultiplier

}

// 处理单个符号的中奖情况
func (s *betOrderService) symbolWinMultiplier(w winInfo) winResult {

	return winResult{
		Symbol:             w.Symbol,
		SymbolCount:        w.SymbolCount,
		LineCount:          w.LineCount,
		BaseLineMultiplier: w.Odds,
		TotalMultiplier:    w.Multiplier,
	}
}

func (s *betOrderService) handleSymbolGrid() {

	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			s.symbolGrid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
}

func (s *betOrderService) updateSpinBonusAmount(bonusAmount decimal.Decimal) {
	s.client.ClientOfFreeGame.IncrGeneralWinTotal(bonusAmount.Round(2).InexactFloat64())
	s.client.ClientOfFreeGame.IncRoundBonus(bonusAmount.Round(2).InexactFloat64())

	if s.isFreeRound() {
		s.client.ClientOfFreeGame.IncrFreeTotalMoney(bonusAmount.Round(2).InexactFloat64())
	}

}

func (s *betOrderService) moveSymbols() int64Grid {

	nextSymbolGrid := s.symbolGrid // 下一轮 step 符号网格

	rows := _rowCount
	cols := _colCount

	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.winGrid[r][c] > 0 {
				nextSymbolGrid[r][c] = 0
				if s.symbolGrid[r][c] >= ERTIAO_k && s.symbolGrid[r][c] <= FA_k {
					nextSymbolGrid[r][c] = _wild
				}
			}
		}
	}

	// 从下往上检查每一列
	for col := 0; col < cols; col++ {
		// 从底部开始向上寻找空位
		for row := rows - 1; row >= 0; row-- {
			if nextSymbolGrid[row][col] == 0 { // 找到空位
				// 向上寻找非空元素来填充
				for above := row - 1; above >= 0; above-- {
					if nextSymbolGrid[above][col] != 0 { // 找到非空元素
						// 将非空元素移动到空位
						nextSymbolGrid[row][col] = nextSymbolGrid[above][col]
						nextSymbolGrid[above][col] = 0 // 原位置变为空
						break
					}
				}
			}
		}
	}

	return nextSymbolGrid
}
func (s *betOrderService) fallingWinSymbols(nextSymbolGrid int64Grid, stage int8) {

	//符号转换一下
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			s.scene.SymbolRoller[c].BoardSymbol[r] = nextSymbolGrid[r][c]
		}
	}

	for i, r := range s.scene.SymbolRoller {
		r.ringSymbol(s.gameConfig, stage, i)
		s.scene.SymbolRoller[i] = r
	}
}

// 符号反转
func (s *betOrderService) reverseSymbolInPlace(SymbolGrid int64Grid) int64Grid {
	SymbolGridTmp := SymbolGrid
	for i := 0; i < len(SymbolGridTmp)/2; i++ {
		j := len(SymbolGridTmp) - 1 - i
		SymbolGridTmp[i], SymbolGridTmp[j] = SymbolGridTmp[j], SymbolGridTmp[i]
	}
	return SymbolGridTmp
}

// 奖励反转
func (s *betOrderService) reverseWinInPlace(winGrid int64Grid) int64GridW {
	var specialWin int64GridW
	winGridTmp := winGrid
	for i := 0; i < len(winGridTmp)/2; i++ {
		j := len(winGridTmp) - 1 - i
		winGridTmp[i], winGridTmp[j] = winGridTmp[j], winGridTmp[i]
	}

	for r := int64(0); r < _rowCountWin; r++ {
		specialWin[r] = winGridTmp[r]
	}

	return specialWin
}
