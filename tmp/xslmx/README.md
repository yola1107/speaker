# XSLM2 - 血色浪漫2

## 📋 基本信息

| 项目 | 值 |
|------|---|
| **游戏ID** | 18892 |
| **类型** | 老虎机 (Slot) |
| **网格** | 4行×5列 |
| **玩法** | 576 Ways |

---

## 🎲 符号赔率表（从低到高）

| ID | 符号 | 3连 | 4连 | 5连 |
|----|------|-----|-----|-----|
| 1 | 方块 ♦ | 2 | 3 | 5 |
| 2 | 梅花 ♣ | 2 | 3 | 5 |
| 3 | 红桃 ♥ | 2 | 3 | 5 |
| 4 | 黑桃 ♠ | 2 | 3 | 5 |
| 5 | 木桩 | 3 | 6 | 9 |
| 6 | 十字架 | 5 | 10 | 15 |
| **7** | **女性A** | **8** | **12** | **20** |
| **8** | **女性B** | **8** | **12** | **20** |
| **9** | **女性C** | **8** | **12** | **20** |
| 10 | 女性A百搭 | 10 | 15 | 25 |
| 11 | 女性B百搭 | 10 | 15 | 25 |
| 12 | 女性C百搭 | 10 | 15 | 25 |
| 13 | 🎭 **百搭** | - | - | - |
| 14 | 🌙 **夺宝** | - | - | - |

---

## ⚙️ 符号替换规则

### 基本规则

```
百搭(13): 可替换 1-12 所有符号

女性百搭(10-12):
  • 可替换基础符号 1-9（包括普通符号1-6和女性符号7-9）
  • 女性百搭之间不能相互替换（10不能替换11、12等）
  • 查找符号10-12时，只能被百搭(13)替换，不能被其他女性百搭替换
```

### ⚠️ 特殊规则（重要）

由于女性百搭(10-12)可能出现在任意列，不同于普通Ways玩法，需要特别处理：

#### 规则1: 女性百搭可以替换基础符号(1-9)

**示例1：查找符号1时，女性百搭可以替换**
```
     列0  列1  列2  列3  列4
行0: 13   10    1    1    1      ← 查找符号1时
行1: 13   10    1    1    1         列0: 13(百搭) ✓
行2: 13   10    1    1    1         列1: 10(女性百搭) ✓ (可以替换符号1)
行3:  1    1    1    1    1         列2: 1(真正的符号1) ✓

判断: 列0-2连续，且有真正的符号1出现
结果: 符号1中奖 ✓ (3列, Ways=3×3×3=27)
```

**示例2：查找符号10时，百搭可以替换**
```
     列0  列1  列2  列3  列4
行0: 13   13   10    1    1      ← 查找符号10时
行1: 13   13   10    1    1         列0: 13(百搭) ✓
行2: 13   13   10    1    1         列1: 13(百搭) ✓
行3:  1    1    1    1    1         列2: 10(真正的女性百搭) ✓

判断: 列0-2连续，且有真正的符号10出现
结果: 符号10中奖 ✓ (3列, Ways=3×3×3=27)
```

**关键**: 
- 查找符号1-9时，使用 `exist` 标志确保至少有一个真正的目标符号出现
- 查找符号10-12时，不需要 `exist` 标志（因为本身就是目标符号）

#### 规则2: 女性百搭不能相互替换

```
10 不能替换 11、12
11 不能替换 10、12
12 不能替换 10、11

但 13(百搭) 可以替换 10、11、12
```

**示例：女性百搭互不替换**
```
     列0  列1  列2  列3  列4
行0: 10   11   12    1    1      ← 查找符号10时
行1: 10   11   12    1    1         列0: 10 ✓
行2: 10   11   12    1    1         列1: 11 ✗ (不能替换)
行3:  1    1    1    1    1         断线

结果: 符号10不中奖 (只有1列)
```

#### 规则3: 符号替换详细规则

```
查找符号1-9(基础符号，包括普通符号1-6和女性符号7-9):
  • 可被 13(百搭) 替换
  • 可被 10-12(女性百搭) 替换（任何女性百搭都可以替换任何基础符号）
  • 必须至少有一个真正的目标符号出现 (exist标志)

查找符号10-12(女性百搭):
  • 可被 13(百搭) 替换
  • 不可被其他女性百搭替换（10不能替换11、12等）
  • 不需要exist标志（因为本身就是目标符号）
```

**注意**：根据代码实现，女性百搭(10-12)可以替换所有基础符号(1-9)，不仅仅是对应的女性符号。

---

## 🎮 游戏规则

### 基础规则

**中奖方式**: 576 Ways（从左到右连续出现3个及以上相同符号）

**投注计算**: `投注金额 = 基础金额 × 倍数 × 20`

**示例**: 基础0.02 × 倍数5 × 20 = **2.00**

---

### 🔄 消除机制详解

#### 一、基础模式消除流程

**消除规则**：

1. **检查是否有女性符号中奖**。若无，按普通消除处理（不触发连消）。
2. **若女性符号中奖**：
   - **触发条件**：盘面上有百搭符号（`hasWildSymbol(s.symbolGrid)`）
   - **消除条件**：中奖路径上有百搭符号（`infoHasBaseWild(w.WinGrid)`）
   - **消除内容**：
     - **无夺宝符号时**：中奖的女性符号和所有百搭都会消除
     - **有夺宝符号时**：仅消除中奖女性符号，百搭保留
   - 消除后同时保留 Wild（规则描述中强调"百搭留在屏幕上"）
3. **连续清除**直到不再有女性符号中奖或无法继续；每次消除后按顺序下落、补位。
4. **若过程中出现 3/4/5 个夺宝符号**，对应赠送 7/10/15 个免费次数。**赠送免费次数后，当前回合结束后应进入免费模式流程**。

**关键机制**：
- **一次请求一个Step**: 一次 BetOrder 调用只处理一个 Step
- **客户端轮询**: 客户端检查 `isRoundOver`，如果为 false 需要再次调用
- **连消不扣费**: 连消 Step 的 `amount=0`
- **消除下落**: 消除中奖符号 → 符号下落 → 顶部填充新符号
- **计算所有中奖奖金**（包括普通符号、女性符号等），但只消除女性符号(7-9)和百搭(13)

---

#### 二、免费模式消除流程

**免费次数赠送**（依据 `doc/xslm2_rules.md`）：
- 单个夺宝符号即赠送 1 次免费次数（连消结束时统计）
- 免费局开始时女性收集状态保留

**消除规则**：

1. **消除仅统计女性符号及女性百搭参与的中奖**
2. **若检测到"全屏消除"条件**（三种女性分别都已收集 ≥10），进入全屏模式：
   - 有女性百搭的中奖符号全部消除（含女性百搭本体）
   - 若回合后续补落仍有女性参与中奖，则继续连消
   - 每个连消回合记录女性收集数量
3. **普通免费模式**（三种女性未都≥10）：
   - 中奖女性符号(7-9)及对应女性百搭(10-12)消除
   - 野生百搭(13)不消除（在免费模式中可替换女性符号参与中奖，但不参与消除）

**女性符号收集**（依据 `doc/xslm2_rules.md`）：
- 每次连消后统计中奖网格中真实女性符号（7/8/9），每种最多记 10
- 当某一女性达到 10 个时，在后续回合中其普通符号转化为对应女性百搭
- 免费模式中，若三种女性符号都达到 10 个，则进入"全屏消除"状态

**免费模式结束条件**（依据 `doc/xslm2_rules.md`）：
- 连续连消直到无女性符号参与中奖或补落后不再有女性中奖为止
- 每次回合结束时统计夺宝数量并赠送额外次数
- 免费次数用尽或场景重置时将女性收集状态归零

**两种模式对比**:

| 项目 | 部分消除 (女性<10) | 全屏消除 (女性都≥10) |
|------|-------------------|---------------------|
| **消除内容** | 中奖女性符号(7-9)及对应女性百搭(10-12) | 有女性百搭参与的中奖路径上的所有符号（除百搭13外） |
| **继续条件** | 有女性符号中奖 | 有任意符号中奖（含女性百搭） |
| **百搭符号(13)** | 不消除 | 不消除（规则强调"百搭留在屏幕上"） |
| **夺宝符号** | 不消除，留在原地 | 不消除，留在原地 |

---

#### 三、免费游戏触发

**基础模式触发**:

| 夺宝数量 | 免费次数 |
|---------|---------|
| 3个 | 7次 |
| 4个 | 10次 |
| 5个 | 15次 |

**免费模式额外奖励**:
- 每出现1个夺宝符号 → 免费次数+1
- 回合结束时统计夺宝数量

**场景数据持久化**:
```
进入免费游戏时:
  • 从Redis读取女性符号收集进度
  • 从Redis读取NextSymbolGrid和SymbolRollers
  • 恢复到 spin 结构体

免费游戏进行中:
  • 每个Step结束后保存收集进度到Redis
  • 保存NextSymbolGrid（已消除下落填充的网格）
  • 保存SymbolRollers（包含BoardSymbol和End位置）
  • 确保断线重连后数据不丢失

状态恢复机制:
  • 优先使用NextSymbolGrid（向后兼容）
  • 从SymbolRollers.BoardSymbol恢复并验证一致性
  • 如果不一致会panic（确保数据完整性）

免费游戏结束后:
  • 清空Redis中的收集数据
  • 重置计数为 [0,0,0]
  • 清空NextSymbolGrid和SymbolRollers
```

---

## 🏗️ 技术实现

### 核心特性

#### 1. 动态符号生成 ⭐

**替代 Redis 预设数据**，使用配置驱动：

```go
// 基础模式: 使用 roll_cfg.base
// 免费模式: 根据女性符号收集状态选择配置
//   "000" → 三种都未满10
//   "001" → 只有C满10
//   "111" → 三种都满10（全屏消除）

symbolGrid := _cnf.initSpinSymbol(isFreeRound, femaleCounts)
```

#### 2. 场景数据持久化

```go
// Redis 存储结构
type SpinSceneData struct {
    FemaleCountsForFree [3]int64              // [A计数, B计数, C计数]
    NextSymbolGrid      *int64Grid            // 下一step的符号网格（已消除下落填充）
    SymbolRollers       *[_colCount]SymbolRoller  // 滚轴状态（包含BoardSymbol）
    RollerKey           string                // 滚轴配置key（基础=base / 免费=收集状态）
}

// Key: game:scene:{gameId}:{memberId}
// TTL: 90天
```

**状态恢复机制**:
- **优先使用 `NextSymbolGrid`**: 向后兼容，直接恢复网格
- **从 `SymbolRollers.BoardSymbol` 恢复**: 主要状态存储，包含当前网格符号
- **一致性验证**: 恢复时会验证 `BoardSymbol` 与 `NextSymbolGrid` 是否一致
- **坐标转换**: `BoardSymbol` 从下往上存储，需要转换为标准的 `symbolGrid` 坐标系统

#### 3. 滚轴配置切换

免费模式根据收集状态动态选择配置：

| 收集状态 | Key | 说明 |
|---------|-----|------|
| A=5,B=3,C=7 | "000" | 都未满10 |
| A=10,B=5,C=7 | "100" | A满10 |
| A=10,B=10,C=7 | "110" | A/B满10 |
| A=10,B=10,C=10 | "111" | 全满10（全屏消除）|

#### 4. 滚轴状态管理 ⭐

**SymbolRoller 结构**:
```go
type SymbolRoller struct {
    Real        int              // 使用的 RealData 索引
    Start       int              // 当前起始位置（会递减）
    End         int              // 当前最后一个符号的位置（用于获取下一个符号）
    Col         int              // 列索引 (0-4)
    BoardSymbol [_rowCount]int64 // 当前网格的符号（从下往上存储）
}
```

**关键方法**:
- **`getFallSymbol()`**: 从滚轴获取下一个符号，使用 `End` 位置循环获取（`nextPos = (End + 1) % len`）
- **`ringSymbol()`**: 补充掉下来导致的空缺位置（填充 `BoardSymbol` 中的 0）

**坐标系统**:
- `symbolGrid`: 标准坐标系统，`[0][col]` 为顶部，`[3][col]` 为底部
- `BoardSymbol`: 从下往上存储，`[0]` 为底部，`[3]` 为顶部
- 转换函数: `handleSymbolGrid()` 和 `fallingWinSymbols()` 处理坐标转换  

### 文件结构

```
xslm2/
├── xsl2_const.go              // 常量定义（符号ID、网格尺寸）
├── xsl2_config.go             // 配置管理（动态符号生成、赔率表、滚轴方法）
├── xsl2_config_json.go        // 游戏配置JSON（滚轴数据、权重）
├── xsl2_spin.go               // 核心逻辑（Spin、中奖查找、消除下落）
├── xsl2_order.go              // 下注流程（BetOrder入口）
├── xsl2_order_scene.go        // 场景数据管理（Redis持久化）
├── xsl2_order_step.go         // Step处理（奖金计算、免费游戏）
├── xsl2_util.go               // 工具函数（包含网格辅助函数：坐标转换、一致性验证）
├── rtp_test.go                // RTP压测（1000万局）
├── rtp_benchmark_test.go      // RTP基准测试（快速验证）
├── README.md                  // 游戏文档（本文件）
└── CODE_SAFETY_REPORT.md      // 代码安全检查报告
```

### 核心流程详解

#### 服务端处理流程（单个Step）

```
┌─────────────────────────────────────────────────────┐
│ 1. 接收请求 (BetOrder)                               │
│    - 获取用户信息、商户信息、游戏配置                 │
│    - 加锁防止并发                                     │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│ 2. 判断Step类型                                      │
│    - 检查上一个订单状态                               │
│    - isFirst = (lastOrder == nil || IsRoundOver)    │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│ 3. 加载场景数据 (reloadScene)                        │
│    - 从Redis读取: FemaleCountsForFree、NextSymbolGrid│
│    - 恢复女性符号收集进度                             │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│ 4. 初始化 & 扣费                                     │
│    - 首次Step: amount = baseMoney × multiple × 20   │
│    - 连消Step: amount = 0 (不扣费)                  │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│ 5. 执行baseSpin (核心逻辑)                           │
│    ① 获取符号网格                                    │
│      • isFirst=true: 动态生成新网格                  │
│      • isFirst=false: 使用NextSymbolGrid（已处理）   │
│    ② 查找中奖 (findWinInfos)                        │
│      • 扫描符号1-9（普通+女性）                       │
│      • 扫描符号10-12（女性百搭）                     │
│    ③ 判断消除模式（execEliminateGrid）              │
│      • 基础: hasFemaleWin → 调用fillElimBase（检查中奖路径上是否有百搭）│
│      • 免费部分: hasFemaleWin → 调用fillElimFreePartial│
│      • 免费全屏: enableFullElimination && hasFemaleWildWin → 调用fillElimFreeFull│
│      • 如果消除数量=0，返回cascadeModeNone（不触发消除）│
│    ④ 计算中奖结果（updateStepResults）              │
│      • 基础模式连消: 计算所有中奖奖金（包括普通符号、女性符号等）│
│      • 但只消除女性符号(7-9)和百搭(13)，普通符号保留 │
│      • 免费模式: 计算所有中奖符号奖金                 │
│    ⑤ 消除下落填充（如果未结束）                      │
│      • fillElimBase/fillElimFreePartial/fillElimFreeFull: 标记消除位置│
│      • dropSymbols: 符号下落填补空位                 │
│      • fillBlanks: 顶部填充新符号（使用getFallSymbol，更新End位置）│
│      • convertFemaleToWild: 免费模式转换女性符号为百搭│
│      • fallingWinSymbols: 更新SymbolRoller.BoardSymbol（在processStep中调用）│
│      • 保存到nextSymbolGrid和SymbolRollers供下一step使用│
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│ 6. 更新订单 & 结算                                   │
│    - 计算奖金: bonusAmount = betAmount / BaseBat × stepMultiplier│
│    - 如果stepMultiplier=0，重置bonusAmount=0         │
│    - 更新余额                                         │
│    - 保存订单到数据库                                 │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│ 7. 保存场景数据 (saveScene)                          │
│    - 保存女性符号计数到Redis                          │
│    - 保存NextSymbolGrid和SymbolRollers（如果未结束）│
│    - SymbolRollers包含BoardSymbol（主要状态存储）    │
│    - 免费游戏结束时清空场景                           │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│ 8. 返回结果给客户端                                  │
│    - symbolGrid: 原始网格（用于动画）                │
│    - winResults: 中奖详情                            │
│    - isRoundOver: 是否结束（客户端判断是否继续）     │
└─────────────────────────────────────────────────────┘
```

#### 消除下落填充流程（execEliminateGrid）

```
输入: s.symbolGrid (当前step的网格), s.winInfos (中奖信息)

Step 1: 复制网格
┌──────────────────────────────────────┐
│ nextGrid = *s.symbolGrid (深拷贝)    │
│ clearBlockedCells(&nextGrid)         │
└──────────────────────────────────────┘

Step 2: 标记消除位置 (fillElimBase/fillElimFreePartial/fillElimFreeFull)
┌──────────────────────────────────────┐
│ 根据消除模式标记消除位置:             │
│   • 基础模式: 只标记女性符号(7-9)和百搭(13)│
│   • 免费部分: 标记女性符号(7-12)      │
│   • 免费全屏: 标记所有非百搭符号      │
│   • 检查保护规则:                     │
│     - 夺宝符号: 不消除                │
│     - 百搭符号: 基础模式有夺宝时不消除 │
│   • 标记: newGrid[r][c] = _eliminated│
│                                      │
│ 返回: eliminatedCount (消除数量)     │
└──────────────────────────────────────┘

Step 3: 符号下落 (dropSymbols)
┌──────────────────────────────────────┐
│ 按列处理（列0到列4）:                 │
│   从顶部往底部扫描:                   │
│     • _eliminated → 转换为_blank      │
│     • 非空符号 → 移到底部writePos     │
│     • 空白符号保留在顶部               │
│     • 列0和列4的writePos从1开始（跳过阻塞位）│
│                                      │
│ 结果: 空白符号全部移到顶部            │
└──────────────────────────────────────┘
        ↓ 示例:
    消除前      下落后
    0  1  2    1  2  3  ← 非空符号下移
    0  2  3    0  0  0  ← 空白在顶部
    1  0  4

Step 4: 填充新符号 (fillBlanks)
┌──────────────────────────────────────┐
│ 遍历网格:                             │
│   如果 newGrid[r][c] == _blank:      │
│     • 使用SymbolRoller.getFallSymbol()│
│     • 从RealData循环获取下一个符号     │
│     • 更新SymbolRoller.End位置（nextPos = (End + 1) % len）│
│     • newGrid[r][c] = 新符号          │
│     • 返回填充数量（应与消除数量相等） │
└──────────────────────────────────────┘

Step 5: 转换女性符号 (convertFemaleToWild，仅免费模式)
┌──────────────────────────────────────┐
│ 如果免费模式且部分/全屏消除:          │
│   遍历网格:                           │
│     • 如果女性符号计数≥10             │
│     • 将对应的女性符号转换为女性百搭   │
│     • 例如：A满10 → 7转换为10         │
└──────────────────────────────────────┘

输出: nextGrid (处理完成的网格), mode (消除模式)

注意: fallingWinSymbols 在 processStep 中调用（不在 execEliminateGrid 内部）
┌──────────────────────────────────────┐
│ processStep 中:                       │
│   • 调用 execEliminateGrid 得到 nextGrid│
│   • 调用 fallingWinSymbols 更新 BoardSymbol│
│   • 坐标转换：从symbolGrid到BoardSymbol │
│   • BoardSymbol从下往上存储             │
│   • 调用ringSymbol确保无空白符号        │
└──────────────────────────────────────┘
```

---

## 🧪 测试

### RTP测试

```bash
cd game/xslm2
go test -run TestRtp
```

**测试规模**:
- 1000万局（10,000,000局）
- 基础模式 + 免费模式
- 预计耗时: 10-20分钟（取决于CPU）

**测试报告内容**:
```
【基础模式统计】
  总局数: 10000000
  总投注: 200000000.00
  总奖金: xxxxx.xx
  RTP: xx.xx%
  中奖局数: xxxxx (xx.xx%)
  平均连消步数: x.xx
  最大连消步数: xx

【连消机制统计】
  Wild触发连消: xxxxx次 (xx.xx%)
  女性+Wild组合: xxxxx次 (xx.xx%)

【夺宝符号统计】
  3个夺宝: xxxxx次 → 预期xxxxx次免费
  4个夺宝: xxxxx次 → 预期xxxxx次免费
  5个夺宝: xxxxx次 → 预期xxxxx次免费
  免费触发次数: xxxxx (xx.xx%)

【免费模式统计】
  总局数: xxxxxxx
  总奖金: xxxxx.xx
  RTP: xx.xx%
  中奖局数: xxxxx (xx.xx%)
  有连消局数: xxxxx (xx.xx%)
  无连消局数: xxxxx (xx.xx%)

【全屏消除统计】
  触发次数: xxxxx
  触发率: xx.xx%

【免费次数核算】
  理论总免费次数: xxxxx (基础xxxxx + 额外xxxxx)
  实际玩的免费次数: xxxxx
  差异: xx (xx.xx%)

【总计】
  总投注金额: 200000000.00
  总奖金金额: xxxxx.xx
  总回报率(RTP): xx.xx%
  基础贡献: xx.xx% | 免费贡献: xx.xx%
```

### 调试开关

```go
// rtp_test.go
const (
    testRounds       = 1e7   // 测试局数
    progressInterval = 1e5   // 进度输出间隔
    debugFileOpen    = false // 调试文件输出（详细信息）
)

// 开启调试输出
debugFileOpen = true  // 会生成 logs/xslm2_rtp_yyyymmdd_hhmmss.txt
```

**调试文件内容**:
- 每一局的符号网格
- 中奖信息详情
- 连消步骤追踪
- 女性符号收集进度
- 全屏消除触发记录

---

## 🚀 快速开始

### 基础调用

```go
req := &request.BetOrderReq{
    GameId:     18892,
    BaseMoney:  0.02,
    Multiple:   5,
}
result, err := xslm2.BetOrder(req)
```

### 返回结果说明

**示例：基础模式触发消除**

```json
{
  "orderSN": "1734567890123456",
  "currentBalance": 98.50,
  "baseBet": 0.02,
  "multiplier": 5,
  "betAmount": 2.0,
  "symbolGrid": [
    [99, 7, 7, 7, 99],
    [1, 13, 1, 1, 2],
    [13, 1, 13, 1, 2],
    [1, 1, 1, 1, 2]
  ],
  "winGrid": [
    [0, 7, 7, 7, 0],
    [0, 13, 0, 0, 0],
    [13, 0, 13, 0, 0],
    [0, 0, 0, 0, 0]
  ],
  "winResults": [
    {
      "symbol": 7,
      "symbolCount": 3,
      "lineCount": 27,
      "baseLineMultiplier": 8,
      "totalMultiplier": 216
    },
    {
      "symbol": 1,
      "symbolCount": 4,
      "lineCount": 12,
      "baseLineMultiplier": 3,
      "totalMultiplier": 36
    },
    {
      "symbol": 2,
      "symbolCount": 5,
      "lineCount": 9,
      "baseLineMultiplier": 5,
      "totalMultiplier": 45
    }
  ],
  "bonusAmount": 29.70,
  "stepMultiplier": 297,
  "lineMultiplier": 297,
  "isRoundOver": false,
  "hasFemaleWin": true,
  "isFreeRound": false,
  "newFreeRoundCount": 0,
  "totalFreeRoundCount": 0,
  "remainingFreeRoundCount": 0,
  "femaleCountsForFree": [0, 0, 0],
  "nextFemaleCountsForFree": [0, 0, 0],
  "enableFullElimination": false,
  "treasureCount": 0,
  "spinBonusAmount": 29.70,
  "freeBonusAmount": 0.0,
  "roundBonus": 29.70
}
```

**说明**：
- `symbolGrid`: 4×5符号网格，行0为顶部，行3为底部。**注意**：`symbolGrid[0][0]` 和 `symbolGrid[0][4]` 位置固定为 `99`（墙格标记，不可消除）
- `winGrid`: 中奖标记网格，非0值表示中奖符号，0表示未中奖。**注意**：墙格位置（[0][0] 和 [0][4]）不会参与中奖计算
- `blocked`: 4×5布尔网格，标识哪些位置是墙格（不可消除、不参与中奖）。`true` 表示墙格，`false` 表示正常格子
- `winResults`: 包含所有中奖符号的详情（女性符号7、普通符号1和2都中奖）
- `bonusAmount`: 本Step奖金 = betAmount / BaseBat × stepMultiplier = 2.0 / 20 × 297 = 29.70
- `stepMultiplier`: 所有中奖的倍数总和 = 216 + 36 + 45 = 297
- `isRoundOver`: false 表示需要继续调用（触发消除，继续连消）
- `hasFemaleWin`: true 表示有女性符号中奖，且中奖路径上有百搭，触发消除
- `treasureCount`: 夺宝符号数量，本示例为0（未触发免费游戏）

### 字段说明

| 字段 | 类型 | 说明 |
|------|-----|------|
| `symbolGrid` | `[][]int64` | 符号网格（4×5），墙格位置固定为99 |
| `winGrid` | `[][]int64` | 中奖标记网格（0=未中奖，非0=中奖符号） |
| `blocked` | `[][]bool` | 墙格标记网格（true=墙格，false=正常格子） |
| `winResults` | `[]WinResult` | 中奖详情数组 |
| `bonusAmount` | `float64` | 本Step奖金 |
| `stepMultiplier` | `int64` | 本Step倍数 |
| `isRoundOver` | `bool` | **关键**: true=回合结束，false=需继续调用 |
| `hasFemaleWin` | `bool` | 是否有女性符号中奖 |
| `isFreeRound` | `bool` | 是否免费回合 |
| `femaleCountsForFree` | `[3]int64` | 女性符号收集进度 [A, B, C] |
| `enableFullElimination` | `bool` | 是否触发全屏消除 |

### 客户端集成示例

```go
// 客户端轮询逻辑
func playRound(req *request.BetOrderReq) {
    for {
        result, err := xslm2.BetOrder(req)
        if err != nil {
            log.Error("BetOrder failed", err)
            break
        }
        
        // 显示动画
        displayAnimation(result.SymbolGrid, result.WinGrid)
        displayWinAmount(result.BonusAmount)
        
        // 检查是否结束
        if result.IsRoundOver {
            log.Info("Round finished")
            break
        }
        
        // 继续下一step
        log.Info("Cascade continues...")
        time.Sleep(500 * time.Millisecond) // 动画延迟
    }
}
```

---

## ⚠️ 注意事项

### 1. 并发安全
- 所有BetOrder调用已加锁（`c.BetLock.Lock()`）
- 同一用户的请求会串行处理
- 无需额外并发控制

### 2. 状态一致性
- **必须检查 `isRoundOver`**: 客户端根据此字段判断是否继续
- **场景数据TTL**: Redis中的场景数据90天有效期
- **断线重连**: 场景数据持久化，支持断线重连

### 3. 扣费机制
- **首次Step**: 扣除完整投注金额
- **连消Step**: 不扣费（`amount=0`）
- **免费游戏**: 完全不扣费

### 4. 性能优化
- 使用对象池 (`sync.Pool`) 管理随机数生成器
- 动态符号生成替代Redis预设数据
- 单次请求时间 < 10ms（正常情况）

### 5. 已知限制
- 最大连消步数: 理论无限，实际RTP测试中最大约20步
- 女性符号收集上限: 每种10个（>=10触发转换）
- 夺宝符号: 1-5个有效，超过5个按5个计算

---

## 🔧 故障排查

### 问题1: 消除不触发
**检查**:
- 基础模式：是否有女性符号(7-9)中奖？中奖路径上是否有百搭符号(13)？
- 免费模式：是否有女性符号中奖（部分消除）或任意符号中奖（全屏消除）？
- 注意：基础模式需要中奖路径上有百搭，不仅仅是盘面上有百搭

### 问题2: 消除后卡住
**检查日志**:
- `WARN: no symbols eliminated but round not over` → 已自动强制结束
- `ERROR: nextGrid is nil in non-first step` → 场景数据异常
- `panic: BoardSymbol 恢复的网格与 nextGrid 不一致` → 状态不一致，需要检查数据完整性

### 问题3: 女性符号计数错误
**检查**:
- 只有中奖路径上的女性符号(7-9)才计入收集
- 女性百搭(10-12)不参与收集，只参与消除
- 在 `fillElimFreePartial` 中通过 `tryCollectFemaleSymbol` 收集
- 计数上限为10，达到10后不再增加

### 问题4: RTP偏差过大
**正常范围**: ±2% (例如: 设计96% → 实际94%-98%)
**检查**:
- 测试局数是否足够（建议1000万局）
- 基础模式和免费模式RTP分别统计
- 查看连消触发率、全屏消除触发率

---
