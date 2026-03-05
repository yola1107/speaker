package jqs

import (
	"egame-grpc/game/common"
	"egame-grpc/game/common/pb"
	"egame-grpc/gamelogic"
	"egame-grpc/gamelogic/game_replay"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/utils/json"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// 初始化
func (s *betOrderService) initialize() error {
	s.client.ClientOfFreeGame.ResetFreeClean()
	var err error
	switch {
	case s.scene.Stage != _spinTypeFree:
		err = s.initStepForFirstStep()
	default:
		err = s.initStepForNextStep()
	}
	if err != nil {
		return err
	}
	return nil
}

func (s *betOrderService) initStepForFirstStep() error {
	switch {
	case !s.updateBetAmount():
		return InvalidRequestParams
	case !s.checkBalance():
		return InsufficientBalance
	}
	s.client.SetLastMaxFreeNum(0)
	s.client.ClientOfFreeGame.Reset()
	s.client.ClientOfFreeGame.ResetGeneralWinTotal()
	s.client.ClientOfFreeGame.ResetRoundBonus()
	s.client.ClientOfFreeGame.SetBetAmount(s.betAmount.Round(2).InexactFloat64())
	s.amount = s.betAmount
	return nil
}

// 初始化spin后续step
func (s *betOrderService) initStepForNextStep() error {
	if s.isRtp {
		s.req.BaseMoney = 1
		s.req.Multiple = 1
	} else {
		s.req.BaseMoney = s.lastOrder.BaseAmount
		s.req.Multiple = s.lastOrder.Multiple
	}
	s.betAmount = decimal.NewFromFloat(s.client.ClientOfFreeGame.GetBetAmount())
	s.amount = decimal.Zero
	s.client.ClientOfFreeGame.ResetRoundBonus()
	return nil
}

// 初始化step预设数据 - 从滚轴生成
func (s *betOrderService) initSymbolGrid() error {
	switch s.scene.Stage {
	case _spinTypeBase:
		// 判定是否触发Re-spin
		if s.calculateTrigger(s.gameConfig.RespinTriggerRate) {
			s.scene.NextStage = _spinTypeFree
			s.state = 1 // 进入免费模式
			// Re-spin模式不需要设置次数，会一直spin直到中奖
		} else if s.calculateTrigger(s.gameConfig.FakeRespinTriggerRate) {
			s.state = 3 // 炸胡状态
		}
	case _spinTypeFree:
		// Re-spin模式：中间列固定为百搭，不重复判定触发
		// 未中奖继续的逻辑在updateStepResult中处理
	}
	s.symbolGrid = s.getSceneSymbol() // 生成滚轴数据
	return nil
}

/*func (s *betOrderService) initMoneySymbol() error {
	s.moneySymbolNum = 0
	for i := 0; i < _rowCount; i++ {
		for j := 0; j < _colCount; j++ {
			if s.symbolGrid[i][j] == _money {
				s.symbolGrid[i][j] = _prize + s.moneyMultiplier()
				s.moneySymbolNum += 1
			}
		}
	}
	return nil
}
*/
// 更新step结果
func (s *betOrderService) updateStepResult() error {
	//s.printSymbolGrid("地图", s.symbolGrid)

	s.findWinInfo(s.symbolGrid) // 找到中奖线路
	s.processWinInfos()         // 计算奖金金额

	// Re-spin模式：未中奖则继续，中奖则返回基础模式
	// 注意：需要判断Stage或NextStage，因为initSymbolGrid可能刚设置了NextStage
	if s.scene.Stage == _spinTypeFree || s.scene.NextStage == _spinTypeFree {
		if s.stepMultiplier > 0 {
			// 中奖了，返回基础模式
			s.scene.NextStage = _spinTypeBase
			s.isRoundCompleted = true // 免费回合完成
		} else {
			// 未中奖，继续Re-spin
			s.scene.NextStage = _spinTypeFree
			s.isRoundCompleted = false // 免费回合未完成
		}
	} else {
		s.scene.NextStage = _spinTypeBase
		s.isRoundCompleted = true // 基础模式step完成即回合完成
	}

	return nil
}

// 更新订单
func (s *betOrderService) updateGameOrder() (bool, error) {
	// 判断是否为免费模式（考虑NextStage）
	isFreeRound := s.scene.Stage == _spinTypeFree || s.scene.NextStage == _spinTypeFree
	if !s.isRtp {
		s.orderSn = common.GenerateOrderSn(s.member, s.lastOrder, !isFreeRound, isFreeRound)
	}
	isFree := int64(0)
	if isFreeRound {
		isFree = 1
	}
	s.gameOrder = &game.GameOrder{
		MerchantID:        s.merchant.ID,
		Merchant:          s.merchant.Merchant,
		MemberID:          s.member.ID,
		Member:            s.member.MemberName,
		GameID:            s.game.ID,
		GameName:          s.game.GameName,
		BaseMultiple:      _baseMultiplier,
		Multiple:          s.req.Multiple,
		LineMultiple:      s.lineMultiplier,
		BonusHeadMultiple: 1,
		BonusMultiple:     s.stepMultiplier,
		BaseAmount:        s.req.BaseMoney,
		Amount:            s.amount.Round(2).InexactFloat64(),
		ValidAmount:       s.amount.Round(2).InexactFloat64(),
		BonusAmount:       s.bonusAmount.Round(2).InexactFloat64(),
		CurBalance:        s.getCurrentBalance(),
		OrderSn:           s.orderSn.OrderSN,
		ParentOrderSn:     s.orderSn.ParentOrderSN,
		FreeOrderSn:       s.orderSn.FreeOrderSN,
		State:             1,
		BonusTimes:        0,
		HuNum:             0,
		FreeNum:           int64(s.client.ClientOfFreeGame.GetFreeNum()),
		FreeTimes:         int64(s.client.ClientOfFreeGame.GetFreeTimes()),
		CreatedAt:         time.Now().Unix(),
		IsFree:            isFree,
	}
	return s.fillInGameOrderDetails()
}

// 结算step
func (s *betOrderService) settleStep() error { //
	return gamelogic.SaveTransfer(&gamelogic.SaveTransferParam{
		Client:      s.client,
		GameOrder:   s.gameOrder,
		MerchantOne: s.merchant,
		MemberOne:   s.member,
		Ip:          s.req.Ip,
	}).Err
}

func (s *betOrderService) findWinInfo(board int64Grid) {
	//if s.scene.Stage == _spinTypeFree {
	//	return
	//}
	var infos []winInfo
	for lineNo, lines := range s.gameConfig.Lines {
		var symbol, count int64
		var loc = make([]int64, 0, 3)
		locMap := int64Grid{}
		for _, p := range lines {
			r := p / _colCount
			c := p % _colCount
			if symbol == 0 {
				symbol = board[r][c]
				//if symbol == _money || symbol > _prize || symbol == _blank || symbol == _lock {
				//	break
				//}
				if symbol == _blank {
					break
				}
				count++
				loc = append(loc, int64(p))
			} else if symbol == _wild || symbol == board[r][c] || board[r][c] == _wild {
				if symbol == _wild {
					/*	if board[r][c] > _prize {
						break
					}*/
					symbol = board[r][c]
				}
				// 如果当前位置是百搭且已确定的symbol不是百搭，则百搭替代为symbol
				if board[r][c] == _wild && symbol != _wild {
					// 百搭替代为当前连线的符号，保持symbol不变
				}
				count++
				loc = append(loc, int64(p))
			} else {
				break
			}
		}
		if count >= 3 {
			for _, l := range loc {
				locMap[l/int64(_colCount)][l%int64(_colCount)] = 1
			}
			index := int(count) - 1
			tmp := winInfo{
				Symbol:      symbol,
				SymbolCount: count,
				LineCount:   1,
				Odds:        s.gameConfig.PayTable[symbol-1][index],
				LineNo:      lineNo + 1,
				Positions:   locMap,
			}
			infos = append(infos, tmp)
			//if debugLogging {
			//	fmt.Println(fmt.Sprintf("中奖图标：%d，图标数量：%d，图标倍率：%4d，线序：%d，线路：%v", symbol, count, tmp.Odds, lineNo+1, lines))
			//}
		}
	}
	s.winInfos = infos
}

// 处理中奖信息
func (s *betOrderService) processWinInfos() {
	var winResults []*winResult
	var winGrid int64Grid
	lineMul := int64(0)
	for _, info := range s.winInfos {
		// 使用 findWinInfo 中已计算好的 Odds，不要重新从 PayTable 取
		baseLineMul := info.Odds
		totalMul := baseLineMul * info.LineCount
		winResults = append(winResults, &winResult{
			Symbol:             info.Symbol,
			SymbolCount:        info.SymbolCount,
			LineCount:          info.LineCount,
			BaseLineMultiplier: baseLineMul,
			TotalMultiplier:    totalMul,
			Position:           info.Positions,
			LineNo:             int64(info.LineNo),
		})
		for r, cols := range info.Positions {
			for c, v := range cols {
				if v == 1 {
					winGrid[r][c] = s.symbolGrid[r][c]
				}
			}
		}
		lineMul += totalMul
	}
	/*	s.moneySymbolMul = 0
		if s.moneySymbolNum >= 5 {
			for i := 0; i < _rowCount; i++ {
				for j := 0; j < _colCount; j++ {
					if s.symbolGrid[i][j] > _prize {
						s.moneySymbolMul += s.symbolGrid[i][j] - _prize
						winGrid[i][j] = s.symbolGrid[i][j] - _prize
						s.moneyGrid[i][j] = s.symbolGrid[i][j] - _prize
					}
				}
			}
		}*/
	// 检查是否全百搭，触发最大中奖倍数（取最大值，不覆盖其他中奖）
	if s.isAllWild() {
		if int64(s.gameConfig.MaxPayMultiple) > lineMul {
			lineMul = int64(s.gameConfig.MaxPayMultiple)
		}
	}

	s.lineMultiplier = lineMul
	s.stepMultiplier = s.lineMultiplier // + s.moneySymbolMul
	s.winResults = winResults
	s.winGrid = winGrid

	s.scene.RoundMultiplier = s.stepMultiplier

	// 注意：Re-spin模式的NextStage逻辑已在updateStepResult中处理
	// 这里不需要重复设置
}

// 获取当前余额
func (s *betOrderService) getCurrentBalance() float64 {
	currBalance := decimal.NewFromFloat(s.member.Balance).
		Sub(s.amount).
		Add(s.bonusAmount).
		Round(2).
		InexactFloat64()
	return currBalance
}

// 填充订单细节
func (s *betOrderService) fillInGameOrderDetails() (bool, error) { // 932
	betRawDetail, err := json.CJSON.MarshalToString(s.symbolGrid)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return false, err
	}
	s.gameOrder.BetRawDetail = betRawDetail
	winRawDetail, err := json.CJSON.MarshalToString(s.winGrid)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return false, err
	}
	s.gameOrder.BonusRawDetail = winRawDetail
	s.gameOrder.BetDetail = s.symbolGridToString()
	s.gameOrder.BonusDetail = s.winGridToString()
	winDetails, err := json.CJSON.MarshalToString(s.getWinDetailsMap())
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return false, err
	}
	s.gameOrder.WinDetails = winDetails
	return true, nil
}

// 获取中奖详情
func (s *betOrderService) getWinDetailsMap() any {
	s.winArr = make([]*pb.Jqs_WinArr, len(s.winResults))
	for i, row := range s.winResults {
		s.winArr[i] = &pb.Jqs_WinArr{
			Val:     &row.Symbol,
			RoadNum: &row.LineCount,
			StarNum: &row.SymbolCount,
			Odds:    &row.BaseLineMultiplier,
			Mul:     &row.TotalMultiplier,
			LineNo:  &row.LineNo,
			Grid:    s.int64GridToPbBoard(row.Position),
		}
	}

	return &WinDetails{
		FreeWin:  s.client.ClientOfFreeGame.FreeTotalMoney,
		TotalWin: s.client.ClientOfFreeGame.GeneralWinTotal,
		State:    s.state,
		WinArr:   s.winArr,
		//MoneyMul: s.moneySymbolMul,
		//WinGrid: s.int64GridToPbBoard(s.moneyGrid),
		WinGrid: s.int64GridToPbBoard(s.winGrid),
	}
}

// 获取中奖详情
func (s *betOrderService) replayByOrder(req *request.BetOrderReq, gameOrder *game.GameOrder) (*game_replay.InternalResponse, error) {
	winDetail := new(WinDetails)
	if err := json.CJSON.UnmarshalFromString(gameOrder.WinDetails, winDetail); err != nil {
		return nil, errors.New(fmt.Sprintf("jqs replayByOrder winDetail UnmarshalFromString err:%v", err))
	}

	symbolGrid := int64Grid{}
	if err := json.CJSON.UnmarshalFromString(gameOrder.BetRawDetail, &symbolGrid); err != nil {
		return nil, errors.New(fmt.Sprintf("jqs replayByOrder symbolGrid UnmarshalFromString err:%v", err))
	}

	freeNum := gameOrder.FreeNum - gameOrder.FreeTimes
	next := freeNum > 0

	review := int64(0)
	pbData, jsonData, err := s.MarshalData(&pb.Jqs_BetOrderResponse{
		OrderSN:    &gameOrder.OrderSn,
		Balance:    &gameOrder.CurBalance,
		BetAmount:  &gameOrder.Amount,
		CurrentWin: &gameOrder.BonusAmount,
		FreeWin:    &winDetail.FreeWin,
		TotalWin:   &winDetail.TotalWin,
		Free:       gameOrder.IsFree == 1,
		Review:     &review,
		WinInfo: &pb.Jqs_WinInfo{
			Next:     next,
			Multi:    &gameOrder.BonusMultiple,
			State:    &winDetail.State,
			FreeNum:  &freeNum,
			FreeTime: &gameOrder.FreeTimes,
			WinArr:   winDetail.WinArr,
			//MoneyMul: &winDetail.MoneyMul,
			WinGird: winDetail.WinGrid,
		},
		Cards: s.int64GridToPbBoard(symbolGrid),
	})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("jqs replayByOrder MarshalData err:%v", err))
	}

	return game_replay.NewPbInternalResponse(jsonData, pbData), nil
}

// 检查是否九个位置全部为百搭
func (s *betOrderService) isAllWild() bool {
	for row := 0; row < _rowCount; row++ {
		for col := 0; col < _colCount; col++ {
			if s.symbolGrid[row][col] != _wild {
				return false
			}
		}
	}
	return true
}
