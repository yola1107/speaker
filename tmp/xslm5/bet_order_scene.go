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

	// 新增字段
	Steps               uint64                  `json:"steps"`        // step步数，也是连赢次数
	Stage               int8                    `json:"stage"`        // 运行阶段
	NextStage           int8                    `json:"nStage"`       // 下一阶段
	SymbolRoller        [_colCount]SymbolRoller `json:"sRoller"`      // 滚轮符号表
	FemaleCountsForFree [3]int64                `json:"femaleCounts"` // 女性符号计数
	FreeNum             int64                   `json:"freeNum"`      // 剩余免费次数（独立统计，不依赖client）
	TreasureNum         int64                   `json:"treasureNum"`  // 夺宝符号数量 (每局结束写入)
}

func (s *betOrderService) backupScene() bool {
	s.scene.isRoundOver = s.client.IsRoundOver
	s.scene.lastPresetID = uint64(s.preset.ID)
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

// -----------------------------------------------------------------------
/*

var sceneDataKeyPrefix = fmt.Sprintf("scene-%d", _gameID)

// 加载场景数据
func (s *betOrderService) reloadScene() error {
	s.scene = new(SpinSceneData)

	if err := s.loadCacheSceneData(); err != nil {
		global.GVA_LOG.Error("reloadScene", zap.Error(err))
		s.cleanScene()
		return nil
	}

	s.scene.Stage = _spinTypeBase
	//新免费回合开始
	if s.scene.NextStage > 0 && s.scene.NextStage != s.scene.Stage {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}

	return nil
}



func (s *betOrderService) sceneKey() string {
	return fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
}

func (s *betOrderService) cleanScene() {
	global.GVA_REDIS.Del(context.Background(), s.sceneKey())
}

// 保存场景
func (s *betOrderService) saveScene() error {
	sceneStr, _ := jsoniter.MarshalToString(s.scene)
	if err := global.GVA_REDIS.Set(context.Background(), s.sceneKey(), sceneStr, time.Hour*24*90).Err(); err != nil {
		global.GVA_LOG.Error("saveScene", zap.Error(err))
		return err
	}
	return nil

}

func (s *betOrderService) loadCacheSceneData() error {
	v := global.GVA_REDIS.Get(context.Background(), s.sceneKey()).Val()
	if len(v) > 0 {
		tmpSc := new(SpinSceneData)
		if err := jsoniter.UnmarshalFromString(v, tmpSc); err != nil {
			return err
		}
		s.scene = tmpSc
	}
	return nil
}
*/
