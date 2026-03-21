package main

func CheckWin(grid Matrix) (details []WinDetail, totalWin int, winPositions []Position) {
	// Base game only cares about bottom 4 rows for winning checks.
	judgeRowStart := Rows - 4 // visible bottom 4 rows: [judgeRowStart..Rows-1]

	for lineIdx := 0; lineIdx < Lines; lineIdx++ {
		idxs := LinesJSON[lineIdx]
		if len(idxs) != Cols {
			continue
		}

		// line stop indexes are flattened as idx = row*Cols + col (top-left is (0,0)).
		originIdx := idxs[0]
		originRow := originIdx / Cols
		originCol := originIdx % Cols
		if originRow < judgeRowStart {
			continue
		}

		originSymbol := grid[originRow][originCol]

		// 确定候选符号列表（对齐sjxj生产版逻辑）
		// - Wild首格：枚举1-9所有符号
		// - 普通符号(1-9)：只检查该符号
		// - Scatter/空：不触发线奖
		var symbolCandidates [9]int
		candCount := 0

		if originSymbol == Wild {
			// Wild首格：枚举1-9所有符号
			for sym := 1; sym <= 9; sym++ {
				symbolCandidates[candCount] = sym
				candCount++
			}
		} else if originSymbol >= 1 && originSymbol <= 9 {
			// 普通符号：只检查该符号
			symbolCandidates[0] = int(originSymbol)
			candCount = 1
		} else {
			// Scatter(11)或空格等不触发线奖
			continue
		}

		// 对每个候选符号计算线奖
		for candIdx := 0; candIdx < candCount; candIdx++ {
			targetSymbol := symbolCandidates[candIdx]
			length := 0
			positions := []Position{}

			for step := 0; step < Cols; step++ {
				idx := idxs[step]
				row := idx / Cols
				col := idx % Cols

				// If the payline goes into the hidden (top 4) area, base game stops evaluating.
				if row < judgeRowStart {
					break
				}

				sym := grid[row][col]
				// 匹配条件：符号相同 或 当前格为Wild（Wild可替代普通符号）
				if sym == targetSymbol || sym == Wild {
					length++
					positions = append(positions, Position{Row: row, Col: col})
				} else {
					break
				}
			}

			if length >= 3 {
				pay := PayTableJSON[targetSymbol-1][length-1]
				if pay > 0 {
					details = append(details, WinDetail{
						LineIndex: lineIdx,
						SymbolID:  targetSymbol,
						LineLen:   length,
						Win:       pay,
					})
					totalWin += pay
					winPositions = append(winPositions, positions...)
				}
			}
		}
	}

	return details, totalWin, winPositions
}
