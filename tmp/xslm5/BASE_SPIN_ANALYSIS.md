# xslm3 baseSpin 逻辑详细分析

## 一、整体流程概览

`baseSpin()` 是游戏的核心旋转逻辑，负责处理一次完整的游戏回合，包括状态管理、符号生成、中奖检测、消除处理和场景数据更新。

### 主要执行流程

```193:249:game/xslm3/bet_order.go
func (s *betOrderService) baseSpin() error {
	// 1. 状态跳转处理
	s.handleStageTransition()
	
	// 2. 加载女性符号计数（免费模式）
	s.loadSceneFemaleCount()
	
	// 3. 初始化符号网格（新回合开始时）
	if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
		s.scene.SymbolRoller = s.getSceneSymbol()
	}
	
	// 4. 处理符号网格、查找中奖、更新结果
	s.handleSymbolGrid()
	s.findWinInfos()
	s.updateStepResults(false)
	
	// 5. 处理消除和结果
	hasElimination := s.processElimination()
	if s.isFreeRound {
		s.eliminateResultForFree(hasElimination)
	} else {
		s.eliminateResultForBase(hasElimination)
	}
	
	// 6. 更新当前余额
	s.updateCurrentBalance()
	return nil
}
```

---

## 二、状态跳转机制

### 2.1 状态类型定义

游戏有4种主要状态：

```45:50:game/xslm3/const.go
const (
	_spinTypeBase    = 1  //普通
	_spinTypeBaseEli = 11 //普通消除
	_spinTypeFree    = 21 //免费
	_spinTypeFreeEli = 22 //免费消除
)
```

### 2.2 状态跳转逻辑

状态跳转通过 `handleStageTransition()` 处理：

```114:132:game/xslm3/bet_order_scene.go
func (s *betOrderService) handleStageTransition() {
	// 初始化 Stage（如果是首次或未设置）
	if s.scene.Stage == 0 {
		s.scene.Stage = _spinTypeBase
	}

	// 处理状态切换：如果 NextStage 已设置且与当前 Stage 不同，则切换
	if s.scene.NextStage > 0 {
		if s.scene.NextStage != s.scene.Stage {
			s.scene.Stage = s.scene.NextStage
		}
		// 无论是否切换，都清零 NextStage（避免状态混淆）
		s.scene.NextStage = 0
	}

	// 根据当前 Stage 设置 isFreeRound（在状态切换后更新）
	s.isFreeRound = s.isFreeRoundStage()
}
```

### 2.3 状态跳转图

```
初始状态: _spinTypeBase (1)
    |
    | [有消除]
    v
_spinTypeBaseEli (11) -> [继续消除] -> _spinTypeBaseEli (11)
    |
    | [无消除，有免费次数]
    v
_spinTypeFree (21) -> [有消除] -> _spinTypeFreeEli (22) -> [继续消除] -> _spinTypeFreeEli (22)
    |                                                              |
    | [无消除，还有免费次数]                                        | [无消除，免费次数用完]
    +--------------------------------------------------------------+
    |
    v
_spinTypeBase (1) [重新开始]
```

### 2.4 状态跳转触发点

状态跳转在 `eliminateResultForBase()` 和 `eliminateResultForFree()` 中设置 `NextStage`：

**基础模式结束时的状态设置：**

```251:292:game/xslm3/bet_order.go
func (s *betOrderService) eliminateResultForBase(hasElimination bool) {
	if hasElimination {
		// 有消除，继续消除状态
		s.isRoundOver = false
		s.client.IsRoundOver = false
		s.scene.Steps++
		s.scene.NextStage = _spinTypeBaseEli  // 继续消除
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
			s.scene.NextStage = _spinTypeFree  // 进入免费模式
			s.scene.TreasureNum = 0
		} else {
			// 不触发免费模式，继续基础模式
			s.scene.NextStage = _spinTypeBase  // 继续基础模式
			s.scene.TreasureNum = 0
		}
	}
}
```

**免费模式结束时的状态设置：**

```314:380:game/xslm3/bet_order.go
func (s *betOrderService) eliminateResultForFree(hasElimination bool) {
	// ... 夺宝处理逻辑 ...

	if hasElimination {
		// 有消除，继续消除状态
		s.isRoundOver = false
		s.client.IsRoundOver = false
		s.scene.Steps++
		s.scene.NextStage = _spinTypeFreeEli  // 继续免费消除
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
			s.scene.NextStage = _spinTypeFree  // 还有免费次数，继续免费模式
		} else {
			s.scene.NextStage = _spinTypeBase  // 免费次数用完，回到基础模式
			s.scene.FemaleCountsForFree = [3]int64{}
			s.scene.TreasureNum = 0
		}
	}
}
```

---

## 三、Scene 数据结构与存储

### 3.1 Scene 结构定义

```12:31:game/xslm3/bet_order_scene.go
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
```

### 3.2 Scene 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `Steps` | `uint64` | 当前回合的消除步数（连赢次数），每次有消除时+1，回合结束时重置为0 |
| `Stage` | `int8` | 当前游戏阶段（1=基础，11=基础消除，21=免费，22=免费消除） |
| `NextStage` | `int8` | 下一轮的状态（用于状态跳转） |
| `SymbolRoller` | `[5]SymbolRoller` | 5列的滚轴状态，存储每列的符号和位置信息 |
| `FemaleCountsForFree` | `[3]int64` | 免费模式下3种女性符号（A/B/C）的收集计数 |
| `FreeNum` | `int64` | 剩余免费游戏次数 |
| `TreasureNum` | `int64` | 当前回合收集的夺宝符号数量 |

### 3.3 Scene 加载流程

```72:82:game/xslm3/bet_order_scene.go
func (s *betOrderService) reloadScene() error {
	s.scene = new(scene)

	if err := s.loadCacheSceneData(); err != nil {
		global.GVA_LOG.Error("reloadScene", zap.Error(err))
		s.cleanScene()
		return nil
	}

	return nil
}
```

```102:112:game/xslm3/bet_order_scene.go
func (s *betOrderService) loadCacheSceneData() error {
	v := global.GVA_REDIS.Get(context.Background(), s.sceneKey()).Val()
	if len(v) > 0 {
		tmpSc := new(scene)
		if err := jsoniter.UnmarshalFromString(v, tmpSc); err != nil {
			return err
		}
		s.scene = tmpSc
	}
	return nil
}
```

### 3.4 Scene 保存流程

```93:100:game/xslm3/bet_order_scene.go
func (s *betOrderService) saveScene2() error {
	sceneStr, _ := jsoniter.MarshalToString(s.scene)
	if err := global.GVA_REDIS.Set(context.Background(), s.sceneKey(), sceneStr, time.Hour*24*90).Err(); err != nil {
		global.GVA_LOG.Error("saveScene", zap.Error(err))
		return err
	}
	return nil
}
```

**保存时机：** 在 `doBetOrder()` 中，每次 `baseSpin()` 执行完成后保存：

```136:159:game/xslm3/bet_order.go
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
```

### 3.5 Scene 清理逻辑

```88:90:game/xslm3/bet_order_scene.go
func (s *betOrderService) cleanScene() {
	global.GVA_REDIS.Del(context.Background(), s.sceneKey())
}
```

**清理时机：** 当用户没有上一局订单时（新用户或长时间未游戏）：

```99:101:game/xslm3/bet_order.go
if s.lastOrder == nil {
	s.cleanScene()
}
```

---

## 四、符号网格处理流程

### 4.1 符号网格初始化

**新回合开始时生成符号：**

```223:233:game/xslm3/bet_order.go
// 初始化符号网格（新回合开始时）
if s.scene.Steps == 0 && (s.scene.Stage == _spinTypeBase || s.scene.Stage == _spinTypeFree) {
	s.scene.SymbolRoller = s.getSceneSymbol()
	global.GVA_LOG.Debug(
		"新回合开始",
		zap.Int8("Stage", s.scene.Stage),
		zap.Int64("FreeNum", s.scene.FreeNum),
		zap.Bool("isFreeRound", s.isFreeRound),
		zap.Any("scene", s.scene),
	)
}
```

**`getSceneSymbol()` 逻辑：**

```66:137:game/xslm3/bet_order_configs.go
func (s *betOrderService) getSceneSymbol() [_colCount]SymbolRoller {
	// 1. 根据模式选择滚轴配置
	rollCfg := s.gameConfig.RollCfg.Base
	if s.isFreeRound {
		// 免费模式：根据女性符号收集状态选择配置
		key := ""
		for i := 0; i < 3; i++ {
			if s.femaleCountsForFree[i] >= _femaleFullCount {
				key += "1"
			} else {
				key += "0"
			}
		}
		if freeCfg, ok := s.gameConfig.RollCfg.Free[key]; ok {
			rollCfg = freeCfg
		}
	}

	// 2. 根据权重随机选择预设数据
	realIndex := 0
	r := rand.IntN(rollCfg.WTotal)
	for i, w := range rollCfg.Weight {
		if r < w {
			realIndex = rollCfg.UseKey[i]
			break
		}
		r -= w
	}

	// 3. 生成每列的符号
	var rollers [_colCount]SymbolRoller
	for col := 0; col < int(_colCount); col++ {
		data := s.gameConfig.RealData[realIndex][col]
		start := rand.IntN(len(data))
		roller := SymbolRoller{Real: realIndex, Start: start, Fall: end, Col: col}

		for row := 0; row < int(_rowCount); row++ {
			symbol := data[(start+row)%len(data)]
			
			// 处理墙格（左上角和右上角）
			if row == 0 && (col == 0 || col == int(_colCount-1)) {
				symbol = _blocked
			}

			// 免费模式下，根据计数转换女性符号为女性百搭
			if s.isFreeRound {
				if symbol >= _femaleA && symbol <= _femaleC {
					idx := symbol - _femaleA
					if s.femaleCountsForFree[idx] >= _femaleFullCount {
						symbol = _wildFemaleA + idx
					}
				}
			}

			roller.BoardSymbol[int(_rowCount)-1-row] = symbol
		}
		rollers[col] = roller
	}

	return rollers
}
```

### 4.2 从 Scene 读取符号网格

```500:510:game/xslm3/bet_order.go
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
```

**注意：** `SymbolRoller.BoardSymbol` 是从下往上存储的，所以需要反转索引。

---

## 五、中奖检测与结果更新

### 5.1 查找中奖信息

```166:188:game/xslm3/bet_order_spin.go
func (s *betOrderService) findWinInfos() bool {
	var winInfos []*winInfo
	// 查找基础符号中奖（1-9）
	for symbol := _blank + 1; symbol < _wildFemaleA; symbol++ {
		if info, ok := s.findNormalSymbolWinInfo(symbol); ok {
			if symbol >= _femaleA {
				s.hasFemaleWin = true
			}
			if infoHasFemaleWild(info.WinGrid) {
				s.hasFemaleWildWin = true
			}
			winInfos = append(winInfos, info)
		}
	}
	// 查找女性百搭符号中奖（10-12）
	for symbol := _wildFemaleA; symbol < _wild; symbol++ {
		if info, ok := s.findWildSymbolWinInfo(symbol); ok {
			s.hasFemaleWildWin = true
			winInfos = append(winInfos, info)
		}
	}
	s.winInfos = winInfos
	return len(winInfos) > 0
}
```

### 5.2 更新步骤结果

```251:283:game/xslm3/bet_order_spin.go
func (s *betOrderService) updateStepResults(partialElimination bool) {
	var winResults []*winResult
	var winGrid int64Grid
	lineMultiplier := int64(0)
	for _, info := range s.winInfos {
		if partialElimination && info.Symbol < _femaleA {
			continue
		}
		// 从配置表获取倍率
		baseLineMultiplier := s.gameConfig.PayTable[info.Symbol-1][info.SymbolCount-1]
		totalMultiplier := baseLineMultiplier * info.LineCount
		result := winResult{
			Symbol:             info.Symbol,
			SymbolCount:        info.SymbolCount,
			LineCount:          info.LineCount,
			BaseLineMultiplier: baseLineMultiplier,
			TotalMultiplier:    totalMultiplier,
			WinGrid:            info.WinGrid,
		}
		winResults = append(winResults, &result)
		// 合并所有中奖网格
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				if info.WinGrid[r][c] != _blank {
					winGrid[r][c] = info.WinGrid[r][c]
				}
			}
		}
		lineMultiplier += totalMultiplier
	}
	s.stepMultiplier = lineMultiplier
	s.winResults = winResults
	s.winGrid = &winGrid
}
```

---

## 六、消除处理机制

### 6.1 消除流程

```393:422:game/xslm3/bet_order.go
func (s *betOrderService) processElimination() bool {
	if len(s.winInfos) == 0 || s.stepMultiplier == 0 || s.winGrid == nil {
		return false
	}

	isFree := s.isFreeRound
	nextGrid := *s.symbolGrid

	var cnt int
	switch {
	case !isFree && s.hasFemaleWin && s.hasWildSymbol():
		// 基础模式：有女性中奖且有百搭
		cnt = s.fillElimBase(&nextGrid)
	case isFree && s.enableFullElimination && s.hasFemaleWildWin:
		// 免费模式：全屏情况（3种女性都>=10）
		cnt = s.fillElimFreeFull(&nextGrid)
	case isFree && (!s.enableFullElimination) && s.hasFemaleWin:
		// 免费模式：非全屏情况
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
```

### 6.2 消除规则

**基础模式消除：**

```424:448:game/xslm3/bet_order.go
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
				// 消除女性符号和百搭（如果有夺宝则百搭不消除）
				if (sym >= _femaleA && sym <= _femaleC) || (sym == _wild && !hasTreasure) {
					grid[r][c] = _eliminated
					count++
				}
			}
		}
	}
	return count
}
```

**免费模式全屏消除：**

```450:471:game/xslm3/bet_order.go
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
				// 全屏情况：除百搭13之外的符号都全部消除
				if sym >= (_blank+1) && sym <= _wildFemaleC && sym != _wild {
					grid[r][c] = _eliminated
					count++
				}
			}
		}
	}
	return count
}
```

**免费模式非全屏消除：**

```473:496:game/xslm3/bet_order.go
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
				// 消除女性符号和女性百搭
				if sym >= _femaleA && sym <= _wildFemaleC {
					grid[r][c] = _eliminated
					count++
				}
			}
		}
	}
	return count
}
```

### 6.3 符号掉落与填充

**掉落处理：**

```542:567:game/xslm3/bet_order.go
func (s *betOrderService) dropSymbols(grid *int64Grid) {
	for c := int64(0); c < _colCount; c++ {
		writePos := int64(0)
		if c == 0 || c == _colCount-1 {
			writePos = 1  // 第一列和最后一列从第2行开始（第1行是墙格）
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
```

**填充处理：**

```512:540:game/xslm3/bet_order.go
func (s *betOrderService) fallingWinSymbols(nextSymbolGrid int64Grid) {
	// 1. 将填充后的网格写回 SymbolRoller
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			// BoardSymbol 从下往上存储，所以需要反转索引
			s.scene.SymbolRoller[c].BoardSymbol[r] = nextSymbolGrid[_rowCount-1-r][c]
		}
	}
	
	// 2. 补充掉下来导致的空缺位置
	for i, _ := range s.scene.SymbolRoller {
		s.scene.SymbolRoller[i].ringSymbol(s.gameConfig)
	}

	// 3. 免费模式下，填充后需要根据 ABC 计数转换符号
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
```

---

## 七、女性符号收集机制（免费模式）

### 7.1 加载女性符号计数

```569:586:game/xslm3/bet_order.go
func (s *betOrderService) loadSceneFemaleCount() {
	if !s.isFreeRound {
		s.femaleCountsForFree = [3]int64{}
		s.nextFemaleCountsForFree = [3]int64{}
		return
	}

	// 从 scene 加载计数
	for i, c := range s.scene.FemaleCountsForFree {
		s.femaleCountsForFree[i] = c
		s.nextFemaleCountsForFree[i] = c
	}
	
	// 全屏清除检查（3种女性符号都>=10）
	s.enableFullElimination =
		s.femaleCountsForFree[0] >= _femaleFullCount &&
			s.femaleCountsForFree[1] >= _femaleFullCount &&
			s.femaleCountsForFree[2] >= _femaleFullCount
}
```

### 7.2 收集中奖女性符号

```588:603:game/xslm3/bet_order.go
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
```

### 7.3 更新 Scene 中的计数

在 `eliminateResultForFree()` 中，有消除时会更新计数：

```329:335:game/xslm3/bet_order.go
if hasElimination {
	// 有消除，继续消除状态
	s.isRoundOver = false
	s.client.IsRoundOver = false
	s.scene.Steps++
	s.scene.NextStage = _spinTypeFreeEli
	s.scene.FemaleCountsForFree = s.nextFemaleCountsForFree  // 更新计数
}
```

---

## 八、夺宝符号处理

### 8.1 基础模式：触发免费游戏

```267:284:game/xslm3/bet_order.go
// 基础模式：只在 roundOver 时统计夺宝数量并判断是否进入免费
s.treasureCount = s.getTreasureCount()
s.newFreeRoundCount = s.getFreeRoundCountFromTreasure()

if s.newFreeRoundCount > 0 {
	// 触发免费模式
	s.scene.FreeNum = s.newFreeRoundCount
	s.client.ClientOfFreeGame.SetFreeNum(uint64(s.newFreeRoundCount))
	s.client.SetLastMaxFreeNum(uint64(s.newFreeRoundCount))
	s.scene.NextStage = _spinTypeFree
	// 进入免费模式时，重置 TreasureNum 为 0
	s.scene.TreasureNum = 0
} else {
	// 不触发免费模式，继续基础模式
	s.scene.NextStage = _spinTypeBase
	// 不触发免费模式，重置 TreasureNum 为 0，开始新的基础模式计数
	s.scene.TreasureNum = 0
}
```

### 8.2 免费模式：增加免费次数

```314:327:game/xslm3/bet_order.go
func (s *betOrderService) eliminateResultForFree(hasElimination bool) {
	// 计算本步骤增加的夺宝数量
	s.treasureCount = s.getTreasureCount()
	s.stepAddTreasure = s.treasureCount - s.scene.TreasureNum
	s.scene.TreasureNum = s.treasureCount

	// 免费模式：每收集1个夺宝符号则免费游戏次数+1
	s.newFreeRoundCount = s.stepAddTreasure
	if s.newFreeRoundCount > 0 {
		s.client.ClientOfFreeGame.Incr(uint64(s.newFreeRoundCount))
		s.client.IncLastMaxFreeNum(uint64(s.newFreeRoundCount))
		s.scene.FreeNum += s.newFreeRoundCount
	}
	// ...
}
```

---

## 九、关键数据清理时机

### 9.1 Steps 重置

- **重置时机：** 回合结束时（`isRoundOver = true`）
- **重置位置：** `eliminateResultForBase()` 和 `eliminateResultForFree()` 中

```264:264:game/xslm3/bet_order.go
s.scene.Steps = 0
```

### 9.2 FemaleCountsForFree 清理

- **基础模式：** 每次有消除或回合结束时都清零
- **免费模式：** 有消除时更新为 `nextFemaleCountsForFree`，免费次数用完时清零

```258:258:game/xslm3/bet_order.go
s.scene.FemaleCountsForFree = [3]int64{}
```

```354:354:game/xslm3/bet_order.go
s.scene.FemaleCountsForFree = [3]int64{}
```

### 9.3 TreasureNum 重置

- **基础模式：** 回合结束时重置为0（无论是否触发免费）
- **免费模式：** 免费次数用完回到基础模式时重置为0

```278:278:game/xslm3/bet_order.go
s.scene.TreasureNum = 0
```

```356:356:game/xslm3/bet_order.go
s.scene.TreasureNum = 0
```

### 9.4 SymbolRoller 更新

- **新回合开始：** `Steps == 0` 时重新生成
- **有消除：** 通过 `fallingWinSymbols()` 更新填充后的符号

---

## 十、完整执行流程图

```
用户请求
  ↓
betOrder()
  ↓
reloadScene()  [从Redis加载场景数据]
  ↓
doBetOrder()
  ↓
baseSpin()
  ├─→ handleStageTransition()      [状态跳转]
  ├─→ loadSceneFemaleCount()         [加载女性计数]
  ├─→ getSceneSymbol()               [新回合生成符号]
  ├─→ handleSymbolGrid()             [读取符号网格]
  ├─→ findWinInfos()                 [查找中奖]
  ├─→ updateStepResults()            [计算倍率]
  ├─→ processElimination()           [处理消除]
  │    ├─→ fillElimBase/Free()        [标记消除位置]
  │    ├─→ collectFemaleSymbol()      [收集女性符号]
  │    ├─→ dropSymbols()              [符号掉落]
  │    └─→ fallingWinSymbols()        [填充新符号]
  ├─→ eliminateResultForBase/Free()  [处理结果，设置NextStage]
  └─→ updateCurrentBalance()          [更新余额]
  ↓
updateGameOrder()                    [更新订单]
  ↓
settleStep()                         [结算]
  ↓
saveScene2()                         [保存场景到Redis]
  ↓
返回结果
```

---

## 十一、注意事项

1. **状态跳转延迟：** `NextStage` 在当前回合设置，下一回合开始时才生效
2. **符号存储方向：** `SymbolRoller.BoardSymbol` 从下往上存储，需要反转索引
3. **消除循环：** 有消除时会继续下一轮消除，直到没有消除或达到限制
4. **免费模式计数：** 女性符号计数在免费模式中持续累积，直到免费次数用完
5. **夺宝计数：** 基础模式只在回合结束时统计，免费模式每步都统计增量
6. **Scene持久化：** 每次 `baseSpin()` 后都会保存到Redis，确保断线重连后能恢复状态

