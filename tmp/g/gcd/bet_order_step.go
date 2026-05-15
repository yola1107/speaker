package gcd

import (
	"fmt"

	"egame-grpc/game/common/rand"

	"github.com/shopspring/decimal"
)

func (s *betOrderService) betSpins() error {
	if err := s.initialize(); err != nil {
		return err
	}
	if err := s.initSymbolGrid(); err != nil {
		return internalServerError
	}

	isFree := s.isFreeMode()
	s.findWinInfo()
	s.processWinInfos()

	if isFree {
		s.stepIndex = s.scene.FreeStep
	} else {
		s.stepIndex = s.scene.NormalStep
	}

	if len(s.winResults) > 0 {
		var multiArr []int64
		var round int64
		if isFree {
			s.scene.FreeStep += 1
			round = s.scene.FreeStep
			multiArr = s.gameConfig.GetFreeCfgByType(s.freeType).Multi
		} else {
			s.scene.NormalStep += 1
			round = s.scene.NormalStep
			multiArr = s.gameConfig.BaseMulti
		}
		if round > int64(len(multiArr)) {
			round = int64(len(multiArr))
		}
		s.roundMulti = multiArr[round-1]
		if s.roundMulti > 1 {
			s.stepMultiplier *= s.roundMulti
		}

		s.clearWinSymbols()
		debugPrintSymbolGrid("消除中奖符号后", s.symbolGrid)
		s.moveSymbols()
		debugPrintSymbolGrid("消除掉落后", s.symbolGrid)
		s.fallingSymbols()

		if debugLogging {
			for r := int64(0); r < _rowCount; r++ {
				for c := int64(0); c < _colCount; c++ {
					s.symbolGrid[r][c] = s.scene.Roller[c].Symbol[r]
				}
			}
			debugPrintSymbolGrid("填充后", s.symbolGrid)
		}
		if s.isFreeMode() {
			s.scene.NextStage = _freeModeEli
		} else {
			s.scene.NextStage = _normalModeEli
		}
		s.isRoundOver = false
	} else {
		s.treasureNum = countSymbol(s.symbolGrid, _treasure)
		if s.treasureNum >= s.gameConfig.Free.ScatterMin {
			if s.isFreeMode() {
				freeInFree += 1
				s.newFreeTimes = s.gameConfig.GetFreeCfgByType(s.freeType).Times
				s.scene.FreeNum += s.newFreeTimes
				debugPrintTips(fmt.Sprintf("触发免费中免费,免费次数 %d", s.newFreeTimes))
			} else {
				s.scene.FreeType = 0
				s.scene.FreeNum = 0
				s.scene.FreeTimes = 0
				s.scene.BonusState = _bonusStatePending
				s.scene.ScatterNum = s.treasureNum
			}
			s.scene.NextStage = _freeMode
		}
		if isFree && s.scene.FreeNum > 0 {
			s.scene.NextStage = _freeMode
		}
		s.isRoundOver = true
	}
	return nil
}

func (s *betOrderService) initialize() error {
	var err error
	switch {
	case s.scene.Stage == _normalMode:
		err = s.initStepForFirstStep()
	default:
		err = s.initStepForNextStep()
	}
	if err != nil {
		return err
	}
	return nil
}

func (s *betOrderService) initStepForFirstStep() error {
	switch {
	case !s.updateBetAmount():
		return invalidRequestParams
	case !s.checkBalance():
		return insufficientBalance
	}
	s.scene.Reset()
	s.amount = s.betAmount
	return nil
}

func (s *betOrderService) initStepForNextStep() error {
	s.req.BaseMoney = s.lastOrder.BaseAmount
	s.req.Multiple = s.lastOrder.Multiple
	s.betAmount = decimal.NewFromFloat(s.lastOrder.BaseAmount * float64(s.lastOrder.BaseMultiple) * float64(s.lastOrder.Multiple))
	s.amount = decimal.Zero
	s.scene.RoundWin = 0
	return nil
}

func (s *betOrderService) initSymbolGrid() error {
	if s.scene.Stage == _normalMode {
		s.initRoller(getValByWeight(s.gameConfig.RollCfg.Base.UseKey, s.gameConfig.RollCfg.Base.Weight))
	} else if s.scene.Stage == _freeMode {
		freeCfn := s.gameConfig.GetRollCfgByType(s.freeType)
		s.initRoller(getValByWeight(freeCfn.UseKey, freeCfn.Weight))
	}
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			s.symbolGrid[r][c] = s.scene.Roller[c].Symbol[r]
		}
	}
	s.clientSymbolGrid = s.symbolGrid
	debugPrintSymbolGrid("牌面", s.symbolGrid)
	return nil
}

func (s *betOrderService) findWinInfo() {
	var infos []WinInfo
	for lineNo, lines := range s.gameConfig.Lines {
		var symbol, count int64
		var loc = make([]int64, 0, 3)
		locMap := Int64Grid{}
		for _, p := range lines {
			r := p / int(_colCount)
			c := p % int(_colCount)
			if symbol == _blank {
				symbol = s.symbolGrid[r][c]
				count++
				loc = append(loc, int64(p))
			} else if symbol == _wild || symbol == s.symbolGrid[r][c] || s.symbolGrid[r][c] == _wild {
				if symbol == _wild {
					symbol = s.symbolGrid[r][c]
				}
				if symbol == _treasure {
					break
				}
				count++
				loc = append(loc, int64(p))
			} else {
				break
			}
		}
		if count >= 3 {
			for _, l := range loc {
				locMap[l/_colCount][l%_colCount] = 1
			}
			index := int(count) - 1
			tmp := WinInfo{
				Symbol:      symbol,
				SymbolCount: count,
				LineCount:   1,
				Odds:        s.gameConfig.PayTable[symbol][index],
				LineNo:      lineNo + 1,
				Positions:   locMap,
			}
			infos = append(infos, tmp)
			if debugLogging {
				fmt.Println(fmt.Sprintf("中奖图标：%d，中奖长度：%d，图标倍率：%d，线序：%d，线路：%v", symbol, count, tmp.Odds, lineNo+1, lines))
			}
		}
	}
	s.winInfos = infos
}

func (s *betOrderService) processWinInfos() {
	var winResults []*WinResult
	var winGrid Int64Grid
	lineMul := int64(0)
	for _, info := range s.winInfos {
		baseLineMul := info.Odds
		totalMul := baseLineMul * info.LineCount
		winResults = append(winResults, &WinResult{
			Symbol:             info.Symbol,
			SymbolCount:        info.SymbolCount,
			LineCount:          info.LineCount,
			BaseLineMultiplier: baseLineMul,
			TotalMultiplier:    totalMul,
			Position:           info.Positions,
			LineNo:             int64(info.LineNo),
		})
		for r, cols := range info.Positions {
			for c, v := range cols {
				if v == 1 {
					winGrid[r][c] = s.symbolGrid[r][c]
				}
			}
		}
		lineMul += totalMul
	}
	s.lineMultiplier = lineMul
	s.stepMultiplier = s.lineMultiplier
	s.winResults = winResults
	s.winGrid = winGrid
}

func (s *betOrderService) initRoller(realIndex int64) {
	s.scene.Real = realIndex
	realData := s.gameConfig.RealData[realIndex]
	startIndexGroup := make([]int, _colCount)
	for c := 0; c < int(_colCount); c++ {
		realLineLen := len(realData[c])
		startIndex := rand.IntN(realLineLen)
		startIndexGroup[c] = startIndex
		for r := 0; r < int(_rowCount); r++ {
			index := (startIndex + r) % realLineLen
			symbol := realData[c][index]
			s.scene.Roller[c].Symbol[r] = symbol
			s.scene.Roller[c].Index = index
		}
	}
	if debugLogging {
		debugPrintTips(fmt.Sprintf("滚轴下标：%v", startIndexGroup))
	}
}

func (s *betOrderService) clearWinSymbols() {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.winGrid[r][c] != 0 {
				s.symbolGrid[r][c] = 0
			}
		}
	}
}

func (s *betOrderService) moveSymbols() {
	nextSymbolGrid := s.symbolGrid
	for col := 0; col < int(_colCount); col++ {
		for row := int64(0); row < _rowCount; row++ {
			if nextSymbolGrid[row][col] == 0 {
				for below := row + 1; below < _rowCount; below++ {
					if nextSymbolGrid[below][col] != 0 {
						nextSymbolGrid[row][col] = nextSymbolGrid[below][col]
						nextSymbolGrid[below][col] = 0
						break
					}
				}
			}
		}
	}
	s.symbolGrid = nextSymbolGrid
}

func (s *betOrderService) fallingSymbols() {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			s.scene.Roller[c].Symbol[r] = s.symbolGrid[r][c]
		}
	}
	for col, info := range s.scene.Roller {
		var newBoard [_rowCount]int64
		var zeroIndex []int
		for i, symbol := range info.Symbol {
			if symbol != 0 {
				newBoard[i] = symbol
			} else {
				zeroIndex = append(zeroIndex, i)
			}
		}
		for _, index := range zeroIndex {
			newBoard[index] = s.getFallSymbol(col)
		}
		s.scene.Roller[col].Symbol = newBoard
	}
}

func (s *betOrderService) getFallSymbol(col int) int64 {
	s.scene.Roller[col].Index++
	if s.scene.Roller[col].Index >= len(s.gameConfig.RealData[s.scene.Real][col]) {
		s.scene.Roller[col].Index = 0
	}
	return s.gameConfig.RealData[s.scene.Real][col][s.scene.Roller[col].Index]
}
