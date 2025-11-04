# XSLM2 - 血色浪漫2

## 📋 游戏概述

**游戏ID**: `18892`  
**游戏名称**: 血色浪漫2 (XSLM2)  
**游戏类型**: 老虎机 (Slot Game)  
**包名**: `package xslm2`  
**总代码量**: ~1200 行  
**设计尺寸**: 756 x 1346 (竖版)

> **🔧 实现说明**: xslm2 基于 xslm 重构，采用**动态符号生成**方式，通过配置文件中的 `RollCfg` 和 `RealData` 实时生成符号网格，无需预设数据系统。功能与 xslm 保持一致。

---

## 🎭 游戏背景

### 故事设定
**哥特吸血鬼世界**，吸血鬼王想要复活他的三个妻子，并毁灭世界上的一切。

### 视觉风格
- **场景**: 夜晚，教堂内景，窗户上的彩绘玻璃描绘了吸血鬼王和他的三个妻子为祸人间的场景
- **风格**: 性感哥特华丽风
- **音乐**: 哥特风格纯音乐（管风琴风格）

---

## 🎮 核心游戏特性

### 1. 网格配置
- **网格布局**: **3×4×4×4×3** (5列，每列行数不同)
  - 第1列: 3行
  - 第2列: 4行
  - 第3列: 4行
  - 第4列: 4行
  - 第5列: 3行
- **中奖路数**: **576路** (3×4×4×4×3 = 576)
- **基础倍率**: 20倍 (`_baseMultiplier`)
- **玩法类型**: Ways 玩法（路数支付）
- **最小连线**: 3个符号连续

### 2. 核心机制 ⭐

#### 🎯 女性符号收集系统
- **3种女性符号**: Female A (7)、Female B (8)、Female C (9)
- **收集目标**: 每种符号收集10个
- **转换机制**: 达到10个后，该女性符号会转变为对应的女性百搭符号
- **触发条件**: 3种女性符号全部转变为百搭后，启用全屏消除模式

#### 🔄 消除机制
```
普通模式:
• 有百搭符号 + 女性符号中奖 → 两者都消失，下落补齐
• 有夺宝符号在场 → 百搭符号不消失

免费模式:
• 女性符号中奖 → 直接消失，下落补齐（无需百搭）
• 有百搭符号在场 → 百搭符号不消失
```

#### 🎁 免费游戏
- **触发条件**: 收集3/4/5个夺宝符号
- **免费次数**: 
  - 3个夺宝: **7次**
  - 4个夺宝: **10次**
  - 5个夺宝: **15次**
- **额外奖励**: 免费游戏中每收集1个夺宝符号 → 免费次数+1

---

## 🎲 符号系统

### 基础符号

| 符号ID | 名称 | 出现5个 | 出现4个 | 出现3个 | 说明 |
|--------|------|---------|---------|---------|------|
| 1 | 方块 | 5 | 3 | 2 | 普通符号 |
| 2 | 梅花 | 5 | 3 | 2 | 普通符号 |
| 3 | 红桃 | 5 | 3 | 2 | 普通符号 |
| 4 | 黑桃 | 5 | 3 | 2 | 普通符号 |
| 5 | 尖头木桩 | 912 | 68 | 35 | 高倍符号 |
| 6 | 十字架 | 15 | 10 | 56 | 特殊符号 |
| **7** | **女性A** | **2025** | **1215** | **810** | **可收集女性符号** |
| **8** | **女性B** | **2025** | **1215** | **810** | **可收集女性符号** |
| **9** | **女性C** | **2025** | **1215** | **810** | **可收集女性符号** |
| 10 | 女性A百搭 | 2540 | 1525 | 1015 | 女性A转换的百搭 |
| 11 | 女性B百搭 | 2540 | 1525 | 1015 | 女性B转换的百搭 |
| 12 | 女性C百搭 | 2540 | 1525 | 1015 | 女性C转换的百搭 |
| 13 | 百搭(吸血鬼王) | - | - | - | 替代所有符号(除夺宝) |
| **14** | **夺宝(血月)** | - | - | - | **触发免费游戏** |

### 特殊符号说明

#### 1. 百搭符号 (Wild)
- **形象**: 吸血鬼王，上边有"百搭"字样
- **出现位置**: **只出现在第2、3、4列**
- **替代规则**: 可替代除夺宝符号外的所有符号
- **消除规则**:
  - 普通模式: 与女性符号一起消失（除非有夺宝符号在场）
  - 免费模式: 不消失（即使有女性符号消除）

#### 2. 女性百搭符号
- **形象**: 女性A/B/C的吸血鬼新娘化，上边有"百搭"字样
- **转换条件**: 在免费游戏中，收集10个对应的女性符号
- **替代规则**: 可替代除夺宝和百搭外的所有符号
- **特殊机制**: 3种女性百搭全部出现后，启用全屏消除模式

#### 3. 夺宝符号 (Treasure)
- **形象**: 血月，上面有"夺宝"字样
- **出现限制**: **每列最多出现1个**
- **触发规则**:
  - 3个夺宝 → 7次免费游戏
  - 4个夺宝 → 10次免费游戏
  - 5个夺宝 → 15次免费游戏
- **额外奖励**: 免费游戏中每收集1个夺宝 → 免费次数+1

---

## 💰 投注系统

### 投注档位

#### 投注大小 (Bet Size)
```
共3个档位:
• 0.02
• 0.10
• 0.50
```

#### 投注倍数 (Bet Multiple)
```
共10个档位:
• 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
```

#### 基础投注 (Base Bat)
```
固定值: 20
```

### 投注金额计算表

| 倍数 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 |
|------|---|---|---|---|---|---|---|---|---|---|
| **0.02** | 0.40 | 0.80 | 1.20 | 1.60 | 2.00 | 2.40 | 2.80 | 3.20 | 3.60 | 4.00 |
| **0.10** | 2.00 | 4.00 | 6.00 | 8.00 | 10.00 | 12.00 | 14.00 | 16.00 | 18.00 | 20.00 |
| **0.50** | 10.00 | 20.00 | 30.00 | 40.00 | 50.00 | 60.00 | 70.00 | 80.00 | 90.00 | 100.00 |

**计算公式**:
```
投注金额 = 投注大小 × 投注倍数 × 基础投注(20)
```

### 加减 Bet 档位
```
0.4, 0.8, 1.2, 1.6, 2.0, 2.4, 2.8, 3.2, 3.6, 4.0,
6.0, 8.0, 10.0, 12.0, 14.0, 16.0, 18.0, 20.0,
30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0
```

**初始投注额**: 0.80

---

## 🎰 中奖与奖励

### 中奖规则

#### 1. 中奖路数
- **总路数**: **576路** (3×4×4×4×3)
- **路数计算**: 第1列个数 × 第2列个数 × 第3列个数 × 第4列个数 × 第5列个数

#### 2. 中奖判断
- **规则**: 从左到右，同一个符号连续3列或更多
- **最小连线**: 3列
- **Wild替代**: 百搭符号可替代除夺宝外的所有符号参与中奖

### 奖励计算

```
奖励 = 投注大小 × 投注倍数 × 返奖倍数

返奖倍数 = 符号倍率 × 路数

路数 = 第1列个数 × 第2列个数 × 第3列个数 × 第4列个数 × 第5列个数
```

**示例**:
```
投注: 0.10 × 5倍 × 20 = 10.00
中奖: 女性A 3个，路数 = 2×3×1 = 6路
倍率: 810 (女性A 3个的倍率)
奖励: 10.00 × 810 × 6 = 48,600.00
```

---

## 🎯 游戏玩法详解

### 普通模式玩法

#### 1. 基础规则
- **中奖不消除**: 单回合一把结束（无连消）
- **特殊消除**: 见下方说明

#### 2. 百搭符号消除规则
```
条件: 出现百搭符号 + 女性符号中奖
    ↓
执行: 百搭符号和女性符号都消失
    ↓
下落: 自然下落补齐
    ↓
检查: 如果补齐后仍有中奖且包含女性符号
    ↓
继续: 女性符号继续消除，继续补齐
    ↓
循环: 直到没有中奖
```

**特殊规则**: 如果屏幕上有夺宝符号，则百搭符号不消失，直到此次spin结束。

### 免费模式玩法

#### 1. 触发条件
- 收集3/4/5个夺宝符号
- 获得对应的免费次数

#### 2. 女性符号消除规则
```
条件: 女性符号中奖（无需百搭符号）
    ↓
执行: 女性符号直接消失
    ↓
下落: 自然下落补齐
    ↓
检查: 如果补齐后仍有中奖且包含女性符号
    ↓
继续: 女性符号继续消除，继续补齐
    ↓
循环: 直到没有中奖
```

**特殊规则**: 如果屏幕上有百搭符号，则百搭符号不消失，直到此次spin结束。

#### 3. 女性符号收集与转换
```
免费游戏开始
    ↓
每次Spin中奖的女性符号被收集
    ↓
独立计数器: [FemaleA计数, FemaleB计数, FemaleC计数]
    ↓
任意一种达到10个
    ↓
该女性符号 → 对应的女性百搭符号
    ↓
3种全部转换为女性百搭后
    ↓
🔥 启用全屏消除模式
```

#### 4. 全屏消除模式
```
启用条件: 3种女性符号全部转换为女性百搭
    ↓
执行规则:
  • 有女性百搭参与的中奖 → 所有中奖符号消失
  • 女性百搭符号会消失
  • 百搭符号不消失
  • 自然下落补齐
  • 继续查找中奖
  • 循环直到无中奖
```

#### 5. 夺宝符号额外奖励
- 免费游戏中每收集1个夺宝符号 → 免费次数 +1
- 夺宝符号在免费游戏中不消失

---

## 📝 Tips 提示语

### 普通模式 Tips

1. 高达1000倍赢奖！
2. 留意【夺宝符号】的神秘效果！
3. 收集3个【夺宝符号】赢得免费游戏！
4. 在夺宝模式中有机会获得更多的【百搭符号】！
5. 576中奖路！
6. 【百搭符号】出现时，【百搭符号】会连同中奖的【女性A】、【女性B】、【女性C】一起消失。
7. 【夺宝符号】出现时，即便有【女性A】、【女性B】、【女性C】符号中奖，【百搭符号】也不会消失。

### 出现2个夺宝符号时

1. 再来一个【夺宝符号】。

### 赢得免费旋转时

1. 赢得免费旋转！

### 免费模式 Tips

1. 免费模式中，【夺宝符号】不会消失。
2. 免费模式中，消除10个【女性A】，【女性A】会变为【女性A百搭】。
3. 免费模式中，消除10个【女性B】，【女性B】会变为【女性B百搭】。
4. 免费模式中，消除10个【女性C】，【女性C】会变为【女性C百搭】。

---

## 📁 代码结构

### 文件组织（20个文件）

```
xslm2/
├── 📄 导出接口
│   ├── exported.go              (44行) - BetOrder/MemberLogin 接口
│   └── member_login.go          (90行) - 用户登录逻辑
│
├── 📄 配置与常量
│   ├── const.go                 (43行) - 常量定义
│   ├── type.go                  (25行) - 数据类型
│   ├── misc.go                  (29行) - 符号倍率/免费次数配置
│   └── helper.go                (18行) - 随机种子生成
│
├── 📄 下注服务主逻辑
│   ├── bet_order.go             (136行) - 下注主入口
│   ├── bet_order_step.go        (224行) - Step 处理核心
│   └── bet_order_helper.go      (86行) - 下注辅助函数
│
├── 📄 初始化逻辑
│   ├── bet_order_first_step.go  (98行) - 首次 Step 初始化
│   ├── bet_order_next_step.go   (40行) - 后续 Step 处理
│   ├── bet_order_base_step.go   (22行) - 基础模式 Step
│   └── bet_order_free_step.go   (27行) - 免费模式 Step
│
├── 📄 Spin 旋转逻辑
│   ├── bet_order_spin.go        (38行) - Spin 主逻辑
│   ├── bet_order_spin_base.go   (15行) - 基础模式 Spin
│   ├── bet_order_spin_free.go   (53行) - 免费模式 Spin
│   └── bet_order_spin_helper.go (184行) - Spin 辅助（中奖查找）
│
├── 📄 数据管理
│   ├── bet_order_scene.go       (46行) - 场景数据管理
│   ├── bet_order_mdb.go         (62行) - MySQL 数据库操作
│   ├── bet_order_rdb.go         (93行) - Redis 操作 (Preset)
│   └── bet_order_log.go         (23行) - 日志记录
│
└── 📄 测试文件
    └── bet_test.go              - 单元测试
```

---

## 🔧 核心数据结构

### betOrderService (主服务)
```go
type betOrderService struct {
    // 请求上下文
    req              *request.BetOrderReq
    merchant         *merchant.Merchant
    member           *member.Member
    client           *client.Client
    
    // 游戏状态
    isFreeRound      bool          // 是否免费回合
    isFirst          bool          // 是否首次 Spin
    
    // Preset 系统
    presetID         int64         // 当前 Preset ID
    presetKind       int64         // Preset 类型 (0=base, 1=free)
    presetMultiplier int64         // Preset 预设倍率
    
    // 场景与 Spin
    scene            scene         // 场景数据
    spin             spin          // Spin 数据
    
    // 金额计算
    betAmount        decimal.Decimal
    bonusAmount      decimal.Decimal
}
```

### spin (游戏核心逻辑)
```go
type spin struct {
    preset                   *slot.XSLM     // Preset 数据
    stepMap                  *stepMap       // Step 映射
    symbolGrid               *int64Grid     // 符号网格 (3×4×4×4×3)
    winGrid                  *int64Grid     // 中奖网格
    
    // 女性符号收集系统 (免费游戏专用)
    femaleCountsForFree      [3]int64       // 当前计数 [A, B, C]
    nextFemaleCountsForFree  [3]int64       // 下一轮计数
    enableFullElimination    bool           // 是否启用全屏消除
    
    // 中奖数据
    winInfos                 []*winInfo     // 中奖信息
    winResults               []*winResult   // 中奖结果
    hasFemaleWin             bool           // 是否有女性符号中奖
    
    // 倍率与状态
    lineMultiplier           int64          // 中奖总倍率
    newFreeRoundCount        int64          // 新增免费次数
    isRoundOver              bool           // 回合是否结束
}
```

---

## 🔍 关键算法

### 1. 中奖查找算法 (Ways玩法)

```go
// 从左到右扫描，计算路数
func findNormalSymbolWinInfo(symbol int64) (*winInfo, bool) {
    exist := false
    lineCount := int64(1)
    var winGrid int64Grid
    
    // 按列扫描 (3×4×4×4×3)
    for col := 0; col < _colCount; col++ {
        count := 0
        for row := 0; row < getRowCount(col); row++ {  // 每列行数不同
            currSymbol := symbolGrid[row][col]
            if currSymbol == symbol || isWild(currSymbol) {
                if currSymbol == symbol {
                    exist = true
                }
                count++
                winGrid[row][col] = currSymbol
            }
        }
        
        if count == 0 {
            // 断线
            if col >= _minMatchCount && exist {
                return &winInfo{...}, true
            }
            break
        }
        
        lineCount *= count  // 累乘计算路数
    }
    
    return nil, false
}
```

### 2. 女性符号收集与转换

```go
// 免费游戏中收集中奖的女性符号
func updateFemaleCountForFree(symbol int64) {
    switch symbol {
    case _femaleA:
        if count[0] < 10 {
            count[0]++
            // 达到10个 → 转换为女性A百搭
            if count[0] == 10 {
                enableFemaleWildA = true
            }
        }
    case _femaleB:
        if count[1] < 10 {
            count[1]++
            if count[1] == 10 {
                enableFemaleWildB = true
            }
        }
    case _femaleC:
        if count[2] < 10 {
            count[2]++
            if count[2] == 10 {
                enableFemaleWildC = true
            }
        }
    }
    
    // 检查是否3种全部转换
    if enableFemaleWildA && enableFemaleWildB && enableFemaleWildC {
        enableFullElimination = true
    }
}
```

### 3. 消除机制实现

```go
// 普通模式消除
func processStepForBase() {
    if hasWildSymbol() && hasFemaleWin() {
        // 检查是否有夺宝符号
        if getTreasureCount() > 0 {
            // 有夺宝 → 百搭不消失
            removeFemaleSymbolsOnly()
        } else {
            // 无夺宝 → 百搭和女性符号都消失
            removeWildAndFemaleSymbols()
        }
        // 下落补齐
        fallDown()
        // 继续检查中奖
        continueCheckWin()
    }
}

// 免费模式消除
func processStepForFree() {
    if hasFemaleWin() {
        // 女性符号直接消失
        removeFemaleSymbols()
        
        // 检查是否有百搭符号
        if hasWildSymbol() {
            // 百搭不消失
        }
        
        // 下落补齐
        fallDown()
        // 继续检查中奖
        continueCheckWin()
    }
    
    // 全屏消除模式
    if enableFullElimination {
        // 有女性百搭参与的中奖 → 全部消失
        removeAllWinSymbols()
        // 女性百搭消失，百搭不消失
        // 下落补齐，继续循环
    }
}
```

---

## 📊 Preset 系统

### Preset 类型

| 类型ID | 名称 | 说明 |
|--------|------|------|
| 0 | Normal Base | 基础游戏预设 |
| 1 | Normal Free | 免费游戏预设 |

### Preset 选择流程

```
1. 查询动态概率配置 (MySQL)
   ↓
2. 计算期望倍率
   ↓
3. 根据期望倍率选择 Preset
   ↓
4. 从 Redis Hash 加载 Preset 数据
   ↓
5. 根据 lastMapID 选择当前 StepMap
```

---

## 🗄️ 数据库设计

### MySQL 表

#### 1. **member** - 玩家信息
```sql
-- 表名: egame.member
-- 金币字段: balance (float64)

SELECT id, member_name, balance, currency, merchant
FROM egame.member
WHERE id=? AND merchant=?;
```

#### 2. **game** - 游戏基本信息
```sql
SELECT id, game_name, game_type, status
FROM egame.game
WHERE id=18892;
```

#### 3. **game_dynamic_prob** - 动态概率配置
```sql
SELECT merchant_id, prob_multiplier, prob_weight
FROM egame.game_dynamic_prob
WHERE game_id=18892 AND merchant_id=?;
```

### Redis 缓存

```
Key 格式:
• Preset 数据: "{site}:slot_xslm_data" (Hash)
  - Field: presetID
  - Value: JSON(Preset完整数据)

• Preset ID 池: "{site}:slot_xslm_id:{merchantID}:{memberID}" (Set)
  - Members: presetID 列表

过期时间: 90天
```

---

## 🎮 API 接口

### 1. 下注接口

```go
func (g *Game) BetOrder(req *request.BetOrderReq) (map[string]any, error)
```

**请求参数**:
```json
{
  "merchantId": 20020,
  "memberId": 1,
  "gameId": 18892,
  "baseMoney": 0.02,
  "multiple": 1
}
```

**返回数据**:
```json
{
  "orderSN": "1234567890",
  "symbolGrid": [[7,8,9,1,2], ...],
  "winGrid": [[7,0,9,0,0], ...],
  "winResults": [
    {
      "symbol": 7,
      "symbolCount": 3,
      "lineCount": 6,
      "baseLineMultiplier": 810,
      "totalMultiplier": 4860
    }
  ],
  "betAmount": 0.4,
  "bonusAmount": 194.4,
  "isFree": true,
  "freeNum": 8,
  "femaleCountsForFree": [3, 5, 2]
}
```

### 2. 登录接口

```go
func (g *Game) MemberLogin(req *pb.LoginStreamReq, c *client.Client) (string, error)
```

---

## 🛠️ 配置说明

### 常量配置

```go
const (
    _gameID                             = 18892  // 游戏ID
    _baseMultiplier                     = 20     // 基础倍率
    _rowCount                           = 4      // 最大行数
    _colCount                           = 5      // 列数
    _minMatchCount                      = 3      // 最小连线数
    _triggerTreasureCount               = 3      // 触发免费的 Treasure 数
    _femaleSymbolCountForFullElimination = 10    // 全屏消除阈值
)
```

### 免费次数配置

```go
var _freeRounds = []int64{7, 10, 15}
//                         ↑  ↑   ↑
//                         3个 4个 5个 Treasure
```

### 符号倍率配置

```go
// 注意: 代码中的倍率表需要与设计文档中的倍率表对应
// 设计文档中的倍率表:
// 女性A/B/C: 5个=2025, 4个=1215, 3个=810
// 女性A/B/C百搭: 5个=2540, 4个=1525, 3个=1015
```

---

## 🎯 游戏流程详解

### 完整下注流程

```
1. 参数验证
   ├─ 获取商户信息 (MySQL)
   ├─ 获取玩家信息 (MySQL)
   ├─ 获取游戏信息 (MySQL)
   └─ 检查余额

2. 初始化
   ├─ 判断是否首次 Spin
   ├─ 初始化策略 (Strategy)
   ├─ 加载或初始化 Preset
   └─ 备份场景数据

3. 选择 Preset
   ├─ 查询动态概率配置 (MySQL)
   ├─ 计算期望倍率
   ├─ 从 Redis 随机选择 Preset
   └─ 加载 Preset 的 SpinMaps

4. 执行 Spin
   ├─ 加载 StepMap (预设符号)
   ├─ 转换为符号网格 (3×4×4×4×3)
   ├─ 查找中奖组合 (576路)
   ├─ 计算倍率和奖金
   └─ 处理消除逻辑

5. 结算
   ├─ 更新玩家余额
   ├─ 保存订单到 MySQL
   ├─ 保存场景到 Redis
   └─ 返回结果
```

### 免费游戏流程

```
触发: 基础游戏发现 ≥3 个夺宝符号
    ↓
计算免费次数:
  • 3个夺宝 → 7次
  • 4个夺宝 → 10次
  • 5个夺宝 → 15次
    ↓
设置 freeNum
    ↓
免费模式循环:
  ├─ 执行免费 Spin
  ├─ 女性符号中奖 → 直接消除
  ├─ 收集女性符号计数
  ├─ 检查是否达到10个 → 转换女性百搭
  ├─ 检查是否3种全部转换 → 启用全屏消除
  ├─ 检查夺宝符号 → 免费次数+1
  ├─ freeNum--
  └─ 直到 freeNum=0 或结束
```

---

## 📈 性能优化

### 1. 对象池
```go
// 随机数生成器池
var randPool = sync.Pool{
    New: func() interface{} {
        return rand.New(rand.NewSource(getSeed()))
    },
}
```

### 2. Redis 缓存
- Preset 数据缓存 90 天
- 减少数据库查询
- 快速 Preset 选择

### 3. 并发控制
```go
// 玩家级别锁
c.BetLock.Lock()
defer c.BetLock.Unlock()
```

---

## 🔐 安全机制

### 1. 余额检查
```go
func checkBalance() bool {
    return gamelogic.CheckMemberBalance(betAmount, member)
}
```

### 2. 参数验证
- 商户验证
- 玩家验证
- 游戏状态验证

### 3. 事务安全
```go
// 场景备份与恢复机制
backupScene()    // 执行前备份
// ... 执行业务逻辑
restoreScene()   // 失败时恢复
```

---

## 🧪 测试指南

### 单元测试
```bash
cd game/xslm2
go test -v
```

### 测试覆盖的功能
- [ ] 符号中奖查找 (576路)
- [ ] Wild 符号替代逻辑
- [ ] 女性符号收集机制
- [ ] 女性符号转换 (10个 → 女性百搭)
- [ ] 全屏消除触发
- [ ] 免费游戏流程
- [ ] 消除机制 (普通模式 vs 免费模式)
- [ ] 夺宝符号额外奖励

---

## 🛠️ 调试与日志

### 日志级别

| 级别 | 使用场景 |
|------|---------|
| Error | Preset加载失败、数据库错误、状态不一致 |
| Warn | 余额不足、参数异常 |
| Info | 下注成功、中奖信息、女性符号收集 |

### 关键日志点

```go
// 1. Preset 加载
global.GVA_LOG.Error("initPreset", 
    zap.Int64("presetID", presetID))

// 2. 女性符号收集
global.GVA_LOG.Info("femaleCollection",
    zap.Int64s("counts", femaleCountsForFree),
    zap.Bool("enableFullElimination", enableFullElimination))

// 3. 消除机制
global.GVA_LOG.Info("elimination",
    zap.Bool("isFreeRound", isFreeRound),
    zap.Bool("hasWild", hasWildSymbol()),
    zap.Int64("treasureCount", getTreasureCount()))
```

---

## 📚 技术栈

### 核心依赖
- **Go**: 1.18+ (支持泛型)
- **GORM**: ORM 框架
- **Redis**: v8 客户端
- **Decimal**: 高精度金额计算
- **Zap**: 结构化日志
- **jsoniter**: 高性能 JSON

### 数据库
- **MySQL**: 玩家、订单、配置数据
- **Redis**: Preset 缓存、场景状态

---

## 🎨 游戏特色

### 1. 独特的网格布局 🎯
- **3×4×4×4×3** 非对称网格
- 576路中奖组合
- 百搭符号只出现在中间3列

### 2. 双模式消除机制 💎
- **普通模式**: 百搭+女性符号一起消除
- **免费模式**: 女性符号独立消除
- **全屏消除**: 3种女性百搭全部出现后启用

### 3. 女性符号转换系统 🎲
- 收集10个 → 转换为女性百搭
- 3种全部转换 → 全屏消除模式
- 独特的渐进式奖励机制

---

## 📖 开发指南

### 修改免费次数
```go
// 修改 misc.go
var _freeRounds = []int64{7, 10, 15}
//                         ↑  ↑   ↑
//                         3个 4个 5个 Treasure
```

### 修改全屏消除阈值
```go
// const.go
const _femaleSymbolCountForFullElimination = 10  // 改为其他值
```

### 调整倍率表
```go
// 需要与设计文档中的倍率表保持一致
// 设计文档: 女性A/B/C 5个=2025, 4个=1215, 3个=810
```

---

## 🔗 相关游戏参考

| 游戏 | 游戏ID | 相似特性 |
|------|--------|---------|
| **XXG2** | 18891 | Ways玩法、Treasure触发免费 |
| **JZTDMM** | 18895 | 金符号系统、消除机制 |
| **Mahjong2** | - | 消除玩法、符号下落 |

---

## 📞 技术支持

**项目**: egame-grpc03  
**模块**: game/xslm2  
**维护状态**: Active Development  
**Go版本**: 1.18+  
**最后更新**: 2025-11  

---

## 📄 许可证

本项目为内部游戏项目，版权所有。

---

## 🎯 快速开始

### 本地运行
```bash
# 1. 启动服务
make run

# 2. 测试下注
curl -X POST http://localhost:8819/bet \
  -d '{"gameId":18892,"baseMoney":0.02,"multiple":1}'

# 3. 查看日志
tail -f logs/xslm2.log
```

### Docker 部署
```bash
# 构建镜像
make docker

# 重建容器
make recreate
```

---

**注意**: 本文档基于代码分析和设计文档生成，游戏设计细节以产品文档为准。
