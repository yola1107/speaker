package mahjong

import (
	"fmt"
	"strconv"

	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// 获取请求上下文
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

// 初始化游戏redis
func (s *betOrderService) selectGameRedis() {
	index := _gameID % int64(len(global.GVA_GAME_REDIS))
	s.gameRedis = global.GVA_GAME_REDIS[index]
}

// 更新下注金额
func (s *betOrderService) updateBetAmount() bool {
	betAmount := decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_baseMultiplier))
	s.betAmount = betAmount
	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
		return false
	}
	return true
}

// 检查用户余额
func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

// 符号网格转换为字符串
func (s *betOrderService) symbolGridToString(symbolGrid int64Grid) string {
	typeCard := ""
	num := 0
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			typeCard += strconv.Itoa(num + 1)
			typeCard += ":"
			typeCard += strconv.Itoa(int(symbolGrid[i][j]))
			typeCard += "; "
			num++
		}
	}
	return typeCard
}

// 中奖网格转换为字符串
func (s *betOrderService) winGridToString(result *BaseSpinResult) string {
	winCard := ""
	numW := 0
	for i := 0; i < 4; i++ {
		for j := 0; j < 5; j++ {
			winCard += strconv.Itoa(numW + 1)
			winCard += ":"
			winCard += strconv.Itoa(int(result.winGrid[i][j]))
			winCard += "; "
			numW++
		}
	}
	return winCard
}

// 更新奖金金额
func (s *betOrderService) updateBonusAmount(stepMultiplier int64) decimal.Decimal {
	bonusAmount := decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(stepMultiplier))
	s.bonusAmount = bonusAmount
	return bonusAmount
}

// 获得中奖路数及详情
// 1中奖路数 2投注倍数 3连续中奖倍数（头部倍数）4连线倍数 5免费次数 6 免费局连中次数
func (s *betOrderService) getWinDetail(routeDetails []CardType, nwin int64, freeCount, freeNum, scatter int64) string {
	var returnRouteDetail []CardType
	if nwin > 0 {
		for _, v := range routeDetails {
			returnRouteDetail = append(returnRouteDetail, CardType{
				Type:     v.Type,
				Route:    v.Route,
				Multiple: v.Multiple,
				Way:      v.Way,
			})
		}
	}

	if freeNum > 0 && freeCount >= scatter && len(returnRouteDetail) == 0 {
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
	var stepMultiplier int64
	for _, info := range infos {
		wRes := s.symbolWinMultiplier(*info)
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
	rounded := bonusAmount.Round(2).InexactFloat64()
	s.client.ClientOfFreeGame.IncrGeneralWinTotal(rounded)
	s.client.ClientOfFreeGame.IncRoundBonus(rounded)
	if s.isFreeRound {
		s.client.ClientOfFreeGame.IncrFreeTotalMoney(rounded)
	}
}

func (s *betOrderService) moveSymbols() int64Grid {
	nextSymbolGrid := s.symbolGrid
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

	for col := 0; col < _colCount; col++ {
		for row := _rowCount - 1; row >= 0; row-- {
			if nextSymbolGrid[row][col] == 0 {
				for above := row - 1; above >= 0; above-- {
					if nextSymbolGrid[above][col] != 0 {
						nextSymbolGrid[row][col] = nextSymbolGrid[above][col]
						nextSymbolGrid[above][col] = 0
						break
					}
				}
			}
		}
	}
	return nextSymbolGrid
}
func (s *betOrderService) fallingWinSymbols(nextSymbolGrid int64Grid, stage int8) {
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
