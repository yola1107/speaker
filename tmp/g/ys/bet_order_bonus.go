package ys

import (
	"fmt"

	"egame-grpc/global/client"
	"egame-grpc/model/game/request"
)

func (s *betOrderService) betBonus(req *request.BetBonusReq) (map[string]any, error) {
	if req == nil || req.BonusNum < 1 || req.BonusNum > 3 {
		return nil, InvalidRequestParams
	}

	c, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		return nil, fmt.Errorf("betBonus. client not exist")
	}
	c.BetLock.Lock()
	defer c.BetLock.Unlock()

	s.client = c
	scene, err := loadScene(req.MemberId)
	if err != nil {
		return nil, err
	}
	if scene == nil {
		return nil, fmt.Errorf("scene not found")
	}
	s.scene = scene
	if s.scene.BonusState != _bonusStatePending || s.scene.BonusNum != 0 ||
		s.scene.ScatterNum < s.gameConfig.Free.ScatterMin {
		return nil, fmt.Errorf("bonus state is not pending. bonusState: %d, bonusNum: %d, scatterNum: %d",
			s.scene.BonusState, s.scene.BonusNum, s.scene.ScatterNum)
	}

	freeNum, err := s.selectFreeBonus(req.BonusNum)
	if err != nil {
		return nil, err
	}
	if err = s.scene.save(req.MemberId); err != nil {
		return nil, err
	}
	return map[string]any{
		"free":       req.BonusNum,
		"freeNum":    freeNum,
		"scatterNum": s.scene.ScatterNum,
	}, nil
}

func (s *betOrderService) selectFreeBonus(bonusNum int64) (int64, error) {
	freeNum := s.gameConfig.getFreeCfgByType(bonusNum).Times
	if freeNum <= 0 {
		return 0, InvalidRequestParams
	}
	s.scene.FreeNum = freeNum
	s.scene.FreeTimes = 0
	s.scene.BonusNum = bonusNum
	s.scene.BonusState = _bonusStateSelected
	s.scene.NextStage = _spinTypeFree
	return freeNum, nil
}
