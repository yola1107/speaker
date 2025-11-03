package xslm2

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	mathRand "math/rand"
	"strconv"
	"sync"
	"time"

	"egame-grpc/gamelogic"
	"egame-grpc/global"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// ========== 请求和验证 ==========

// getRequestContext 获取请求上下文（商户、用户、游戏信息）
func (s *betOrderService) getRequestContext() bool {
	switch {
	case !s.mdbGetMerchant():
		return false
	case !s.mdbGetMember():
		return false
	case !s.mdbGetGame():
		return false
	default:
		return true
	}
}

// selectGameRedis 选择游戏Redis
func (s *betOrderService) selectGameRedis() {
	index := _gameID % int64(len(global.GVA_GAME_REDIS))
	s.gameRedis = global.GVA_GAME_REDIS[index]
}

// updateBetAmount 计算下注金额
func (s *betOrderService) updateBetAmount() bool {
	betAmount := decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_baseMultiplier))
	s.betAmount = betAmount
	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
		return false
	}
	return true
}

// checkBalance 检查余额
func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

// updateBonusAmount 计算奖金金额
func (s *betOrderService) updateBonusAmount() {
	bonusAmount := decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(s.spin.stepMultiplier))
	s.bonusAmount = bonusAmount
}

// ========== 网格转字符串 ==========

// symbolGridToString 符号网格转字符串
func (s *betOrderService) symbolGridToString() string {
	symbolStr := ""
	symbolSN := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			symbolStr += strconv.Itoa(symbolSN)
			symbolStr += ":"
			symbolStr += strconv.FormatInt(s.spin.symbolGrid[row][col], 10)
			symbolStr += "; "
			symbolSN++
		}
	}
	return symbolStr
}

// winGridToString 中奖网格转字符串
func (s *betOrderService) winGridToString() string {
	if s.spin.winGrid == nil {
		return ""
	}
	winningStr := ""
	winningSN := 1
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			winningStr += strconv.Itoa(winningSN)
			winningStr += ":"
			winningStr += strconv.FormatInt(s.spin.winGrid[row][col], 10)
			winningStr += "; "
			winningSN++
		}
	}
	return winningStr
}

// ========== 日志函数 ==========

// showPostUpdateErrorLog 显示状态不匹配错误日志
func (s *betOrderService) showPostUpdateErrorLog() {
	global.GVA_LOG.Error(
		"showPostUpdateErrorLog",
		zap.Error(errors.New("step state mismatch")),
		zap.Int64("id", s.member.ID),
		zap.Bool("isFreeRound", s.isFreeRound),
		zap.Uint64("lastWinID", s.client.ClientOfFreeGame.GetLastWinId()),
		zap.Uint64("lastMapID", s.client.ClientOfFreeGame.GetLastMapId()),
		zap.Uint64("freeNum", s.client.ClientOfFreeGame.GetFreeNum()),
		zap.Uint64("freeTimes", s.client.ClientOfFreeGame.GetFreeTimes()),
		zap.String("orderSn", s.orderSN),
		zap.String("parentOrderSN", s.parentOrderSN),
		zap.String("freeOrderSN", s.freeOrderSN),
	)
}

// ========== 随机数和配置 ==========

// getSeed 获取随机种子
func getSeed() int64 {
	var seed int64
	if err := binary.Read(rand.Reader, binary.BigEndian, &seed); err != nil {
		global.GVA_LOG.Error("getSeed", zap.Error(err))
		return time.Now().UnixNano()
	}
	return seed
}

// randPool 随机数池
var randPool = sync.Pool{
	New: func() interface{} {
		return mathRand.New(mathRand.NewSource(getSeed()))
	},
}

// ========== 游戏配置数据 ==========

// _freeRounds 免费次数配置（根据夺宝数量）
// [0] = 3个夺宝 → 7次
// [1] = 4个夺宝 → 10次
// [2] = 5个夺宝 → 15次
var _freeRounds = []int64{7, 10, 15}

// _symbolMultiplierGroups 符号倍率表
// 每个符号的[3列, 4列, 5列]倍率
var _symbolMultiplierGroups = [][]int64{
	{2, 3, 5},    // 符号1
	{2, 3, 5},    // 符号2
	{2, 3, 5},    // 符号3
	{2, 3, 5},    // 符号4
	{5, 8, 12},   // 符号5
	{6, 10, 15},  // 符号6
	{10, 15, 25}, // 符号7（femaleA）
	{10, 15, 25}, // 符号8（femaleB）
	{10, 15, 25}, // 符号9（femaleC）
	{15, 25, 40}, // 符号10（wildFemaleA）
	{15, 25, 40}, // 符号11
	{15, 25, 40}, // 符号12
}
