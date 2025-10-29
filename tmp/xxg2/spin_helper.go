package xxg2

import (
	"fmt"
	"math/rand/v2"
	"strconv"

	"egame-grpc/gamelogic"
	"egame-grpc/global"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// getRequestContext 获取请求上下文
func (s *betOrderService) getRequestContext() bool {
	return s.mdbGetMerchant() && s.mdbGetMember() && s.mdbGetGame()
}

// selectGameRedis 初始化游戏redis
func (s *betOrderService) selectGameRedis() {
	if s.forRtpBench {
		return
	}
	if len(global.GVA_GAME_REDIS) == 0 {
		return
	}
	index := _gameID % int64(len(global.GVA_GAME_REDIS))
	s.gameRedis = global.GVA_GAME_REDIS[index]
}

// updateBetAmount 更新下注金额
func (s *betOrderService) updateBetAmount() bool {
	s.betAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_baseMultiplier))

	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
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

// symbolGridToString 符号网格转换为字符串
func (s *betOrderService) symbolGridToString() string {
	var result string
	sn := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			result += strconv.Itoa(sn) + ":" + strconv.FormatInt(s.symbolGrid[row][col], 10) + "; "
			sn++
		}
	}
	return result
}

// winGridToString 中奖网格转换为字符串
func (s *betOrderService) winGridToString() string {
	if s.winGrid == nil {
		return ""
	}
	var result string
	sn := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			result += strconv.Itoa(sn) + ":" + strconv.FormatInt(s.winGrid[row][col], 10) + "; "
			sn++
		}
	}
	return result
}

// showPostUpdateErrorLog 错误日志记录
func (s *betOrderService) showPostUpdateErrorLog() {
	global.GVA_LOG.Error(
		"showPostUpdateErrorLog",
		zap.Error(fmt.Errorf("step state mismatch")),
		zap.Int64("id", s.member.ID),
		zap.Bool("isFree", s.isFree),
		zap.Uint64("lastWinID", s.client.ClientOfFreeGame.GetLastWinId()),
		zap.Uint64("lastMapID", s.client.ClientOfFreeGame.GetLastMapId()),
		zap.Uint64("freeNum", s.client.ClientOfFreeGame.GetFreeNum()),
		zap.Uint64("freeTimes", s.client.ClientOfFreeGame.GetFreeTimes()),
		zap.String("orderSn", s.orderSN),
		zap.String("parentOrderSN", s.parentOrderSN),
		zap.String("freeOrderSN", s.freeOrderSN),
	)
}

// countTreasureSymbols 计算夺宝符号数量和位置
func (s *betOrderService) countTreasureSymbols() {
	positions := s.scanSymbolPositions(isTreasure)
	s.treasureCount, s.treasurePositions = int64(len(positions)), positions
	s.stepMap.TreatCount = s.treasureCount // 更新step Map
}

// scanSymbolPositions 扫描符合条件的符号位置
func (s *betOrderService) scanSymbolPositions(matchFunc func(int64) bool) []*position {
	var positions []*position

	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			if matchFunc(s.symbolGrid[row][col]) {
				positions = append(positions, &position{Row: row, Col: col})
			}
		}
	}

	return positions
}

// collectBat 收集蝙蝠移动和Wind转换信息
func (s *betOrderService) collectBat() {
	/*
		一、基础模式（Base Mode）
		触发条件：
		当前盘面中 _treasure（蝙蝠/夺宝符）数量S 满足 1 ≤ S ≤ 2。
		实现步骤：
		1>扫描当前盘面，统计所有属于【老人 / 小孩 / 女人】的符号位置，保存为 allPositions。
		2>从 allPositions 中随机选择 S 个位置（若数量不足 S，则全选）。
		3>将这些位置上的符号转换为 Wind（Wild）。


		二、免费模式（Free Mode）
		触发条件：
		当前处于免费游戏模式（Free Game）。

		实现步骤：
		1>扫描当前盘面，统计所有 _treasure（蝙蝠）符号的位置，保存为 batPositions。
		2>从 batPositions 中随机选择 S 个蝙蝠（若数量不足 S，则全选）。
		3>对每个被选中的蝙蝠执行以下操作：
		随机选择一个可行的方向（上、下、左、右）；
		将蝙蝠向该方向移动一格，得到新位置（若越界可忽略或重新随机）。
		收集所有移动后的小格子位置，检查这些格子上的符号：
		若符号属于【老人 / 小孩 / 女人】，则将其转换为 Wind（Wild）。
		记录这些移动后的小格子位置，作为下一次免费模式中蝙蝠的新起始点，使其能持续移动。

	*/

	treasureCount, treasurePositions := s.treasureCount, s.treasurePositions

	if s.isFree {
		s.stepMap.Bat = s.transformToWildFreeMode(treasurePositions)
	} else {
		s.stepMap.Bat = s.transformToWildBaseMode(treasureCount, treasurePositions)
	}
}

// transformToWildBaseMode 基础模式Wind转换（1-2个夺宝符时触发）
func (s *betOrderService) transformToWildBaseMode(treasureCount int64, treasurePositions []*position) []*Bat {
	if treasureCount < 1 || treasureCount > 2 {
		return nil
	}

	// 找到所有可转换为Wind符号位置 （小孩/女人/老人）
	allWindPositions := s.scanSymbolPositions(canTransformToWind)
	if len(allWindPositions) == 0 {
		return nil
	}

	// 随机选择S个Wind符号（若数量不足则全选）
	selectCount := min(int(treasureCount), len(allWindPositions))

	// 随机打乱并取前N个
	indices := rand.Perm(len(allWindPositions))[:selectCount]

	// 建立treasure→Wind映射并转换
	bats := make([]*Bat, 0, selectCount)
	for i, idx := range indices {
		windPos := allWindPositions[idx]
		treasurePos := treasurePositions[i%len(treasurePositions)]
		windSymbol := s.symbolGrid[windPos.Row][windPos.Col]

		bats = append(bats, createBat(treasurePos, windPos, windSymbol, _wild))
		s.symbolGrid[windPos.Row][windPos.Col] = _wild
	}

	return bats
}

// transformToWildFreeMode 免费模式Wind转换（蝙蝠持续移动）
func (s *betOrderService) transformToWildFreeMode(treasurePositions []*position) []*Bat {
	// 获取蝙蝠位置：有保存的用保存的（持续移动），否则用treasure位置（首次）
	batPositions := treasurePositions
	if len(s.scene.BatPositions) > 0 {
		batPositions = s.scene.BatPositions
	}

	if len(batPositions) == 0 {
		return nil
	}

	// 最多_maxBatPositions个格子进行移动
	if len(batPositions) >= _maxBatPositions {
		batPositions = batPositions[:_maxBatPositions] // TODO 可随机n个
	}

	bats := make([]*Bat, 0, len(batPositions))

	// 对每个蝙蝠执行移动和转换
	for _, batPos := range batPositions {
		// 蝙蝠随机方向移动一格
		newPos := moveBatOneStep(batPos)
		targetSymbol := s.symbolGrid[newPos.Row][newPos.Col]
		originalSymbol := s.symbolGrid[batPos.Row][batPos.Col]

		// 如果新位置是Wind符号，转换为Wild
		if canTransformToWind(targetSymbol) {
			s.symbolGrid[newPos.Row][newPos.Col] = _wild
			bats = append(bats, createBat(batPos, newPos, targetSymbol, _wild))
		} else {
			// 只记录移动
			bats = append(bats, createBat(batPos, newPos, originalSymbol, targetSymbol))
		}
	}

	return bats
}

// direction 方向结构
type direction struct {
	dRow int64
	dCol int64
}

var allDirections = []direction{
	{-1, 0}, {1, 0}, {0, -1}, {0, 1}, // 上、下、左、右
}

// isValidPosition 检查位置是否在边界内
func isValidPosition(row, col int64) bool {
	return row >= 0 && row < _rowCount && col >= 0 && col < _colCount
}

// moveBatOneStep 蝙蝠向随机可行方向移动一格
func moveBatOneStep(pos *position) *position {
	// 收集可行方向
	var validDirs []direction
	for _, dir := range allDirections {
		if isValidPosition(pos.Row+dir.dRow, pos.Col+dir.dCol) {
			validDirs = append(validDirs, dir)
		}
	}

	if len(validDirs) == 0 {
		return pos
	}

	// 随机选择一个方向并移动
	dir := validDirs[rand.IntN(len(validDirs))]
	return &position{Row: pos.Row + dir.dRow, Col: pos.Col + dir.dCol}
}

// createBat 创建Bat数据
func createBat(fromPos, toPos *position, oldSymbol, newSymbol int64) *Bat {
	return &Bat{
		X: fromPos.Row, Y: fromPos.Col,
		TransX: toPos.Row, TransY: toPos.Col,
		Syb: oldSymbol, Sybn: newSymbol,
	}
}

// calculateFreeTimes 计算触发的免费次数
func (s *betOrderService) calculateFreeTimes(treasureCount int64) int64 {
	if treasureCount < _triggerTreasureCount {
		return 0
	}

	extraScatters := treasureCount - _triggerTreasureCount
	return s.gameConfig.FreeGameInitTimes + (extraScatters * s.gameConfig.ExtraScatterExtraTime)
}

// calculateFreeAddTimes 计算免费游戏中追加的次数
func (s *betOrderService) calculateFreeAddTimes(treasureCount int64) int64 {
	if treasureCount == 0 {
		return 0
	}
	return treasureCount * s.gameConfig.ExtraScatterExtraTime
}
