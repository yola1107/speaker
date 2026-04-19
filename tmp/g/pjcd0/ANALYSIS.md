# 破茧成蝶 (pjcd) 代码功能与结构分析

## 一、游戏基础信息

| 属性 | 值 |
|------|-----|
| 游戏ID | 18984 |
| 游戏名称 | 破茧成蝶 (pjcd) |
| 盘面规格 | 3行 × 5列 |
| 中奖线数 | 20条 |
| 特色符号 | 多形态黏性百搭 (毛虫→蝶茧→蝴蝶)、SCATTER |
| 判奖类型 | LineGame (20条赔付线) |
| 核心机制 | 百搭形态进化、轮次倍数递增、蝴蝶百搭增加倍数 |

---

## 二、模块与职责

| 模块 | 路径 | 职责 |
|------|------|------|
| **对外入口** | `exported.go` | `NewBetOrder`（下注）、`MemberLogin`（登录） |
| **下注流程** | `bet_order.go`、`bet_order_*.go` | 下注主流程、场景管理、订单处理、余额扣减 |
| **旋转逻辑** | `spin/` | Base/Free 旋转、滚轮构建、线奖计算、倍数应用 |
| **登录** | `member_login.go` | 登录、场景恢复、上次订单回传 |
| **配置** | `game_json.go`、`bet_order_configs.go` | 配置加载、校验、赔付表、滚轮权重 |
| **常量与类型** | `const.go`、`types.go` | 游戏常量、矩阵/结果/场景类型定义 |

---

## 三、目录与文件结构

```
game/pjcd/
├── exported.go              # 游戏入口：NewBetOrder、MemberLogin
├── bet_order.go             # 下注主流程：betOrder → baseSpin → updateGameOrder → settleStep → getBetResultMap
├── bet_order_step.go        # 初始化/订单更新/余额结算
├── bet_order_scene.go       # 场景 Redis 读写：saveScene、reloadScene、cleanScene
├── bet_order_spin.go        # 旋转流程：processWin/processNoWin、倍数计算、免费触发
├── bet_order_helper.go      # 辅助函数：symbolGridToString、winGridToString、updateBonusAmount
├── bet_order_configs.go     # 配置解析：parseGameConfigs、calculateRollWeight、getStreakMultiplier
├── bet_order_wild.go        # 百搭机制：initWildStateGrid、evolveWilds、preserveWildStates
├── member_login.go          # 登录：memberLogin、doMemberLogin、selectOrderRedis
├── game_json.go             # 配置常量：_gameJsonConfigsRaw (pay_table/lines/real_data等)
├── const.go                 # 常量定义：GameID=18984、矩阵3x5、符号0-12、百搭形态1-3
├── types.go                 # 类型定义：int64Grid、WildStateGrid、WinInfo、SpinSceneData
├── doc/
│   ├── README.md            # 规则说明
│   ├── rules.md             # 详细规则
│   └── game.json            # 示例配置
└── 文档/
    ├── 破茧成蝶 - 游戏设计分析.md     # 完整设计文档
    └── 破茧成蝶 - 轮轴构建机制分析.md # 滚轮构建详解
```

---

## 四、核心数据流

### 4.1 下注流程 `betOrder`（bet_order.go）

```
请求进入 (req)
    ↓
getRequestContext()           # 获取商户/会员/游戏信息
    ↓
client.BetLock.Lock()         # 用户锁
    ↓
GetLastOrder()                # 获取上次订单
    ↓
cleanScene()                  # 若无上次订单则清理场景
    ↓
reloadScene()                 # 从 Redis 加载场景 → syncGameStage() 判断是否免费局
    ↓
baseSpin()                    # 旋转逻辑 (见下节)
    ↓
updateGameOrder()             # 构建订单：fillInGameOrderDetails() 保存 symbolGrid/winGrid
    ↓
settleStep()                  # 余额结算：扣费/奖金/池记录
    ↓
saveScene()                   # 保存场景到 Redis
    ↓
getBetResultMap()             # 构建 proto 响应：Pjcd_BetOrderResponse + buildWinInfo()
    ↓
MarshalData()                 # proto.Marshal + json.MarshalToString → 返回 (pbData, jsonData, error)
```

### 4.2 旋转流程 `baseSpin`（bet_order_spin.go）

```
初始化场景数据：SymbolRoller、WildStateGrid、ButterflyCount
    ↓
handleSymbolGrid()            # 从 scene.SymbolRoller 构建 4x5 symbolGrid
    ↓
initWildStateGrid()           # 中间3列若出现百搭则设为毛虫形态(1)
    ↓
checkSymbolGridWin()          # 按20条线判奖 → WinInfo列表
    ↓
processWinInfos()             # 有中奖 → processWin()；无中奖 → processNoWin()
```

#### processWin() 分支：
```
getStreakMultiplier()         # 获取轮次倍数 + 蝴蝶贡献倍数
    ↓
handleWinElemsMultiplier()    # 线奖之和
    ↓
stepMultiplier = lineMultiplier * gameMultiple
    ↓
isRoundOver = false
    ↓
scene.Steps++ / ContinueNum++
    ↓
RoundMultiplier += stepMultiplier
    ↓
evolveWilds()                 # 百搭形态进化：毛虫→蝶茧→蝴蝶→累加ButterflyCount
    ↓
moveSymbols()                 # 中奖符号消除、下落补位、保留黏性百搭
    ↓
fallingWinSymbols()           # 更新 scene.SymbolRoller
    ↓
设置 NextStage = _spinTypeBaseEli / _spinTypeFreeEli
    ↓
updateBonusAmount()           # 奖金累加
```

#### processNoWin() 分支：
```
gameMultiple/lineMultiplier/stepMultiplier = 0
    ↓
isRoundOver = true
    ↓
getScatterCount()             # 统计全屏 SCATTER(9) 个数
    ↓
scene.Steps = 0 / ContinueNum = 0
    ↓
免费模式重置 ButterflyCount
    ↓
calcNewFreeGameNum()          # 根据 scatterCount 计算免费次数 → 更新 AllFreeTime/FreeTime
    ↓
若免费结束：NextStage = _spinTypeBase
否则：NextStage = _spinTypeFree
    ↓
updateBonusAmount(0)
```

### 4.3 滚轮构建（spin/roll_base.go）

**基础模式滚轮生成**（每10次spin或首次）：
1. **取符号**：按 `base_symbol_weights` 权重选基础符号（1-7），不能连续相同
2. **选排列**：按 `symbol_permutation_weights` 权重选排列形式（单/二/三连）
3. **替换特殊**：按概率 `base_scatter_prob`/`base_wild_prob` 替换为 SCATTER(9)/百搭(8)

**免费模式滚轮生成**（仅首次）：
- 使用 `free_symbol_weights`、`free_scatter_prob`、`free_wild_prob`
- 其它逻辑相同

---

## 五、核心机制实现

### 5.1 百搭形态进化（bet_order_wild.go）

- **初始化**：`initWildStateGrid()` - 中间3列出现百搭时设为毛虫(1)
- **进化**：`evolveWilds()` - 参与中奖时：毛虫→蝶茧(2)→蝴蝶(3)→累加`ButterflyCount`并消除
- **保留**：`preserveWildStates()` - 消除时保留非蝴蝶百搭状态

### 5.2 倍数计算（bet_order_configs.go）

```go
// getStreakMultiplier() 返回：gameMul, multipliers, index, butterflyMultiplier
gameMul = multipliers[index]  // 轮次倍数：基础[1,2,3,5]，免费[3,6,9,15]

// 第4轮额外倍数
if index == 3 && butterflyMultiplier > 0 {
    gameMul += butterflyMultiplier  // ButterflyCount * WildAddFourthMultiple(5)
}
```

### 5.3 中奖判奖（bet_order_helper.go）

- **线判奖**：`checkSymbolGridWin()` - 遍历20条线，从左到右找3+连续匹配
- **百搭当万能**：`currSymbol == symbol || currSymbol == _wild`
- **形态记录**：中奖百搭记录 `WildForm`

### 5.4 场景管理（bet_order_scene.go）

- **Key**：`{Site}:scene-{GameID}:{MemberID}`
- **数据**：`SpinSceneData` 包含 Stage/NextStage、FreeNum、Steps、ContinueNum、RoundMultiplier、SymbolRoller、WildStateGrid、ButterflyCount 等
- **同步**：`syncGameStage()` - 根据 Stage/NextStage 更新 `isFreeRound`

---

## 六、配置结构（game_json.go）

- **pay_table**：7种符号×5档倍数 [0,0,2,5,10] 等
- **lines**：20条赔付线，每条5个位置(0-24)
- **roll_cfg**：base/free 的 use_key/weight
- **real_data**：2组滚轮数据，每组5列，每列100+个符号
- **轮次倍数**：base_round_multipliers [1,2,3,5]、free_round_multipliers [3,6,9,15]
- **免费触发**：scatter_count/free_spins 对应触发次数
- **百搭配置**：wild_add_fourth_multiple=5（蝴蝶增加第4轮倍数）

---

## 七、类型与常量

- **矩阵类型**：`int64Grid [3][5]int64`、`WildStateGrid [3][5]int64`
- **符号**：0=Wild、1-7=普通花、8=百搭、9=Scatter、10-12=特殊
- **百搭形态**：1=毛虫、2=蝶茧、3=蝴蝶
- **阶段**：1=基础、11=基础消除、21=免费、22=免费消除

---

## 八、登录流程（member_login.go）

- **memberLogin**：取客户端场景 → 若无则返回空 → 取LastOrder → doMemberLogin
- **doMemberLogin**：按 key `{site}:{merchantId}:{memberId}:{gameId}:lastBetRecord` 取上次记录 → 拼装 orderMap（含lastOrder）→ 返回JSON

---

## 九、与其它游戏差异

| 对比项目 | pjcd | sgz | qlxr2 |
|----------|------|-----|-------|
| **Proto响应** | 有 Pjcd_BetOrderResponse | 有 Sgz_BetOrderResponse | 无，直接返回SpinResult JSON |
| **盘面** | 3×5 LineGame | 4×5 LineGame | 5×5 LineGame |
| **消除机制** | 有，连续消除倍增 | 有，连续消除倍增 | 无，单次旋转 |
| **百搭特色** | 三形态黏性进化 | 固定百搭 | 百搭扩展为整列 |
| **免费触发** | Scatter 3+个 | Scatter 统计 | Scatter 3+个 |
| **倍数系统** | 轮次倍增+蝴蝶额外 | 线倍+连续消除 | 列乘倍+全屏高倍 |
| **场景存储** | Redis | Redis | Redis |
| **订单落库** | 有 | 有 | 无 |

---

## 十、总结

| 维度 | 说明 |
|------|------|
| **功能** | 3×5消除LineGame，黏性三形态百搭，轮次倍数递增，蝴蝶百搭增加额外倍数，Scatter触发免费 |
| **结构** | 入口(exported) → 下注(bet_order) → 旋转流程(bet_order_spin) → 场景管理(bet_order_scene)，配置/常量/类型分离 |
| **数据流** | 请求 → 场景加载 → 旋转(判奖/消除/倍增/补位)循环 → 订单保存 → 场景保存 → Proto响应 |
| **核心特色** | 百搭形态进化(毛虫→蝶茧→蝴蝶)，第4轮倍数受蝴蝶影响，免费模式倍数累加 |
| **实现要点** | WildStateGrid跟踪形态，processWin/processNoWin分支处理，Redis持久化场景，支持断线重连 |

完整说明（含详细数据流、配置字段、机制实现、与其它游戏对比）已写在 **`/game/pjcd/ANALYSIS.md`**，可直接打开该文件查看。
