package xslm2

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"egame-grpc/gamelogic"
	"egame-grpc/global"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// ========== 请求和验证 ==========

// getRequestContext 获取请求上下文（商户、用户、游戏信息）
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

// selectGameRedis 选择游戏Redis
func (s *betOrderService) selectGameRedis() {
	index := _gameID % int64(len(global.GVA_GAME_REDIS))
	s.gameRedis = global.GVA_GAME_REDIS[index]
}

// updateBetAmount 计算下注金额
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

// checkBalance 检查余额
func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

// updateBonusAmount 计算奖金
func (s *betOrderService) updateBonusAmount() {
	if s.spin.stepMultiplier <= 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	bonusAmount := s.betAmount.
		Div(decimal.NewFromInt(_baseMultiplier)).
		Mul(decimal.NewFromInt(s.spin.stepMultiplier))
	s.bonusAmount = bonusAmount
}

// ========== spin辅助函数 ==========

// loadStepData 从预设数据加载当前step的符号网格
// 预设数据中每个step都包含完整的符号布局（无需下落填充）
// 免费模式下还需加载女性符号收集计数
func (s *spin) loadStepData(isFreeRound bool) bool {
	// 1. 加载符号网格（从预设的一维数组转为4×5网格）
	var symbolGrid int64Grid
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			symbolGrid[row][col] = s.stepMap.Map[row*_colCount+col]
		}
	}
	s.symbolGrid = &symbolGrid

	// 2. 基础模式无需加载女性计数
	if !isFreeRound {
		return true
	}

	// 3. 免费模式加载女性符号收集计数
	if int64(len(s.stepMap.FemaleCountsForFree)) != _femaleC-_femaleA+1 {
		global.GVA_LOG.Error(
			"loadStepData",
			zap.Error(fmt.Errorf("unexpected femaleCountsForFree len: %v", len(s.stepMap.FemaleCountsForFree))),
			zap.Int64("presetID", s.preset.ID),
			zap.Int64("stepID", s.stepMap.ID),
			zap.Int64s("femaleCountsForFree", s.stepMap.FemaleCountsForFree),
		)
		return false
	}
	for i, c := range s.stepMap.FemaleCountsForFree {
		s.femaleCountsForFree[i] = c
		s.nextFemaleCountsForFree[i] = c
	}
	return true
}

// updateStepData 检查是否触发全屏消除
// 当任意女性符号收集满10个时，启用全屏消除标志
func (s *spin) updateStepData() {
	if len(s.stepMap.FemaleCountsForFree) == 0 {
		return
	}

	// 检查是否有任意女性符号达到10个
	for _, c := range s.stepMap.FemaleCountsForFree {
		if c < _femaleSymbolCountForFullElimination {
			return
		}
	}

	// 所有女性符号都>=10，启用全屏消除
	s.enableFullElimination = true
}

func (s *spin) hasWildSymbol() bool {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.symbolGrid[r][c] == _wild {
				return true
			}
		}
	}
	return false
}

func (s *spin) getTreasureCount() int64 {
	count := int64(0)
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if s.symbolGrid[r][c] == _treasure {
				count++
			}
		}
	}
	return count
}

func (s *spin) findWinInfos() bool {
	var winInfos []*winInfo
	for symbol := _blank + 1; symbol < _wildFemaleA; symbol++ {
		if info, ok := s.findNormalSymbolWinInfo(symbol); ok {
			if symbol >= _femaleA {
				s.hasFemaleWin = true
			}
			winInfos = append(winInfos, info)
		}
	}
	for symbol := _wildFemaleA; symbol < _wild; symbol++ {
		if info, ok := s.findWildSymbolWinInfo(symbol); ok {
			s.hasFemaleWin = true
			winInfos = append(winInfos, info)
		}
	}
	s.winInfos = winInfos
	return len(winInfos) > 0
}

func (s *spin) findNormalSymbolWinInfo(symbol int64) (*winInfo, bool) {
	exist := false
	lineCount := int64(1)
	var winGrid int64Grid
	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || (currSymbol >= _wildFemaleA && currSymbol <= _wild) {
				if currSymbol == symbol {
					exist = true
				}
				count++
				winGrid[r][c] = currSymbol
			}
		}
		if count == 0 {
			if c >= _minMatchCount && exist {
				info := winInfo{Symbol: symbol, SymbolCount: c, LineCount: lineCount, WinGrid: winGrid}
				return &info, true
			}
			break
		}
		lineCount *= count
		if c == _colCount-1 && exist {
			info := winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, WinGrid: winGrid}
			return &info, true
		}
	}
	return nil, false
}

func (s *spin) findWildSymbolWinInfo(symbol int64) (*winInfo, bool) {
	lineCount := int64(1)
	var winGrid int64Grid
	for c := int64(0); c < _colCount; c++ {
		count := int64(0)
		for r := int64(0); r < _rowCount; r++ {
			currSymbol := s.symbolGrid[r][c]
			if currSymbol == symbol || currSymbol == _wild {
				count++
				winGrid[r][c] = currSymbol
			}
		}
		if count == 0 {
			if c >= _minMatchCount {
				info := winInfo{Symbol: symbol, SymbolCount: c, LineCount: lineCount, WinGrid: winGrid}
				return &info, true
			}
			break
		}
		lineCount *= count
		if c == _colCount-1 {
			info := winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, WinGrid: winGrid}
			return &info, true
		}
	}
	return nil, false
}

// updateStepResults 更新步骤结果（计算中奖倍率）
// 参数：
//
//	partialElimination - 部分消除模式
//	  true:  仅计算女性符号（7-9）的中奖
//	  false: 计算所有符号的中奖
//
// 用途：
//   - 基础模式：有女性中奖+有Wild时，只计算女性符号（等待下一step消除其他符号）
//   - 免费模式：类似逻辑
func (s *spin) updateStepResults(partialElimination bool) {
	var winResults []*winResult
	var winGrid int64Grid
	lineMultiplier := int64(0)

	for _, info := range s.winInfos {
		// 部分消除模式下，跳过非女性符号
		if partialElimination && info.Symbol < _femaleA {
			continue
		}

		// 计算中奖倍率
		baseLineMultiplier := _symbolMultiplierGroups[info.Symbol-1][info.SymbolCount-_minMatchCount]
		totalMultiplier := baseLineMultiplier * info.LineCount

		result := winResult{
			Symbol:             info.Symbol,
			SymbolCount:        info.SymbolCount,
			LineCount:          info.LineCount,
			BaseLineMultiplier: baseLineMultiplier,
			TotalMultiplier:    totalMultiplier,
			WinGrid:            info.WinGrid,
		}
		winResults = append(winResults, &result)

		// 合并中奖网格
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if info.WinGrid[r][c] != _blank {
					winGrid[r][c] = info.WinGrid[r][c]
				}
			}
		}
		lineMultiplier += totalMultiplier
	}

	s.stepMultiplier = lineMultiplier
	s.winResults = winResults
	s.winGrid = &winGrid
}

// ========== 辅助函数 ==========

func (s *betOrderService) showPostUpdateErrorLog() {
	global.GVA_LOG.Error(
		"updateStepResult",
		zap.Error(errors.New("inconsistent state")),
		zap.Int64("merchantID", s.merchant.ID),
		zap.Int64("memberID", s.member.ID),
		zap.String("orderSN", s.orderSN),
		zap.Bool("isRoundOver", s.client.IsRoundOver),
		zap.Uint64("lastMapID", s.client.ClientOfFreeGame.GetLastMapId()),
		zap.Uint64("freeNum", s.client.ClientOfFreeGame.GetFreeNum()),
	)
}

func (s *betOrderService) symbolGridToString() string {
	builder := ""
	for r := int64(0); r < _rowCount; r++ {
		builder += "["
		for c := int64(0); c < _colCount; c++ {
			builder += strconv.FormatInt(s.spin.symbolGrid[r][c], 10)
			if c < _colCount-1 {
				builder += ","
			}
		}
		builder += "]"
		if r < _rowCount-1 {
			builder += "\n"
		}
	}
	return builder
}

func (s *betOrderService) winGridToString() string {
	builder := ""
	for r := int64(0); r < _rowCount; r++ {
		builder += "["
		for c := int64(0); c < _colCount; c++ {
			builder += strconv.FormatInt(s.spin.winGrid[r][c], 10)
			if c < _colCount-1 {
				builder += ","
			}
		}
		builder += "]"
		if r < _rowCount-1 {
			builder += "\n"
		}
	}
	return builder
}

func (s *betOrderService) sleepForDebug() {
	if s.debug.open {
		time.Sleep(time.Millisecond * time.Duration(s.debug.delayMillis))
	}
}
