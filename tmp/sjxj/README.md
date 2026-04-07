# sjxj（世界小姐 ）

## 项目说明

| 项 | 说明 |
|----|------|
| **GameId** | `18969` |
| **实现策略** | 服务端框架与工程结构可参考自 **sgz** 拷贝的代码；**本游戏为新品类，玩法、结算、免费模式等逻辑须按本文档与 `doc/` 重写**，勿直接沿用 sgz 业务规则。 |

## 文档来源

| 类型 | 地址 |
|------|------|
| **策划文档（Axure）** | [世界小姐 - 在线原型](https://5zevov.axshare.com/?g=14&id=0nmyz0&p=%E4%B8%96%E7%95%8C%E5%B0%8F%E5%A7%90) |
| **玩法参考（同类机制）** | [JILI - Shanghai Beauty（上海美人）](https://jiligames.com/PlusIntro/17?showGame=true)<br/>Slot、50 线、Bonus / Respin、盘面扩展至 8×5<br/>免费段：收集 heart（爱心）；每集满 4 个扩展一行，最高 8×5<br/>本游戏 Scatter/爱心式扩展与解锁节奏可对照体验；**数值与触发条件以本 README 与 Axure 为准**。 |

---

## 本地文档（doc/）

以下文件为策划/视觉依据，实现与联调时以 **在线文档 + 下列原图** 为准：

| 文件 | 内容摘要 |
|------|----------|
| `doc/base-board.png` | **普通模式界面**：5 轴 × 4 行基础盘面、主 UI、押注与 Spin 等区域示意。 |
| `doc/paylines.png` | **50 条中奖线**（**4 行 × 5 列**图示）。**免费模式**虽为 **8×5** 盘面，**线奖仍只按与基础模式相同的 50 条线，且仅统计最下方 4 行**（见 §3.0）。 |
| `doc/symbols.png` | **符号表**（约 11 个）：含 **Wild（主角/百搭）**、**Scatter**；各符号 **3/4/5 连赔率**；另有 **「爱心随机范围」** 表（按初始 4 行及第 5～8 行分列数值，用于随机或权重配置）。 |
| `doc/payouts.png` | **Symbol Payout Values**：普通符号赔付档位示意；**Wild 可替代除 Scatter 外所有符号**（Scatter 不参与替代）。具体倍率以数值表与线注结合为准。 |
| `doc/free-board.png` | **免费模式 UI**：锁定行 **TO UNLOCK**；解锁条件见下文 **S ≥ 阈值**（配置 `free_unlock_thresholds`）。 |
| `doc/free-flow.png` | **免费游戏流程**：部分行锁定；每次一次 Spin；Scatter 与解锁、+3 次、结束条件等以本 README 定稿为准。 |

界面参考图（非 doc 内）：`game/sjxj/img1.png`（基础）、`game/sjxj/img2.png`（免费）。

---

## 一、游戏形态（当前代码实现口径）

> 本节以 `game/sjxj/*.go` 当前实现为准，用于联调、回放、问题排查。  
> 若与策划/Axure存在差异，请以“实现口径”和“目标口径”分别记录并走评审。

- **类型**：视频老虎机（Video Slot）。
- **服务端盘面**：统一 **8×5**（权威盘面，回包也为 8×5）。
- **基础模式（Base）**：使用 `real_data[0]` 随机生成 8×5；线奖使用配置中的 50 条线（当前线坐标为底部 4 行下标）。
- **免费模式（Free）**：使用 `real_data[1]`；先按 `scatterLock` 固定夺宝位置，再填充其余格子。
- **当前结算特征**：基础模式计算线奖；免费模式不逐步计算线奖，主要做解锁与夺宝倍数累计，免费结束时一次性结算。

---

## 二、基础模式（Base）流程

### 2.1 单次请求主链

1. `betOrder()` 获取上下文、拿客户端锁、加载上一单与场景。
2. `reloadScene()` 从 Redis 恢复 `SpinSceneData`，并用 `syncGameStage()` 切到本次有效阶段。
3. `baseSpin()` 进入基础分支：
   - `initialize()` -> `initFirstStepForSpin()` 校验下注与余额，扣费金额 `amount = betAmount`。
   - `getSceneSymbolBase()` 生成盘面（`real_data[0]`）。
   - `checkSymbolGridWin()` 计算线奖（Wild 可替代普通符号，不替代 Scatter）。
   - `getScatterCount()` 统计 Scatter（基础模式统计底部 4 行）。
4. 若 `scatterCount >= free_game_scatter_min`（默认 4）：
   - `scene.FreeNum += free_game_times`（默认 +3）；
   - `NextStage = Free`；
   - `UnlockedRows = 4`；
   - `lockScatter()` 锁定触发盘中全盘 Scatter 并分配固定倍数。
5. 若未触发免费：`NextStage = Base`。
6. 统一执行：`updateGameOrder()` -> `settleStep()` -> `saveScene()` -> `getBetResultMap()`。

### 2.2 基础模式免费次数变化

| 时机 | FreeNum 变化 | 说明 |
|---|---:|---|
| 基础局开始 | 0（通常） | 基础局本身不消耗免费次数 |
| 基础局结束且未触发免费 | 不变 | 仍为基础阶段 |
| 基础局结束且触发免费 | `+free_game_times` | 默认 `+3`，下次请求切到 Free |

> **术语**：全文 **Scatter** 与策划/UI 中的「夺宝」等为同一符号，协议与实现统一使用 **Scatter**。

---

## 三、免费模式（Free）流程

### 3.1 进入免费首局时

- 上一基础局已把 `NextStage=Free` 写入场景。
- 本次 `reloadScene()` 后 `syncGameStage()` 会把 `Stage` 切到 Free。
- `UnlockedRows` 初始为 4，`PrevUnlockedRows` 初始为 4。
- `BaseEnterFreeFirstStep=true`：表示本次为“基础进入免费”的首局，需要在填充滚轴符号前仅一次做阶段1解锁（Stage 1），解锁依据为当前 `scatterLock` 中已锁定夺宝的数量推进 `UnlockedRows`。
- `scatterLock[r][c] > 0` 表示该位置固定为 Scatter 且值即固定倍数。

### 3.2 免费局执行顺序（当前实现）

1. `baseSpin()` 开始时先消耗 1 次免费：`FreeNum--`。
2. 若 `BaseEnterFreeFirstStep=true`（仅免费首局生效）：
   - 在 `initSpinSymbol()/handleSymbolGrid()` 之前调用 `unlockByLockedScatter()`，基于 `ScatterLock` 中已锁定夺宝数量推进 `UnlockedRows`（Stage 1）；
   - 随后清空 `BaseEnterFreeFirstStep=false`，避免重复执行。
3. 生成免费盘面（锁定位置优先）：
   - 锁定位置直接放 Scatter；
   - 其余格子从 `real_data[1]` 补齐。
4. `processWinInfos()` 的免费分支执行：
   - 清空 `winInfos` / `winGrid`（当前实现不做免费线奖逐局结算）；
   - `scatterCount = getScatterCount()`（按已解锁行统计）；
   - `tryUnlockNextRow()`：当 `S >= threshold[UnlockedRows]` 时可连续解多行；
   - `calcCurrentFreeGameMul()`：补齐本次免费回合 scatterLock、统计已解锁区 scatter 总数，并判断是否已满屏夺宝。
     - 对本次新出现 Scatter 分配固定倍数并写回 `scatterLock`；
     - 统计已解锁区的夺宝倍数和；
     - 判断“已解锁区是否满屏夺宝”。
5. 解锁后补次：
   - 若本局有新增解锁，且当前 `FreeNum < free_unlock_reset_spins`（默认 3），补到 3；
   - `addFreeTime = 补入数量`（用于回包与统计）。
6. 结束判定：
   - 若 `FreeNum <= 0` 或 `isFullTreasureScreen == true`：
     - 本局 `stepMultiplier = freeGameMul`（一次性结算）；
     - `NextStage = Base`；
     - 清空 `scatterLock` / 重置解锁行。
   - 否则：
     - `stepMultiplier = 0`；
     - `NextStage = Free`，继续下一免费局。

### 3.3 免费次数流转（关键）

| 节点 | FreeNum | 说明 |
|---|---:|---|
| 触发免费后（基础局结尾） | `+3` | 默认 `free_game_times=3` |
| 免费局开始 | `-1` | 每次免费开局即消耗 1 次 |
| 免费局内若解锁新行 | 补到 `max(当前,3)` | 由 `free_unlock_reset_spins` 控制 |
| 免费结束 | 置 0 | 回基础模式并清空锁定状态 |

---

## 四、实现差异提醒（代码 vs 目标规则）

1. **免费线奖**：当前代码免费局不逐局结算 50 线，仅免费结束时一次性按夺宝倍数结算。
2. **满屏结束判定**：当前判定范围为“已解锁行”，非严格全 8×5。
3. **配置字段名**：代码使用 `free_unlock_reset_spins`，不是 `free_unlock_add_spins`。
4. **文档用途**：联调用本 README“实现口径”；策划验收需另附“目标口径”清单。

---

## 五、整体流程（简图）

```
Base 请求
  -> load scene/sync stage
  -> 生成基础盘面(real_data[0])
  -> 计算线奖 + 统计Scatter(底部4行)
  -> Scatter>=4 ? 是: freeNum+=3, lockScatter, next=Free : next=Base
  -> 落单、保存场景、回包

Free 请求
  -> load scene/sync stage(进入Free)
  -> freeNum先-1
  -> BaseEnterFreeFirstStep?是：unlockByLockedScatter（阶段1，仅一次）; BaseEnterFreeFirstStep=false
  -> 生成免费盘面(real_data[1], scatterLock优先)
  -> processWinInfos(Stage2)：统计S(已解锁行)、tryUnlockNextRow、calcCurrentFreeGameMul（新Scatter写入lock并赋倍数）
  -> 若解锁则freeNum补到>=3
  -> freeNum<=0 或 已解锁区满屏Scatter ?
       是: stepMul=freeMul, clearLock, next=Base
       否: stepMul=0, next=Free
  -> 落单、保存场景、回包
```

---

## 六、关键配置说明

1. `free_game_scatter_min`：基础局触发免费的 Scatter 最小数（默认 4）。
2. `free_game_times`：基础触发免费时初始赠送次数（默认 3）。
3. `free_unlock_thresholds`：按 `UnlockedRows` 索引的解锁阈值（长度必须是 8）。
4. `free_unlock_reset_spins`：免费局解锁后重置到的最小剩余次数（默认 3）。
5. `free_scatter_multiplier_by_row`：按行配置的 Scatter 固定倍数随机池。

---

## 七、数据结构与状态持久化

- Redis 场景键：`scene-18969:<memberID>`（含站点前缀）。
- 核心场景字段：
  - `Stage/NextStage`：阶段切换状态机；
  - `FreeNum`：剩余免费次数；
  - `ScatterLock[8][5]`：锁定散布与倍数；
  - `UnlockedRows/PrevUnlockedRows`：当前与上局解锁行数。
  - `BaseEnterFreeFirstStep`：基础进入免费首局仅执行一次的阶段1解锁标识（填符号前按 `ScatterLock` 推进 `UnlockedRows`）。
- 每次请求结束都会 `saveScene()`，支持断线恢复。

---

## 八、回包关键字段（联调重点）

- `Free`：当前是否免费阶段。
- `FreeNum`：当前剩余免费次数。
- `FreeTime`：已进行的免费局计数。
- `ScatterCount`：当前统计范围内的 Scatter 数。
- `UnlockedRows` / `PrevUnlockedRows`：解锁行变化。
- `AddFreeNum`：本局新增免费次数（来自解锁补次或基础触发免费）。
- `Cards` / `WinGrid` / `WinInfo`：盘面、中奖位、中奖详情。

---

## 九、测试建议（按当前实现）

1. 基础局触发免费：验证 `FreeNum=3`、`NextStage=Free`、`ScatterLock` 已写入。
2. 免费局消耗：每次开局先 `FreeNum--`。
3. 免费解锁补次：当 `UnlockedRows` 增长时，`FreeNum` 被补到 `>=3`。
4. 免费结束结算：`FreeNum<=0` 或“已解锁区满屏”时一次性结算 `freeGameMul`。
5. 结束回基础：`NextStage=Base` 且 `ScatterLock` 清空。

---

## 十、流程审阅清单（排查时使用）

- 是否出现 `Stage/NextStage` 不一致导致阶段错乱。
- 是否出现 `FreeNum` 与客户端免费计数不同步。
- `UnlockedRows` 是否越界或 `PrevUnlockedRows > UnlockedRows`。
- `scatterLock` 是否在免费结束后彻底清空。
- 是否错误读取配置（Redis 覆盖本地配置）。

---

## 十一、完整流程图

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           用户请求下注 (betOrder)                       │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 1) getRequestContext + Client加锁 + GetLastOrder + reloadScene         │
│ 2) syncGameStage: Stage = NextStage? (有则切换并清空NextStage)         │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                    ┌────────────────┴────────────────┐
                    │                                 │
                    ▼                                 ▼
          ┌──────────────────────┐          ┌──────────────────────┐
          │   Stage = Base       │          │   Stage = Free       │
          └──────────────────────┘          └──────────────────────┘
                    │                                 │
                    ▼                                 ▼
          ┌──────────────────────┐          ┌──────────────────────┐
          │ initFirstStepForSpin │          │ initStepForNextStep  │
          │ 校验下注/余额, amount>0│         │ amount=0             │
          └──────────┬───────────┘          └──────────┬───────────┘
                     │                                  │
                     ▼                                  ▼
          ┌──────────────────────┐          ┌──────────────────────┐
          │ getSceneSymbolBase   │          │ FreeNum-- (先消耗一次) │
          │ real_data[0]随机8x5   │          │ Stage1: unlockByLockedScatter │
          └──────────┬───────────┘          │ getSceneSymbolFree(lock优先) │
                     │                      └──────────┬───────────┘
                     ▼                                 │
          ┌──────────────────────┐                     ▼
          │ checkSymbolGridWin   │          ┌──────────────────────┐
          │ 计算50线, 得lineMul    │          │ tryUnlockNextRow     │
          └──────────┬───────────┘          │ S>=阈值可连续解锁     │
                     │                      └──────────┬───────────┘
                     ▼                                 │
          ┌──────────────────────┐                     ▼
          │ getScatterCount      │          ┌──────────────────────┐
          │ Base统计底部4行S      │          │ calcCurrentFreeMul    │
          └──────────┬───────────┘          │ 新Scatter写lock并赋倍数│
                     │                      │ 统计freeMul/满屏判定   │
                     ▼                      └──────────┬───────────┘
          ┌──────────────────────┐                     │
          │ S >= free_min ?      │                     ▼
          └──────────┬───────────┘          ┌──────────────────────┐
                     │                      │ 解锁后补次: FreeNum   │
           ┌─────────┴─────────┐            │ < reset_spins ? 补到3 │
           ▼                   ▼            └──────────┬───────────┘
 ┌──────────────────┐  ┌──────────────────┐            │
 │ FreeNum += 3      │  │ NextStage=Base   │            ▼
 │ NextStage=Free    │  │ stepMul=lineMul  │   ┌──────────────────────┐
 │ lockScatter写入   │  └──────────────────┘   │ Free结束判定          │
 │ stepMul=lineMul   │                         │ FreeNum<=0 或 满屏?   │
 └─────────┬─────────┘                         └──────────┬───────────┘
           │                                             │
           └──────────────────────┬──────────────────────┘
                                  ▼
                ┌──────────────────────────────────────┐
                │ Yes: stepMul=freeMul, clearLock,     │
                │      NextStage=Base                  │
                │ No : stepMul=0, NextStage=Free       │
                └──────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ updateGameOrder -> settleStep(SaveTransfer) -> saveScene -> 返回回包   │
└─────────────────────────────────────────────────────────────────────────┘
```

