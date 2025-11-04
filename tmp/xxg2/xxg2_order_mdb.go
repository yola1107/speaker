package xxg2

import (
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"

	"go.uber.org/zap"
)

// 获取商户信息
func (s *betOrderService) mdbGetMerchant() bool {
	if s.debug.open {
		s.merchant = &merchant.Merchant{
			ID:       20020,
			Merchant: "Jack23",
		}
		return true
	}

	var m merchant.Merchant
	if err := global.GVA_DB.Where("id=?", s.req.MerchantId).First(&m).Error; err != nil {
		global.GVA_LOG.Error("mdbGetMerchant", zap.Error(err), zap.Int64("merchantID", s.req.MerchantId))
		return false
	}
	s.merchant = &m
	return true
}

// 获取用户信息
func (s *betOrderService) mdbGetMember() bool {
	if s.debug.open {
		s.member = &member.Member{
			ID:         3566020,
			MemberName: "Jack23",
			Balance:    10000000,
		}
		return true
	}

	var m member.Member
	if err := global.GVA_DB.Where("id=? and merchant=?", s.req.MemberId, s.merchant.Merchant).First(&m).Error; err != nil {
		global.GVA_LOG.Error("mdbGetMember", zap.Error(err),
			zap.Int64("memberID", s.req.MemberId),
			zap.String("merchant", s.merchant.Merchant))
		return false
	}
	s.member = &m
	return true
}

// 获取游戏信息
func (s *betOrderService) mdbGetGame() bool {
	if s.debug.open {
		s.game = &game.Game{
			ID:       GameID,
			GameType: 11,
			GameName: "XXG2",
		}
		return true
	}

	var g game.Game
	if err := global.GVA_DB.Where("id=? and status=1", s.req.GameId).First(&g).Error; err != nil {
		global.GVA_LOG.Error("mdbGetGame", zap.Error(err), zap.Int64("gameID", GameID))
		return false
	}

	var mg merchant.MerchantGame
	if err := global.GVA_DB.Where("merchant=? and game_id=?", s.merchant.Merchant, GameID).First(&mg).Error; err != nil {
		global.GVA_LOG.Error("mdbGetGame", zap.Error(err),
			zap.Int64("gameID", GameID),
			zap.String("merchant", s.merchant.Merchant))
		return false
	}
	s.game = &g
	return true
}
