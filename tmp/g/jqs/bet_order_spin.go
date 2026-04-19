package jqs

// baseSpin：初始化 → 盘面 → 结算倍率与下一阶段 → 奖金
func (s *betOrderService) baseSpin() error {
	if err := s.initialize(); err != nil {
		return err
	}
	s.initSymbolGrid()
	s.findWinInfo()     // 找到中奖线路
	s.processWinInfos() // 计算奖金金额
	return nil
}

func (s *betOrderService) processWinInfos() {
	lineMul := int64(0)
	for _, elem := range s.winInfos {
		lineMul += elem.Odds
	}
	// 检查是否全百搭，触发最大中奖倍数（取最大值，不覆盖其他中奖）
	if s.isAllWild() {
		if int64(s.gameConfig.MaxPayMultiple) > lineMul {
			lineMul = int64(s.gameConfig.MaxPayMultiple)
		}
	}
	s.stepMultiplier = lineMul

	if s.scene.Stage == _spinTypeFree {
		if s.stepMultiplier > 0 {
			s.scene.NextStage = _spinTypeBase
		} else {
			s.scene.NextStage = _spinTypeFree
		}
	} else {
		s.scene.NextStage = _spinTypeBase
	}

	s.updateBonusAmount()
}
