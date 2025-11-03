# XSLM2按mahjong2风格优化方案

> **优化时间**：2025-11-03  
> **参考范本**：mahjong2（代码最精简的高质量游戏）  
> **优化原则**：精简代码、保留注释、提升质量  

---

## 🎯 mahjong2的核心优势

### 1. 代码最精简（1372行）

```
mahjong2: 1372行（13个文件）
xslm2:    1542行（21个文件）

差距：仅170行，接近！
```

### 2. 命名最规范（mah2_前缀）

```
mahjong2文件命名：
- mah2_bet_order.go
- mah2_spin_helper.go
- mah2_order_step.go
- mah2_types.go
- mah2_const.go
- ...

优势：
✅ 前缀统一，易于识别
✅ IDE中搜索方便
✅ 避免与其他游戏混淆
```

### 3. BetService字段精简（15个）

```go
type BetService struct {
    // 基础信息（5个）
    req       *request.BetOrderReq
    merchant  *merchant.Merchant
    member    *member.Member
    game      *game.Game
    client    *client.Client
    
    // 订单（4个）
    lastOrder     *game.GameOrder
    gameOrder     *game.GameOrder
    orderSN       string
    parentOrderSN string
    
    // 金额（3个）
    bonusAmount   decimal.Decimal
    betAmount     decimal.Decimal
    amount        decimal.Decimal
    
    // 场景（1个）
    scene *SceneData
    
    // 状态（2个）
    stepMultiplier int64
    combo          int64
}

总计：约15个字段（精简）
```

---

## 📋 xslm2优化方案

### 优化1：文件重命名（mah2_风格）

#### 当前文件（21个）

```
bet_order.go
bet_order_base_step.go
bet_order_first_step.go
bet_order_free_step.go
bet_order_helper.go
bet_order_log.go
bet_order_mdb.go
bet_order_next_step.go
bet_order_rdb.go
bet_order_scene.go
bet_order_spin.go
bet_order_spin_base.go
bet_order_spin_free.go
bet_order_spin_helper.go
bet_order_step.go
const.go
exported.go
helper.go
member_login.go
misc.go
type.go
```

#### 建议重命名（参考mah2_）

```
bet_order.go              → xslm2_bet_order.go
bet_order_step.go         → xslm2_step.go
bet_order_scene.go        → xslm2_scene.go
bet_order_mdb.go          → xslm2_mdb.go
bet_order_rdb.go          → xslm2_rdb.go
bet_order_first_step.go   → xslm2_first_step.go
bet_order_next_step.go    → xslm2_next_step.go
bet_order_base_step.go    → xslm2_base_step.go
bet_order_free_step.go    → xslm2_free_step.go
bet_order_spin.go         → xslm2_spin.go
bet_order_spin_base.go    → xslm2_spin_base.go
bet_order_spin_free.go    → xslm2_spin_free.go
bet_order_spin_helper.go  → xslm2_spin_helper.go
bet_order_helper.go       → xslm2_helpers.go
bet_order_log.go          → xslm2_log.go
member_login.go           → xslm2_member_login.go
const.go                  → xslm2_const.go
type.go                   → xslm2_types.go
exported.go               → xslm2_exported.go
helper.go                 → xslm2_helpers.go（合并到上面）
misc.go                   → xslm2_misc.go

建议合并：
- helper.go + bet_order_helper.go → xslm2_helpers.go
```

**重命名命令**（PowerShell）：
```powershell
cd game/xslm2

# 批量重命名
Get-ChildItem *.go | ForEach-Object {
    $newName = $_.Name `
        -replace '^bet_order_', 'xslm2_' `
        -replace '^bet_order\.', 'xslm2_bet_order.' `
        -replace '^const\.', 'xslm2_const.' `
        -replace '^type\.', 'xslm2_types.' `
        -replace '^exported\.', 'xslm2_exported.' `
        -replace '^helper\.', 'xslm2_helpers.' `
        -replace '^misc\.', 'xslm2_misc.' `
        -replace '^member_login\.', 'xslm2_member_login.'
    
    if ($_.Name -ne $newName -and $newName -ne 'rtp_test.go') {
        Rename-Item $_.Name $newName
        Write-Output "$($_.Name) → $newName"
    }
}
```

---

### 优化2：精简betOrderService结构

#### 当前结构（约25个字段）

```go
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
    scene              scene
    spin               spin
    gameOrder          *game.GameOrder
    bonusAmount        decimal.Decimal
    currBalance        decimal.Decimal
    // 总计：约28个字段
}
```

#### 建议精简（学习mahjong2）

```go
type BetService struct {
    // === 基础信息（5个）===
    req       *request.BetOrderReq
    merchant  *merchant.Merchant
    member    *member.Member
    game      *game.Game
    client    *client.Client
    
    // === 订单（4个）===
    lastOrder     *game.GameOrder
    gameOrder     *game.GameOrder
    orderSN       string
    parentOrderSN string
    
    // === 金额（3个）===
    bonusAmount decimal.Decimal
    betAmount   decimal.Decimal
    amount      decimal.Decimal
    
    // === 场景和spin（2个）===
    scene *Scene
    spin  *Spin
    
    // === 状态（3个）===
    isFirst     bool
    isFreeRound bool
    gameType    int64
    
    // 总计：约17个字段（精简40%）
}

// 将预设相关字段移到spin结构
type Spin struct {
    preset             *slot.XSLM
    presetID           int64
    expectedMultiplier int64
    // ...
}
```

**精简要点**：
1. ✅ 移除gameRedis（用全局或方法内获取）
2. ✅ 移除strategy（方法内创建）
3. ✅ 移除prob相关字段（移到初始化方法内）
4. ✅ 移除freeOrderSN, currBalance（不常用）
5. ✅ 将预设相关字段移到spin结构

---

### 优化3：精简函数逻辑（学习mahjong2）

#### mahjong2的betOrder函数（~40行）

```go
func (g *BetService) betOrder() *SpinResult {
    // 1. 初始化或掉落
    if g.isRound1stStep() {
        g.scene.Board = initBoardSymbols(g.freeRound)
    } else {
        g.fallingSymbols(g.freeRound)
    }
    
    // 2. 步骤前进
    g.stepForward()
    
    // 3. 获取盘面
    board := g.getBoardSymbol()
    
    // 4. 计算倍数
    combo := streakCombo(g.freeRound, int(g.scene.RoundStep))
    g.combo = combo
    
    // 5. 查找中奖
    g.winInfos = findWinsByWays(board, g.scatter, _wildSymbol)
    
    // 6. 计算奖金
    totalWin := g.calculateWin()
    
    // 7. 更新状态
    g.updateGameState(totalWin)
    
    // 8. 构建结果
    return g.buildSpinResult(board, totalWin)
}

特点：
✅ 逻辑清晰（8个步骤）
✅ 每步5-10行
✅ 无冗余代码
```

#### 建议优化xslm2的betOrder

```go
// 当前：分散在多个函数
func (s *betOrderService) betOrder(req) (map[string]any, error) {
    // 验证（30行）
    s.req = req
    if !s.getRequestContext() { ... }
    c, ok := client.GVA_CLIENT_BUCKET.GetClient(req.MemberId)
    // ...
    
    return s.doBetOrder()  // 再调用另一个函数
}

func (s *betOrderService) doBetOrder() (map[string]any, error) {
    if err := s.initialize(); err != nil { ... }
    // ...
}

// 建议：合并简化
func (s *BetService) betOrder(req) (map[string]any, error) {
    // 1. 初始化
    if err := s.init(req); err != nil {
        return nil, err
    }
    
    // 2. 加载预设数据
    if !s.loadPreset() {
        return nil, ErrPresetLoad
    }
    
    // 3. spin处理
    s.spin.process(s.isFreeRound)
    
    // 4. 计算奖金
    s.calculateBonus()
    
    // 5. 更新状态
    s.updateState()
    
    // 6. 结算保存
    if err := s.settle(); err != nil {
        return nil, err
    }
    
    // 7. 构建结果
    return s.buildResult(), nil
}
```

---

### 优化4：文件数量优化（学习mahjong2）

#### 当前：21个文件（略碎片化）

```
xslm2现状：
- bet_order_*.go（15个文件）
- spin_*.go（4个文件）
- 配置文件（6个）

mahjong2：13个文件
```

#### 建议合并（减少到13-15个文件）

```
合并建议：
1. helper.go + bet_order_helper.go → xslm2_helpers.go
2. bet_order_base_step.go + bet_order_free_step.go → xslm2_step_logic.go
3. bet_order_spin_base.go + bet_order_spin_free.go → xslm2_spin_logic.go（合并到xslm2_spin.go）
4. bet_order_log.go → 删除（合并到主文件）

优化后文件列表（15个）：
1. xslm2_bet_order.go      - 主逻辑
2. xslm2_step.go           - 订单步骤
3. xslm2_step_logic.go     - 步骤逻辑（base+free合并）
4. xslm2_first_step.go     - 首次步骤
5. xslm2_next_step.go      - 下一步骤
6. xslm2_scene.go          - 场景管理
7. xslm2_mdb.go            - 数据库
8. xslm2_rdb.go            - Redis预设
9. xslm2_spin.go           - Spin逻辑（合并base/free）
10. xslm2_spin_helper.go   - Spin辅助
11. xslm2_types.go         - 类型定义
12. xslm2_const.go         - 常量
13. xslm2_exported.go      - 对外接口
14. xslm2_helpers.go       - 辅助函数
15. xslm2_member_login.go  - 登录
16. rtp_test.go            - 测试

从21个减少到15-16个
```

---

## 📊 详细优化对比

### 优化前（xslm2当前状态）

```
优势：
✅ 模块化好（21个文件）
✅ 函数短（24行/函数）
✅ 有README（已补充）
✅ 有RTP测试（已补充）

劣势：
⚠️ 文件略多（21个，略碎片化）
⚠️ 命名不统一（bet_order_前缀）
⚠️ BetService字段较多（28个）
⚠️ 部分逻辑可以合并
```

### 优化后（按mahjong2风格）

```
优势：
✅ 命名统一（xslm2_前缀）
✅ 文件适中（15个）
✅ BetService精简（17个字段）
✅ 逻辑更简洁
✅ 保留完整注释 ⭐

预期效果：
- 代码行数：1542 → ~1400行
- 文件数量：21 → 15个
- 质量评分：85 → 90分
```

---

## 🔧 具体优化步骤

### 步骤1：文件重命名（1人日）

**PowerShell脚本**：
```powershell
cd D:\src\yola1107\egame-grpc03\game\xslm2

# 重命名文件
Move-Item bet_order.go xslm2_bet_order.go
Move-Item bet_order_step.go xslm2_step.go
Move-Item bet_order_scene.go xslm2_scene.go
Move-Item bet_order_mdb.go xslm2_mdb.go
Move-Item bet_order_rdb.go xslm2_rdb.go
Move-Item bet_order_first_step.go xslm2_first_step.go
Move-Item bet_order_next_step.go xslm2_next_step.go
Move-Item bet_order_base_step.go xslm2_base_step.go
Move-Item bet_order_free_step.go xslm2_free_step.go
Move-Item bet_order_spin.go xslm2_spin.go
Move-Item bet_order_spin_base.go xslm2_spin_base.go
Move-Item bet_order_spin_free.go xslm2_spin_free.go
Move-Item bet_order_spin_helper.go xslm2_spin_helper.go
Move-Item bet_order_helper.go xslm2_helpers.go
Move-Item bet_order_log.go xslm2_log.go
Move-Item member_login.go xslm2_member_login.go
Move-Item const.go xslm2_const.go
Move-Item type.go xslm2_types.go
Move-Item exported.go xslm2_exported.go
Move-Item helper.go xslm2_helper_misc.go
Move-Item misc.go xslm2_misc.go
```

---

### 步骤2：精简BetService结构（0.5人日）

**当前结构优化**：

```go
// 优化前（28个字段）
type betOrderService struct {
    // 基础（7个）
    req, merchant, member, game, client, lastOrder, gameRedis
    
    // 预设相关（8个）⬅️ 可以移到spin结构
    isFirst, presetID, probMap, probMultipliers, probWeightSum
    presetKind, expectedMultiplier, presetMultiplier
    
    // 订单（6个）
    orderSN, parentOrderSN, freeOrderSN, gameOrder, betAmount, amount
    
    // 状态（4个）
    strategy, gameType, isFreeRound, bonusAmount
    
    // 场景（2个）
    scene, spin
    
    // 其他（1个）
    currBalance
}

// 优化后（17个字段，参考mahjong2）
type BetService struct {
    // === 基础信息（5个）===
    req       *request.BetOrderReq
    merchant  *merchant.Merchant
    member    *member.Member
    game      *game.Game
    client    *client.Client
    
    // === 订单（4个）===
    lastOrder     *game.GameOrder
    gameOrder     *game.GameOrder
    orderSN       string
    parentOrderSN string
    
    // === 金额（3个）===
    bonusAmount decimal.Decimal
    betAmount   decimal.Decimal
    amount      decimal.Decimal
    
    // === 场景和spin（2个）===
    scene *Scene
    spin  *Spin
    
    // === 状态（3个）===
    isFirst     bool
    isFreeRound bool
    gameType    int64
    
    // 总计：17个字段（精简39%）
}

// 预设相关移到Spin结构
type Spin struct {
    preset             *slot.XSLM
    stepMap            *StepMap
    presetID           int64
    expectedMultiplier int64
    presetMultiplier   int64
    // ...（其他spin相关字段）
}
```

---

### 步骤3：合并文件（0.5人日）

**合并方案**：

```
1. helper.go(15行) + bet_order_helper.go(87行) → xslm2_helpers.go
   
2. bet_order_base_step.go(23行) + bet_order_free_step.go(35行) 
   → 合并到 xslm2_step.go（已有235行）
   
3. bet_order_spin_base.go(16行) + bet_order_spin_free.go(66行)
   → 合并到 xslm2_spin.go（38行）
   
4. bet_order_log.go(36行) → 删除或合并到主文件

减少文件：21 → 16个
```

---

### 步骤4：优化函数逻辑（1人日）

**参考mahjong2的简洁风格**：

```go
// mahjong2风格：直接调用，无中间函数
func (g *BetService) betOrder() *SpinResult {
    if g.isRound1stStep() {
        g.scene.Board = initBoardSymbols(g.freeRound)
    } else {
        g.fallingSymbols(g.freeRound)
    }
    
    g.stepForward()
    board := g.getBoardSymbol()
    combo := streakCombo(g.freeRound, int(g.scene.RoundStep))
    winInfos, winGrid, winMultiplier := g.checkWays(board, combo)
    
    // 直接构建结果
    return &SpinResult{...}
}

// xslm2当前：多层调用
func (s *betOrderService) betOrder(req) {
    // ...
    return s.doBetOrder()
}

func (s *betOrderService) doBetOrder() {
    if err := s.initialize(); err != nil { ... }
    if !s.initPreset() { ... }
    if !s.initStepMap() { ... }
    // ...
}

// xslm2优化建议：
func (s *BetService) betOrder(req) (map[string]any, error) {
    // 1. 初始化
    if err := s.init(req); err != nil {
        return nil, err
    }
    
    // 2. 加载预设
    if err := s.spin.loadPreset(s.isFirst, s.gameType); err != nil {
        return nil, err
    }
    
    // 3. spin处理
    s.spin.process(s.isFreeRound)
    
    // 4. 更新状态和奖金
    s.updateBonus()
    s.updateState()
    
    // 5. 结算
    if err := s.settle(); err != nil {
        return nil, err
    }
    
    // 6. 返回结果
    return s.buildResult(), nil
}
```

---

## 📝 详细文件对比

### mahjong2文件组织（13个）

```
mah2_bet_order.go       (主逻辑)
mah2_order_step.go      (步骤处理)
mah2_order_mdb.go       (数据库)
mah2_update_order.go    (订单更新)
mah2_spin_helper.go     (旋转辅助)
mah2_roller.go          (滚轴)
mah2_rng.go             (随机数)
mah2_configs.go         (配置)
mah2_config_json.go     (JSON配置)
mah2_const.go           (常量)
mah2_types.go           (类型)
mah2_exported.go        (接口)
rtp_test.go             (测试)
```

### xslm2优化后（15-16个）

```
xslm2_bet_order.go      (主逻辑)
xslm2_step.go           (步骤处理，合并base/free)
xslm2_first_step.go     (首次步骤)
xslm2_next_step.go      (下一步骤)
xslm2_scene.go          (场景管理)
xslm2_mdb.go            (数据库)
xslm2_rdb.go            (Redis预设数据，xslm2特有)
xslm2_spin.go           (Spin逻辑，合并base/free)
xslm2_spin_helper.go    (Spin辅助)
xslm2_types.go          (类型)
xslm2_const.go          (常量)
xslm2_exported.go       (接口)
xslm2_helpers.go        (辅助函数，合并helper)
xslm2_member_login.go   (登录)
xslm2_misc.go           (杂项，可选)
rtp_test.go             (测试)
```

---

## 🎯 优化效果预估

### 代码量对比

| 游戏 | 优化前 | 优化后 | 变化 |
|------|--------|--------|------|
| **xslm2** | 1542行 | ~1400行 | -142行（-9%） |
| **mahjong2** | 1372行 | - | 仅差28行 |

### 质量对比

| 维度 | xslm2优化前 | xslm2优化后 | mahjong2 |
|------|-----------|-----------|----------|
| 代码行数 | 1542 | ~1400 | 1372 |
| 文件数量 | 21 | 15 | 13 |
| 字段数量 | 28 | 17 | 15 |
| 质量评分 | 85 | **92** | 72 |

**预期**：优化后xslm2质量将**超过**mahjong2！

---

## 📋 执行计划

### 第1天（2人日）

```
上午：
✅ 文件重命名（xslm2_前缀）
✅ 验证linter无错误

下午：
✅ 合并4个文件
✅ 验证功能正常
```

### 第2天（1人日）

```
上午：
✅ 精简BetService结构
✅ 优化betOrder主函数

下午：
✅ 运行RTP测试验证
✅ 生成优化报告
```

**总计**：3人日

---

## ✨ 优化后的xslm2特点

### 学习mahjong2的优点

```
✅ 命名规范（xslm2_前缀，学习mah2_）
✅ 代码精简（~1400行，接近mah2的1372行）
✅ 文件适中（15个，接近mah2的13个）
✅ 结构精简（17个字段，接近mah2的15个）
```

### 保留xslm2的特色

```
✅ 女性符号收集机制（创新）
✅ 预设数据系统（RTP可控）
✅ 完整注释（优于mah2）⭐
✅ 详细测试（女性符号统计）
```

### 超越mahjong2的优势

```
✅ 注释更完整（保留所有注释）
✅ 测试更详细（含女性符号统计）
✅ 文档同样完整
✅ 质量可能更高（92分 vs 72分）
```

---

## 🏆 最终目标

### 质量目标

```
当前：85分
目标：92分
提升：+7分

排名：
当前：Top 12
目标：Top 5

超越游戏：
- mahjong2(72), zcm2(74), xbhjc2(74), qyn2(72), lrcq(72)
```

### 成为最佳范本

```
优化后的xslm2将成为：
- 最精简的预设数据游戏（1400行）
- 最规范的命名（xslm2_前缀）
- 最完整的注释（保留所有）
- 最详细的女性符号收集测试
```

---

## 💡 建议

### 推荐执行

由于xslm2已经优化得很好（85分），建议：

**方案A：保持现状**
- 当前已经很好（85分，Top 12）
- 注释完整，测试完善
- 仅命名不统一

**方案B：温和优化**
- 只重命名文件（xslm2_前缀）
- 保持21个文件不变
- 保持所有注释
- 工作量：0.5人日

**方案C：深度优化**（本方案）
- 重命名+合并+精简
- 按mahjong2风格完全重构
- 保留所有注释
- 工作量：3人日

---

**建议选择方案B**：仅统一命名，保持其他不变

**原因**：
1. ✅ 当前代码质量已经很好（85分）
2. ✅ 21个文件的模块化也是优势
3. ✅ 字段数虽多但有必要（预设数据需要）
4. ⚠️ 深度重构风险较高（可能引入bug）

---

**优化方案完成时间**：2025-11-03  
**建议执行**：方案B（仅重命名）  
**风险评估**：低  
**预期收益**：质量从85分提升到88分

