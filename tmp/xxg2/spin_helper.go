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
		zap.Bool("isFree", s.isFreeRound()),
		zap.Uint64("lastWinID", s.client.ClientOfFreeGame.GetLastWinId()),
		zap.Uint64("lastMapID", s.client.ClientOfFreeGame.GetLastMapId()),
		zap.Uint64("freeNum", s.client.ClientOfFreeGame.GetFreeNum()),
		zap.Uint64("freeTimes", s.client.ClientOfFreeGame.GetFreeTimes()),
		zap.String("orderSn", s.orderSN),
		zap.String("parentOrderSN", s.parentOrderSN),
		zap.String("freeOrderSN", s.freeOrderSN),
	)
}

// collectBat 收集蝙蝠移动和Wind转换信息
//
// 规则说明：
//
//	基础模式：1-2个夺宝时，随机选择对应数量的人符号(7/8/9)转换为Wild
//	免费模式：蝙蝠从上次位置持续移动，移动到人符号位置则转换为Wild
//	重要：每列最多1个夺宝符号，因此夺宝数量最多5个
func (s *betOrderService) collectBat() {
	treasureCount, treasurePositions := s.countTreasureSymbols()

	if s.isFreeRound() {
		s.stepMap.Bat = s.transformToWildFreeMode(treasureCount, treasurePositions)
	} else {
		s.stepMap.Bat = s.transformToWildBaseMode(treasureCount, treasurePositions)
	}
}

// countTreasureSymbols 计算当前盘面的夺宝符号数量和位置
func (s *betOrderService) countTreasureSymbols() (int64, []*position) {
	var positions []*position
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			if s.symbolGrid[row][col] == _treasure {
				positions = append(positions, &position{Row: row, Col: col})
			}
		}
	}

	s.treasureCount = int64(len(positions))
	s.treasurePositions = positions
	s.stepMap.TreatCount = s.treasureCount

	return s.treasureCount, positions
}

// transformToWildBaseMode 基础模式Wind转换
//
// 规则：1-2个夺宝时触发，随机选择对应数量的人符号(7/8/9)转换为Wild
func (s *betOrderService) transformToWildBaseMode(treasureCount int64, treasurePositions []*position) []*Bat {
	if treasureCount < 1 || treasureCount > 2 {
		return nil
	}

	// 扫描所有人符号位置(7/8/9)
	var windPositions []*position
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			symbol := s.symbolGrid[row][col]
			if symbol == _child || symbol == _woman || symbol == _oldMan {
				windPositions = append(windPositions, &position{Row: row, Col: col})
			}
		}
	}

	if len(windPositions) == 0 {
		return nil
	}

	// 随机选择N个人符号转换
	selectCount := min(int(treasureCount), len(windPositions))
	bats := make([]*Bat, 0, selectCount)

	for i, idx := range rand.Perm(len(windPositions))[:selectCount] {
		pos := windPositions[idx]
		oldSymbol := s.symbolGrid[pos.Row][pos.Col]
		s.symbolGrid[pos.Row][pos.Col] = _wild
		bats = append(bats, createBat(treasurePositions[i%len(treasurePositions)], pos, oldSymbol, _wild))
	}

	return bats
}

// transformToWildFreeMode 免费模式Wind转换（蝙蝠持续移动）
//
// 逻辑：
//  1. 从scene.BatPositions获取蝙蝠当前位置
//  2. 每只蝙蝠随机8个方向移动一格
//  3. 移动到人符号(7/8/9)位置则转换为Wild
//  4. 返回所有蝙蝠的移动记录
func (s *betOrderService) transformToWildFreeMode(treasureCount int64, treasurePositions []*position) []*Bat {
	batPositions := s.scene.BatPositions
	if len(batPositions) == 0 {
		return nil
	}

	bats := make([]*Bat, 0, len(batPositions))

	// 保存原始符号，支持多只蝙蝠移入同一格子
	originalSymbols := make(map[string]int64)

	// 移动每只蝙蝠并转换Wind符号
	for _, batPos := range batPositions {
		newPos := moveBatOneStep(batPos)

		// 获取目标位置的原始符号（支持多只蝙蝠移入同一格子）
		posKey := fmt.Sprintf("%d_%d", newPos.Row, newPos.Col)
		targetSymbol, exists := originalSymbols[posKey]
		if !exists {
			targetSymbol = s.symbolGrid[newPos.Row][newPos.Col]
			originalSymbols[posKey] = targetSymbol
		}

		// 如果新位置是人符号(7/8/9)，转换为Wild
		if targetSymbol == _child || targetSymbol == _woman || targetSymbol == _oldMan {
			s.symbolGrid[newPos.Row][newPos.Col] = _wild
			bats = append(bats, createBat(batPos, newPos, targetSymbol, _wild))
		} else {
			bats = append(bats, createBat(batPos, newPos, s.symbolGrid[batPos.Row][batPos.Col], targetSymbol))
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
	{-1, -1}, {-1, 1}, {1, -1}, {1, 1}, // 左上、右上、左下、右下
}

// isValidPosition 检查位置是否在边界内
func isValidPosition(row, col int64) bool {
	return row >= 0 && row < _rowCount && col >= 0 && col < _colCount
}

// moveBatOneStep 蝙蝠随机移动一格（8个方向）
func moveBatOneStep(pos *position) *position {
	var validDirs []direction
	for _, dir := range allDirections {
		if isValidPosition(pos.Row+dir.dRow, pos.Col+dir.dCol) {
			validDirs = append(validDirs, dir)
		}
	}

	if len(validDirs) == 0 {
		return pos
	}

	dir := validDirs[rand.IntN(len(validDirs))]
	return &position{Row: pos.Row + dir.dRow, Col: pos.Col + dir.dCol}
}

// createBat 创建蝙蝠移动记录
func createBat(fromPos, toPos *position, oldSymbol, newSymbol int64) *Bat {
	return &Bat{
		X:      fromPos.Row,
		Y:      fromPos.Col,
		TransX: toPos.Row,
		TransY: toPos.Col,
		Syb:    oldSymbol,
		Sybn:   newSymbol,
	}
}
