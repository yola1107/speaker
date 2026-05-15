package tmtg

import (
	"math/rand/v2"
	//"egame-grpc/game/common/rand"
)

func int64GridToArray(grid int64Grid) []int64 {
	elements := make([]int64, _rowCount*_colCount)
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			elements[r*_colCount+c] = grid[r][c]
		}
	}
	return elements
}

func symbolCount(symbolGrid int64Grid, symbol int64) int {
	var count int
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if symbolGrid[r][c] == symbol {
				count++
			}
		}
	}
	return count
}

// Perm 返回 [0, n) 的随机全排列
func Perm(n int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	rand.Shuffle(n, func(i, j int) { p[i], p[j] = p[j], p[i] })
	return p
}

func (s *betOrderService) getScatterCount() int64 {
	return int64(symbolCount(s.symbolGrid, _treasure))
}

func (s *betOrderService) handleSymbolGrid() {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			s.symbolGrid[r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
}

// resolveMode 根据当前游戏状态选择对应的 ModeConfig
func (s *betOrderService) resolveMode() *ModeConfig {
	switch {
	case s.scene.PurchaseAmount > 0:
		return &s.gameConfig.FreeBuy
	case s.isFreeRound:
		return &s.gameConfig.FreeGame
	default:
		return &s.gameConfig.BaseGame
	}
}

// initSpinSymbol 生成盘面并动态注入 scatter/bomb/wild
func (s *betOrderService) initSpinSymbol() {
	isPurchase := s.scene.PurchaseAmount > 0
	cfg := s.gameConfig
	mode := s.resolveMode()

	var rollCfg RollCfgType
	switch {
	case isPurchase && !s.isFreeRound:
		rollCfg = cfg.RollCfg.FreeBuyBase
	case isPurchase && s.isFreeRound:
		rollCfg = cfg.RollCfg.FreeBuy
	case s.isFreeRound:
		rollCfg = cfg.RollCfg.FreeGame
	default:
		rollCfg = cfg.RollCfg.BaseGame
	}
	rollers := cfg.getSceneSymbol(rollCfg)

	// 购买首步：按 InitialScatter 权重注入 scatter
	if isPurchase && !s.isFreeRound && mode._initialScatWT > 0 {
		if n := pickWeightIndex(mode.InitialScatter, mode._initialScatWT); n > 0 {
			if n > _colCount {
				n = _colCount
			}
			for _, col := range Perm(_colCount)[:n] {
				rollers[col].BoardSymbol[rand.IntN(_rowCount)] = _treasure
			}
		}
		s.scene.SymbolRoller = rollers
		return
	}

	// 每列以 ProbPerCol 概率放置最多 1 个 bomb
	s.bombMulSum = 0
	for col := 0; col < _colCount; col++ {
		if mode.Multiplier.ProbPerCol <= 0 || rand.Float64() >= mode.Multiplier.ProbPerCol {
			continue
		}
		var candidates []int
		for r := 0; r < _rowCount; r++ {
			if rollers[col].BoardSymbol[r] != _treasure {
				candidates = append(candidates, r)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		rollers[col].BoardSymbol[candidates[rand.IntN(len(candidates))]] = _bomb
		s.bombMulSum += mode.Multiplier.Multiplier[pickWeightIndex(mode.Multiplier.Weight, mode.Multiplier.WTotal)]
	}

	// 按 InitialSpawn 权重决定 wild 数量，随机放置
	if wildCount := pickWeightIndex(mode.WildGen.InitialSpawn, mode.WildGen._initialSpawnWT); wildCount > 0 {
		if wildCount > cfg.WildMaxLimit {
			wildCount = cfg.WildMaxLimit
		}
		var candidates []int
		for col := 0; col < _colCount; col++ {
			for r := 0; r < _rowCount; r++ {
				sym := rollers[col].BoardSymbol[r]
				if sym != _wild && sym != _bomb && sym != _treasure {
					candidates = append(candidates, col*_rowCount+r)
				}
			}
		}
		if wildCount > len(candidates) {
			wildCount = len(candidates)
		}
		for _, idx := range Perm(len(candidates))[:wildCount] {
			rollers[candidates[idx]/_rowCount].BoardSymbol[candidates[idx]%_rowCount] = _wild
		}
	}
	s.scene.SymbolRoller = rollers
}

// eliBombSymbols 十字爆破 wild所在行列全消除，返回消除后下落的新盘面
func (s *betOrderService) eliBombSymbols() int64Grid {
	nextSymbolGrid := s.symbolGrid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.symbolGrid[r][c] != _wild {
				continue
			}
			for cc := 0; cc < _colCount; cc++ {
				nextSymbolGrid[r][cc] = 0
			}
			for rr := 0; rr < _rowCount; rr++ {
				nextSymbolGrid[rr][c] = 0
			}
		}
	}
	s.dropSymbols(&nextSymbolGrid)
	return nextSymbolGrid
}

// moveSymbols 清除中奖格并下落，wildKeep=true 时保留 wild 不消除
func (s *betOrderService) moveSymbols(wildKeep bool) int64Grid {
	nextSymbolGrid := s.symbolGrid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.winGrid[r][c] > 0 {
				if wildKeep && s.symbolGrid[r][c] == _wild {
					continue
				}
				nextSymbolGrid[r][c] = 0
			}
		}
	}
	s.dropSymbols(&nextSymbolGrid)
	return nextSymbolGrid
}

// dropSymbols 符号下落：0 视为空位，把非 0 符号压到底部
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

func (s *betOrderService) fallingWinSymbols() {
	s.fillBombs(&s.nextSymbolGrid)
	s.fillWilds(&s.nextSymbolGrid)

	for col := range s.scene.SymbolRoller {
		roller := &s.scene.SymbolRoller[col]
		data := s.gameConfig.RealData[roller.Real][col]
		for r := _rowCount - 1; r >= 0; r-- {
			sym := s.nextSymbolGrid[r][col]
			if sym <= 0 {
				roller.Start--
				if roller.Start < 0 {
					roller.Start = len(data) - 1
				}
				if sym < 0 {
					sym = -sym
				} else {
					sym = data[roller.Start]
				}
			}
			roller.BoardSymbol[r] = sym
		}
	}
}

// fillBombs 消除补位时，无 bomb 的列按概率注入 bomb，写入 -_bomb 标记
func (s *betOrderService) fillBombs(grid *int64Grid) {
	mode := s.resolveMode()
	if mode.Multiplier.ProbPerCol <= 0 {
		return
	}
	for col := 0; col < _colCount; col++ {
		hasBomb := false
		var blanks []int
		for r := 0; r < _rowCount; r++ {
			switch (*grid)[r][col] {
			case _bomb:
				hasBomb = true
			case 0:
				blanks = append(blanks, r)
			}
		}
		if hasBomb || len(blanks) == 0 || rand.Float64() >= mode.Multiplier.ProbPerCol {
			continue
		}
		(*grid)[blanks[rand.IntN(len(blanks))]][col] = -_bomb
		s.bombMulSum += mode.Multiplier.Multiplier[pickWeightIndex(mode.Multiplier.Weight, mode.Multiplier.WTotal)]
	}
}

// fillWilds 在空白位置中按权重选 wild 补位，写入 -_wild 标记
func (s *betOrderService) fillWilds(grid *int64Grid) {
	mode := s.resolveMode()
	n := pickWeightIndex(mode.WildGen.TumbleRefill, mode.WildGen._tumbleRefillWT)
	if n <= 0 {
		return
	}
	remaining := s.gameConfig.WildMaxLimit
	var blanks []int
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			switch (*grid)[r][c] {
			case _wild:
				remaining--
			case 0:
				blanks = append(blanks, r*_colCount+c)
			}
		}
	}
	if n = min(n, remaining, len(blanks)); n <= 0 {
		return
	}
	for _, idx := range Perm(len(blanks))[:n] {
		(*grid)[blanks[idx]/_colCount][blanks[idx]%_colCount] = -_wild
	}
}

// findWinInfos 查找中奖信息
func (s *betOrderService) findWinInfos() {
	var (
		winInfos []WinInfo
		winGrid  int64Grid
		counter  [14]int64
	)

	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			counter[s.symbolGrid[r][c]]++
		}
	}

	wildCount := counter[_wild]
	for symbol := int64(1); symbol < _wild; symbol++ {
		if counter[symbol] <= 0 {
			continue
		}
		matchCount := counter[symbol]
		if symbol != _treasure {
			matchCount += wildCount
		}
		if odds := s.gameConfig.getSymbolBaseMultiplier(symbol, int(matchCount)); odds > 0 {
			var symWinGrid int64Grid
			for r := 0; r < _rowCount; r++ {
				for c := 0; c < _colCount; c++ {
					if s.symbolGrid[r][c] == symbol || s.symbolGrid[r][c] == _wild {
						winGrid[r][c] = symbol
						symWinGrid[r][c] = symbol
					}
				}
			}
			winInfos = append(winInfos, WinInfo{
				Symbol:      symbol,
				SymbolCount: matchCount,
				Count:       counter[symbol],
				Odds:        odds,
				WinGrid:     symWinGrid,
			})
		}
	}

	s.winInfos = winInfos
	s.winGrid = winGrid
	s.counter = counter
}

// 处理封顶
func (s *betOrderService) applyMaxWinLimit() {
	if s.stepMultiplier <= 0 {
		return
	}
	oneX := s.req.BaseMoney * float64(s.req.Multiple) // 1 倍线注现金（与 updateBonusAmount 倍数分母一致）
	if oneX <= 0 {
		return
	}
	old := s.stepMultiplier
	capF := s.betAmount.InexactFloat64() * float64(s.gameConfig.MaxWinCap)
	alreadyF := s.scene.TotalWin
	winThisF := oneX * float64(old)
	if alreadyF+winThisF < capF {
		return
	}
	// 剩余可拿（现金）不足本步理论赢时，按 oneX 的整数倍步长截断
	if leftF := capF - alreadyF; leftF < winThisF {
		if c := int64(leftF / oneX); c <= 0 {
			s.stepMultiplier = 0
		} else if c < old {
			s.stepMultiplier = c
		}
	}
	s.limit = true
	s.addFreeTime = 0
	s.scene.FreeNum = 0
	s.scene.NextStage = _spinTypeBase
	//s.scene.Lock = [16]int64{}
	//s.scene.PurchaseAmount = 0
}
