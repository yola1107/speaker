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
			ID:          20020,
			Merchant:    "Jack23",
			UserID:      0,
			NickName:    "",
			Fee:         0,
			Balance:     0,
			PlatformNum: 0,
			Status:      0,
			Secret:      "",
			Qq:          "",
			TelPrefix:   "",
			Tel:         "",
			GameNum:     0,
			Email:       "",
			Website:     "",
			DoCode:      "",
			CreatedAt:   0,
			UpdatedAt:   0,
		}
		return true
	}

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

	if s.debug.open {
		s.member = &member.Member{
			ID:            3566020,
			MemberName:    "Jack23",
			Password:      "",
			NickName:      "",
			Balance:       10000000,
			Currency:      "",
			State:         0,
			LastLoginTime: 0,
			IP:            "",
			MerchantID:    0,
			Merchant:      "",
			Remark:        "",
			IsDelete:      0,
			MemberType:    0,
			TrueName:      "",
			TelPrefix:     "",
			VipLevel:      0,
			Phone:         "",
			ParentID:      "",
			Email:         "",
			CreatedAt:     0,
			UpdatedAt:     0,
			Version:       0,
		}
		return true
	}

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

	if s.debug.open {
		s.game = &game.Game{
			ID:             _gameID,
			GameType:       11,
			GameName:       "Jztdmm",
			Icon:           "",
			HotTag:         0,
			NewTag:         0,
			ShowAndroid:    0,
			ShowIos:        0,
			Status:         0,
			Sort:           0,
			CreatedAt:      0,
			UpdatedAt:      0,
			DeletedAt:      0,
			PurchaseStatus: 0,
		}
		return true
	}

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
