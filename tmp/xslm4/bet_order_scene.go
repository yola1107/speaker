package xslm

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
