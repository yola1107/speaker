package pjcd

/*
// initWildStateGrid 初始化百搭状态网格
func (s *betOrderService) initWildStateGrid() {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			// 只处理中间3列(列2,3,4即索引1,2,3)的百搭
			if c < 1 || c > 3 {
				continue
			}
			if s.scene.WildStateGrid[r][c] == 0 && s.symbolGrid[r][c] == _wild {
				s.scene.WildStateGrid[r][c] = _wildFormCaterpillar // 新百搭默认毛虫形态
			}
		}
	}
}

// evolveWilds 进化参与中奖的百搭
func (s *betOrderService) evolveWilds() {
	for r := 0; r < _rowCountReward; r++ {
		for c := 0; c < _colCount; c++ {
			// 只处理中奖位置且有百搭形态的位置
			if s.winGrid[r][c] > 0 && s.scene.WildStateGrid[r][c] > 0 {
				currentForm := s.scene.WildStateGrid[r][c]
				switch currentForm {
				case _wildFormCaterpillar:
					// 毛虫→蝶茧
					s.scene.WildStateGrid[r][c] = _wildFormChrysalis
				case _wildFormChrysalis:
					// 蝶茧→蝴蝶
					s.scene.WildStateGrid[r][c] = _wildFormButterfly
				case _wildFormButterfly:
					// 蝴蝶参与中奖：累加蝴蝶百搭个数，然后消除
					s.scene.ButterflyCount++
					s.scene.WildStateGrid[r][c] = 0 // 消除
				}
			}
		}
	}
}

// preserveWildStates 在消除时保留百搭状态
func (s *betOrderService) preserveWildStates(nextGrid *int64Grid, nextWildGrid *int64Grid) {
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			// 保留非蝴蝶百搭状态（蝴蝶在evolveWilds中已处理）
			if s.scene.WildStateGrid[r][c] > 0 && s.scene.WildStateGrid[r][c] != _wildFormButterfly {
				(*nextGrid)[r][c] = _wild
				(*nextWildGrid)[r][c] = s.scene.WildStateGrid[r][c]
			}
		}
	}
}

// getWildFormAt 获取指定位置的百搭形态
func (s *betOrderService) getWildFormAt(r, c int) int64 {
	if r < 0 || r >= _rowCount || c < 0 || c >= _colCount {
		return 0
	}
	return s.scene.WildStateGrid[r][c]
}

// isWildAt 检查指定位置是否为百搭
func (s *betOrderService) isWildAt(r, c int) bool {
	return s.symbolGrid[r][c] == _wild
}

// hasWildState 检查指定位置是否有百搭状态（黏性百搭）
func (s *betOrderService) hasWildState(r, c int) bool {
	return s.scene.WildStateGrid[r][c] > 0
}
*/
