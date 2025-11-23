package mahjong

func (s *betOrderService) baseSpin() (*BaseSpinResult, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}

	var runState int8 = runStateNormal

	if s.isFreeRound() {
		runState = runStateFreeGame
		if s.isRoundFirstStep {
			s.client.ClientOfFreeGame.IncrFreeTimes()
			s.client.ClientOfFreeGame.Decr()
		}
	}

	//判断是否为round的第一个step
	if s.isRoundFirstStep {
		//初始化SymbolRoller
		s.scene.SymbolRoller = s.initSpinSymbol(s.scene.Stage)
		s.isRoundFirstStep = false
		s.gameMultiple = 1
		s.removeNum = 0
	}

	//prettyRandSymbol(s.scene.SymbolRoller)

	s.scene.Steps++
	//zap.L().Info("s.scene.steps:", zap.Any(":", s.scene.steps))

	//转换成通用符号
	s.handleSymbolGrid()

	//prettyInt64Grid(symbolGrid)

	winInfos := s.checkSymbolGridWin()

	s.reversalSymbolGrid = s.reverseSymbolInPlace(s.symbolGrid)
	s.reversalWinGrid = s.reverseWinInPlace(s.winGrid)

	var baseSpinResult = BaseSpinResult{
		lineMultiplier:    0,
		stepMultiplier:    0,
		bonusHeadMultiple: 0,
		bonusTimes:        0,
		scatterCount:      0,
		freeTime:          0,
		addFreeTime:       0,
		winGrid:           s.reversalWinGrid,
		SpinOver:          false,
		winInfo: WinInfo{
			Next:          false,
			Over:          false,
			Multi:         0,
			State:         runState,
			FreeNum:       0,
			FreeTime:      0,
			TotalFreeTime: 0,
			FreeMultiple:  s.gameMultiple,
			WinArr:        make([]WinElem, 0),
		},
		cards: s.reversalSymbolGrid,
	}

	//有中奖
	if len(winInfos) > 0 {

		s.scene.RoundOver = false
		lineMultiplier := s.handleWinInfosMultiplier(winInfos)

		baseSpinResult.lineMultiplier = lineMultiplier
		if s.isFreeRound() {
			s.gameMultiple = s.gameConfig.FreeGameMulti[s.removeNum]
		} else {
			s.gameMultiple = s.gameConfig.BaseGameMulti[s.removeNum]
		}

		stepMultiplierMulCombo := lineMultiplier * s.gameMultiple

		baseSpinResult.stepMultiplier = stepMultiplierMulCombo
		baseSpinResult.bonusHeadMultiple = s.gameMultiple
		s.scene.RoundMultiplier += stepMultiplierMulCombo
		s.scene.SpinMultiplier += stepMultiplierMulCombo
		if s.scene.IsFreeRound {
			s.scene.FreeMultiplier += stepMultiplierMulCombo
		}

		bonusAmount := s.updateBonusAmount(stepMultiplierMulCombo)

		s.updateSpinBonusAmount(bonusAmount)

		baseSpinResult.winInfo.Multi = s.scene.RoundMultiplier

		s.nextSymbolGrid = s.moveSymbols()
		baseSpinResult.nextSymbolGrid = s.nextSymbolGrid

		baseSpinResult.winInfo.Next = true
		//掉落补充符号
		s.fallingWinSymbols(s.nextSymbolGrid, s.scene.Stage)
		//中奖后下回合倍数增加
		if s.removeNum < 3 {
			s.removeNum += 1
		}

		for _, info := range winInfos {
			baseSpinResult.winInfo.WinArr = append(baseSpinResult.winInfo.WinArr, WinElem{
				Val:     info.Symbol,
				RoadNum: info.LineCount,
				StarNum: info.SymbolCount,
				Odds:    info.Odds,
				Mul:     info.Multiplier,
				Loc:     info.WinGrid,
			})

			baseSpinResult.winResult = append(baseSpinResult.winResult, CardType{
				Type:     int(info.Symbol),
				Way:      int(info.LineCount),
				Multiple: int(info.Odds),
				Route:    int(info.SymbolCount),
			})
		}

		s.scene.NextStage = _spinTypeBaseEli
		if s.isFreeRound() {
			s.scene.NextStage = _spinTypeFreeEli
		}

	} else {

		//没中奖逻辑
		s.scene.Steps = 0
		s.scene.NextStage = _spinTypeBase
		if s.isFreeRound() {
			s.scene.NextStage = _spinTypeFree
		}

		s.scene.RoundOver = true

		if s.isFreeRound() {
			baseSpinResult.bonusHeadMultiple = s.gameConfig.FreeGameMulti[s.removeNum]
		} else {
			baseSpinResult.bonusHeadMultiple = s.gameConfig.BaseGameMulti[s.removeNum]
		}
		baseSpinResult.bonusTimes = s.removeNum

		s.removeNum = 0
		s.gameMultiple = 1
		s.nextSymbolGrid = s.symbolGrid
		baseSpinResult.nextSymbolGrid = s.nextSymbolGrid

		scatterCount := s.getScatterCount()

		baseSpinResult.scatterCount = scatterCount

		// 设置round结束
		s.client.ClientOfFreeGame.SetLastMapId(0)

		//普通模式里面
		if s.isBaseRound() {
			//Scatter数量大于配置，要进入到免费模式
			scatterCountTmp := scatterCount
			if scatterCountTmp >= 5 {
				scatterCountTmp = 5
			}
			if scatterCount >= s.gameConfig.FreeGameMin {
				baseSpinResult.winInfo.State = runStateFreeGame
				s.scene.NextStage = _spinTypeFree
				addFreeTimes := uint64(s.gameConfig.FreeGameTimes + (scatterCount-s.gameConfig.FreeGameMin)*s.gameConfig.FreeGameAddTimes)
				s.client.SetMaxFreeNum(addFreeTimes)
				s.client.ClientOfFreeGame.SetFreeNum(addFreeTimes)
				s.gameMultiple = 1
				baseSpinResult.winInfo.Next = true
				baseSpinResult.freeTime = int64(addFreeTimes)

			} else { //否则设置spin和round都退出
				baseSpinResult.winInfo.Next = false
				s.client.ClientOfFreeGame.SetLastWinId(0)
				s.scene.IsFreeRound = false
				baseSpinResult.SpinOver = true
			}
		} else { //本来在免费模式里面
			//增加免费次数
			if scatterCount >= s.gameConfig.FreeGameMin {
				addFreeTimes := int64(s.gameConfig.FreeGameTimes + (scatterCount-s.gameConfig.FreeGameMin)*s.gameConfig.FreeGameAddTimes)
				baseSpinResult.addFreeTime = addFreeTimes
				baseSpinResult.freeTime = int64(addFreeTimes)
				s.client.ClientOfFreeGame.Incr(uint64(baseSpinResult.addFreeTime))
			}
			//是否是最后一次？
			if s.client.ClientOfFreeGame.GetFreeNum() < 1 {
				//设置spin结束
				s.client.ClientOfFreeGame.SetLastWinId(0)
				s.scene.NextStage = _spinTypeBase
				baseSpinResult.winInfo.Next = false
				baseSpinResult.SpinOver = true
			} else {
				s.scene.IsFreeRound = true
				baseSpinResult.winInfo.Next = true
			}
		}
		baseSpinResult.winInfo.ScatterCount = int64(scatterCount)
	}

	s.scene.RemoveNum = s.removeNum
	s.scene.GameWinMultiple = s.gameMultiple
	baseSpinResult.gameMultiple = s.gameMultiple
	baseSpinResult.winInfo.FreeNum = s.client.ClientOfFreeGame.GetFreeNum()
	baseSpinResult.winInfo.FreeTime = s.client.ClientOfFreeGame.GetFreeTimes()
	baseSpinResult.winInfo.TotalFreeTime = baseSpinResult.winInfo.FreeNum + baseSpinResult.winInfo.FreeTime
	baseSpinResult.winInfo.Over = !baseSpinResult.winInfo.Next
	baseSpinResult.winInfo.IsRoundOver = s.scene.RoundOver
	baseSpinResult.winInfo.AddFreeTime = baseSpinResult.addFreeTime
	baseSpinResult.winInfo.WinGrid = s.reversalWinGrid
	baseSpinResult.winInfo.NextSymbolGrid = s.nextSymbolGrid

	return &baseSpinResult, nil

}
