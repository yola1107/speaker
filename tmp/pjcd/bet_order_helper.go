package pjcd

import (
	"fmt"
	"strconv"
	"strings"

	"egame-grpc/game/common"
	"egame-grpc/game/common/pb"
	"egame-grpc/gamelogic"
	"egame-grpc/global"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// getRequestContext 获取请求上下文
func (s *betOrderService) getRequestContext() bool {
	mer, mem, ga, ok := common.GetRequestContext(s.req)
	if !ok {
		global.GVA_LOG.Error("getRequestContext error.")
		return false
	}
	s.merchant, s.member, s.game = mer, mem, ga
	return true
}

// updateBetAmount 更新投注金额
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

// checkBalance 检查余额
func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

// symbolGridToString 符号网格转字符串
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

// winGridToString 中奖网格转字符串
func (s *betOrderService) winGridToString(winGrid int64Grid) string {
	var b strings.Builder
	b.Grow(512)
	cellIndex := 0
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			b.WriteString(strconv.Itoa(cellIndex + 1))
			b.WriteString(":")
			b.WriteString(strconv.FormatInt(winGrid[r][c], 10))
			b.WriteString("; ")
			cellIndex++
		}
	}
	return b.String()
}

// updateBonusAmount 更新奖金金额
// 总倍数 = stepMultiplier + butterflyBonus
func (s *betOrderService) updateBonusAmount(stepMultiplier int64) {
	// RTP测试模式或无倍数时直接返回
	if s.debug.open || stepMultiplier == 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	// 总倍数 = step倍数 + 蝴蝶百搭加成
	totalMultiplier := stepMultiplier + s.butterflyBonus
	bonusAmount := s.betAmount.
		Mul(decimal.NewFromInt(totalMultiplier)).
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

// getScatterCount 获取夺宝符号数量
func (s *betOrderService) getScatterCount() int64 {
	var count int64
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.symbolGrid[r][c] == _scatter {
				count++
			}
		}
	}
	return count
}

// handleSymbolGrid 处理符号网格（从轮轴构建盘面）
func (s *betOrderService) handleSymbolGrid() {
	var symbolGrid int64Grid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			// BoardSymbol 是从下到上存储，需要翻转
			symbolGrid[_rowCount-1-r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
	s.symbolGrid = symbolGrid
}

// checkSymbolGridWin 检查符号网格中奖情况
func (s *betOrderService) checkSymbolGridWin() {
	var winInfos []WinInfo
	var totalWinGrid int64Grid

	// 遍历所有中奖线
	for lineIndex, line := range s.gameConfig.Lines {
		// 检查每个符号（从1到7）
		for symbol := _clover; symbol <= _rose; symbol++ {
			var count int64
			var winGrid int64Grid

			// 沿着中奖线检查连续符号
			// 注意：支付线位置编号从1开始，需要减1转换为0开始的索引
			for _, pos := range line {
				row := (pos - 1) / _colCount
				col := (pos - 1) % _colCount
				if row >= _rowCount {
					break
				}

				currSymbol := s.symbolGrid[row][col]
				// 百搭可以替代任何普通符号
				if currSymbol == symbol || currSymbol == _wild {
					winGrid[row][col] = currSymbol
					count++
				} else {
					break // 连续中断
				}
			}

			// 至少3连才算中奖
			if count >= _minMatchCount {
				odds := s.gameConfig.GetSymbolOdds(symbol, int(count))
				if odds > 0 {
					roundMultiplier := s.getRoundMultiplier()
					winInfos = append(winInfos, WinInfo{
						Symbol:      symbol,
						SymbolCount: count,
						LineIndex:   int64(lineIndex),
						Odds:        odds,
						Multiplier:  odds * roundMultiplier,
						WinGrid:     winGrid,
					})
					// 更新总中奖网格
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
}

// moveSymbols 移动符号（消除中奖符号后下落）
// 规则：
// 1. 夺宝符号(SCATTER)不能被消除
// 2. 蝴蝶百搭参与中奖后被消除
// 3. 非蝴蝶百搭参与中奖后保留在盘面
func (s *betOrderService) moveSymbols() int64Grid {
	nextSymbolGrid := s.symbolGrid
	// 将中奖位置置空（应用特殊规则）
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			// 未中奖位置保留
			if s.winGrid[r][c] == 0 {
				continue
			}
			symbol := s.symbolGrid[r][c]
			// 夺宝符号不能被消除
			if symbol == _scatter {
				continue
			}
			// 百搭符号：只有蝴蝶才消除
			if symbol == _wild {
				if s.wildStates[r][c] == _wildStateButterfly {
					nextSymbolGrid[r][c] = 0 // 蝴蝶百搭消除
				}
				// 非蝴蝶百搭保留
				continue
			}
			// 普通符号消除
			nextSymbolGrid[r][c] = 0
		}
	}
	// 符号下落填充空位
	s.dropSymbols(&nextSymbolGrid)
	return nextSymbolGrid
}

// dropSymbols 符号下落（重力效果）
func (s *betOrderService) dropSymbols(grid *int64Grid) {
	for c := 0; c < _colCount; c++ {
		// 从下往上扫描，将非空符号下沉
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

// fallingWinSymbols 下落填充新符号到轮轴
func (s *betOrderService) fallingWinSymbols(nextSymbolGrid int64Grid) {
	// 更新轮轴的盘面符号
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.scene.SymbolRoller[c].BoardSymbol[_rowCount-1-r] = nextSymbolGrid[r][c]
		}
	}
	// 为每列轮轴填充空位（传递列索引用于WILD限制）
	for i := range s.scene.SymbolRoller {
		s.scene.SymbolRoller[i].ringSymbol(s, i)
	}
}

// calcNewFreeGameNum 计算新增免费游戏次数
func (s *betOrderService) calcNewFreeGameNum(scatterCount int64) int64 {
	if s.isFreeRound {
		// 免费模式内再触发
		return s.gameConfig.CalcRetriggerSpins(scatterCount)
	}
	// 基础模式触发
	return s.gameConfig.CalcInitialFreeSpins(scatterCount)
}

// getRoundMultiplier 获取当前轮次倍数
func (s *betOrderService) getRoundMultiplier() int64 {
	return s.gameConfig.GetRoundMultiplier(int(s.scene.MultipleIndex), s.isFreeRound)
}

// writeGridToBuilder 调试输出网格
func writeGridToBuilder(buf *strings.Builder, grid *int64Grid, winGrid *int64Grid) {
	if grid == nil {
		buf.WriteString("(空)\n")
		return
	}
	// 从顶行到底行输出（翻转行序）
	for r := _rowCount - 1; r >= 0; r-- {
		for c := 0; c < _colCount; c++ {
			symbol := (*grid)[r][c]
			isWin := winGrid != nil && (*winGrid)[r][c] != 0
			if isWin {
				_, _ = fmt.Fprintf(buf, "%2d*|", symbol)
			} else {
				_, _ = fmt.Fprintf(buf, "%2d |", symbol)
			}
		}
		buf.WriteString("\n")
	}
}

// PjcdBoard 转换为protobuf Board
func PjcdBoard(grid int64Grid) *pb.Board {
	elements := make([]int64, _rowCount*_colCount)
	for row := 0; row < _rowCount; row++ {
		for col := 0; col < _colCount; col++ {
			elements[row*_colCount+col] = grid[row][col]
		}
	}
	return &pb.Board{Elements: elements}
}

// PjcdWildStateBoard 将百搭状态网格转换为protobuf Board
func PjcdWildStateBoard(grid WildStateGrid) *pb.Board {
	elements := make([]int64, _rowCount*_colCount)
	for row := 0; row < _rowCount; row++ {
		for col := 0; col < _colCount; col++ {
			elements[row*_colCount+col] = int64(grid[row][col])
		}
	}
	return &pb.Board{Elements: elements}
}
