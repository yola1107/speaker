package pjcd

import (
	"context"
	"fmt"
	"time"

	"egame-grpc/global"

	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

// SpinSceneData 游戏场景数据（存储到Redis）
type SpinSceneData struct {
	Steps            uint64                  `json:"steps"`   // step步数
	Stage            int8                    `json:"stage"`   // 当前阶段
	NextStage        int8                    `json:"nStage"`  // 下一阶段
	FreeNum          int64                   `json:"freeNum"` // 剩余免费次数
	ContinueNum      int64                   `json:"cNum"`    // 连续消除次数（本轮）
	MultipleIndex    int64                   `json:"mIndex"`  // 当前轮次在倍数数组中的索引（0-based）
	RoundMultiplier  int64                   `json:"rMul"`    // 回合倍数累计
	SymbolRoller     [_colCount]SymbolRoller `json:"sRoller"` // 5条轮轴（当前盘面）
	WildStates       WildStateGrid           `json:"wildSt"`  // 百搭状态网格
	ButterflyBonus   int64                   `json:"bfBonus"` // 蝴蝶百搭累加倍数（免费模式跨spin）
	IsRoundFirstStep bool                    `json:"isFirst"` // 是否为回合首step
	// 轮轴持久化（服务器重启后恢复）
	BaseReelData      [][]int64 `json:"baseReel"` // 基础模式完整轮轴（5列×100符号）
	FreeReelData      [][]int64 `json:"freeReel"` // 免费模式完整轮轴（5列×100符号）
	BaseReelSpinCount int64     `json:"brSpin"`   // 基础轮轴已使用次数
}

var sceneDataKeyPrefix = fmt.Sprintf("scene-%d", GameID)

func (s *betOrderService) sceneKey() string {
	return fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, s.member.ID)
}

func (s *betOrderService) cleanScene() {
	global.GVA_REDIS.Del(context.Background(), s.sceneKey())
}

func (s *betOrderService) saveScene() error {
	sceneStr, err := jsoniter.MarshalToString(s.scene)
	if err != nil {
		global.GVA_LOG.Error("saveScene: marshal failed", zap.Error(err))
		return err
	}
	if err = global.GVA_REDIS.Set(context.Background(), s.sceneKey(), sceneStr, time.Hour*24*90).Err(); err != nil {
		global.GVA_LOG.Error("saveScene: redis set failed", zap.Error(err))
		return err
	}
	return nil
}

func (s *betOrderService) reloadScene() error {
	s.scene = new(SpinSceneData)
	if v := global.GVA_REDIS.Get(context.Background(), s.sceneKey()).Val(); len(v) > 0 {
		if err := jsoniter.UnmarshalFromString(v, s.scene); err != nil {
			s.cleanScene()
			return err
		}
	}

	// 加载场景后进行状态初始化
	s.syncGameStage()
	return nil
}

// syncGameStage 同步游戏阶段状态
func (s *betOrderService) syncGameStage() {
	// 初始阶段
	if s.scene.Stage == 0 {
		s.scene.Stage = _spinTypeBase
	}

	// 阶段切换
	if s.scene.NextStage > 0 {
		s.scene.Stage = s.scene.NextStage
		s.scene.NextStage = 0
	}

	// 判断是否免费回合
	s.isFreeRound = s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli

	// 加载百搭状态
	s.wildStates = s.scene.WildStates
	s.butterflyBonus = s.scene.ButterflyBonus
}
