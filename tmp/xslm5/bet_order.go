package xslm

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
	switch {
	case err != nil:
		global.GVA_LOG.Error("betOrder", zap.Error(err))
		return nil, InternalServerError
	case lastOrder == nil:
		s.saveScene(0, 0)
	}
	s.lastOrder = lastOrder
	s.selectGameRedis()
	switch {
	case s.lastOrder == nil:
		s.isFirst = true
	case s.client.ClientOfFreeGame.GetLastMapId() == 0:
		s.isFirst = true
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
	if err := s.initialize(); err != nil {
		return err
	}
	if false {
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

	// 加载女性符号计数
	s.loadSceneFemaleCount()

	// 初始化符号网格
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.scene.SymbolRoller = s.getSceneSymbol()
	}

	s.handleSymbolGrid()
	s.findWinInfos()
	s.updateStepResults(false)
	hasElimination := s.processElimination()
	if s.isFreeRound {
		s.eliminateResultForFree(hasElimination)
	} else {
		s.eliminateResultForBase(hasElimination)
	}
	s.updateCurrentBalance()
	return nil
}

func (s *betOrderService) eliminateResultForBase(hasElimination bool) {
	if hasElimination {
		s.isRoundOver = false
		s.client.IsRoundOver = false
		s.scene.Steps++
		s.scene.NextStage = _spinTypeBaseEli
		s.scene.FemaleCountsForFree = [3]int64{}

	} else {
		// 没有消除，结束当前回合
		s.isRoundOver = true
		s.client.IsRoundOver = true

		// 更新免费次数 （判断是否进入免费）
		s.treasureCount = s.getTreasureCount()
		if s.treasureCount >= _triggerTreasureCount {
			//s.newFreeRoundCount = _freeRounds[s.treasureCount-_triggerTreasureCount]
			idx := int(s.treasureCount - 1)
			if idx >= len(s.gameConfig.FreeSpinCount) {
				idx = len(s.gameConfig.FreeSpinCount) - 1
			}
			newFree := s.gameConfig.FreeSpinCount[idx]
			s.newFreeRoundCount = newFree
			s.scene.FreeNum = newFree
			if s.newFreeRoundCount > 0 {
				s.client.ClientOfFreeGame.SetFreeNum(uint64(s.newFreeRoundCount))
				s.client.SetLastMaxFreeNum(uint64(s.newFreeRoundCount))
			}
		}

		// 更新状态
		s.scene.Steps = 0
		s.scene.NextStage = _spinTypeFree
		if s.scene.FreeNum <= 0 {
			s.scene.NextStage = _spinTypeBase
			s.scene.FemaleCountsForFree = [3]int64{}
		}
	}

	// 更新结果
	if s.stepMultiplier > 0 {
		s.updateBonusAmount()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncRoundBonus(s.bonusAmount.Round(2).InexactFloat64())
	}
}

func (s *betOrderService) eliminateResultForFree(hasElimination bool) {
	// 免费模式
	if hasElimination {
		s.isRoundOver = false
		s.client.IsRoundOver = false
		s.scene.Steps++
		s.scene.NextStage = _spinTypeFreeEli
		s.scene.FemaleCountsForFree = s.nextFemaleCountsForFree

	} else {
		// 没有消除，结束当前回合
		s.isRoundOver = true
		s.client.IsRoundOver = true

		// 统计免费次数
		if s.newFreeRoundCount = s.getTreasureCount(); s.newFreeRoundCount > 0 {
			s.client.ClientOfFreeGame.Incr(uint64(s.newFreeRoundCount))
			s.client.IncLastMaxFreeNum(uint64(s.newFreeRoundCount))
			s.scene.FreeNum += s.newFreeRoundCount
		}
		s.client.ClientOfFreeGame.IncrFreeTimes()
		s.client.ClientOfFreeGame.Decr()
		s.scene.FreeNum--
		if s.scene.FreeNum < 0 {
			s.scene.FreeNum = 0
		}

		// 更新状态
		s.scene.Steps = 0
		s.scene.NextStage = _spinTypeFree
		if s.scene.FreeNum <= 0 {
			// 如果没有免费次数了，下一局应该是_spinTypeBase 状态
			s.scene.NextStage = _spinTypeBase
			s.scene.FemaleCountsForFree = [3]int64{}
		}
	}

	// 更新结果
	if s.stepMultiplier > 0 {
		s.updateBonusAmount()
		s.client.ClientOfFreeGame.IncrGeneralWinTotal(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncrFreeTotalMoney(s.bonusAmount.Round(2).InexactFloat64())
		s.client.ClientOfFreeGame.IncRoundBonus(s.bonusAmount.Round(2).InexactFloat64())
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
				if sym >= (_blank+1) && sym <= _wildFemaleC {
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
						s.femaleCountsForFree[idx] >= _femaleFullCount {
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

func (s *betOrderService) tryCollectFemaleSymbol(sym int64) {
	if sym < _femaleA || sym > _femaleC {
		return
	}
	if idx := sym - _femaleA; idx >= 0 && idx < int64(len(s.nextFemaleCountsForFree)) {
		i := int(idx)
		if s.nextFemaleCountsForFree[i] < _femaleA {
			s.nextFemaleCountsForFree[i]++
		}
	}
}
