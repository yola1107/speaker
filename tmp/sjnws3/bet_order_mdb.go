package sjnws3

import (
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"

	"go.uber.org/zap"
)

// 获取商户信息
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

// 获取用户信息
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

// 获取游戏信息
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
