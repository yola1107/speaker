package xslm3

import (
	"errors"
	"fmt"

	"egame-grpc/global"
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"
	"egame-grpc/model/slot"
	"egame-grpc/strategy"

	"github.com/go-redis/redis/v8"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

var _debugLogOpen = false

type betOrderService struct {
	req                *request.BetOrderReq
	merchant           *merchant.Merchant
	member             *member.Member
	game               *game.Game
	client             *client.Client
	lastOrder          *game.GameOrder
	gameRedis          *redis.Client
	isFirst            bool
	betAmount          decimal.Decimal
	amount             decimal.Decimal
	strategy           *strategy.Strategy
	gameType           int64
	orderSN            string
	parentOrderSN      string
	freeOrderSN        string
	isFreeRound        bool
	presetID           int64
	probMap            map[int64]game.GameDynamicProb
	probMultipliers    []int64
	probWeightSum      int64
	presetKind         int64
	expectedMultiplier int64
	presetMultiplier   int64
	gameOrder          *game.GameOrder
	bonusAmount        decimal.Decimal
	currBalance        decimal.Decimal

	// Spin fields (extracted from spin struct)
	gameConfig              *gameConfigJson
	scene                   *scene
	preset                  *slot.XSLM
	stepMap                 *stepMap
	femaleCountsForFree     [_femaleC - _femaleA + 1]int64
	nextFemaleCountsForFree [_femaleC - _femaleA + 1]int64
	enableFullElimination   bool
	isRoundOver             bool
	symbolGrid              *int64Grid
	winInfos                []*winInfo
	winResults              []*winResult
	winGrid                 *int64Grid
	hasFemaleWin            bool
	lineMultiplier          int64
	stepMultiplier          int64
	treasureCount           int64
	newFreeRoundCount       int64

	nextSymbolGrid   *int64Grid
	hasFemaleWildWin bool
	stepAddTreasure  int64
}

func newBetOrderService() *betOrderService {
	s := &betOrderService{}
	s.initGameConfigs()
	return s
}

func (s *betOrderService) betOrder(req *request.BetOrderReq) (map[string]any, error) {
	s.req = req
	if !s.getRequestContext() {
		return nil, InternalServerError
	}
	c, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
	if !ok {
		global.GVA_LOG.Error("betOrder", zap.Error(errors.New("user not exists")))
		return nil, fmt.Errorf("client not exist")
	}
	s.client = c
	c.BetLock.Lock()
	defer c.BetLock.Unlock()

	lastOrder, _, err := c.GetLastOrder()
	if err != nil {
		return nil, InternalServerError
	}
	s.lastOrder = lastOrder

	if s.lastOrder == nil {
		s.cleanScene()
	}

	if err = s.reloadScene(); err != nil {
		return nil, err
	}
	return s.doBetOrder()
}

func (s *betOrderService) doBetOrder() (map[string]any, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}
	if false {
		switch {
		case !s.initPreset():
			return nil, InternalServerError
		case !s.backupScene():
			return nil, InternalServerError
		case !s.initStepMap():
			return nil, InternalServerError
		case !s.updateStepResult():
			s.restoreScene()
			return nil, InternalServerError
		case !s.updateGameOrder():
			s.restoreScene()
			return nil, InternalServerError
		case !s.settleStep():
			s.restoreScene()
			return nil, InternalServerError
		}
		s.finalize()

		return s.getBetResultMap(), nil
	}

	// 重构逻辑
	if true {
		if err := s.baseSpin(); err != nil {
			global.GVA_LOG.Error("betOrder", zap.Error(err))
			return nil, InternalServerError
		}

		// 更新订单
		if !s.updateGameOrder() {
			return nil, InternalServerError
		}

		// 结算
		if !s.settleStep() {
			return nil, InternalServerError
		}

		// 保存场景数据
		if err := s.saveScene2(); err != nil {
			global.GVA_LOG.Error("doBetOrder.saveScene", zap.Error(err))
			return nil, InternalServerError

		}
	}

	return s.getBetResultMap(), nil
}

func (s *betOrderService) getBetResultMap() map[string]any {
	return map[string]any{
		"orderSN":                 s.gameOrder.OrderSn,
		"isFreeRound":             s.isFreeRound,
		"femaleCountsForFree":     s.femaleCountsForFree,
		"enableFullElimination":   s.enableFullElimination,
		"symbolGrid":              s.symbolGrid,
		"winGrid":                 s.winGrid,
		"winResults":              s.winResults,
		"baseBet":                 s.req.BaseMoney,
		"multiplier":              s.req.Multiple,
		"betAmount":               s.betAmount.Round(2).InexactFloat64(),
		"bonusAmount":             s.bonusAmount.Round(2).InexactFloat64(),
		"spinBonusAmount":         s.client.ClientOfFreeGame.GetGeneralWinTotal(),
		"freeBonusAmount":         s.client.ClientOfFreeGame.GetFreeTotalMoney(),
		"roundBonus":              s.client.ClientOfFreeGame.RoundBonus,
		"currentBalance":          s.gameOrder.CurBalance,
		"isRoundOver":             s.isRoundOver,
		"hasFemaleWin":            s.hasFemaleWin,
		"nextFemaleCountsForFree": s.nextFemaleCountsForFree,
		"newFreeRoundCount":       s.newFreeRoundCount,
		"totalFreeRoundCount":     s.client.GetLastMaxFreeNum(),
		"remainingFreeRoundCount": s.client.ClientOfFreeGame.GetFreeNum(),
		"lineMultiplier":          s.lineMultiplier,
		"stepMultiplier":          s.stepMultiplier,
	}
}

// _____________________________________________________________________________________
func (s *betOrderService) baseSpin() error {
	/*	if false {
			if err := s.initialize(); err != nil {
				return err
			}
			switch {
			case !s.initPreset():
				return InternalServerError
			case !s.backupScene():
				return InternalServerError
			case !s.initStepMap():
				return InternalServerError
			case !s.updateStepResult():
				s.restoreScene()
				return InternalServerError
			case !s.updateGameOrder():
				s.restoreScene()
				return InternalServerError
			case !s.settleStep():
				s.restoreScene()
				return InternalServerError
			}
			s.finalize()

			return nil
		}

		if false {
			s.handleStageTransition() // 状态跳转
			s.loadSceneFemaleCount()  // 加载女性符号计数

			// 初始化符号网格（新回合开始时）
			if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
				s.scene.SymbolRoller = s.getSceneSymbol()

				if _debugLogOpen {
					global.GVA_LOG.Debug(
						"新回合开始",
						zap.Int8("Stage", s.scene.Stage),
						zap.Int64("FreeNum", s.scene.FreeNum),
						zap.Bool("isFreeRound", s.isFreeRound),
						zap.Any("scene", s.scene),
					)
				}
			}

			// 处理符号网格、查找中奖、更新结果
			s.handleSymbolGrid()
			s.findWinInfos()

			// 免费模式全屏情况下，需要重新查找所有符号的中奖（包括只有女性百搭的way）
			// 因为全屏消除会消除除百搭13之外的所有符号，需要先算分
			if s.isFreeRound && s.enableFullElimination {
				s.findAllWinInfosForFullElimination()
			}

			s.updateStepResults(false)

			// 处理消除和结果
			hasElimination := s.processElimination()
			if s.isFreeRound {
				s.eliminateResultForFree(hasElimination)
			} else {
				s.eliminateResultForBase(hasElimination)
			}
			s.updateCurrentBalance()
			return nil
		}*/

	// 调用优化后的逻辑
	return s.baseSpin2()
}

func (s *betOrderService) eliminateResultForBase(hasElimination bool) {
	if hasElimination {
		// 有消除，继续消除状态
		s.isRoundOver = false
		s.client.IsRoundOver = false
		s.scene.Steps++
		s.scene.NextStage = _spinTypeBaseEli
		s.scene.FemaleCountsForFree = [3]int64{}
		s.newFreeRoundCount = 0
	} else {
		// 没有消除，结束当前回合（roundOver）
		s.isRoundOver = true
		s.client.IsRoundOver = true
		s.scene.Steps = 0
		s.scene.FemaleCountsForFree = [3]int64{}

		// 基础模式：只在 roundOver 时统计夺宝数量并判断是否进入免费
		s.treasureCount = s.getTreasureCount()
		s.newFreeRoundCount = s.getFreeRoundCountFromTreasure()

		if s.newFreeRoundCount > 0 {
			// 触发免费模式
			s.scene.FreeNum = s.newFreeRoundCount
			s.client.ClientOfFreeGame.SetFreeNum(uint64(s.newFreeRoundCount))
			s.client.SetLastMaxFreeNum(uint64(s.newFreeRoundCount))
			s.scene.NextStage = _spinTypeFree
			// 进入免费模式时，重置 TreasureNum 为 0，避免将基础模式的夺宝数量计入免费模式
			s.scene.TreasureNum = 0
		} else {
			// 不触发免费模式，继续基础模式
			s.scene.NextStage = _spinTypeBase
			// 不触发免费模式，重置 TreasureNum 为 0，开始新的基础模式计数
			s.scene.TreasureNum = 0
		}
	}

	// 更新结果
	if s.stepMultiplier > 0 {
		s.updateBonusAmount()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncRoundBonus(s.bonusAmount.Round(2).InexactFloat64())
	}

	if _debugLogOpen {
		str := "\t->基础回合结束"
		if s.isRoundOver {
			str = "基础回合结束"
		}
		global.GVA_LOG.Debug(
			str,
			zap.Int8("Stage", s.scene.Stage),
			zap.Int64("FreeNum", s.scene.FreeNum),
			zap.Any("scene", s.scene),
		)
	}
}

// getFreeRoundCountFromTreasure 根据夺宝数量从配置获取免费次数
func (s *betOrderService) getFreeRoundCountFromTreasure() int64 {
	if s.treasureCount < _triggerTreasureCount {
		return 0
	}
	idx := int(s.treasureCount - 1)
	if idx >= len(s.gameConfig.FreeSpinCount) {
		idx = len(s.gameConfig.FreeSpinCount) - 1
	}
	return s.gameConfig.FreeSpinCount[idx]
}

func (s *betOrderService) eliminateResultForFree(hasElimination bool) {
	// 计算本步骤增加的夺宝数量
	s.treasureCount = s.getTreasureCount()
	s.stepAddTreasure = s.treasureCount - s.scene.TreasureNum
	s.scene.TreasureNum = s.treasureCount

	// 免费模式：每收集1个夺宝符号则免费游戏次数+1
	// stepAddTreasure 就是本步骤增加的夺宝数量，也就是新增的免费次数
	s.newFreeRoundCount = s.stepAddTreasure
	if s.newFreeRoundCount > 0 {
		s.client.ClientOfFreeGame.Incr(uint64(s.newFreeRoundCount))
		s.client.IncLastMaxFreeNum(uint64(s.newFreeRoundCount))
		s.scene.FreeNum += s.newFreeRoundCount
	}

	if hasElimination {
		// 有消除，继续消除状态
		s.isRoundOver = false
		s.client.IsRoundOver = false
		s.scene.Steps++
		s.scene.NextStage = _spinTypeFreeEli
		s.scene.FemaleCountsForFree = s.nextFemaleCountsForFree
	} else {
		// 没有消除，结束当前回合
		s.isRoundOver = true
		s.client.IsRoundOver = true
		s.scene.Steps = 0

		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
		if s.scene.FreeNum < 0 {
			s.scene.FreeNum = 0
		}

		// 更新状态
		if s.scene.FreeNum > 0 {
			s.scene.NextStage = _spinTypeFree
		} else {
			s.scene.NextStage = _spinTypeBase
			s.scene.FemaleCountsForFree = [3]int64{}
			// 从免费模式回到基础模式时，重置 TreasureNum 为 0，开始新的基础模式计数
			s.scene.TreasureNum = 0
		}
	}

	// 更新结果
	if s.stepMultiplier > 0 {
		s.updateBonusAmount()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncrFreeTotalMoney(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncRoundBonus(s.bonusAmount.Round(2).InexactFloat64())
	}

	if _debugLogOpen {
		str := "免费回合结束"
		if !s.isRoundOver {
			str = "->免费回合"
		}
		global.GVA_LOG.Debug(
			str,
			zap.Int8("Stage", s.scene.Stage),
			zap.Int64("FreeNum", s.scene.FreeNum),
			zap.Any("scene", s.scene),
		)
	}
}

//_____________________________________________________________________________________________

/*
	算分：女性百搭（10，11，12）可替换为基础符号（1，2，3，4，5，6，7，8，9），但连线上必须要有基础符号

	消除：
		基础模式：消除中奖的女性符号（7，8，9）及百搭，如果盘面有夺宝则百搭不消除
		免费模式：
			1> 全屏情况：每个中奖Way找女性百搭，找到则改way除百搭13之外的符号都全部消除
			2> 非全屏情况：每个中奖way找女性，找到该way女性及女性百搭都消除
*/
// processElimination 计算并执行消除网格
func (s *betOrderService) processElimination() bool {
	if len(s.winInfos) == 0 || s.stepMultiplier == 0 || s.winGrid == nil {
		return false
	}

	isFree := s.isFreeRound
	nextGrid := *s.symbolGrid

	var cnt int
	switch {
	case !isFree && s.hasFemaleWin && s.hasWildSymbol():
		cnt = s.fillElimBase(&nextGrid)
	case isFree && s.enableFullElimination && s.hasFemaleWildWin:
		cnt = s.fillElimFreeFull(&nextGrid)
	case isFree && (!s.enableFullElimination) && s.hasFemaleWin:
		cnt = s.fillElimFreePartial(&nextGrid)
	}

	if cnt == 0 {
		return false
	}

	// 有消除，执行掉落和填充
	s.collectFemaleSymbol()       // 收集中奖女性符号
	s.dropSymbols(&nextGrid)      // 消除后掉落
	s.fallingWinSymbols(nextGrid) // 掉落后填充，设置 SymbolRoller
	s.nextSymbolGrid = &nextGrid
	return cnt > 0
}

func (s *betOrderService) fillElimBase(grid *int64Grid) int {
	count := 0
	hasTreasure := getTreasureCount(s.symbolGrid) > 0
	for _, w := range s.winInfos {
		if w == nil || w.Symbol < _femaleA || w.Symbol > _femaleC {
			continue
		}
		if !infoHasBaseWild(w.WinGrid) {
			continue
		}
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if w.WinGrid[r][c] == _blank || isBlockedCell(r, c) {
					continue
				}
				sym := s.symbolGrid[r][c]
				if (sym >= _femaleA && sym <= _femaleC) || (sym == _wild && !hasTreasure) {
					grid[r][c] = _eliminated
					count++
				}
			}
		}
	}
	return count
}

func (s *betOrderService) fillElimFreeFull(grid *int64Grid) int {
	count := 0
	for _, w := range s.winInfos {
		if w == nil || !infoHasFemaleWild(w.WinGrid) {
			continue
		}
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if w.WinGrid[r][c] == _blank || isBlockedCell(r, c) {
					continue
				}
				sym := s.symbolGrid[r][c]
				// 全屏情况：除百搭13之外的符号都全部消除（女性百搭符号会消失，但百搭符号不消失）
				if sym >= (_blank+1) && sym <= _wildFemaleC && sym != _wild {
					grid[r][c] = _eliminated
					count++
				}
			}
		}
	}
	return count
}

func (s *betOrderService) fillElimFreePartial(grid *int64Grid) int {
	count := 0
	for _, w := range s.winInfos {
		if w == nil || w.Symbol < _femaleA || w.Symbol > _femaleC {
			continue
		}
		if !infoHasFemale(w.WinGrid) {
			continue
		}
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if w.WinGrid[r][c] == _blank || isBlockedCell(r, c) {
					continue
				}
				sym := s.symbolGrid[r][c]
				if sym >= _femaleA && sym <= _wildFemaleC {
					grid[r][c] = _eliminated
					count++
				}
			}
		}
	}
	return count
}

//_____________________________________________________________________________________________

func (s *betOrderService) handleSymbolGrid() {
	var symbolGrid int64Grid
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			// BoardSymbol 从下往上存储，所以需要反转索引
			// symbolGrid[0][col] 对应 BoardSymbol[3]，symbolGrid[3][col] 对应 BoardSymbol[0]
			symbolGrid[_rowCount-1-r][c] = s.scene.SymbolRoller[c].BoardSymbol[r]
		}
	}
	s.symbolGrid = &symbolGrid
}

func (s *betOrderService) fallingWinSymbols(nextSymbolGrid int64Grid) {
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			// BoardSymbol 从下往上存储，所以需要反转索引
			s.scene.SymbolRoller[c].BoardSymbol[r] = nextSymbolGrid[_rowCount-1-r][c]
		}
	}
	for i, _ := range s.scene.SymbolRoller {
		s.scene.SymbolRoller[i].ringSymbol(s.gameConfig)
	}

	// 免费模式下，填充后需要根据 ABC 计数转换符号
	if s.isFreeRound {
		for col := 0; col < int(_colCount); col++ {
			for row := 0; row < int(_rowCount); row++ {
				symbol := s.scene.SymbolRoller[col].BoardSymbol[row]
				// 检查是否是女性符号（A/B/C），且对应的计数 >= 10
				if symbol >= _femaleA && symbol <= _femaleC {
					idx := symbol - _femaleA
					if idx >= 0 && idx < 3 &&
						s.nextFemaleCountsForFree[idx] >= _femaleFullCount {
						// 转换为对应的 wild 版本
						s.scene.SymbolRoller[col].BoardSymbol[row] = _wildFemaleA + idx
					}
				}
			}
		}
	}
}

func (s *betOrderService) dropSymbols(grid *int64Grid) {
	for c := int64(0); c < _colCount; c++ {
		writePos := int64(0)
		if c == 0 || c == _colCount-1 {
			writePos = 1
		}

		for r := int64(0); r < _rowCount; r++ {
			if isBlockedCell(r, c) {
				continue
			}
			switch val := (*grid)[r][c]; val {
			case _eliminated:
				(*grid)[r][c] = _blank
			case _blank:
				continue
			default:
				if r != writePos {
					(*grid)[writePos][c] = val
					(*grid)[r][c] = _blank
				}
				writePos++
			}
		}
	}
}

func (s *betOrderService) loadSceneFemaleCount() {
	if !s.isFreeRound {
		s.femaleCountsForFree = [3]int64{}
		s.nextFemaleCountsForFree = [3]int64{}
		return
	}

	for i, c := range s.scene.FemaleCountsForFree {
		s.femaleCountsForFree[i] = c
		s.nextFemaleCountsForFree[i] = c
	}
	// 全屏清除检查
	s.enableFullElimination =
		s.femaleCountsForFree[0] >= _femaleFullCount &&
			s.femaleCountsForFree[1] >= _femaleFullCount &&
			s.femaleCountsForFree[2] >= _femaleFullCount

}

func (s *betOrderService) collectFemaleSymbol() {
	if !s.isFreeRound {
		return
	}
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbol := s.winGrid[r][c]
			if symbol >= _femaleA && symbol <= _femaleC {
				idx := symbol - _femaleA
				if s.nextFemaleCountsForFree[idx] < _femaleFullCount {
					s.nextFemaleCountsForFree[idx]++
				}
			}
		}
	}
}
