package hcsqy

import (
	"fmt"
	"strconv"
	"strings"

	"egame-grpc/game/common"
	//"egame-grpc/game/common/rand"
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

// moveSymbols 清除中奖格并下落（sgz 同款流程；坐标与 hcsqy2 一致：row 0 为顶、row 越大越靠下，下落压向底行）。
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
func (s *betOrderService) dropSymbols(grid *int64Grid) {
	for c := 0; c < _colCount; c++ {
		writePos := _rowCount - 1
		for r := _rowCount - 1; r >= 0; r-- {
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

// s.scene.SymbolRoller[c].BoardSymbol[r-1] = metrix[r][c]
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

	// 第一列有wild时，检查盘面所有符号；否则使用默认符号列表
	checkSymbols := _checkList
	hasWild, symbols := s.checkWildInFirstCol()
	if hasWild {
		checkSymbols = symbols
	}

	for _, symbol := range checkSymbols {
		info, ok := s.findSymbolWinInfo(symbol, hasWild)
		if !ok {
			continue
		}
		winInfos = append(winInfos, *info)
		// 合并中奖位置到总网格（用于消除）
		for r := 0; r < _rowCount; r++ {
			for c := int64(0); c < info.SymbolCount; c++ {
				if info.WinGrid[r][c] != 0 {
					totalWinGrid[r][c] = info.WinGrid[r][c]
				}
			}
		}
	}

	s.winInfos = winInfos
	s.winGrid = totalWinGrid
}

// checkWildInFirstCol 检查第一列是否有wild，若有则返回盘面所有不重复符号
func (s *betOrderService) checkWildInFirstCol() (bool, []int64) {
	// 检查第一列是否有wild
	for row := 0; row < _rowCount; row++ {
		if s.symbolGrid[row][0] == _wild {
			// 收集盘面不重复符号
			seen := make(map[int64]bool, 10)
			symbols := make([]int64, 0, 10)
			for r := 0; r < _rowCount; r++ {
				for c := 0; c < _colCount; c++ {
					sym := s.symbolGrid[r][c]
					if sym > 0 && sym <= _wild && !seen[sym] {
						seen[sym] = true
						symbols = append(symbols, sym)
					}
				}
			}
			return true, symbols
		}
	}
	return false, nil
}

// findSymbolWinInfo 查找符号中奖（Ways玩法：从左到右连续，至少3列，Wild可替代）
func (s *betOrderService) findSymbolWinInfo(symbol int64, hasWild bool) (*WinInfo, bool) {
	hasRealSymbol := false
	lineCount := int64(1)
	var winGrid int64Grid

	// 特殊处理 前三列都有wild的特殊情况
	if hasWild {
		hasRealSymbol = true
	}

	// 逐列扫描，统计匹配的符号
	for c := 0; c < _colCount; c++ {
		matchCount := 0
		for r := 0; r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || currSymbol == _wild {
				if currSymbol == symbol {
					hasRealSymbol = true
				}
				matchCount++
				winGrid[r][c] = currSymbol // 存储实际符号值
			}
		}

		// 当前列没有匹配
		if matchCount == 0 {
			if c >= _minMatchCount && hasRealSymbol {
				if odds := s.getSymbolBaseMultiplier(symbol, c); odds > 0 {
					return &WinInfo{Symbol: symbol, SymbolCount: int64(c), LineCount: lineCount, Odds: odds, Multiplier: odds * lineCount, WinGrid: winGrid}, true
				}
			}
			return nil, false
		}

		// 计算路数：每列匹配数相乘
		lineCount *= int64(matchCount)

		// 如果到了最后一列且有真实符号，返回中奖信息
		if c == _colCount-1 && hasRealSymbol {
			odds := s.getSymbolBaseMultiplier(symbol, _colCount)
			if odds > 0 {
				return &WinInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, Odds: odds, Multiplier: odds * lineCount, WinGrid: winGrid}, true
			}
		}
	}

	return nil, false
}

func btoi(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
