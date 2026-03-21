package sbyymx2

import "math/rand/v2"

func (s *betOrderService) baseSpin() error {
	if s.debug.open {
		s.syncGameStage()
	}
	if err := s.initialize(); err != nil {
		return err
	}
	s.fillSymbolGrid()
	s.processGame()
	return nil
}

// fillSymbolGrid 优先按策划四套权重独立抽样每格；否则按三列条带连续取 3 格（兼容旧配置）
func (s *betOrderService) fillSymbolGrid() {
	if s.gameConfig.useWeightTables() {
		for r := 0; r < _rowCount; r++ {
			for c := 0; c < _colCount; c++ {
				s.symbolGrid[r][c] = s.pickSymbolForCell(r, c)
			}
		}
		return
	}
	for c := 0; c < _colCount; c++ {
		strip := s.gameConfig.Reels[c]
		n := len(strip)
		start := rand.IntN(n)
		for r := 0; r < _rowCount; r++ {
			s.symbolGrid[r][c] = strip[(start+r)%n]
		}
	}
}

// processGame 中间行判奖并结算本步倍数与奖金
func (s *betOrderService) processGame() {
	s.lineMultiplier = 0
	s.stepMultiplier = 0
	s.winInfos = nil
	s.winResults = nil
	s.winGrid = int64Grid{}
	s.findWinInfos()
	s.processWinInfos()
	s.updateBonusAmount(s.stepMultiplier)
}
