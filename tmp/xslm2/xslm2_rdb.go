package xslm2

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"egame-grpc/global"
	"egame-grpc/model/slot"
	"egame-grpc/utils/json"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

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
