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
	if !s.forRtpBench {
		s.orderSN = strconv.FormatInt(snow.GenarotorID(s.member.ID), 10)
	}

	// 首次spin或基础模式
	if s.scene == nil || s.scene.Stage == _spinTypeBase || s.scene.Stage == 0 {
		if err := s.initFirstStepForSpin(); err != nil {
			return err
		}
		s.isSpinFirstRound = false
		s.scene.SpinFirstRound = 1 // 处理场景中间数据（标记非首次）
	} else {
		// 免费模式后续spin
		s.initStepForNextStep()
	}

	return nil
}

// 初始化首次step
func (s *betOrderService) initFirstStepForSpin() error {
	// 验证下注金额和余额
	if !s.forRtpBench {
		if !s.updateBetAmount() {
			return InvalidRequestParams
		}
		if !s.checkBalance() {
			return InsufficientBalance
		}
	}

	s.resetClientState()

	// 优化：RTP测试时跳过不必要的操作
	if !s.forRtpBench {
		s.client.ClientOfFreeGame.SetBetAmount(s.betAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.SetLastWinId(uint64(time.Now().UnixNano()))
	}
	s.amount = s.betAmount

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
		BaseMultiple:      _baseMultiplier,
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

// 结算step
func (s *betOrderService) settleStep() bool {
	s.gameOrder.CreatedAt = time.Now().Unix()

	saveParam := &gamelogic.SaveTransferParam{
		Client:      s.client,
		GameOrder:   s.gameOrder,
		MerchantOne: s.merchant,
		MemberOne:   s.member,
		Ip:          s.req.Ip,
	}

	res := gamelogic.SaveTransfer(saveParam)
	if res.Err != nil {
		return false
	}
	return true
}

// 查找中奖信息（Ways玩法：从左到右连续匹配）
func (s *betOrderService) findWinInfos() {
	// 优化：预分配切片容量，减少内存分配
	infos := make([]*winInfo, 0, 10)

	// 如果第一列有wild，使用盘面去重后的符号列表；否则使用默认checkWin
	checkSymbols := checkWin
	if hasWild, symbols := s.checkWindInFirstCol(); hasWild {
		checkSymbols = symbols
	}

	// 遍历符号列表查找中奖
	for _, symbol := range checkSymbols {
		var info *winInfo
		var ok bool

		// 百搭符号(Wild)使用独立判断逻辑
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

// 统计第一列是否出现wild，如果出现wild 获取盘面所有的symbols (去掉重复出现)
func (s *betOrderService) checkWindInFirstCol() (bool, []int64) {
	// 检查第一列（col=0）是否有wild符号（10）
	hasWild := false
	for row := int64(0); row < _rowCount; row++ {
		symbol := s.symbolGrid[row][0]
		if symbol == _wild {
			hasWild = true
			break
		}
	}

	// 如果第一列没有wild，返回false
	if !hasWild {
		return false, nil
	}

	// 收集盘面所有不重复的符号（一次遍历完成）
	symbolSet := make(map[int64]struct{})
	symbols := make([]int64, 0, _treasure)
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			symbol := s.symbolGrid[row][col]
			if symbol == _treasure {
				continue
			}
			// 只在第一次遇到时添加
			if _, exists := symbolSet[symbol]; !exists {
				symbolSet[symbol] = struct{}{}
				symbols = append(symbols, symbol)
			}
		}
	}

	return true, symbols
}

// 处理中奖信息（计算倍率和构建中奖网格）
func (s *betOrderService) processWinInfos() {
	// 优化：预分配切片容量
	winResults := make([]*winResult, 0, len(s.winInfos))
	var winGrid int64Grid
	totalLineMultiplier := int64(0)

	// 遍历所有中奖信息
	for _, info := range s.winInfos {
		// 1. 从PayTable获取基础倍率（根据符号和匹配列数）
		baseMultiplier := s.getSymbolMultiplier(info.Symbol, info.SymbolCount)

		// 2. 计算总倍率 = 基础倍率 × Ways倍数
		totalMultiplier := baseMultiplier * info.LineCount

		// 3. 构建中奖位置网格（标记哪些位置中奖）
		winPositions := s.buildWinPositions(info, &winGrid)

		// 4. 构建中奖结果
		winResults = append(winResults, &winResult{
			Symbol:             info.Symbol,
			SymbolCount:        info.SymbolCount,
			LineCount:          info.LineCount,
			BaseLineMultiplier: baseMultiplier,
			TotalMultiplier:    totalMultiplier,
			WinPositions:       winPositions,
		})

		// 5. 累加总倍率
		totalLineMultiplier += totalMultiplier
	}

	// 保存结果
	s.lineMultiplier = totalLineMultiplier
	s.stepMultiplier = totalLineMultiplier
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

// 填充订单细节
func (s *betOrderService) fillInGameOrderDetails() bool {
	// 序列化符号网格
	betRawDetail, err := json.CJSON.MarshalToString(s.symbolGrid)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal symbolGrid", zap.Error(err))
		return false
	}
	s.gameOrder.BetRawDetail = betRawDetail

	// 序列化中奖网格
	winRawDetail, err := json.CJSON.MarshalToString(s.winGrid)
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal winGrid", zap.Error(err))
		return false
	}
	s.gameOrder.BonusRawDetail = winRawDetail

	// 转换为字符串格式
	s.gameOrder.BetDetail = s.symbolGridToString()
	s.gameOrder.BonusDetail = s.winGridToString()

	// 序列化中奖详情
	winDetails, err := json.CJSON.MarshalToString(s.getWinDetailsMap())
	if err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal winDetails", zap.Error(err))
		return false
	}
	s.gameOrder.WinDetails = winDetails

	return true
}

// 获取符号中奖信息（Ways玩法）
// 规则：从左到右连续匹配，至少3列，百搭可替代
func (s *betOrderService) findSymbolWinInfo(symbol int64) (*winInfo, bool) {
	lineCount := int64(1) // Ways倍数初始值
	var positions []*position
	hasRealSymbol := false // 是否有真实符号（非百搭）

	// 从左到右遍历每一列
	for col := int64(0); col < _colCount; col++ {
		count := int64(0)

		// 统计该列中匹配的符号数量（符号本身 或 百搭）
		for row := int64(0); row < _rowCount; row++ {
			currSymbol := s.symbolGrid[row][col]

			// 匹配条件：当前符号 == 目标符号 OR 当前符号 == 百搭
			if currSymbol == symbol || currSymbol == _wild {
				count++
				if currSymbol == symbol {
					hasRealSymbol = true // 标记有真实符号
				}
				positions = append(positions, &position{Row: row, Col: col})
			}
		}

		// 该列无匹配符号 → 中断
		if count == 0 {
			// 检查是否满足中奖条件（至少3列 且 有真实符号）
			if col >= _minMatchCount && hasRealSymbol {
				return &winInfo{Symbol: symbol, SymbolCount: col, LineCount: lineCount, Positions: positions}, true
			}
			return nil, false
		}

		// Ways倍数累乘：每列的匹配数量
		lineCount *= count

		// 到达最后一列 → 全屏中奖
		if col == _colCount-1 && hasRealSymbol {
			return &winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, Positions: positions}, true
		}
	}

	return nil, false
}

// 获取百搭符号中奖信息（纯百搭连线）
// 规则：只统计纯百搭符号，不与其他符号混合
func (s *betOrderService) findWildWinInfo(symbol int64) (*winInfo, bool) {
	lineCount := int64(1) // Ways倍数初始值
	var positions []*position

	// 从左到右遍历每一列
	for col := int64(0); col < _colCount; col++ {
		count := int64(0)

		// 统计该列中百搭符号数量
		for row := int64(0); row < _rowCount; row++ {
			if s.symbolGrid[row][col] == _wild {
				count++
				positions = append(positions, &position{Row: row, Col: col})
			}
		}

		// 该列无百搭符号 → 中断
		if count == 0 {
			// 检查是否满足中奖条件（至少3列）
			if col >= _minMatchCount {
				return &winInfo{Symbol: symbol, SymbolCount: col, LineCount: lineCount, Positions: positions}, true
			}
			return nil, false
		}

		// Ways倍数累乘
		lineCount *= count

		// 到达最后一列 → 全屏百搭
		if col == _colCount-1 {
			return &winInfo{Symbol: symbol, SymbolCount: _colCount, LineCount: lineCount, Positions: positions}, true
		}
	}

	return nil, false
}

// 获取中奖详情
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

// getSymbolMultiplier 读取符号的倍率（参考 mahjong 的 getSymbolBaseMultiplier）
func (s *betOrderService) getSymbolMultiplier(symbol int64, symbolCount int64) int64 {
	if len(s.gameConfig.PayTable) < int(symbol) {
		return 0
	}
	table := s.gameConfig.PayTable[symbol-1]
	// symbolCount 是匹配的列数，需要转换为索引（3列对应索引0）
	index := int(symbolCount - _minMatchCount)
	if index < 0 || index >= len(table) {
		return 0
	}
	return table[index]
}
