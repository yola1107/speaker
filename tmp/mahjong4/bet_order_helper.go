package mahjong

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

// 检查用户余额
func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

// 符号网格转换为字符串
func (s *betOrderService) symbolGridToString(symbolGrid int64Grid) string {
	typeCard := ""
	num := 0
	for i := 0; i < _rowCount; i++ {
		for j := 0; j < _colCount; j++ {
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
	for i := 0; i < _rowCountReward; i++ {
		for j := 0; j < _colCount; j++ {
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
	bonusAmount := s.betAmount.
		Mul(decimal.NewFromInt(stepMultiplier)).
		Div(decimal.NewFromInt(_baseMultiplier))
	s.bonusAmount = bonusAmount
	return bonusAmount
}

// 获得中奖路数及详情
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

// 获取夺宝符号数量
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

// checkSymbolGridWin 检查中奖
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

// 处理中奖列表，计算总倍数
func (s *betOrderService) handleWinInfosMultiplier(infos []*winInfo) int64 {
	var stepMultiplier int64
	for _, info := range infos {
		stepMultiplier += info.Multiplier
	}
	return stepMultiplier
}

// handleSymbolGrid 从 BoardSymbol 生成 symbolGrid（反转映射）
// BoardSymbol[0]（最下面）→ symbolGrid[3]（Row 3），BoardSymbol[3]（最上面）→ symbolGrid[0]（Row 0）
func (s *betOrderService) handleSymbolGrid() int64Grid {
	var symbolGrid int64Grid
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbolGrid[_rowCount-1-r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
	return symbolGrid
}

// toWinGridReward 将 int64Grid 转换为 int64GridW（只复制前 _rowCountReward 行）
func toWinGridReward(winGrid int64Grid) int64GridW {
	var winGridW int64GridW
	for r := int64(0); r < _rowCountReward; r++ {
		winGridW[r] = winGrid[r]
	}
	return winGridW
}

// winGridRewardToFull 将 int64GridW 转换为 int64Grid
func winGridRewardToFull(winGridW int64GridW) int64Grid {
	var fullWinGrid int64Grid
	for r := int64(0); r < _rowCountReward; r++ {
		fullWinGrid[r] = winGridW[r]
	}
	return fullWinGrid
}

func (s *betOrderService) updateSpinBonusAmount(bonusAmount decimal.Decimal) {
	if s.debug.open {
		return
	}
	rounded := bonusAmount.Round(2).InexactFloat64()
	s.client.ClientOfFreeGame.IncrGeneralWinTotal(rounded)
	s.client.ClientOfFreeGame.IncRoundBonus(rounded)
	if s.isFreeRound {
		s.client.ClientOfFreeGame.IncrFreeTotalMoney(rounded)
	}
}

// moveSymbols 消除中奖符号并下落（只消除前3行，Row 3 是缓冲区）
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

// dropSymbols 符号下落：非零符号前移，零值留在后面
func (s *betOrderService) dropSymbols(grid *int64Grid) {
	for c := int64(0); c < _colCount; c++ {
		writePos := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			if val := (*grid)[r][c]; val != 0 {
				if r != writePos {
					(*grid)[writePos][c] = val
					(*grid)[r][c] = 0 // _blank
				}
				writePos++
			}
		}
	}
}

// fallingWinSymbols 将 nextSymbolGrid 同步到 BoardSymbol 并填充新符号
func (s *betOrderService) fallingWinSymbols(nextSymbolGrid int64Grid, _ int8) {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			s.scene.SymbolRoller[c].BoardSymbol[r] = nextSymbolGrid[_rowCount-1-r][c]
		}
	}
	for i := range s.scene.SymbolRoller {
		s.scene.SymbolRoller[i].ringSymbol(s.gameConfig)
	}
}

func printGridToBuf(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
	if grid == nil {
		buf.WriteString("(空)\n")
		return
	}
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbol := grid[r][c]
			isWin := winGrid != nil && winGrid[r][c] != 0
			if symbol == 0 {
				if isWin {
					buf.WriteString("   *|")
				} else {
					buf.WriteString("    |")
				}
			} else {
				if isWin {
					fmt.Fprintf(buf, " %2d*|", symbol)
				} else {
					fmt.Fprintf(buf, " %2d |", symbol)
				}
			}
			if c < _colCount-1 {
				buf.WriteString(" ")
			}
		}
		buf.WriteString("\n")
	}
}

// setupBonusNumAndFreeTimes 设置 BonusNum 并计算免费次数
func (s *betOrderService) setupBonusNumAndFreeTimes(scatterCount int64, bonusNum int) int {
	if bonusNum <= 0 || bonusNum > 3 {
		bonusNum = _bonusNum3
	}

	// 根据 scatterCount 和 FreeBonusMap 配置计算免费次数
	bonusItem, ok := s.gameConfig.FreeBonusMap[bonusNum]
	if !ok {
		panic(fmt.Errorf("BonusNum %d not found in FreeBonusMap", bonusNum))
	}
	extraScatterCount := scatterCount - int64(s.gameConfig.FreeGameScatterMin)
	freeTimes := bonusItem.Times + int(extraScatterCount)*bonusItem.AddTimes

	// 更新场景数据
	s.scene.BonusNum = bonusNum
	s.scene.ScatterNum = scatterCount
	s.scene.FreeNum = int64(freeTimes)
	s.scene.BonusState = _bonusStateSelected                // 已选择免费游戏类型
	s.client.ClientOfFreeGame.SetFreeNum(uint64(freeTimes)) // 设置客户端免费次数
	s.scene.NextStage = _spinTypeFree                       // 设置下一阶段为免费模式，确保下次 betOrder 时切换

	return freeTimes
}
