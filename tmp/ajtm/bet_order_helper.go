package ajtm

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

func (s *betOrderService) updateBonusAmount(stepMultiplier int64) {
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

// moveSymbols 清除中奖格并下落（row 0 为顶、row 越大越靠下，下落压向底行）。
func (s *betOrderService) moveSymbols() int64Grid {
	nextSymbolGrid := s.symbolGrid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.winGrid[r][c] > 0 {
				nextSymbolGrid[r][c] = 0
			}
		}
	}
	s.dropSymbols(&nextSymbolGrid)
	return nextSymbolGrid
}

// dropSymbols 符号下落：0 视为空位，把非 0 符号压到底部（row 越大越靠下）。
// 第0列和第4列的顶角(行0)和底角(行5)不参与下落，保持为空
func (s *betOrderService) dropSymbols(grid *int64Grid) {
	for c := 0; c < _colCount; c++ {
		isEdgeCol := c == 0 || c == _colCount-1
		// 边缘列的底角位置（行_rowCount-1）不参与下落
		writePos := _rowCount - 1
		if isEdgeCol {
			writePos = _rowCount - 2 // 从行4开始（跳过底角）
		}
		for r := _rowCount - 1; r >= 0; r-- {
			// 跳过边缘列的顶角(行0)和底角(行_rowCount-1)
			if isEdgeCol && (r == 0 || r == _rowCount-1) {
				continue
			}
			if val := (*grid)[r][c]; val != 0 {
				if r != writePos {
					(*grid)[writePos][c] = val
					(*grid)[r][c] = 0
				}
				writePos--
			}
		}
	}
}

// fallingWinSymbols 将下落后的网格写回滚轴板面并 ring 填充
func (s *betOrderService) fallingWinSymbols(nextSymbolGrid int64Grid) {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.scene.SymbolRoller[c].BoardSymbol[r] = nextSymbolGrid[r][c]
		}
	}
	for i := range s.scene.SymbolRoller {
		s.scene.SymbolRoller[i].ringSymbol(s.gameConfig)
	}
}

// findWinInfos 查找中奖信息（Ways玩法：从左到右连续匹配）
func (s *betOrderService) findWinInfos() {
	winInfos := make([]WinInfo, 0, _wild-_blank-1)
	var totalWinGrid int64Grid
	var mysMulTotal int64

	for symbol := _blank + 1; symbol < _wild; symbol++ {
		info, ok := s.findSymbolWinInfo(symbol)
		if !ok {
			continue
		}
		winInfos = append(winInfos, *info)
		for r := 0; r < _rowCount; r++ {
			for c := 0; c < _colCount; c++ {
				if info.WinGrid[r][c] > 0 {
					totalWinGrid[r][c] = info.WinGrid[r][c]
					if info.WinGrid[r][c] > _longSymbol {
						//mysMulTotal += s.mysMultipliers[r-1][c]
					}
				}
			}
		}
	}

	//// 神秘符号倍数累加：遍历 symbolGrid 检测尾巴（尾巴不加入 winGrid，但倍数仍需累加）
	//for col := 1; col < _colCount-1; col++ {
	//	for row := 1; row < _rowCount; row++ {
	//		if s.symbolGrid[row][col] > _longSymbol && totalWinGrid[row-1][col] > 0 {
	//			// 头部中奖，累加尾巴的倍数
	//			mysMulTotal += s.mysMultipliers[row-1][col]
	//		}
	//	}
	//}

	if mysMulTotal > 0 {
		s.scene.MysMultiplierTotal += mysMulTotal
	}

	if s.scene.MysMultiplierTotal > 0 {
		for i := range winInfos {
			winInfos[i].MysMultiplier = s.scene.MysMultiplierTotal
		}
	}

	s.winInfos = winInfos
	s.winGrid = totalWinGrid
}

// findSymbolWinInfo 查找符号中奖（Ways玩法：从左到右连续，至少3列，Wild可替代）
// 神秘符号头部为正数，尾巴为 _longSymbol + symbol，只检测头部，尾巴不标记（不消除）
func (s *betOrderService) findSymbolWinInfo(symbol int64) (*WinInfo, bool) {
	lineCount := int64(1)
	var winGrid int64Grid
	hasRealSymbol := false

	for c := 0; c < _colCount; c++ {
		matchCount := 0
		for r := 0; r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]

			if currSymbol == symbol || currSymbol == _wild {
				if currSymbol == symbol {
					hasRealSymbol = true
				}
				matchCount++
				winGrid[r][c] = currSymbol
				for row := r + 1; row < _rowCount && s.symbolGrid[row][c] > _longSymbol; row++ {
					winGrid[row][c] = s.symbolGrid[row][c]
				}
			}
		}

		if matchCount == 0 {
			if c >= _minMatchCount && hasRealSymbol {
				if odds := s.getSymbolBaseMultiplier(symbol, c); odds > 0 {
					return &WinInfo{Symbol: symbol, SymbolCount: int64(c), LineCount: lineCount, Odds: odds, Multiplier: odds * lineCount, WinGrid: winGrid}, true
				}
			}
			return nil, false
		}

		lineCount *= int64(matchCount)

		if c == _colCount-1 && hasRealSymbol {
			odds := s.getSymbolBaseMultiplier(symbol, _colCount)
			if odds > 0 {
				return &WinInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, Odds: odds, Multiplier: odds * lineCount, WinGrid: winGrid}, true
			}
		}
	}

	return nil, false
}
