package jwjzy

import "math/rand/v2"

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}
	if s.isFreeRound && s.scene.IsRoundFirstStep {
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
		s.scene.IsRoundFirstStep = false
	}
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.scene.SymbolRoller = s.initSpinSymbol()
	}
	s.handleSymbolGrid()
	s.checkSymbolGridWin()
	s.processWinInfos()
	return nil
}

func (s *betOrderService) processWinInfos() {
	s.addFreeTime = 0
	if len(s.winInfos) > 0 {
		s.processWin()
	} else {
		s.processNoWin()
	}
}

func (s *betOrderService) processWin() {
	// 本步 WayGame 基础赔付倍数：sum(odds * routeNum)
	baseWinMultiplier := int64(0)
	for _, elem := range s.winInfos {
		baseWinMultiplier += elem.Odds * elem.LineCount
	}

	// 金色框->wild、非金色长符号清除与补位，并激活酒杯轴
	cupSum, wildConvertedCount := s.eliminateAndRefill()
	s.addWildEliCount = wildConvertedCount

	s.wildMultiplier = cupSum
	s.stepMultiplier = baseWinMultiplier * cupSum
	s.isRoundOver = false

	s.scene.Steps++
	s.scene.RoundMultiplier += s.stepMultiplier

	if s.isFreeRound {
		s.scene.NextStage = _spinTypeFreeEli
	} else {
		s.scene.NextStage = _spinTypeBaseEli
	}
	s.updateBonusAmount(s.stepMultiplier)
}

func (s *betOrderService) processNoWin() {
	s.stepMultiplier = 0
	s.addWildEliCount = 0
	s.wildMultiplier = 0
	s.isRoundOver = true
	s.scatterCount = s.getScatterCount()

	s.scene.Steps = 0
	s.scene.ContinueNum = 0

	// 基础模式：每局结算后酒杯轴与累计蝴蝶百搭需要清空
	//（免费模式：在 free 结束前不清空）
	if !s.isFreeRound {
		s.scene.TotalWildEliCount = 0
		s.scene.CupActCounts = [4]int64{}
	}

	s.updateBonusAmount(0)
	s.client.ClientOfFreeGame.SetLastWinId(0)

	if newFree := s.calcNewFreeGameNum(s.scatterCount); newFree > 0 {
		s.client.ClientOfFreeGame.Incr(uint64(newFree))
		s.scene.FreeNum += newFree
		s.addFreeTime = newFree
	}

	if s.isFreeRound {
		if s.scene.FreeNum <= 0 {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
			s.scene.IsRoundFirstStep = false
			s.scene.TotalWildEliCount = 0
			s.scene.CupActCounts = [4]int64{}
		} else {
			s.scene.NextStage = _spinTypeFree
			s.scene.IsRoundFirstStep = true
		}
	} else {
		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
			s.scene.IsRoundFirstStep = true
			s.scene.TotalWildEliCount = 0
			s.scene.CupActCounts = [4]int64{}
		} else {
			s.scene.FreeNum = 0
			s.scene.NextStage = _spinTypeBase
			s.scene.IsRoundFirstStep = false
		}
	}
}

// calcCupMultiplierSumForCount 计算“某列酒杯被激活 k 次”后的倍数累计和
// 策划口径（对应你提供的描述）：
// - 前4次均为 2
// - 第5次开始：第k次倍数 = (k-3)*2
func calcCupMultiplierSumForCount(k int64) int64 {
	if k <= 0 {
		return 0
	}
	if k <= 4 {
		return 2 * k
	}
	// k>4：前4次和=8；后续第i次贡献为 (i-3)*2
	// 累计和 = 8 + (k-1)*(k-4)
	return 8 + (k-1)*(k-4)
}

func (s *betOrderService) cupSumFromScene() int64 {
	var sum int64
	for i := 0; i < 4; i++ {
		sum += calcCupMultiplierSumForCount(s.scene.CupActCounts[i])
	}
	return sum
}

type longSegInfo struct {
	top    int
	len    int
	isGold bool
	isWild bool
}

func (s *betOrderService) eliminateAndRefill() (cupSum int64, convertedLongCount int64) {
	// 注意：本步回包需要展示“消除前盘面”，所以这里对 symbol/gold/long 做拷贝，只把下一步结果写回 scene。
	symbolGrid := s.symbolGrid
	goldGrid := s.goldFrameGrid
	longGrid := s.longGrid

	// 预扫描中间列（col=1..4）长符号的位置/属性
	var segs [6]longSegInfo
	for col := 1; col <= 4; col++ {
		top := -1
		segLen := 0
		isGold := false
		isWild := false
		for r := 0; r < _rowCount; r++ {
			if longGrid[r][col] {
				if top < 0 {
					top = r
				}
				segLen++
				if goldGrid[r][col] {
					isGold = true
				}
				if symbolGrid[r][col] == _wild {
					isWild = true
				}
			}
		}
		if segLen > 0 {
			segs[col] = longSegInfo{top: top, len: segLen, isGold: isGold, isWild: isWild}
		}
	}

	var (
		removedCountPerCol   [6]int64 // 该列被清空后会产生多少空位（用于下落与补位）
		clearedLongLenPerCol [6]int64 // 若该列曾清除“非金色长符号”，记录其长度（用于补同长度长符号）
		clearedLongCols      [6]bool
		convertedLongCols    [6]bool
	)

	// 1) 消除：金色长符号->wild；非金色长符号清除并统计“需要补位的长度”
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			if s.winGrid[r][c] == 0 {
				continue
			}

			// wild 本身不再被消除
			if symbolGrid[r][c] == _wild {
				continue
			}

			// 命中在“长符号占位”内？
			if c >= 1 && c <= 4 && segs[c].len > 0 && r >= segs[c].top && r < segs[c].top+segs[c].len {
				// 命中长符号：如果长符号已经是 wild，则不会消除
				if segs[c].isWild {
					continue
				}

				if segs[c].isGold {
					if !convertedLongCols[c] {
						convertedLongCols[c] = true
						for rr := segs[c].top; rr < segs[c].top+segs[c].len; rr++ {
							symbolGrid[rr][c] = _wild
							goldGrid[rr][c] = false
							// longGrid 保持占位
						}
						convertedLongCount++
						s.scene.CupActCounts[c-1]++
					}
				} else {
					if !clearedLongCols[c] {
						clearedLongCols[c] = true
						for rr := segs[c].top; rr < segs[c].top+segs[c].len; rr++ {
							symbolGrid[rr][c] = 0
							goldGrid[rr][c] = false
							longGrid[rr][c] = false
						}
						removedLen := int64(segs[c].len)
						removedCountPerCol[c] += removedLen
						clearedLongLenPerCol[c] = removedLen
					}
				}
				continue
			}

			// 命中非长符号：单格消除
			symbolGrid[r][c] = 0
			goldGrid[r][c] = false
			longGrid[r][c] = false
			removedCountPerCol[c]++
		}
	}

	// 累计 wild 转换个数（用于 pb 的 totalWildEliCount）
	s.scene.TotalWildEliCount += convertedLongCount

	// 2) 下落（压缩）：同时移动 symbol/gold/long 三份属性
	for c := 0; c < _colCount; c++ {
		writePos := 0
		for r := 0; r < _rowCount; r++ {
			if symbolGrid[r][c] == 0 {
				continue
			}
			if writePos != r {
				symbolGrid[writePos][c] = symbolGrid[r][c]
				goldGrid[writePos][c] = goldGrid[r][c]
				longGrid[writePos][c] = longGrid[r][c]

				// 清空原位
				symbolGrid[r][c] = 0
				goldGrid[r][c] = false
				longGrid[r][c] = false
			}
			writePos++
		}
		// 补齐空位
		for r := writePos; r < _rowCount; r++ {
			symbolGrid[r][c] = 0
			goldGrid[r][c] = false
			longGrid[r][c] = false
		}
	}

	// 3) 补位：根据本列空位数与“被清除的长符号长度”在空位区随机塞回长符号（若需要）
	drawBaseSymbols := func(isFree bool) ([]int64, []int) {
		baseSymbols := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12}
		weights := s.gameConfig.BaseSymbolWeights
		if isFree && len(s.gameConfig.FreeSymbolWeights) == len(weights) && len(s.gameConfig.FreeSymbolWeights) > 0 {
			weights = s.gameConfig.FreeSymbolWeights
		}
		if len(weights) != len(baseSymbols) {
			weights = []int{1500, 1500, 1500, 1200, 1200, 800, 800, 500, 400, 300, 300}
		}
		return baseSymbols, weights
	}

	drawWeightedBaseSymbol := func(isFree bool) int64 {
		baseSymbols, weights := drawBaseSymbols(isFree)
		total := 0
		for _, w := range weights {
			total += w
		}
		if total <= 0 {
			return baseSymbols[rand.IntN(len(baseSymbols))]
		}
		r := rand.IntN(total)
		for i, w := range weights {
			if r < w {
				return baseSymbols[i]
			}
			r -= w
		}
		return baseSymbols[len(baseSymbols)-1]
	}

	drawSingleSymbol := func(isFree bool) int64 {
		// 散布为夺宝符（treasure）
		scatterProb := s.gameConfig.BaseScatterProb
		if isFree && s.gameConfig.FreeScatterProb > 0 {
			scatterProb = s.gameConfig.FreeScatterProb
		}
		if scatterProb <= 0 {
			scatterProb = 250
		}
		if rand.IntN(10000) < scatterProb {
			return _treasure
		}
		return drawWeightedBaseSymbol(isFree)
	}

	// 对“掉落长符号”：永远有金色背景，且长符号自身不可为夺宝
	drawLongBaseSymbol := func(isFree bool) int64 {
		// 直接从 base 符号池抽（不允许 treasure）
		return drawWeightedBaseSymbol(isFree)
	}

	for c := 0; c < _colCount; c++ {
		blanksCount := removedCountPerCol[c]
		if blanksCount <= 0 {
			continue
		}
		blankStartRow := _rowCount - int(blanksCount)
		if blankStartRow < 0 {
			blankStartRow = 0
		}

		needLong := (c >= 1 && c <= 4 && clearedLongLenPerCol[c] > 0)
		longLen := int64(0)
		if needLong {
			longLen = clearedLongLenPerCol[c]
			if longLen > blanksCount {
				longLen = blanksCount
			}
		}

		// 如果需要长符号，随机决定起始行（长符号长度固定为 clearedLongLen）
		longTop := -1
		if needLong && longLen > 0 {
			maxOffset := blanksCount - longLen
			if maxOffset < 0 {
				maxOffset = 0
			}
			longTop = blankStartRow + rand.IntN(int(maxOffset)+1)
		}

		for r := blankStartRow; r < _rowCount; r++ {
			if longTop >= 0 && r >= longTop && r < longTop+int(longLen) {
				symbolGrid[r][c] = drawLongBaseSymbol(s.isFreeRound)
				goldGrid[r][c] = true
				longGrid[r][c] = true
			} else {
				symbolGrid[r][c] = drawSingleSymbol(s.isFreeRound)
				goldGrid[r][c] = false
				longGrid[r][c] = false
			}
		}
	}

	// 写回 scene.SymbolRoller：供下一步 handleSymbolGrid 读取
	for col := 0; col < _colCount; col++ {
		for rollerRow := 0; rollerRow < _rowCount; rollerRow++ {
			internalRow := _rowCount - 1 - rollerRow
			s.scene.SymbolRoller[col].BoardSymbol[rollerRow] = symbolGrid[internalRow][col]
			s.scene.SymbolRoller[col].BoardGold[rollerRow] = goldGrid[internalRow][col]
			s.scene.SymbolRoller[col].BoardLong[rollerRow] = longGrid[internalRow][col]
		}
	}

	// 计算酒杯轴累计倍数（本步所有已激活的酒杯倍数相加）
	cupSum = s.cupSumFromScene()
	return cupSum, convertedLongCount
}
