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

// ========== 辅助函数 ==========

// getRequestContext 获取请求上下文
func (s *betOrderService) getRequestContext() bool {
	return s.mdbGetMerchant() && s.mdbGetMember() && s.mdbGetGame()
}

// updateBetAmount 更新下注金额
func (s *betOrderService) updateBetAmount() bool {
	// 校验参数
	if !_cnf.validateBetSize(s.req.BaseMoney) {
		global.GVA_LOG.Warn("invalid baseMoney", zap.Float64("value", s.req.BaseMoney))
		return false
	}
	if !_cnf.validateBetLevel(s.req.Multiple) {
		global.GVA_LOG.Warn("invalid multiple", zap.Int64("value", s.req.Multiple))
		return false
	}

	// 计算下注金额
	s.betAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_cnf.BaseBat))

	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("invalid betAmount", zap.String("amount", s.betAmount.String()))
		return false
	}
	return true
}

// checkBalance 检查用户余额
func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

// updateBonusAmount 更新奖金金额
func (s *betOrderService) updateBonusAmount() {
	s.bonusAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(s.stepMultiplier))
}

// ========== 初始化逻辑 ==========

// 初始化
func (s *betOrderService) initialize() error {
	s.client.ClientOfFreeGame.ResetFreeClean()

	// RTP 测试时跳过订单号生成
	if !s.debug.open {
		s.orderSN = strconv.FormatInt(snow.GenarotorID(s.member.ID), 10)
	}

	// 首次spin或基础模式
	if s.scene == nil || s.scene.Stage == _spinTypeBase || s.scene.Stage == 0 {
		return s.initFirstStepForSpin()
	}

	// 免费模式后续spin
	s.initStepForNextStep()
	return nil
}

// 初始化首次step
func (s *betOrderService) initFirstStepForSpin() error {
	if s.debug.open {
		// RTP测试：最小化初始化
		s.betAmount = decimal.NewFromInt(_cnf.BaseBat)
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

// initStepForNextStep 初始化后续step（免费游戏的连续spin）
func (s *betOrderService) initStepForNextStep() {
	if s.debug.open {
		return
	}

	s.req.BaseMoney = s.lastOrder.BaseAmount
	s.req.Multiple = s.lastOrder.Multiple
	s.betAmount = decimal.NewFromFloat(s.client.ClientOfFreeGame.GetBetAmount())
	s.amount = decimal.Zero

	// 选择父订单号
	if s.lastOrder.ParentOrderSn != "" {
		s.parentOrderSN = s.lastOrder.ParentOrderSn
	} else {
		s.parentOrderSN = s.lastOrder.OrderSn
	}

	// 选择免费订单号
	switch {
	case s.lastOrder.FreeOrderSn != "":
		s.freeOrderSN = s.lastOrder.FreeOrderSn
	case s.lastOrder.ParentOrderSn != "":
		s.freeOrderSN = s.lastOrder.ParentOrderSn
	default:
		s.freeOrderSN = s.lastOrder.OrderSn
	}
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
		BaseMultiple:      _cnf.BaseBat,
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
	res := gamelogic.SaveTransfer(&gamelogic.SaveTransferParam{
		Client:      s.client,
		GameOrder:   s.gameOrder,
		MerchantOne: s.merchant,
		MemberOne:   s.member,
		Ip:          s.req.Ip,
	})
	return res.Err == nil
}

// findWinInfos 查找中奖信息（Ways玩法：从左到右连续匹配）
func (s *betOrderService) findWinInfos() {
	// 第一列有wild时，检查盘面所有符号；否则使用默认符号列表
	checkSymbols := checkWinSymbols
	if hasWild, symbols := s.checkWildInFirstCol(); hasWild {
		checkSymbols = symbols
	}

	infos := make([]*winInfo, 0, len(checkSymbols))
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
	// 检查第一列是否有wild
	for row := int64(0); row < _rowCount; row++ {
		if s.symbolGrid[row][0] == _wild {
			// 收集盘面不重复符号
			seen := make(map[int64]bool, 10)
			symbols := make([]int64, 0, 10)
			for r := int64(0); r < _rowCount; r++ {
				for c := int64(0); c < _colCount; c++ {
					sym := s.symbolGrid[r][c]
					if sym != _treasure && !seen[sym] {
						seen[sym] = true
						symbols = append(symbols, sym)
					}
				}
			}
			return true, symbols
		}
	}
	return false, nil
}

// processWinInfos 处理中奖信息并计算倍率
func (s *betOrderService) processWinInfos() {
	if len(s.winInfos) == 0 {
		s.winResults = nil
		s.winGrid = &int64Grid{}
		s.lineMultiplier = 0
		s.stepMultiplier = 0
		return
	}

	winResults := make([]*winResult, 0, len(s.winInfos))
	var winGrid int64Grid
	totalMultiplier := int64(0)

	for _, info := range s.winInfos {
		baseMultiplier := _cnf.getSymbolMultiplier(info.Symbol, info.SymbolCount)
		totalLine := baseMultiplier * info.LineCount

		winResults = append(winResults, &winResult{
			Symbol:             info.Symbol,
			SymbolCount:        info.SymbolCount,
			LineCount:          info.LineCount,
			BaseLineMultiplier: baseMultiplier,
			TotalMultiplier:    totalLine,
			WinPositions:       s.buildWinPositions(info, &winGrid),
		})

		totalMultiplier += totalLine
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

// getCurrentBalance 获取当前余额
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
	s.gameOrder.BetDetail = gridToString(s.symbolGrid)
	s.gameOrder.BonusDetail = gridToString(s.winGrid)

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
