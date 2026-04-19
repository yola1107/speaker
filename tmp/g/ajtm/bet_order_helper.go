package ajtm

import (
	"fmt"

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

func (s *betOrderService) updateBonusAmount(stepMultiplier int64) {
	if s.debug.open || stepMultiplier == 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	s.bonusAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(s.stepMultiplier))
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		rounded := s.bonusAmount.Round(2).InexactFloat64()
		s.scene.TotalWin += rounded
		s.scene.RoundWin += rounded
		if s.isFreeRound {
			s.scene.FreeWin += rounded
		}
	}
}

func int64GridToArray(grid int64Grid) []int64 {
	elements := make([]int64, _rowCount*_colCount)
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			elements[r*_colCount+c] = grid[r][c]
		}
	}
	return elements
}

// applyMaxWinMultiplierLimit 处理“最大可赢”封顶：
// - 基础模式：当前局累计返奖不可超过 betAmount * MaxWinMultiplier，超出部分不发并直接结束当前局；
// - 免费模式：触发免费前的基础中奖 + 免费累计中奖，同样不可超过该上限，触发即直接结束免费。
func (s *betOrderService) applyMaxWinMultiplierLimit() {
	if s.stepMultiplier <= 0 {
		return
	}

	denom := decimal.NewFromFloat(s.req.BaseMoney).Mul(decimal.NewFromInt(s.req.Multiple))
	if denom.LessThanOrEqual(decimal.Zero) {
		return
	}

	oldStepMul := s.stepMultiplier
	maxWin := s.betAmount.Mul(decimal.NewFromInt(s.gameConfig.MaxWinMultiplier))
	stepWin := denom.Mul(decimal.NewFromInt(oldStepMul))

	// 本步前累计赢金：线上取已入账值；debug 压测取本局已累计倍数（去掉当前步）
	lastTotal := decimal.NewFromFloat(s.scene.TotalWin) //decimal.NewFromFloat(s.client.ClientOfFreeGame.GetGeneralWinTotal())
	if s.debug.open {
		prevRoundMul := s.scene.RoundMultiplier - oldStepMul
		if prevRoundMul < 0 {
			prevRoundMul = 0
		}
		lastTotal = denom.Mul(decimal.NewFromInt(prevRoundMul))
	}

	// 不超上限，直接放行
	if lastTotal.Add(stepWin).LessThan(maxWin) {
		return
	}

	// 超上限时裁剪本步；等于上限时保持本步并直接收口
	remaining := maxWin.Sub(lastTotal)
	if remaining.LessThan(stepWin) {
		capped := remaining.Div(denom).IntPart()
		if capped <= 0 {
			s.stepMultiplier = 0
		} else if capped < oldStepMul {
			s.stepMultiplier = capped
		}
	}

	// 同步修正 RoundMultiplier（它在 processWin/processNoWin 里已经累加过）
	if oldStepMul > s.stepMultiplier {
		delta := oldStepMul - s.stepMultiplier
		s.scene.RoundMultiplier -= delta
		if s.scene.RoundMultiplier < 0 {
			s.scene.RoundMultiplier = 0
		}
	}

	// 封顶时终止当前局，避免继续产生超额赢金。
	s.limit = true
	s.scene.FreeNum = 0
	s.addFreeTime = 0
	s.scene.DownLongs = [_colCount][]Block{}
	s.scene.Steps = 0
	s.isRoundOver = true
	s.scene.NextStage = _spinTypeBase
	s.scene.MysMulTotal = 0
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

	s.debug.originSymbolGrid = s.symbolGrid
}

// moveSymbols 仅按 eliGrid 清除并下落。
// winGrid 只负责展示，不参与真实消除。
func (s *betOrderService) moveSymbols() int64Grid {
	next := s.symbolGrid
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.eliGrid[r][c] > 0 {
				next[r][c] = 0
			}
		}
	}
	s.dropSymbols(&next)
	return next
}

func (s *betOrderService) dropSymbols(grid *int64Grid) {
	for c := 0; c < _colCount; c++ {
		isEdgeCol := c == 0 || c == _colCount-1
		writePos := _rowCount - 1
		if isEdgeCol {
			writePos = _rowCount - 2
		}
		for r := _rowCount - 1; r >= 0; r-- {
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

func (s *betOrderService) fallingWinSymbols(next int64Grid) {
	for col := range s.scene.SymbolRoller {
		roller := &s.scene.SymbolRoller[col]
		for r := 0; r < _rowCount; r++ {
			roller.BoardSymbol[r] = next[r][col]
		}
		if s.isFreeRound {
			roller.ringSymbol(s.gameConfig)
		} else {
			roller.ringSymbolForBase(s.gameConfig)
		}
	}
}

func (s *betOrderService) findWinInfos() {
	winInfos := make([]WinInfo, 0, _wild-_blank-1)
	var winGrid int64Grid
	var eliGrid int64Grid
	var seenLongHead [_rowCount][_colCount]bool

	s.winMys = s.winMys[:0]

	for symbol := _blank + 1; symbol < _wild; symbol++ {
		info, ok := s.findSymbolWinInfo(symbol)
		if !ok {
			continue
		}

		winInfos = append(winInfos, *info)
		s.mergeWinGrids(info.WinGrid, &winGrid, &eliGrid, &seenLongHead)
	}

	s.winInfos = winInfos
	s.winGrid = winGrid
	s.eliGrid = eliGrid
}

func (s *betOrderService) mergeWinGrids(src int64Grid, winGrid, eliGrid *int64Grid, seenLongHead *[_rowCount][_colCount]bool) {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			cell := src[r][c]
			if cell <= 0 {
				continue
			}

			head := s.symbolGrid[r][c]
			isLongHead := r < _rowCount-1 && head > 0 && head < _longSymbol && s.symbolGrid[r+1][c] == _longSymbol+head
			if isLongHead {
				if !seenLongHead[r][c] {
					seenLongHead[r][c] = true
					s.winMys = append(s.winMys, Block{
						Col:       int64(c),
						HeadRow:   int64(r),
						TailRow:   int64(r + 1),
						OldSymbol: head,
					})
				}

				// 长符号需要展示中奖，但不进入实际消除网格。
				(*winGrid)[r][c] = head
				if r+1 < _rowCount {
					(*winGrid)[r+1][c] = s.symbolGrid[r+1][c]
				}
				continue
			}
			(*winGrid)[r][c] = cell
			(*eliGrid)[r][c] = cell
		}
	}
}

// findSymbolWinInfo 按 Ways 从左到右判定，至少命中 3 列。
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
			}
		}

		if matchCount == 0 {
			if c >= _minMatchCount && hasRealSymbol {
				if odds := s.getSymbolBaseMultiplier(symbol, c); odds > 0 {
					return &WinInfo{
						Symbol:      symbol,
						SymbolCount: int64(c),
						LineCount:   lineCount,
						Odds:        odds,
						Multiplier:  odds * lineCount,
						WinGrid:     winGrid,
					}, true
				}
			}
			return nil, false
		}

		lineCount *= int64(matchCount)
		if c == _colCount-1 && hasRealSymbol {
			if odds := s.getSymbolBaseMultiplier(symbol, _colCount); odds > 0 {
				return &WinInfo{
					Symbol:      symbol,
					SymbolCount: _colCount,
					LineCount:   lineCount,
					Odds:        odds,
					Multiplier:  odds * lineCount,
					WinGrid:     winGrid,
				}, true
			}
		}
	}
	return nil, false
}

// snapshotDownLongsFromGrid 免费无中奖且仍有免费次数：按列自上而下收集长符号头部，顺序即相对位置；
// 超上限只保留最靠下几段；再按 n 档沉底公式写 Head/Tail，NewSymbol=randomMysSymbol(旧头)。
func (s *betOrderService) snapshotDownLongsFromGrid() {
	var dl [_colCount][]Block
	maxPerCol := _rowCount / 2
	for c := 1; c < _colCount-1; c++ {
		var olds []int64
		for r := 0; r < _rowCount-1; r++ {
			h := s.symbolGrid[r][c]
			if h > 0 && h < _longSymbol && s.symbolGrid[r+1][c] == _longSymbol+h {
				olds = append(olds, h)
			}
		}
		if len(olds) > maxPerCol {
			olds = olds[len(olds)-maxPerCol:]
		}
		n := len(olds)
		if n == 0 {
			continue
		}
		top := _rowCount - n*2
		for i := 0; i < n; i++ {
			hr := top + i*2
			oldH := olds[i]
			dl[c] = append(dl[c], Block{
				Col: int64(c), HeadRow: int64(hr), TailRow: int64(hr + 1),
				OldSymbol: oldH,
				NewSymbol: randomMysSymbol(oldH),
			})
		}
	}
	s.scene.DownLongs = dl
}

func (s *betOrderService) isLongHeadAt(r, c int) bool {
	if r < 0 || r >= _rowCount-1 || c < 0 || c >= _colCount {
		return false
	}
	head := s.symbolGrid[r][c]
	if head <= 0 || head >= _longSymbol {
		return false
	}
	return s.symbolGrid[r+1][c] == _longSymbol+head
}

func (s *betOrderService) transformWinningLongSymbols() {
	if len(s.winMys) == 0 {
		return
	}

	writeIdx := 0
	for i := 0; i < len(s.winMys); i++ {
		block := s.winMys[i]
		r, c := int(block.HeadRow), int(block.Col)
		if !s.isLongHeadAt(r, c) {
			continue
		}

		oldSymbol := s.symbolGrid[r][c]
		newSymbol := randomMysSymbol(oldSymbol)

		// 长符号中奖后先转变，再参与后续盘面流转。
		s.symbolGrid[r][c] = newSymbol
		s.symbolGrid[r+1][c] = _longSymbol + newSymbol
		s.scene.SymbolRoller[c].BoardSymbol[r] = newSymbol
		s.scene.SymbolRoller[c].BoardSymbol[r+1] = _longSymbol + newSymbol

		block.OldSymbol = oldSymbol
		block.NewSymbol = newSymbol
		s.winMys[writeIdx] = block
		writeIdx++
	}
	s.winMys = s.winMys[:writeIdx]
}
