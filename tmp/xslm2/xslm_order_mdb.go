package xslm2

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"
	"egame-grpc/model/slot"
	"egame-grpc/utils/json"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// ========== 数据库查询 ==========

func (s *betOrderService) mdbGetMerchant() bool {
	var m merchant.Merchant
	query := "id=?"
	args := []any{s.req.MerchantId}
	if tx := global.GVA_DB.Model(&m).Where(query, args...).First(&m); tx.Error != nil {
		global.GVA_LOG.Error("mdbGetMerchant", zap.Error(tx.Error), zap.Int64("merchantID", s.req.MerchantId))
		return false
	}
	s.merchant = &m
	return true
}

func (s *betOrderService) mdbGetMember() bool {
	var m member.Member
	query := "id=? and merchant=?"
	args := []any{s.req.MemberId, s.merchant.Merchant}
	if tx := global.GVA_DB.Model(&m).Where(query, args...).First(&m); tx.Error != nil {
		global.GVA_LOG.Error(
			"mdbGetMember",
			zap.Error(tx.Error),
			zap.Int64("memberID", s.req.MemberId),
			zap.String("merchant", s.merchant.Merchant),
		)
		return false
	}
	s.member = &m
	return true
}

func (s *betOrderService) mdbGetGame() bool {
	var g game.Game
	query := "id=? and status=1"
	args := []any{s.req.GameId}
	if tx := global.GVA_DB.Model(&g).Where(query, args...).First(&g); tx.Error != nil {
		global.GVA_LOG.Error("mdbGetGame", zap.Error(tx.Error), zap.Int64("gameID", _gameID))
		return false
	}
	var mg merchant.MerchantGame
	query = "merchant=? and game_id=?"
	args = []any{s.merchant.Merchant, _gameID}
	if tx := global.GVA_DB.Model(&mg).Where(query, args...).First(&mg); tx.Error != nil {
		global.GVA_LOG.Error(
			"mdbGetGame",
			zap.Error(tx.Error),
			zap.Int64("gameID", _gameID),
			zap.String("merchant", s.merchant.Merchant),
		)
		return false
	}
	s.game = &g
	return true
}

// ========== Redis查询 ==========

func (s *betOrderService) rdbGetPresetByID(id int64) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	key := fmt.Sprintf(_presetDataKeyTpl, global.GVA_CONFIG.System.Site)
	field := strconv.FormatInt(id, 10)
	result, err := s.gameRedis.HGet(ctx, key, field).Result()
	if err != nil {
		global.GVA_LOG.Error(
			"rdbGetPresetByID",
			zap.Error(err),
			zap.String("key", key),
			zap.String("field", field),
		)
		if errors.Is(err, redis.Nil) {
			s.saveScene(0, 0)
		}
		return false
	}
	var preset slot.XSLM
	if err := json.CJSON.Unmarshal([]byte(result), &preset); err != nil {
		global.GVA_LOG.Error("rdbGetPresetByID", zap.Error(err), zap.String("result", result))
		return false
	}
	s.spin.preset = &preset
	s.presetMultiplier = preset.TotalMultiplier
	return true
}

func (s *betOrderService) rdbGetPresetIDByExpectedParam() bool {
	for i := int64(0); i <= s.expectedMultiplier; i++ {
		switch {
		case !s.rdbGetPresetIDByMultiplier(s.expectedMultiplier - i):
			return false
		case s.presetID == 0:
			continue
		default:
			return true
		}
	}
	err := fmt.Errorf("slot not found: [%v,%v]", s.presetKind, s.expectedMultiplier)
	global.GVA_LOG.Error("rdbGetPresetIDByExpectedParam", zap.Error(err))
	return false
}

func (s *betOrderService) rdbGetPresetIDByMultiplier(mul int64) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	key := fmt.Sprintf(_presetIDKeyTpl, global.GVA_CONFIG.System.Site, s.presetKind, mul)
	count, err := s.gameRedis.HLen(ctx, key).Result()
	switch {
	case err != nil:
		global.GVA_LOG.Error("rdbGetSlotByMultiplier", zap.Error(err), zap.String("key", key))
		return false
	case count == 0:
		return true
	}
	r := randPool.Get().(*rand.Rand)
	defer randPool.Put(r)
	field := strconv.FormatInt(r.Int63n(count), 10)
	result, err := s.gameRedis.HGet(ctx, key, field).Result()
	if err != nil {
		global.GVA_LOG.Error(
			"rdbGetSlotByMultiplier",
			zap.Error(err),
			zap.String("key", key),
			zap.String("field", field),
		)
		return false
	}
	id, err := strconv.ParseInt(result, 10, 64)
	if err != nil {
		global.GVA_LOG.Error("rdbGetSlotByMultiplier", zap.Error(err), zap.String("result", result))
		return false
	}
	s.presetID = id
	return true
}

// ========== 场景数据 ==========

type scene struct {
	isRoundOver      bool
	lastPresetID     uint64
	lastStepID       uint64
	spinBonusAmount  float64
	roundBonusAmount float64
	freeNum          uint64
	freeTotalMoney   float64
	lastMaxFreeNum   uint64
	freeTimes        uint64
}

func (s *betOrderService) backupScene() bool {
	s.scene.isRoundOver = s.client.IsRoundOver
	s.scene.lastPresetID = uint64(s.spin.preset.ID)
	s.scene.lastStepID = s.client.ClientOfFreeGame.GetLastMapId()
	s.scene.spinBonusAmount = s.client.ClientOfFreeGame.GetGeneralWinTotal()
	s.scene.roundBonusAmount = s.client.ClientOfFreeGame.RoundBonus
	s.scene.freeNum = s.client.ClientOfFreeGame.GetFreeNum()
	s.scene.freeTotalMoney = s.client.ClientOfFreeGame.GetFreeTotalMoney()
	s.scene.lastMaxFreeNum = s.client.GetLastMaxFreeNum()
	s.scene.freeTimes = s.client.ClientOfFreeGame.GetFreeTimes()
	return true
}

func (s *betOrderService) restoreScene() bool {
	s.client.IsRoundOver = s.scene.isRoundOver
	s.client.ClientOfFreeGame.SetLastWinId(s.scene.lastPresetID)
	s.client.ClientOfFreeGame.SetLastMapId(s.scene.lastStepID)
	s.client.ClientOfFreeGame.GeneralWinTotal = s.scene.spinBonusAmount
	s.client.ClientOfFreeGame.RoundBonus = s.scene.roundBonusAmount
	s.client.ClientOfFreeGame.SetFreeNum(s.scene.freeNum)
	s.client.ClientOfFreeGame.FreeTotalMoney = s.scene.freeTotalMoney
	s.client.SetLastMaxFreeNum(s.scene.lastMaxFreeNum)
	s.client.ClientOfFreeGame.SetFreeTimes(s.scene.freeTimes)
	s.client.ClientGameCache.SaveScenes(s.client)
	return true
}

func (s *betOrderService) saveScene(lastSlotID uint64, lastMapID uint64) {
	s.client.ClientOfFreeGame.SetLastWinId(lastSlotID)
	s.client.ClientOfFreeGame.SetLastMapId(lastMapID)
	s.client.ClientGameCache.SaveScenes(s.client)
}
