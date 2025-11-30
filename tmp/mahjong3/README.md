# Mahjong（麻将胡了）游戏模块技术文档

- 文档地址：https://v32od9.axshare.com/

## 📋 目录

- [游戏概述](#-游戏概述)
- [目录结构](#-目录结构)
- [技术架构](#-技术架构)
- [核心模块详解](#-核心模块详解)
- [数据结构](#-数据结构)
- [游戏流程](#-游戏流程)
- [游戏机制](#-游戏机制)
- [配置说明](#-配置说明)
- [错误处理](#-错误处理)
- [开发指南](#-开发指南)

---

## 🎮 游戏概述

**Mahjong（麻将胡了）** 是一款基于麻将主题的老虎机游戏，采用"Ways玩法"（路数玩法）和瀑布式消除机制。游戏支持金符号系统、免费游戏模式、连续中奖倍数累积等特性。

### 核心特性

| 特性 | 描述 |
|------|------|
| **游戏ID** | 18943 |
| **网格布局** | 5行 × 5列 |
| **有效行数** | 前4行参与中奖计算 |
| **玩法类型** | Ways玩法（路数玩法）|
| **符号种类** | 10种（8种麻将牌 + 胡符号 + 百搭符号）|
| **金符号系统** | 普通符号可转化为金符号（+10），与原符号等价 |
| **中奖机制** | 从左至右连续3列及以上相同符号 |
| **百搭符号** | 符号10（wild），可替代任意普通符号 |
| **胡符号** | 符号9（treasure），3个及以上触发免费游戏 |
| **基础倍数** | 20倍 |

### 麻将符号

| 符号 | 名称 | 值 | 金符号 |
|------|------|-----|--------|
| 🀇 | 二条 | 1 | 11 |
| 🀙 | 二筒 | 2 | 12 |
| 🀇 | 五条 | 3 | 13 |
| 🀙 | 五筒 | 4 | 14 |
| 🀇 | 八万 | 5 | 15 |
| 🀆 | 白板 | 6 | 16 |
| 🀄 | 红中 | 7 | 17 |
| 🀅 | 发财 | 8 | 18 |
| 🎴 | 胡符号 | 9 | - |
| ⭐ | 百搭 | 10 | - |

### 游戏模式

| 模式 | 状态码 | 描述 |
|------|--------|------|
| **普通游戏** | 1 | 基础游戏模式 |
| **普通消除** | 11 | 普通游戏中奖后的消除状态 |
| **免费游戏** | 21 | 免费游戏模式 |
| **免费消除** | 22 | 免费游戏中奖后的消除状态 |

### Ways玩法说明

**Ways玩法**是一种创新的中奖计算方式：
- 不使用传统的固定支付线
- 从第1列开始，连续列中只要有相同符号即可中奖
- 中奖路数 = 每列中奖符号数量的乘积
- 例如：第1列2个、第2列3个、第3列2个 → 路数 = 2×3×2 = 12

---

## 📁 目录结构

```
game/mahjong/
├── 📄 核心业务文件
│   ├── exported.go                 # 对外接口（BetOrder、MemberLogin）
│   ├── spin_start.go               # 下注主流程
│   ├── spin_base.go                # 核心旋转逻辑
│   ├── spin_first_step.go          # 首次spin处理
│   ├── spin_helper.go              # 旋转辅助函数
│   ├── bet_order_step.go           # 订单和步骤处理
│   ├── bet_order_next_step.go      # 下一步骤处理
│   └── bet_order_scene.go          # 场景数据管理（Redis缓存）
├── 📄 配置和类型定义
│   ├── types.go                    # 数据结构定义
│   ├── const.go                    # 常量定义
│   ├── configs.go                  # 游戏配置加载
│   ├── game_json.go                # 游戏配置JSON数据
│   └── mahjong.json                # 游戏配置文件
├── 📄 用户和数据库
│   ├── member_login.go             # 用户登录处理
│   └── bet_order_mdb.go            # 数据库操作
├── 📄 工具和辅助
│   ├── helper_func.go              # 辅助函数
│   └── misc.go                     # 杂项工具函数
└── 📄 测试
    └── rtp_test.go                 # RTP测试
```

---

## 🏗️ 技术架构

### 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                        gRPC 接口层                           │
│                  (exported.go - BetOrder)                   │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                     核心服务层                                │
│                  (betOrderService)                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │  场景管理    │  │  旋转逻辑    │  │  结算逻辑    │     │
│  │ Scene/Redis  │  │  Spin/Drop   │  │  Order/Money │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                      数据持久层                               │
│    ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│    │   MySQL DB   │  │  Redis Cache │  │    Client    │   │
│    │  (订单/用户) │  │  (场景数据)  │  │  (内存状态)  │   │
│    └──────────────┘  └──────────────┘  └──────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 核心技术栈

- **语言**: Go 1.18+
- **RPC框架**: gRPC
- **数据库**: MySQL（订单、用户数据）
- **缓存**: Redis（场景数据）
- **日志**: Zap
- **JSON**: jsoniter（高性能序列化）
- **数值计算**: decimal（精确金额计算）

### 核心服务对象

**betOrderService** - 下注服务核心结构体

```go
type betOrderService struct {
    // 请求上下文
    req                *request.BetOrderReq  // 用户请求
    merchant           *merchant.Merchant    // 商户信息
    member             *member.Member        // 用户信息
    game               *game.Game            // 游戏信息
    client             *client.Client        // 用户上下文
    lastOrder          *game.GameOrder       // 上一订单
    
    // 场景数据
    scene              *SpinSceneData        // 场景数据（核心）
    gameRedis          *redis.Client         // 游戏Redis
    
    // 订单和金额
    gameOrder          *game.GameOrder       // 当前订单
    bonusAmount        decimal.Decimal       // 奖金金额
    betAmount          decimal.Decimal       // 下注金额
    amount             decimal.Decimal       // 扣费金额
    
    // 订单号
    orderSN            string                // 订单号
    parentOrderSN      string                // 父订单号
    freeOrderSN        string                // 触发免费的订单号
    
    // 游戏状态
    stepMultiplier     int64                 // Step倍数
    isRoundFirstStep   bool                  // 是否为第一step
    isSpinFirstRound   bool                  // 是否为Spin的第一回合
    isRoundOver        bool                  // 一轮是否结束
    removeNum          int64                 // 消除次数
    gameMultiple       int64                 // 游戏倍数
    
    // 配置和符号
    gameConfig         *gameConfigJson       // 配置数据
    symbolGrid         int64Grid             // 符号网格（5×5）
    winGrid            int64Grid             // 中奖网格
    nextSymbolGrid     int64Grid             // 下一把符号网格
    reversalSymbolGrid int64Grid             // 反转符号网格
    reversalWinGrid    int64GridW            // 反转中奖网格（4×5）
    winInfos           []*winInfo            // 中奖信息
}
```

---

## 🔍 核心模块详解

### 1. 对外接口层（exported.go）

#### BetOrder() - 下注接口
```go
func (g *Game) BetOrder(req *request.BetOrderReq) (result map[string]any, err error)
```

**功能**: 统一处理普通和免费游戏下注请求

**返回数据**：
```go
{
    "win":            // 当前Step赢
    "cards":          // 盘面符号
    "wincards":       // 中奖网格
    "betMoney":       // 下注额
    "balance":        // 余额
    "free":           // 是否在免费
    "freeNum":        // 剩余免费次数
    "freeTotalMoney": // 当前round赢
}
```

**特性**:
- 包含panic恢复机制
- 简洁的返回结构
- 完整的错误处理

#### MemberLogin() - 登录接口
```go
func (g *Game) MemberLogin(req *pb.LoginStreamReq, c *client.Client) (result string, err error)
```

### 2. 下注主流程（spin_start.go）

#### betOrder() - 核心下注方法
```go
func (s *betOrderService) betOrder(req *request.BetOrderReq) (*SpinResultC, error)
```

**执行流程**：
```
1. 获取请求上下文（商户、用户、游戏）
2. 获取客户端并加锁
3. 加载上一订单
4. 判断是否首次（初始化场景）
5. 加载场景数据（从Redis）
6. 执行baseSpin() 核心旋转
7. 构建SpinResultC结果
8. 更新游戏订单
9. 结算步骤
10. 保存场景数据
11. 返回结果
```

### 3. 核心旋转逻辑（spin_base.go）⭐

#### baseSpin() - 核心旋转方法
```go
func (s *betOrderService) baseSpin() (*BaseSpinResult, error)
```

**核心流程**：

```
1. 初始化（initialize）

2. 判断运行状态（普通/免费）
   ├─ 免费游戏：扣除免费次数
   └─ 普通游戏：正常流程

3. 判断是否为Round第1个Step
   ├─ 是：initSpinSymbol() 初始化符号
   └─ 否：使用场景中的符号

4. Steps计数器++

5. handleSymbolGrid() - 处理符号网格

6. checkSymbolGridWin() - 检查Ways中奖
   ├─ 计算路数（Ways数量）
   ├─ 获取赔率
   └─ 计算倍数

7. 反转网格（reverseSymbolInPlace）

8. 判断是否中奖
   ├─ 有中奖：
   │   ├─ 计算倍数（线倍数 × 游戏倍数）
   │   ├─ 更新奖金金额
   │   ├─ moveSymbols() 移动符号
   │   ├─ fallingWinSymbols() 掉落补充
   │   ├─ removeNum++ （消除次数增加）
   │   ├─ 设置Next = true（继续）
   │   └─ 状态变为消除状态（11或22）
   └─ 未中奖：
       ├─ Steps = 0（重置）
       ├─ removeNum = 0（重置）
       ├─ 检查Scatter符号
       ├─ 触发免费游戏（如果满足条件）
       ├─ 设置RoundOver = true
       └─ 判断是否SpinOver

9. 返回BaseSpinResult
```

**关键特性**：
- 支持普通和免费游戏模式
- 瀑布式消除机制
- Ways玩法中奖计算
- 自动触发免费游戏
- 消除次数递增倍数

### 4. 中奖检查（spin_helper.go）

#### checkSymbolGridWin() - Ways中奖检查
```go
func (s *betOrderService) checkSymbolGridWin() []*winInfo
```

**算法逻辑**：
```go
1. 遍历每一行（0-3行）作为起点
2. 跳过特殊符号（treasure、已检查的符号）
3. 从左至右检查连续列：
   - 统计每列中相同符号的数量
   - 支持金符号（原符号+10）
   - 支持百搭符号（wild）
   - 一列无符号则中断
4. 连续列数 >= 3 时中奖：
   - 获取符号赔率（从PayTable）
   - 计算路数 = 每列符号数乘积
   - 计算倍数 = 赔率 × 路数
5. 返回中奖信息数组
```

#### moveSymbols() - 移动符号
```go
func (s *betOrderService) moveSymbols() int64Grid
```
- 将中奖符号设置为0
- 金符号转换为百搭
- 返回移动后的网格

#### fallingWinSymbols() - 符号掉落
```go
func (s *betOrderService) fallingWinSymbols(grid int64Grid, stage int8)
```
- 收集非空符号下移
- 从滚轴顶部补充新符号
- 应用金符号转换

### 5. 场景数据管理（bet_order_scene.go）

#### SpinSceneData - 场景数据结构
```go
type SpinSceneData struct {
    SymbolRoller      [][]int64  // 符号滚轴
    Steps             int64      // 步骤计数
    Stage             int8       // 阶段（1/11/21/22）
    NextStage         int8       // 下一阶段
    IsFreeRound       bool       // 是否免费回合
    RoundOver         bool       // 回合是否结束
    RoundMultiplier   int64      // 回合倍数
    SpinMultiplier    int64      // Spin倍数
    FreeMultiplier    int64      // 免费倍数
    RemoveNum         int64      // 移除次数
    GameWinMultiple   int64      // 游戏中奖倍数
}
```

#### reloadScene() - 加载场景数据
- 从Redis加载JSON数据
- 反序列化为SpinSceneData结构

#### saveScene() - 保存场景数据
- 序列化SpinSceneData为JSON
- 保存到Redis（过期时间90天）

#### cleanScene() - 清理场景数据
- 从Redis删除场景数据
- 用于游戏结束或重置

### 6. 订单处理（bet_order_step.go）

#### initialize() - 初始化
```go
func (s *betOrderService) initialize() error
```

**流程**：
```
1. 判断是否首次Spin
   ├─ 首次：
   │   ├─ 更新下注金额
   │   ├─ 检查余额
   │   ├─ amount = betAmount
   │   └─ 初始化场景数据
   └─ 后续：
       ├─ 恢复下注配置
       └─ amount = 0（不扣费）

2. 设置客户端下注金额

3. 生成订单号

4. 返回成功
```

#### updateGameOrder() - 更新游戏订单
```go
func (s *betOrderService) updateGameOrder(result *BaseSpinResult) bool
```

**填充字段**：
- 基础信息：商户、用户、游戏
- 倍数信息：基础倍数20、线倍数、游戏倍数
- 金额信息：下注金额、奖金金额、余额
- 订单号：orderSN、parentOrderSN、freeOrderSN
- 游戏数据：胡符号数、免费次数等

#### settleStep() - 结算步骤
```go
func (s *betOrderService) settleStep() error
```
- 创建游戏订单记录
- 调用`SaveTransfer()`执行转账
- 更新用户余额
- 保存到数据库

### 7. 配置加载（configs.go）

#### gameConfigJson - 配置结构
```go
type gameConfigJson struct {
    PayTable           [][]int   // 支付表
    BaseGameMulti      []int64   // 基础游戏倍数
    FreeGameMulti      []int64   // 免费游戏倍数
    GoldSymbolBaseProb int       // 金符号基础概率
    GoldSymbolFreeProb int       // 金符号免费概率
    FreeGameMin        int64     // 触发免费最小数量
    FreeGameTimes      int64     // 免费游戏次数
    FreeGameAddTimes   int64     // 每个胡符号增加次数
    RollCfg            struct {
        Base  []int     // 基础滚轴
        Free  []int     // 免费滚轴
    }
}
```

#### initGameConfigs() - 初始化配置
- 从game_json.go加载配置
- 解析为gameConfigJson结构
- 保存到服务实例

---

## 📊 数据结构

### BaseSpinResult - 旋转结果
```go
type BaseSpinResult struct {
    lineMultiplier    int64       // 线倍数
    stepMultiplier    int64       // 总倍数
    scatterCount      int64       // 胡符号个数
    addFreeTime       int64       // 增加免费次数
    freeTime          int64       // 免费次数
    gameMultiple      int64       // 游戏倍数
    bonusHeadMultiple int64       // 实际倍数
    bonusTimes        int64       // 总消除次数
    SpinOver          bool        // Spin是否结束
    winGrid           int64GridW  // 中奖网格（4×5）
    cards             int64Grid   // 盘面符号（5×5）
    nextSymbolGrid    int64Grid   // 下一符号网格
    winInfo           WinInfo     // 中奖信息
    winResult         []CardType  // 中奖结果
}
```

### WinInfo - 中奖信息
```go
type WinInfo struct {
    Next           bool       // 是否继续
    Over           bool       // 是否结束
    Multi          int64      // 倍数
    State          int8       // 状态（0普通/1免费）
    FreeNum        uint64     // 剩余免费次数
    FreeTime       uint64     // 已用免费次数
    TotalFreeTime  uint64     // 总免费次数
    FreeMultiple   int64      // 免费倍数
    IsRoundOver    bool       // 回合是否结束
    AddFreeTime    int64      // 增加免费次数
    ScatterCount   int64      // 胡符号数量
    WinGrid        int64GridW // 中奖网格
    NextSymbolGrid int64Grid  // 下一符号网格
    WinArr         []WinElem  // 中奖数组
}
```

### SpinResultC - 最终返回结果
```go
type SpinResultC struct {
    Balance    float64   // 余额
    BetAmount  float64   // 下注额
    CurrentWin float64   // 当前Step赢
    AccWin     float64   // 当前round赢
    TotalWin   float64   // 总赢
    Free       int       // 是否在免费
    Review     int       // 回顾
    Sn         string    // 注单号
    LastWinId  uint64    // 上次中奖ID
    MapId      uint64    // 地图ID
    WinInfo    WinInfo   // 中奖信息
    Cards      int64Grid // 盘面符号
    RoundBonus float64   // 回合奖金
}
```

### winInfo - 单次中奖信息
```go
type winInfo struct {
    Symbol      int64     // 符号
    SymbolCount int64     // 符号数量
    LineCount   int64     // 路数
    Odds        int64     // 赔率
    Multiplier  int64     // 倍数
    WinGrid     int64Grid // 中奖网格
}
```

### CardType - 卡片类型
```go
type CardType struct {
    Type     int  // 牌型（符号类型）
    Way      int  // 路数（Ways数量）
    Multiple int  // 倍数（赔率）
    Route    int  // 连续列数
}
```

---

## 🎯 游戏流程

### 完整游戏流程图

```
┌─────────────────────────────────────────────────────────────┐
│                      用户发起下注请求                          │
│                 (BaseMoney, Multiple)                       │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                     1. 初始化检查                             │
│   • 获取请求上下文（商户、用户、游戏）                          │
│   • 获取客户端对象并加锁                                       │
│   • 加载上一订单                                              │
│   • 判断是否首次（初始化场景）                                 │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  2. 加载场景数据（Redis）                      │
│   • 从Redis加载SpinSceneData                                 │
│   • 解析SymbolRoller、Steps、Stage等                         │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  3. initialize 初始化                         │
│   • 判断是否首次Spin                                          │
│   ├─ 首次：检查余额、初始化金额、扣费                          │
│   └─ 后续：从上一订单加载配置、不扣费                          │
│   • 免费游戏：扣除免费次数                                     │
│   • 生成订单号                                                │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  4. baseSpin 核心旋转                         │
│                                                              │
│   ① 判断是否Round首Step                                       │
│      ├─ 是：initSpinSymbol() 初始化符号滚轴                   │
│      └─ 否：使用场景中的符号                                  │
│                                                              │
│   ② Steps++                                                  │
│                                                              │
│   ③ handleSymbolGrid() - 处理符号网格                         │
│                                                              │
│   ④ checkSymbolGridWin() - Ways中奖检查                      │
│      • 遍历每行作为起点                                        │
│      • 从左至右检查连续列                                      │
│      • 计算路数 = 每列符号数乘积                               │
│      • 计算倍数 = 赔率 × 路数                                 │
│                                                              │
│   ⑤ reverseSymbolInPlace() - 反转网格                        │
│                                                              │
│   ⑥ 判断是否中奖                                              │
│      ├─ 有中奖：                                              │
│      │   ├─ 计算倍数（线倍数 × 游戏倍数）                      │
│      │   ├─ 更新奖金金额                                      │
│      │   ├─ moveSymbols() 移动符号                            │
│      │   ├─ fallingWinSymbols() 掉落补充                      │
│      │   ├─ removeNum++ （倍数递增）                          │
│      │   ├─ 设置Next = true                                  │
│      │   └─ 状态变为消除状态（11或22）                         │
│      └─ 未中奖：                                              │
│          ├─ Steps = 0、removeNum = 0                         │
│          ├─ getScatterCount() 统计胡符号                      │
│          ├─ 触发免费游戏（如果≥配置数量）                       │
│          ├─ 设置RoundOver = true                             │
│          └─ 判断SpinOver                                      │
│                                                              │
│   ⑦ 返回BaseSpinResult                                       │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  5. 构建SpinResultC                           │
│   • 封装Balance、BetAmount、CurrentWin等                      │
│   • 设置WinInfo、Cards等                                      │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  6. 订单处理和结算                             │
│   • updateGameOrder() 创建并填充订单                          │
│   • settleStep() 执行转账和保存订单                            │
│   • saveScene() 保存场景到Redis                               │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    7. 返回结果                                │
│   • SpinResultC（盘面、中奖、余额、免费次数等）                │
│   • 前端展示游戏动画和结果                                    │
└─────────────────────────────────────────────────────────────┘
```

### 关键流程说明

#### A. 瀑布式消除流程
```
初始盘面 → checkSymbolGridWin() 检查Ways中奖
    ↓
第1次中奖 → 标记中奖符号 → moveSymbols() → fallingWinSymbols()
    ↓ (removeNum = 1, gameMultiple = BaseGameMulti[1])
第2次中奖 → 标记中奖符号 → moveSymbols() → fallingWinSymbols()
    ↓ (removeNum = 2, gameMultiple = BaseGameMulti[2])
第3次中奖 → ...（循环）
    ↓ (removeNum = 3, gameMultiple = BaseGameMulti[3])
未中奖 → 检查胡符号 → 回合结束
    ↓
触发免费游戏？
    ├─ 是：进入免费游戏模式
    └─ 否：返回普通游戏
```

**游戏倍数递增**：
```go
removeNum = 0: gameMultiple = BaseGameMulti[0]  // 例如 1倍
removeNum = 1: gameMultiple = BaseGameMulti[1]  // 例如 2倍
removeNum = 2: gameMultiple = BaseGameMulti[2]  // 例如 5倍
removeNum = 3: gameMultiple = BaseGameMulti[3]  // 例如 10倍（最大）
```

#### B. 免费游戏触发流程
```
普通游戏 → 出现≥3个胡符号（Scatter）
    ↓
计算免费次数：
  基础次数（如12次）
  + 额外次数 × (胡符号数 - 3)
  例如：5个胡符号 → 12 + 2×2 = 16次
    ↓
设置FreeNum = 计算的次数
State = runStateFreeGame（免费游戏）
Stage = _spinTypeFree（21）
    ↓
进入免费游戏模式：
  • 不扣除下注金额
  • 使用免费滚轴配置
  • 使用免费游戏倍数（FreeGameMulti）
    ↓
每次spin：
  ├─ IncrFreeTimes() 增加已用次数
  └─ Decr() 减少剩余次数
    ↓
免费次数用完 → 返回普通游戏
```

#### C. 免费游戏中再次触发
```
免费游戏中出现≥3个胡符号
    ↓
增加免费次数（不重置）
    ↓
Incr(addFreeTimes) 累加到剩余次数
    ↓
继续免费游戏（累积）
```

---

## 🎲 游戏机制

### 1. Ways玩法机制

**定义**：从左至右连续列中包含相同符号即可中奖，不需要固定支付线。

**计算公式**：
```
路数 = 第1列符号数 × 第2列符号数 × ... × 第N列符号数
倍数 = 符号赔率 × 路数
奖金 = 基础下注 × 用户倍数 × 倍数 × 游戏倍数
```

**示例1 - 简单中奖**：
```
盘面（前4行参与）：
列1  列2  列3  列4  列5
 7    7    3    4    2
 3    7    5    7    1
 2    4    7    9    3
 3    3    2    7    4
 7    7    7    7    7  (第5行不参与)

符号7中奖：
- 第1列：2个（第1行、底行不算）
- 第2列：3个
- 第3列：1个
- 连续3列

路数 = 2 × 3 × 1 = 6路
假设：符号7赔率5、游戏倍数2
倍数 = 5 × 6 × 2 = 60倍
```

**示例2 - 金符号**：
```
盘面：
列1  列2  列3  列4  列5
 7   17    7    4    2  (17 = 7+10，金符号)
 3    7    5    7    1
 2    4   17    9    3  (17 = 7+10，金符号)
 7    7    7    7    7

符号7中奖：
- 第1列：2个
- 第2列：3个（7和17都算）
- 第3列：2个（7和17都算）
- 连续3列

路数 = 2 × 3 × 2 = 12路
```

**示例3 - 百搭符号**：
```
盘面：
列1  列2  列3  列4  列5
 7   10    3    4    2  (10 = wild)
 3    7    5    7    1
 2    4    7    9    3
 7    7    7    7    7

符号7中奖：
- 第1列：2个
- 第2列：3个（10和7都算）
- 第3列：2个
- 连续3列

路数 = 2 × 3 × 2 = 12路
```

### 2. 金符号机制

**定义**：普通符号（1-8）可转换为金符号（11-18），与原符号等价。

**转换规则**：
```
金符号值 = 原符号值 + 10
例如：符号7（红中）→ 金符号17
```

**生成概率**：
- **普通模式**: `GoldSymbolBaseProb`（配置概率）
- **免费模式**: `GoldSymbolFreeProb`（配置概率，通常更高）

**中奖判定**：
- 金符号17与普通符号7**完全等价**
- 在Ways计算中，两者都算作符号7
- 视觉上金色显示，增强游戏体验

### 3. 瀑布式消除机制

**流程**：
```
1. 检查中奖 → 标记中奖符号
2. 标记处理：
   ├─ 金符号 → 转换为百搭（10）
   └─ 普通符号 → 标记为0（空白）
3. 符号下移：
   ├─ moveSymbols() 移动非空符号
   └─ 空位出现在顶部
4. 符号掉落：
   ├─ fallingWinSymbols() 从滚轴补充
   └─ 应用金符号转换
5. removeNum++（消除次数增加）
6. 重新检查中奖
7. 直到不再中奖为止
```

**游戏倍数递增**：
```
第1次消除：gameMultiple = BaseGameMulti[0]（例如1倍）
第2次消除：gameMultiple = BaseGameMulti[1]（例如2倍）
第3次消除：gameMultiple = BaseGameMulti[2]（例如5倍）
第4次及以上：gameMultiple = BaseGameMulti[3]（例如10倍）
```

### 4. 免费游戏机制

**触发条件**：
- 盘面出现 ≥ 配置数量的胡符号（symbol 9）
- 计算免费次数：`基础次数 + (胡符号数 - 最小数量) × 每个增加次数`

**示例**：
```
配置：基础12次，最小3个，每个胡符号+2次
- 3个胡符号：12 + 0 = 12次
- 4个胡符号：12 + 2 = 14次
- 5个胡符号：12 + 4 = 16次
```

**免费游戏特性**：
1. **不扣除下注金额**（所有奖金都是纯盈利）
2. **使用免费滚轴配置**
3. **使用免费游戏倍数**（FreeGameMulti，通常更高）
4. **可累积触发**：免费游戏中再次触发会增加次数

**累积机制**：
```
普通游戏触发 → 设置12次免费
    ↓
免费游戏第3次时再次触发（4个胡符号）
    ↓
增加14次 → 总计 (12-3) + 14 = 23次
    ↓
继续免费游戏
```

### 5. 游戏倍数机制

**定义**：同一Round内连续消除次数越多，游戏倍数越高。

**倍数配置**：
```go
BaseGameMulti: [1, 2, 5, 10]   // 普通模式
FreeGameMulti: [2, 3, 6, 12]   // 免费模式（示例）
```

**应用示例**：
```
Round开始（removeNum = 0）
    ↓
第1次中奖（removeNum = 0）：gameMultiple = 1
  线倍数 = 30（赔率5 × 路数6）
  最终倍数 = 30 × 1 = 30
    ↓
第2次中奖（removeNum = 1）：gameMultiple = 2
  线倍数 = 12（赔率3 × 路数4）
  最终倍数 = 12 × 2 = 24
    ↓
第3次中奖（removeNum = 2）：gameMultiple = 5
  线倍数 = 6（赔率2 × 路数3）
  最终倍数 = 6 × 5 = 30
    ↓
第4次中奖（removeNum = 3）：gameMultiple = 10
  线倍数 = 8（赔率4 × 路数2）
  最终倍数 = 8 × 10 = 80
    ↓
未中奖 → Round结束
总奖金 = 30 + 24 + 30 + 80 = 164倍
```

### 6. 网格反转机制

**定义**：将行列网格反转，便于数据处理和展示。

**反转方式**：
```go
原始网格（行×列）：
int64Grid [5][5]int64  // [行][列]

反转后（列×行）：
int64GridY [5][5]int64  // [列][行]

中奖网格（只有4行）：
int64GridW [4][5]int64  // [行][列]（前4行）
```

**用途**：
- 内部计算使用int64Grid
- 前端展示使用反转后的网格
- 中奖网格只包含参与计算的前4行

---

## ⚙️ 配置说明

### 主要配置项

#### 1. 支付表（PayTable）
```json
"pay_table": [
  [0, 0, 5, 20, 100],    // 二条
  [0, 0, 5, 20, 100],    // 二筒
  [0, 0, 5, 25, 150],    // 五条
  [0, 0, 10, 50, 200],   // 五筒
  [0, 0, 10, 50, 200],   // 八万
  [0, 0, 20, 100, 500],  // 白板
  [0, 0, 50, 250, 1000], // 红中
  [0, 0, 50, 250, 1000]  // 发财
]
```
- 每行5个值对应：1列、2列、3列、4列、5列的赔率
- 前两个值为0表示1-2列不中奖（必须≥3列）

#### 2. 游戏倍数
```json
"base_game_multi": [1, 2, 5, 10],     // 普通模式
"free_game_multi": [2, 3, 6, 12]      // 免费模式
```
- 数组索引对应removeNum（消除次数）
- 第4次及以上使用最后一个值

#### 3. 金符号概率
```json
"gold_symbol_base_prob": 1000,  // 10% (1000/10000)
"gold_symbol_free_prob": 3000   // 30% (3000/10000)
```
- 单位：万分之一
- 免费模式概率通常更高

#### 4. 免费游戏配置
```json
"free_game_min": 3,         // 最小胡符号数量
"free_game_times": 12,      // 基础次数
"free_game_add_times": 2    // 每个胡符号增加次数
```

#### 5. 滚轴配置
```json
"roll_cfg": {
  "base": [0, 1, 2, 3, 4],    // 基础滚轴组
  "free": [5, 6, 7, 8]         // 免费滚轴组
}
```

---

## 🛡️ 错误处理

### 1. Panic恢复机制

所有对外接口都包含panic恢复：
```go
defer func() {
    if r := recover(); r != nil {
        global.GVA_LOG.Error("BetOrder", zap.Any("r", r))
        debug.PrintStack()
        result, err = nil, InternalServerError
        return
    }
}()
```

### 2. 数据验证

#### 余额检查
- 初始化时检查余额是否充足
- 使用`CheckMemberBalance()`验证

#### 下注金额验证
```go
betAmount := decimal.NewFromFloat(req.BaseMoney).
    Mul(decimal.NewFromInt(req.Multiple)).
    Mul(decimal.NewFromInt(_baseMultiplier))
if betAmount.LessThanOrEqual(decimal.Zero) {
    return nil, InvalidRequestParams
}
```

### 3. 并发控制

使用锁防止并发问题：
```go
c.BetLock.Lock()
defer c.BetLock.Unlock()
```

### 4. 错误类型定义

```go
var (
    InternalServerError  = errors.New("internal server error")
    InvalidRequestParams = errors.New("invalid request params")
    InsufficientBalance  = errors.New("insufficient balance")
)
```

### 5. 日志记录

关键操作都记录日志：
```go
global.GVA_LOG.Error("betOrder", zap.Error(errors.New("user not exists")))
```

---

## 📝 开发指南

### 代码规范

1. **命名规范**
   - 公开接口：大写开头（如`BetOrder`、`NewGame`）
   - 私有方法：小写开头（如`baseSpin`、`checkSymbolGridWin`）
   - 私有常量：下划线前缀（如`_gameID`、`_wild`）

2. **注释规范**
   - 公开函数必须有注释说明
   - 复杂算法添加详细注释
   - 重要常量说明用途

3. **错误处理**
   - 所有可能失败的操作返回error
   - 关键接口添加panic恢复
   - 使用zap记录错误日志

### 开发注意事项

#### ⚠️ 必须遵守的规则

1. **金额计算**
   - 必须使用`decimal.Decimal`进行精确计算
   - 避免使用float64直接计算金额
   - 最终金额使用`Round(2)`保留2位小数

2. **数组访问**
   - 访问PayTable前检查索引范围
   - 访问网格时注意边界（5×5网格，但只有前4行参与）
   - removeNum最大为3（对应倍数数组索引）

3. **状态管理**
   - 正确设置Stage（1/11/21/22）
   - Steps在Round结束时重置为0
   - removeNum在Round结束时重置为0
   - gameMultiple根据removeNum动态变化

4. **并发安全**
   - 修改客户端数据前必须加锁
   - 使用`client.BetLock`保护订单操作

5. **Redis数据**
   - SpinSceneData必须序列化后保存
   - 过期时间设置为90天
   - 使用正确的key格式

6. **Ways计算**
   - 只计算前4行（第5行不参与）
   - 从第1列开始检查（从左至右）
   - 连续列中断则不再继续

#### 📋 开发检查清单

- [ ] 是否使用decimal计算金额？
- [ ] 是否添加了panic恢复？
- [ ] 是否检查了数组越界？
- [ ] 是否正确处理了金符号（+10）？
- [ ] 是否正确处理了百搭符号（wild）？
- [ ] 是否正确计算了Ways路数？
- [ ] 是否正确设置了游戏状态？
- [ ] 是否记录了错误日志？
- [ ] 是否使用了并发锁？
- [ ] 是否保存了场景数据？
- [ ] removeNum是否正确递增和重置？
- [ ] 网格反转是否正确？

### 性能优化建议

1. **缓存使用**
   - SpinSceneData使用Redis缓存
   - 配置数据启动时加载（initGameConfigs）

2. **内存管理**
   - 使用固定大小数组（如`[5][5]int64`）
   - 避免频繁的内存分配

3. **算法优化**
   - Ways检查使用map去重
   - 提前中断不可能中奖的检查

### 测试建议

1. **单元测试**
   - 测试Ways计算逻辑
   - 测试金符号转换
   - 测试符号掉落逻辑
   - 测试免费游戏触发

2. **集成测试**
   - 测试完整游戏流程
   - 测试连续消除机制
   - 测试免费游戏累积

3. **RTP测试**
   - 使用`rtp_test.go`进行大量模拟
   - 验证返奖率在合理范围
   - 检查极端情况

### 调试技巧

1. **查看场景数据**
   ```bash
   redis-cli
   > GET scene-用户ID-18943
   > TTL scene-用户ID-18943
   ```

2. **日志查询**
   - 搜索订单号定位问题
   - 查看错误日志堆栈

3. **常见问题排查**
   - Panic → 检查数组越界、nil指针
   - 余额不一致 → 检查decimal计算
   - Ways计算错误 → 检查循环逻辑和金符号处理
   - 免费游戏异常 → 检查Stage和FreeNum
   - 倍数异常 → 检查removeNum和gameMultiple

---

## 🔗 技术栈链接

- [Go语言](https://golang.org/)
- [gRPC](https://grpc.io/)
- [Redis](https://redis.io/)
- [MySQL](https://www.mysql.com/)
- [Zap日志库](https://github.com/uber-go/zap)
- [jsoniter](https://github.com/json-iterator/go)
- [decimal](https://github.com/shopspring/decimal)

---

## 📚 游戏特色总结

### 创新点

1. **Ways玩法**：不依赖固定支付线，提供更多中奖可能性
2. **金符号系统**：增强视觉效果和游戏期待感
3. **瀑布式消除**：连续中奖体验，倍数递增
4. **麻将主题**：传统麻将牌面，符合东方审美

### 游戏公式汇总

```
下注金额 = 基础下注 × 用户倍数 × 基础倍数(20)

线倍数 = 符号赔率 × Ways路数

最终倍数 = 线倍数 × 游戏倍数

Ways路数 = 第1列符号数 × 第2列符号数 × ... × 第N列符号数

奖金金额 = 基础下注 × 用户倍数 × 最终倍数

免费次数 = 基础次数 + (胡符号数 - 最小数量) × 每个增加次数

金符号值 = 原符号值 + 10

游戏倍数 = BaseGameMulti[removeNum] 或 FreeGameMulti[removeNum]
```

---

## 👥 维护团队

**开发团队**: egame-grpc03项目组  
**游戏模块**: mahjong - 麻将胡了  
**游戏ID**: 18943  
**技术支持**: 请通过issue或内部渠道联系

---

*本文档最后更新时间: 2025年10月22日*

*文档版本: v1.0.0*

