package mahjong4

import (
	"fmt"
	"strconv"
	"strings"

	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

func (s *betOrderService) getRequestContext() bool {
	switch {
	case !s.mdbGetMerchant():
		return false
	case !s.mdbGetMember():
		return false
	case !s.mdbGetGame():
		return false
	default:
		return true
	}
}

func (s *betOrderService) selectGameRedis() {
	index := _gameID % int64(len(global.GVA_GAME_REDIS))
	s.gameRedis = global.GVA_GAME_REDIS[index]
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

func (s *betOrderService) winGridToString(winGridW int64GridW) string {
	var b strings.Builder
	b.Grow(512)
	cellIndex := 0
	for r := 0; r < _rowCountReward; r++ {
		for c := 0; c < _colCount; c++ {
			b.WriteString(strconv.Itoa(cellIndex + 1))
			b.WriteString(":")
			b.WriteString(strconv.FormatInt(winGridW[r][c], 10))
			b.WriteString("; ")
			cellIndex++
		}
	}
	return b.String()
}

func (s *betOrderService) updateBonusAmount(stepMultiplier int64) {
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

func (s *betOrderService) getWinDetail(routeDetails []CardType, nwin int64, freeCount, freeNum, scatter int64) string {
	var returnRouteDetail []CardType
	if nwin > 0 {
		returnRouteDetail = append(returnRouteDetail, routeDetails...)
	} else if freeNum > 0 && freeCount >= scatter {
		returnRouteDetail = append(returnRouteDetail, CardType{
			Type:     int(_treasure),
			Route:    int(freeCount),
			Multiple: 0,
			Way:      int(freeNum),
		})
	}
	if len(returnRouteDetail) == 0 {
		return ""
	}
	winDetailsBytes, _ := json.CJSON.Marshal(returnRouteDetail)
	return string(winDetailsBytes)
}

func (s *betOrderService) getScatterCount() int64 {
	var count int64
	for r := int64(0); r < _rowCountReward; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.symbolGrid[r][c] == _treasure {
				count++
			}
		}
	}
	return count
}

// checkSymbolGridWin 检查符号网格中奖情况（核心算法）
func (s *betOrderService) checkSymbolGridWin(symbolGrid int64Grid) ([]*winInfo, int64Grid) {
	var winInfos []*winInfo
	var totalWinGrid int64Grid

	for i, line := range s.gameConfig.WinLines {
		for symbol := _blank + 1; symbol < _wild; symbol++ {
			var count int64
			var winGrid int64Grid

			for _, p := range line {
				r := p / _colCount
				c := p % _colCount
				if r >= _rowCountReward {
					break
				}
				currSymbol := symbolGrid[r][c]
				if currSymbol == symbol || currSymbol == _wild {
					winGrid[r][c] = currSymbol
					count++
				} else {
					break
				}
			}

			if count >= _minMatchCount {
				odds := s.getSymbolBaseMultiplier(symbol, int(count))
				if odds > 0 {
					winInfos = append(winInfos, &winInfo{
						Symbol:      symbol,
						SymbolCount: count,
						LineCount:   int64(i),
						Odds:        odds,
						Multiplier:  odds,
						WinGrid:     winGrid,
					})
					// 合并中奖标记到总中奖网格（只处理前3行，Row 3不参与中奖检测）
					for r := int64(0); r < _rowCountReward; r++ {
						for c := int64(0); c < _colCount; c++ {
							if winGrid[r][c] > 0 {
								totalWinGrid[r][c] = 1
							}
						}
					}
				}
			}
		}
	}
	return winInfos, totalWinGrid
}

func (s *betOrderService) handleWinInfosMultiplier(infos []*winInfo) int64 {
	var stepMultiplier int64
	for _, info := range infos {
		stepMultiplier += info.Multiplier
	}
	return stepMultiplier
}

func (s *betOrderService) handleSymbolGrid() int64Grid {
	var symbolGrid int64Grid
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbolGrid[_rowCount-1-r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
	return symbolGrid
}

func convertToRewardGrid(winGrid int64Grid) int64GridW {
	var winGridW int64GridW
	for r := int64(0); r < _rowCountReward; r++ {
		winGridW[r] = winGrid[r]
	}
	return winGridW
}

func convertRewardGridToFull(winGridW int64GridW) int64Grid {
	var fullWinGrid int64Grid
	for r := int64(0); r < _rowCountReward; r++ {
		fullWinGrid[r] = winGridW[r]
	}
	return fullWinGrid
}

func (s *betOrderService) moveSymbols() int64Grid {
	nextSymbolGrid := s.symbolGrid
	for r := int64(0); r < _rowCountReward; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.winGrid[r][c] > 0 {
				nextSymbolGrid[r][c] = 0
			}
		}
	}
	s.dropSymbols(&nextSymbolGrid)
	return nextSymbolGrid
}

// dropSymbols 符号下落（消除算法核心）
func (s *betOrderService) dropSymbols(grid *int64Grid) {
	for c := int64(0); c < _colCount; c++ {
		writePos := int64(0)
		for r := int64(0); r < _rowCount; r++ {
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
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			s.scene.SymbolRoller[c].BoardSymbol[r] = nextSymbolGrid[_rowCount-1-r][c]
		}
	}
	for i := range s.scene.SymbolRoller {
		s.scene.SymbolRoller[i].ringSymbol(s.gameConfig)
	}
}

func GridToString(grid *int64Grid, winGrid *int64Grid) string {
	if grid == nil {
		return "(空)\n"
	}
	var buf strings.Builder
	buf.Grow(512)
	writeGridToBuilder(&buf, grid, winGrid)
	return buf.String()
}

func writeGridToBuilder(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
	if grid == nil {
		buf.WriteString("(空)\n")
		return
	}
	rGrid := reverseGridRows(grid)
	rWinGrid := reverseGridRows(winGrid)

	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbol := rGrid[r][c]
			isWin := rWinGrid[r][c] != 0
			if symbol == 0 {
				if isWin {
					buf.WriteString("   *|")
				} else {
					buf.WriteString("    |")
				}
			} else {
				if isWin {
					_, _ = fmt.Fprintf(buf, " %2d*|", symbol)
				} else {
					_, _ = fmt.Fprintf(buf, " %2d |", symbol)
				}
			}
			if c < _colCount-1 {
				buf.WriteString(" ")
			}
		}
		buf.WriteString("\n")
	}
}

func reverseGridRows(grid *int64Grid) int64Grid {
	if grid == nil {
		return int64Grid{}
	}
	var reversed int64Grid
	for i := int64(0); i < _rowCount; i++ {
		reversed[i] = grid[_rowCount-1-i]
	}
	return reversed
}

func (s *betOrderService) autoSetupBonusNumAndFreeTimes(scatterCount int64) {
	if s.debug.open {
		s.setupBonusNumAndFreeTimes(scatterCount, _bonusNum3)
	}
}

func (s *betOrderService) setupBonusNumAndFreeTimes(scatterCount int64, bonusNum int) int {
	if bonusNum <= 0 || bonusNum > 3 {
		bonusNum = _bonusNum3
	}
	bonusItem, ok := s.gameConfig.FreeBonusMap[bonusNum]
	if !ok {
		panic(fmt.Errorf("invalid BonusNum %d: not found in FreeBonusMap", bonusNum))
	}
	extraScatterCount := scatterCount - int64(s.gameConfig.FreeGameScatterMin)
	freeTimes := bonusItem.Times + int(extraScatterCount)*bonusItem.AddTimes
	s.scene.BonusNum = bonusNum
	s.scene.ScatterNum = scatterCount
	s.scene.FreeNum = int64(freeTimes)
	s.scene.BonusState = _bonusStateSelected
	s.scene.NextStage = _spinTypeFree
	s.client.ClientOfFreeGame.SetFreeNum(uint64(freeTimes))
	return freeTimes
}

func (s *betOrderService) checkNewFreeGameNum(scatterCount int64) (bool, int64) {
	if scatterCount < int64(s.gameConfig.FreeGameScatterMin) {
		return false, 0
	}
	if s.scene.ContinueNum != 0 {
		return false, 0
	}
	bonusItem, ok := s.gameConfig.FreeBonusMap[s.scene.BonusNum]
	if !ok {
		global.GVA_LOG.Error("checkNewFreeGameNum: invalid BonusNum",
			zap.Int("bonusNum", s.scene.BonusNum))
		return false, 0
	}
	extraScatterCount := scatterCount - int64(s.gameConfig.FreeGameScatterMin)
	newFreeRoundCount := int64(bonusItem.Times) + extraScatterCount*int64(bonusItem.AddTimes)
	return true, newFreeRoundCount
}

func collectWinLineIndex(winArr []WinElem) [_maxWinLines]int {
	var winDetails [_maxWinLines]int
	for _, winElem := range winArr {
		lineIndex := int(winElem.RoadNum)
		if lineIndex >= 0 && lineIndex < len(winDetails) {
			winDetails[lineIndex] = 1
		}
	}
	return winDetails
}
