# 埃及探秘（`ajtm`）代码逻辑说明

> GameID: `18985`
>
> 当前文档以 `game/ajtm` 目录中的实际实现为准，不再沿用历史需求草稿。

## 1. 概述

`ajtm` 是一个 `Ways + 消除 + 长符号` 的老虎机游戏。

当前实现的核心特点：

1. 盘面是 `6 行 × 5 列`。
2. 左右两侧边列只有中间 `4` 个格子有效，四角固定为空。
3. 中奖按 `Ways` 从左到右计算，至少连续命中 `3` 列才算中奖。
4. 中间三列可能生成纵向 `2 格` 的长符号。
5. 长符号中奖时：
   - 会保留在 `winGrid` 中做展示；
   - 不会写入 `eliGrid` 做实际消除；
   - 会先随机转变为其他普通符号，再参与后续下落流程；
   - 转变事件会记录到 `longEvents` 返回给客户端。
6. 免费模式下会继承长符号数量；若当前步未中奖，会按当前滚轴盘面重新计算 `LongCount`，供下一次免费 `spin` 继续使用。

## 2. 目录职责

| 文件 | 作用 |
| --- | --- |
| `bet_order.go` | 服务入口、响应组装、长符号事件输出 |
| `bet_order_spin.go` | 单次结算主流程，区分中奖/未中奖路径 |
| `bet_order_step.go` | 初始化下注、构建订单、落库结算 |
| `bet_order_helper.go` | 中奖判定、展示网格/消除网格合并、长符号转变、下落处理 |
| `bet_order_configs.go` | 游戏配置解析、滚轴窗口构建、长符号布局生成 |
| `bet_order_scene.go` | Redis 场景读取、保存、阶段同步 |
| `types.go` | `int64Grid`、`WinInfo`、`Block` 等核心类型 |
| `const.go` | 常量、符号定义、长符号编码规则 |
| `pb/ajtm.proto` | 对外返回协议定义 |
| `rtpx_test.go` | RTP 调试测试，当前重点测试入口是 `TestRtp2` |

## 3. 盘面与符号

### 3.1 盘面尺寸

- 行数：`_rowCount = 6`
- 列数：`_colCount = 5`

### 3.2 有效格规则

盘面列索引为 `0~4`，行索引为 `0~5`。

- 第 `0` 列和第 `4` 列：
  - `row=0` 和 `row=5` 固定为空；
  - 只有 `row=1~4` 会填充普通符号。
- 第 `1~3` 列：
  - `row=0~5` 都可能有符号。

### 3.3 符号定义

| 符号值 | 含义 |
| --- | --- |
| `0` | 空位 |
| `1~12` | 普通符号 |
| `13` | `wild` |
| `14` | 夺宝符号 |
| `1000 + x` | 长符号尾部，头部符号为 `x` |

### 3.4 长符号编码

长符号占据同一列的上下两格：

1. 头部保留原始符号值，例如 `5`
2. 尾部写成 `_longSymbol + 头部符号`，例如 `1005`

例子：

```text
原列: [A, B, C, D, E, F]
在行 2-3 插入长符号后:
[A, B, C, 1000+C, D, E]
F 被挤出丢弃
```

## 4. 核心状态

### 4.1 `betOrderService`

`betOrderService` 是整局处理的主状态对象，核心字段如下：

| 字段 | 说明 |
| --- | --- |
| `scene` | Redis 持久化场景 |
| `symbolGrid` | 当前盘面 |
| `winGrid` | 中奖展示网格 |
| `eliGrid` | 实际消除网格 |
| `nextSymbolGrid` | 消除下落后的盘面 |
| `winInfos` | 本步中奖信息 |
| `winLongBlocks` | 本步命中的长符号块 |
| `longEvents` | 本步长符号转变事件 |
| `lineMultiplier` | 本步基础中奖倍数 |
| `stepMultiplier` | 本步最终结算倍数 |
| `scatterCount` | 本步盘面的夺宝数量 |
| `addFreeTime` | 本步新增免费次数 |

### 4.2 `SpinSceneData`

场景会持久化到 Redis，用于串联普通局、连消步和免费模式：

| 字段 | 说明 |
| --- | --- |
| `Steps` | 当前 round 已进行了多少个连消 step |
| `Stage` | 当前阶段 |
| `NextStage` | 下一步进入的阶段 |
| `RoundMultiplier` | 当前 round 累计倍数 |
| `FreeNum` | 剩余免费次数 |
| `SymbolRoller` | 当前每列滚轴窗口状态 |
| `MysMultiplierTotal` | 神秘/长符号累计倍数字段 |
| `LongCount` | 免费模式下各中间列当前长符号数量 |

说明：

1. `LongCount` 当前仅用于免费模式继承长符号数量。
2. `MysMultiplierTotal` 当前会在 `ajtm` 代码中参与读取与清零，但没有看到在本目录内被实际累加的入口，属于已预留但未完整接入的状态字段。

### 4.3 阶段定义

| 常量 | 含义 |
| --- | --- |
| `_spinTypeBase = 1` | 普通 `spin` |
| `_spinTypeBaseEli = 11` | 普通消除步 |
| `_spinTypeFree = 21` | 免费 `spin` |
| `_spinTypeFreeEli = 22` | 免费消除步 |

## 5. 主流程

整体入口在 `betOrder()`：

```text
betOrder
  -> getRequestContext
  -> reloadScene
  -> baseSpin
  -> updateGameOrder
  -> settleStep
  -> saveScene
  -> getBetResultMap
```

### 5.1 `reloadScene`

处理逻辑：

1. 从 Redis 读取场景数据。
2. 如果反序列化失败，清场景。
3. 调 `syncGameStage()` 同步阶段。

`syncGameStage()` 的关键规则：

1. `Stage == 0` 时初始化为普通模式。
2. 如果 `NextStage > 0`，本次处理前先切换到 `NextStage`。
3. `Steps == 0` 时，清空本轮的 `RoundMultiplier`。
4. `isFreeRound` 由 `Stage` 是否处于免费阶段决定。

### 5.2 `initialize`

分两条路径：

1. `!isFreeRound && scene.Steps == 0`
   - 视为一个新普通局的第一步；
   - 扣款、校验余额、重置客户端免费局累计信息。
2. 其他情况
   - 视为同一 round 的后续步，或者免费模式中的步；
   - 不重复扣款；
   - 下注参数沿用上一单。

### 5.3 `baseSpin`

`baseSpin()` 是单步处理主入口：

1. 调 `initialize()`。
2. 如果当前是免费 `spin`，先减少免费次数并累加已执行次数。
3. 如果当前是一个 round 的第一步，重新生成盘面：
   - 普通模式：生成基础盘面；
   - 免费模式：生成免费盘面，并带上继承的长符号。
4. 把 `scene.SymbolRoller` 同步到 `symbolGrid`。
5. 查找中奖信息。
6. 处理中奖或未中奖逻辑。

## 6. 盘面生成

### 6.1 `initSpinSymbol`

根据当前模式选择滚轴配置：

- 普通模式用 `RollCfg.Base`
- 免费模式用 `RollCfg.Free`

再通过权重选出一个 `realIndex`，交给 `getSceneSymbol(realIndex)` 生成当前盘面。

### 6.2 `buildSymbolRoller`

负责构造单列滚轴窗口：

1. 随机一个 `start`。
2. 边列只填 `row=1~4`。
3. 中间列填满 `row=0~5`。
4. `Fall` 用于后续补位时回退取符号。

### 6.3 `calcLongBlocks`

当前实现分两种模式：

#### 普通模式

对每个中间列 `1~3`：

1. 由 `BaseBigSyWeights` 抽本列要生成几个长符号。
2. 从 `BigSyMultiples[count-1]` 中取布局模板。
3. 随机起点轮询模板，调用 `tryPatternWithCol()` 检查是否可用。
4. 如果模板头部位置是 `夺宝符号`，则视为冲突，跳过该模板。
5. 如果该列所有模板都冲突，则该列本局不生成长符号。

这部分就是当前"夺宝冲突规则"的实际实现。

#### 免费模式

1. 先按 `scene.LongCount` 恢复继承的长符号数量。
2. 如果总数还没满 `9` 个，再尝试新增 `1` 个长符号。
3. 免费模式调用 `tryPatternWithCol(..., forbidTreasure=false)`，因为免费滚轴当前实现中不会检查夺宝冲突。

### 6.4 `tryPatternWithCol`

职责：

1. 根据布局模板解析出每个长符号的 `HeadRow/TailRow`。
2. 检查越界。
3. 检查和已有长符号是否重叠。
4. 普通模式下额外检查头部是否命中夺宝。

### 6.5 `applyLongBlockToBoard`

将长符号写回中间列盘面：

1. 头部保留原符号；
2. 尾部写成 `1000 + 头部符号`；
3. 尾部以下元素整体下移一格；
4. 最底部元素被挤出；
5. 同时让该列 `Fall--`，保证后续补位位置与盘面一致。

### 6.6 `LongCount` 更新规则

`getSceneSymbol()` 中：

1. 每次生成新盘面时先把 `scene.LongCount` 清零。
2. 如果当前是免费模式，再根据本次盘面中的长符号块重新统计每列数量。

`processNoWin()` 中：

1. 如果当前是免费模式且本步未中奖，会调用 `refreshLongCountFromRoller()`；
2. 该函数直接扫描 `scene.SymbolRoller` 里的长符号头尾，重新写回 `scene.LongCount`；
3. 这样下一次免费 `spin` 会继承当前停留在盘面上的长符号数量。

## 7. 中奖判定

### 7.1 `findSymbolWinInfo`

按 `Ways` 规则从左到右判定：

1. 遍历普通符号 `1~12`。
2. 每列统计该符号或 `wild` 的命中数。
3. `lineCount` 为各列命中数乘积。
4. 至少连续命中 `3` 列才进入赔率计算。
5. `wild` 可替代任意普通符号，但中奖必须至少包含一个真实目标符号。

### 7.2 `findWinInfos`

处理步骤：

1. 遍历 `1~12` 的每个普通符号。
2. 把命中的 `WinInfo` 收集到 `winInfos`。
3. 把每个 `WinInfo.WinGrid` 合并成两张网格：
   - `winGrid`：用于展示；
   - `eliGrid`：用于真实消除。

### 7.3 `winGrid` 和 `eliGrid` 的区别

这是当前实现里最重要的一个拆分：

#### `winGrid`

用途：

1. 返回给客户端展示中奖区。
2. 写入订单 `BonusRawDetail / BonusDetail`。
3. 长符号命中时，头尾都会保留在 `winGrid` 中。

#### `eliGrid`

用途：

1. 只给 `moveSymbols()` 使用；
2. 只表示真正要被清掉的格子；
3. 长符号头尾不会写入 `eliGrid`。

因此，当前长符号的处理逻辑是：

1. 命中时显示中奖；
2. 不直接按长符号原值消除；
3. 先转变；
4. 再参与盘面后续流转。

### 7.4 长符号中奖记录

`recordWinningLongBlock()` 会记录命中的长符号块，写入 `winLongBlocks`：

- `Col`
- `HeadRow`
- `TailRow`
- `OldSymbol`

后续 `transformWinningLongSymbols()` 会使用这些信息。

## 8. 中奖与未中奖处理

### 8.1 `processWinInfos`

1. 重置 `addFreeTime`
2. 统计当前盘面的 `scatterCount`
3. 有中奖走 `processWin()`
4. 无中奖走 `processNoWin()`
5. 最后统一调用 `updateBonusAmount(stepMultiplier)`

### 8.2 `processWin`

中奖路径如下：

1. 累加所有 `WinInfo.Multiplier` 得到 `lineMultiplier`
2. 读取 `scene.MysMultiplierTotal`
   - 如果 `<= 0`，按 `1` 处理
3. 计算 `stepMultiplier = lineMultiplier * mysMul`
4. `scene.Steps++`
5. `scene.RoundMultiplier += stepMultiplier`
6. 调 `transformWinningLongSymbols()`
7. 调 `moveSymbols()` 进行真实消除与下落
8. 调 `fallingWinSymbols()` 把结果写回滚轴并补位
9. 设置下一阶段为普通消除步或免费消除步

### 8.3 `processNoWin`

未中奖路径如下：

#### 免费模式

1. `stepMultiplier = 0`
2. `isRoundOver = true`
3. `scene.Steps = 0`
4. 清空 `longEvents`
5. 调 `refreshLongCountFromRoller()` 更新继承长符号数量
6. 如果免费次数耗尽：
   - 清零 `MysMultiplierTotal`
   - 下阶段回到普通模式
7. 否则：
   - 下阶段继续免费模式

#### 普通模式

1. `stepMultiplier = 0`
2. `isRoundOver = true`
3. `scene.Steps = 0`
4. 清零 `MysMultiplierTotal`
5. 调 `calcNewFreeGameNum(scatterCount)` 判断是否触发免费
6. 若触发免费：
   - 给客户端增加免费次数
   - `scene.FreeNum += newFree`
   - `addFreeTime = newFree`
7. 根据 `scene.FreeNum` 决定下一阶段是普通模式还是免费模式

说明：

`calcNewFreeGameNum()` 返回两个值：

1. 新增免费次数
2. 触发奖励倍数

但当前调用方只使用了第一个返回值，第二个"免费触发奖励倍数"在当前实现中没有接入结算。

## 9. 长符号转变与下落

### 9.1 `transformWinningLongSymbols`

当前实现：

1. 只处理 `winLongBlocks` 中的命中长符号。
2. 新符号通过 `randomLongTransformSymbol()` 生成。
3. 新符号范围是 `1~12`。
4. 排除：
   - 原符号本身
   - `夺宝符号`
5. 转变后同步更新：
   - `symbolGrid`
   - `scene.SymbolRoller`
6. 同时记录到 `longEvents`

### 9.2 `longEvents`

返回给客户端的长符号事件字段，包含：

- `col`
- `headRow`
- `tailRow`
- `oldSymbol`
- `newSymbol`

客户端如果要表现"长符号命中后变成什么"，应该使用这个字段，而不是重新扫描盘面。

### 9.3 `moveSymbols`

`moveSymbols()` 只认 `eliGrid`：

1. `eliGrid > 0` 的格子清零；
2. 调 `dropSymbols()` 执行列内下落；
3. 返回下落后的 `nextSymbolGrid`。

### 9.4 `dropSymbols`

规则：

1. 每列从下往上扫描；
2. 非零元素压到底部；
3. 左右边列的四角不参与下落。

### 9.5 `fallingWinSymbols`

1. 把 `nextSymbolGrid` 写回 `scene.SymbolRoller`；
2. 调每列 `ringSymbol()` 补齐顶部掉落的新符号。

## 10. 免费模式

### 10.1 触发

普通模式下，如果当前盘面 `scatterCount >= FreeGameScatter`，则触发免费模式。

当前实现中真正生效的是：

1. 新增免费次数；
2. 下一个阶段切入免费模式。

### 10.2 免费次数消耗

在 `baseSpin()` 开始时：

1. 如果当前阶段是免费 `spin`
2. 且 `scene.FreeNum > 0`

则：

1. 客户端 `FreeTimes + 1`
2. 客户端剩余免费次数 `-1`
3. `scene.FreeNum--`

### 10.3 免费模式的长符号继承

当前实现按"数量继承"处理，而不是按精确坐标持久化：

1. `scene.LongCount[c]` 记录每列当前有几个长符号；
2. 新一轮免费盘面生成时，从下往上恢复这些长符号块；
3. 当前步未中奖时，再从滚轴重新扫描长符号数量，供下一轮继承。

### 10.4 免费模式结束

如果免费模式本步未中奖，且 `scene.FreeNum <= 0`：

1. 清零 `scene.MysMultiplierTotal`
2. `scene.NextStage = _spinTypeBase`
3. 下次重新回到普通模式

## 11. 订单与返回

### 11.1 订单明细

`fillInGameOrderDetails()` 当前写入规则：

1. `BetRawDetail` / `BetDetail` 存 `symbolGrid`
2. `BonusRawDetail` / `BonusDetail` 存 `winGrid`
3. `WinDetails` 存 `buildWinInfo()` 的结果

也就是说，订单里保存的是"中奖展示网格"，不是"实际消除网格"。

### 11.2 返回协议重点字段

`pb/ajtm.proto` 当前重点字段：

| 字段 | 含义 |
| --- | --- |
| `cards` | 当前盘面 |
| `winGrid` | 中奖展示网格 |
| `longEvents` | 长符号转变事件 |
| `scatterCount` | 当前盘面夺宝数 |
| `multi` | 本步最终倍数 |
| `roundWin` | 当前 round 累计赢分 |
| `currentWin` | 本步赢分 |
| `freeNum` | 剩余免费次数 |
| `freeTime` | 已执行免费次数 |
| `isRoundOver` | 当前 round 是否结束 |
| `isGameOver` | 免费总流程是否结束 |

说明：

1. `multi` 现在已经对齐到 `stepMultiplier`。
2. `roundWin` 现在按 `scene.RoundMultiplier` 计算，表示当前 round 累计赢分，不只是本步赢分。
3. 当前协议里没有 `eliGrid` 字段，对外只返回 `winGrid` 和 `longEvents`。

## 12. 当前实现说明与注意点

### 12.1 已对齐的部分

1. 普通模式下长符号会规避"头部落在夺宝符号上"的冲突。
2. 长符号中奖后不会直接按原值进入消除网格。
3. 长符号中奖后会先转变，再参与后续盘面流转。
4. 免费模式未中奖时会更新 `LongCount`，实现长符号继承。
5. `betOrderService` 已提供 `longEvents` 给客户端使用。

### 12.2 当前仍属于"预留 未完整接入"的部分

1. `MysMultiplierTotal`
   - 当前会被读取、传递、清零；
   - 但在 `game/ajtm` 目录内未看到实际累加逻辑。
2. 免费触发奖励倍数
   - `calcNewFreeGameNum()` 已返回；
   - 但当前未进入最终结算。
3. `proto` 中部分字段仍是预留字段：
   - `mulIndex`
   - `baseMultipliers`
   - `freeMultipliers`
   - `wildEliCount`
   - `totalWildEliCount`

### 12.3 关于文档口径

本文档描述的是"代码当前怎么运行"，不是"策划最初想要它怎么运行"。

如果后续策划文档再调整，优先以代码改动为准同步更新本文档。

## 13. 测试

当前推荐测试入口：

```bash
go test ./game/ajtm -run TestRtp2 -count=1
```

这也是本轮修复和整理时使用的主验证用例。