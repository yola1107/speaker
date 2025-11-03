package xxg2

import (
	"strconv"
	"time"

	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/utils/json"
	"egame-grpc/utils/snow"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// 初始化
func (s *betOrderService) initialize() error {
	s.client.ClientOfFreeGame.ResetFreeClean()

	// RTP 测试时跳过订单号生成
	if !s.debug.open {
		s.orderSN = strconv.FormatInt(snow.GenarotorID(s.member.ID), 10)
	}

	// 首次spin或基础模式
	if s.scene == nil || s.scene.Stage == _spinTypeBase || s.scene.Stage == 0 {
		if err := s.initFirstStepForSpin(); err != nil {
			return err
		}
	} else {
		// 免费模式后续spin
		s.initStepForNextStep()
	}

	return nil
}

// 初始化首次step
func (s *betOrderService) initFirstStepForSpin() error {
	if s.debug.open {
		// RTP测试：最小化初始化
		s.betAmount = decimal.NewFromInt(s.gameConfig.BaseBat)
		s.amount = s.betAmount
		return nil
	}

	// 正常游戏流程
	if !s.updateBetAmount() {
		return InvalidRequestParams
	}
	if !s.checkBalance() {
		return InsufficientBalance
	}

	s.resetClientState()
	s.amount = s.betAmount
	s.client.ClientOfFreeGame.SetBetAmount(s.betAmount.Round(2).InexactFloat64())
	s.client.ClientOfFreeGame.SetLastWinId(uint64(time.Now().UnixNano()))

	return nil
}

// resetClientState 重置客户端状态
func (s *betOrderService) resetClientState() {
	s.client.SetLastMaxFreeNum(0)
	s.client.ClientOfFreeGame.Reset()
	s.client.ClientOfFreeGame.ResetGeneralWinTotal()
	s.client.ClientOfFreeGame.ResetRoundBonus()
	s.client.ClientOfFreeGame.ResetRoundBonusStaging()
}

// 更新订单（参考 mahjong，使用 scene 中的倍数字段）
func (s *betOrderService) updateGameOrder() bool {
	gameOrder := game.GameOrder{
		MerchantID:        s.merchant.ID,
		Merchant:          s.merchant.Merchant,
		MemberID:          s.member.ID,
		Member:            s.member.MemberName,
		GameID:            s.game.ID,
		GameName:          s.game.GameName,
		BaseMultiple:      s.gameConfig.BaseBat,
		Multiple:          s.req.Multiple,
		LineMultiple:      s.lineMultiplier,
		BonusHeadMultiple: 0,                // xxg2 无消除，固定为0
		BonusMultiple:     s.stepMultiplier, // xxg2 无消除，直接用 stepMultiplier
		BaseAmount:        s.req.BaseMoney,
		Amount:            s.amount.Round(2).InexactFloat64(),
		ValidAmount:       s.amount.Round(2).InexactFloat64(),
		BonusAmount:       s.bonusAmount.Round(2).InexactFloat64(),
		CurBalance:        s.getCurrentBalance(),
		OrderSn:           s.orderSN,
		ParentOrderSn:     s.parentOrderSN,
		FreeOrderSn:       s.freeOrderSN,
		State:             1,
		BonusTimes:        0,                    // xxg2 无消除，固定为0
		HuNum:             s.stepMap.TreatCount, // 夺宝符号数量
		FreeNum:           int64(s.client.ClientOfFreeGame.GetFreeNum()),
		FreeTimes:         int64(s.client.ClientOfFreeGame.GetFreeTimes()),
	}
	if s.isFreeRound() {
		gameOrder.IsFree = 1
	}
	s.gameOrder = &gameOrder
	return s.fillInGameOrderDetails()
}

// settleStep 结算step
func (s *betOrderService) settleStep() bool {
	s.gameOrder.CreatedAt = time.Now().Unix()

	saveParam := gamelogic.SaveTransfer(&gamelogic.SaveTransferParam{
		Client:      s.client,
		GameOrder:   s.gameOrder,
		MerchantOne: s.merchant,
		MemberOne:   s.member,
		Ip:          s.req.Ip,
	})

	return saveParam.Err == nil
}

// findWinInfos 查找中奖信息（Ways玩法：从左到右连续匹配）
func (s *betOrderService) findWinInfos() {
	infos := make([]*winInfo, 0, 10)

	// 第一列有wild时，检查盘面所有符号；否则使用默认符号列表
	checkSymbols := checkWinSymbols
	if hasWild, symbols := s.checkWildInFirstCol(); hasWild {
		checkSymbols = symbols
	}

	for _, symbol := range checkSymbols {
		var info *winInfo
		var ok bool
		if symbol == _wild {
			info, ok = s.findWildWinInfo(_wild)
		} else {
			info, ok = s.findSymbolWinInfo(symbol)
		}
		if ok {
			infos = append(infos, info)
		}
	}

	s.winInfos = infos
}

// checkWildInFirstCol 检查第一列是否有wild，若有则返回盘面所有不重复符号
func (s *betOrderService) checkWildInFirstCol() (bool, []int64) {
	hasWild := false
	for row := int64(0); row < _rowCount; row++ {
		if s.symbolGrid[row][0] == _wild {
			hasWild = true
			break
		}
	}
	if !hasWild {
		return false, nil
	}

	// 收集盘面所有不重复符号（排除treasure）
	symbolSet := make(map[int64]struct{})
	symbols := make([]int64, 0, len(checkWinSymbols))
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			symbol := s.symbolGrid[row][col]
			if symbol != _treasure {
				if _, exists := symbolSet[symbol]; !exists {
					symbolSet[symbol] = struct{}{}
					symbols = append(symbols, symbol)
				}
			}
		}
	}
	return true, symbols
}

// processWinInfos 处理中奖信息（计算倍率和构建中奖网格）
func (s *betOrderService) processWinInfos() {
	winResults := make([]*winResult, 0, len(s.winInfos))
	var winGrid int64Grid
	totalMultiplier := int64(0)

	for _, info := range s.winInfos {
		baseMultiplier := s.getSymbolMultiplier(info.Symbol, info.SymbolCount)
		lineMultiplier := baseMultiplier * info.LineCount

		winResults = append(winResults, &winResult{
			Symbol:             info.Symbol,
			SymbolCount:        info.SymbolCount,
			LineCount:          info.LineCount,
			BaseLineMultiplier: baseMultiplier,
			TotalMultiplier:    lineMultiplier,
			WinPositions:       s.buildWinPositions(info, &winGrid),
		})

		totalMultiplier += lineMultiplier
	}

	s.lineMultiplier = totalMultiplier
	s.stepMultiplier = totalMultiplier
	s.winResults = winResults
	s.winGrid = &winGrid
}

// buildWinPositions 构建中奖位置网格
func (s *betOrderService) buildWinPositions(info *winInfo, winGrid *int64Grid) int64Grid {
	var winPositions int64Grid

	for _, pos := range info.Positions {
		currSymbol := s.symbolGrid[pos.Row][pos.Col]
		if currSymbol == info.Symbol || currSymbol == _wild {
			winPositions[pos.Row][pos.Col] = 1
			winGrid[pos.Row][pos.Col] = currSymbol
		}
	}

	return winPositions
}

// 获取当前余额
func (s *betOrderService) getCurrentBalance() float64 {
	return decimal.NewFromFloat(s.member.Balance).
		Sub(s.amount).
		Add(s.bonusAmount).
		Round(2).
		InexactFloat64()
}

// fillInGameOrderDetails 填充订单细节
func (s *betOrderService) fillInGameOrderDetails() bool {
	var err error

	// 序列化符号网格
	if s.gameOrder.BetRawDetail, err = json.CJSON.MarshalToString(s.symbolGrid); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal symbolGrid", zap.Error(err))
		return false
	}

	// 序列化中奖网格
	if s.gameOrder.BonusRawDetail, err = json.CJSON.MarshalToString(s.winGrid); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal winGrid", zap.Error(err))
		return false
	}

	// 转换为字符串格式
	s.gameOrder.BetDetail = s.symbolGridToString()
	s.gameOrder.BonusDetail = s.winGridToString()

	// 序列化中奖详情
	if s.gameOrder.WinDetails, err = json.CJSON.MarshalToString(s.getWinDetailsMap()); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal winDetails", zap.Error(err))
		return false
	}

	return true
}

// findSymbolWinInfo 查找符号中奖（Ways玩法：从左到右连续，至少3列，Wild可替代）
func (s *betOrderService) findSymbolWinInfo(symbol int64) (*winInfo, bool) {
	lineCount := int64(1)
	positions := make([]*position, 0, 16)
	hasRealSymbol := false

	for col := int64(0); col < _colCount; col++ {
		count := int64(0)
		for row := int64(0); row < _rowCount; row++ {
			currSymbol := s.symbolGrid[row][col]
			if currSymbol == symbol || currSymbol == _wild {
				count++
				if currSymbol == symbol {
					hasRealSymbol = true
				}
				positions = append(positions, &position{Row: row, Col: col})
			}
		}

		if count == 0 {
			if col >= _minMatchCount && hasRealSymbol {
				return &winInfo{Symbol: symbol, SymbolCount: col, LineCount: lineCount, Positions: positions}, true
			}
			return nil, false
		}

		lineCount *= count

		if col == _colCount-1 && hasRealSymbol {
			return &winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, Positions: positions}, true
		}
	}

	return nil, false
}

// findWildWinInfo 查找Wild中奖（纯Wild连线）
func (s *betOrderService) findWildWinInfo(symbol int64) (*winInfo, bool) {
	lineCount := int64(1)
	positions := make([]*position, 0, 16)

	for col := int64(0); col < _colCount; col++ {
		count := int64(0)
		for row := int64(0); row < _rowCount; row++ {
			if s.symbolGrid[row][col] == _wild {
				count++
				positions = append(positions, &position{Row: row, Col: col})
			}
		}

		if count == 0 {
			if col >= _minMatchCount {
				return &winInfo{Symbol: symbol, SymbolCount: col, LineCount: lineCount, Positions: positions}, true
			}
			return nil, false
		}

		lineCount *= count

		if col == _colCount-1 {
			return &winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, Positions: positions}, true
		}
	}

	return nil, false
}

// getWinDetailsMap 获取中奖详情（用于数据库存储，使用原始数据）
func (s *betOrderService) getWinDetailsMap() map[string]any {
	return map[string]any{
		"orderSN":            s.gameOrder.OrderSn,
		"symbolGrid":         s.symbolGrid,
		"treasureCount":      s.stepMap.TreatCount,
		"winGrid":            s.winGrid,
		"winResults":         s.winResults,
		"baseBet":            s.req.BaseMoney,
		"multiplier":         s.req.Multiple,
		"betAmount":          s.betAmount.Round(2).InexactFloat64(),
		"bonusAmount":        s.bonusAmount.Round(2).InexactFloat64(),
		"spinBonusAmount":    s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		"freeBonusAmount":    s.client.ClientOfFreeGame.GetFreeTotalMoney(),
		"roundBonus":         s.client.ClientOfFreeGame.RoundBonus,
		"isFree":             s.isFreeRound(),
		"newFreeCount":       s.newFreeCount,
		"totalFreeCount":     s.client.GetLastMaxFreeNum(),
		"remainingFreeCount": s.client.ClientOfFreeGame.GetFreeNum(),
		"lineMultiplier":     s.lineMultiplier,
		"stepMultiplier":     s.stepMultiplier,
	}
}

// getSymbolMultiplier 从PayTable获取符号倍率
func (s *betOrderService) getSymbolMultiplier(symbol int64, symbolCount int64) int64 {
	if len(s.gameConfig.PayTable) < int(symbol) {
		return 0
	}
	table := s.gameConfig.PayTable[symbol-1]
	index := int(symbolCount - _minMatchCount)
	if index < 0 || index >= len(table) {
		return 0
	}
	return table[index]
}
