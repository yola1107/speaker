package ajtm

//
//// 初始化长符号网格
//func (lm *longMatrix) init() {
//	for col := range lm {
//		lm[col] = make([]longBlock, 0)
//	}
//}
//
//// 添加长符号到指定列
//func (lm *longMatrix) add(col int, lb longBlock) {
//	lm[col] = append(lm[col], lb)
//}
//
//// 获取指定列的所有长符号
//func (lm *longMatrix) get(col int) []longBlock {
//	return lm[col]
//}
//
//// 清空指定列的长符号
//func (lm *longMatrix) clear(col int) {
//	lm[col] = lm[col][:0]
//}
//
//// 清空所有长符号
//func (lm *longMatrix) clearAll() {
//	for col := range lm {
//		lm[col] = lm[col][:0]
//	}
//}
//
//// 检查指定位置是否有长符号
//func (lm *longMatrix) hasAt(row, col int) bool {
//	for _, lb := range lm[col] {
//		if lb.count > 0 && int(lb.startRow) <= row && int(lb.endRow) >= row {
//			return true
//		}
//	}
//	return false
//}
//
//// 长符号下落 - 基础符号消除后，长符号下落并落在其他符号上方
//func (lm *longMatrix) dropAfterEliminate(grid *int64Grid) {
//	for col := 1; col < _colCount-1; col++ { // 只处理中间列
//		// 按从下到上的顺序处理，避免互相影响
//		for row := _rowCount - 1; row >= 1; row-- {
//			// 检查当前位置是否为长符号的尾部
//			if (*grid)[row][col] >= _longSymbol {
//				// 获取对应的头部位置
//				headRow := row - 1
//				headSymbol := (*grid)[row][col] - _longSymbol
//
//				// 尝试让这个长符号下落
//				newEndRow := lm.findDropPosition(*grid, col, row)
//
//				if newEndRow != row && newEndRow >= 1 {
//					// 移动长符号
//					(*grid)[headRow][col] = 0    // 清空原头部
//					(*grid)[row][col] = 0        // 清空原尾部
//
//					// 设置新位置
//					newHeadRow := newEndRow - 1
//					(*grid)[newHeadRow][col] = headSymbol      // 新头部
//					(*grid)[newEndRow][col] = _longSymbol + headSymbol  // 新尾部
//				}
//			}
//		}
//	}
//}
//
//// 查找长符号的下落位置（落到其他符号上方）
//func (lm *longMatrix) findDropPosition(grid int64Grid, col, currentEndRow int) int {
//	// 从当前位置向下查找，直到找到阻挡物或底部
//	for row := currentEndRow; row < _rowCount-1; row++ {
//		// 检查下方位置是否有符号
//		if grid[row+1][col] != 0 {
//			// 找到阻挡物，长符号应该停在其上方
//			// 需要确保上方还有空间放置头部
//			if row > 0 {
//				return row
//			}
//			return currentEndRow
//		}
//	}
//	// 如果到底部都没有阻挡，则尽可能下落（需留出头部空间）
//	if _rowCount > 1 {
//		return _rowCount - 1
//	}
//	return currentEndRow
//}
//
//// 长符号重排 - 无中奖时，将长符号移动到底部并依次叠加
//func (lm *longMatrix) rearrange(grid *int64Grid) {
//	for col := 1; col < _colCount-1; col++ { // 只处理中间列
//		// 收集当前列的所有长符号及其信息
//		var tempLongBlocks []longBlock
//
//		// 找出所有长符号
//		for row := 1; row < _rowCount; row++ {
//			if (*grid)[row][col] >= _longSymbol {
//				headSymbol := (*grid)[row][col] - _longSymbol
//				headRow := row - 1
//				tempLongBlocks = append(tempLongBlocks, longBlock{
//					symbol:   headSymbol,
//					startRow: int64(headRow),
//					endRow:   int64(row),
//					count:    1,
//				})
//
//				// 清空原位置
//				(*grid)[headRow][col] = 0
//				(*grid)[row][col] = 0
//			}
//		}
//
//		// 从底部开始重新放置长符号（最多3个，占满6行）
//		currentBottom := _rowCount - 1
//		for _, lb := range tempLongBlocks {
//			if currentBottom >= 1 { // 确保有足够的空间放置2格高的长符号
//				// 在新位置放置长符号
//				newStartRow := currentBottom - 1
//				(*grid)[newStartRow][col] = lb.symbol
//				(*grid)[currentBottom][col] = _longSymbol + lb.symbol
//
//				// 更新lm中的信息（如果需要的话）
//				currentBottom -= 2 // 每个长符号占用2行
//			}
//		}
//	}
//}
