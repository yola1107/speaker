# 原神（`ys`）后端实现说明

> 适用目录：`game/ys`  
> `GameID`：`18976`  
> 策划入口：<https://qr8tls.axshare.com/?g=1&id=agx1gr&p=%E4%B8%80%E3%80%81%E6%B8%B8%E6%88%8F%E7%AE%80%E4%BB%8B_1>  
> 当前代码来源：从其他项目拷贝后的待改造版本

## 1. 文档定位

本文用于把 Axure 策划目录和当前 `game/ys` 代码现状整理成后端实现基线。当前代码已经有 `bet_order`、`scene`、`proto`、配置和 RTP 测试框架，但仍保留了明显的旧游戏痕迹，不能直接视为新游戏定稿。

Axure 页面入口会跳转鉴权；目前可公开抓取的信息主要来自 `data/document.js` 的站点目录和页面资源。具体赔率、倍率、权重、大奖阈值等数值需要策划表或可访问的原型页面补齐。

## 2. Axure 可确认内容

Axure 目录中 `原神` 节点包含以下页面：

1. `一、游戏简介`
   - `主界面展示`
   - `爆衣状态`
2. `二、普通模式流程`
   - `中奖效果展示`
   - `元素反应`
   - `蒸发（水火）`
   - `超载（雷火）`
   - `感电（水雷）`
   - `最终倍乘`
   - `免费游戏`
   - `奖励模式`
   - `三大奖`
3. `三、资源清单`
   - `符号列表`
   - `符号 tips`
   - `跑马灯`
   - `赔率表规则表`

后端侧可先确认三条主线：基础 ways 消除、元素反应/最终倍乘、免费游戏与大奖展示。

## 3. 当前代码现状

### 3.1 已有能力

- `const.go` 定义 `5x6` 盘面、符号、阶段和 `GameID`。
- `bet_order_spin.go` 已有基础 spin 流程：初始化、发牌、判奖、处理中奖或未中奖。
- `bet_order_helper.go` 已有 ways 判奖、wild 替代、中奖格合并、消除下落和补符逻辑。
- `bet_order_scene.go` 已有 Redis 场景保存，支持普通、普通消除、免费、免费消除阶段。
- `bet_order_configs.go` 与 `game_json.go` 已有内置配置：赔付表、免费参数、普通/免费轮带。
- `pb/ys.proto` 已有前端回包字段，包含盘面、中奖格、免费次数、当前倍数、中奖条目等。
- `rtp_test.go` 已有 RTP 跑数骨架。

### 3.2 明显旧代码痕迹

- `pb/ys.proto` 注释仍写 `权能觉醒 (GameID: 19009)`，需要改为本游戏。
- 多处注释仍写 `3行5列` 或 `3x5`，但常量实际是 `_rowCount = 5`、`_colCount = 6`。
- `game_json.go` 轮带中出现符号 `9`，但 `const.go` 只定义到 `_treasure = 8`，且赔付表只有 8 行。
- `pb` 中 `colorMul` 注释是蓝/黄/绿收集玩法，和当前 Axure 目录的元素反应不完全一致。
- `README` 之前的实现状态已经过期，不能作为开发依据。

## 4. 当前规则基线

### 4.1 盘面和下注

- 盘面：`5x6`，按代码常量为准。
- 基础倍率：`_baseMultiplier = 20`。
- 单次投注：`BaseMoney * Multiple * _baseMultiplier`。
- RTP 调试中使用 `_baseMultiplier` 作为单位投注倍数。

### 4.2 符号

当前 `const.go` 定义如下：

- `0`：空白
- `1`：Q
- `2`：K
- `3`：A
- `4`：水神
- `5`：火神
- `6`：雷神
- `7`：wild 百搭
- `8`：treasure 夺宝

待确认点：轮带里的 `9` 是否是未定义符号、女主角、特殊符号，还是旧代码残留。未确认前不应继续调 RTP。

### 4.3 阶段

- `_spinTypeBase = 1`：普通首步
- `_spinTypeBaseEli = 11`：普通消除续步
- `_spinTypeFree = 21`：免费首步
- `_spinTypeFreeEli = 22`：免费消除续步

服务端通过 `scene.NextStage` 驱动下一次请求进入对应阶段。

### 4.4 判奖和消除

当前判奖逻辑是从左到右 ways：

1. 每列统计目标符号或 wild 的数量。
2. 连续命中列数至少 `_minMatchCount = 3`。
3. 路数为各命中列匹配数相乘。
4. 赔率来自 `pay_table[symbol-1][symbolCount-1]`。
5. 中奖后清除 `winGrid`，符号下落，再从对应轮带补符。
6. 有中奖则回合未结束，前端继续请求下一步消除；无中奖则回合结束或进入免费。

当前 `processWin` 只把各中奖项的 `Odds` 相加，没有把 `LineCount` 计入 `stepMultiplier`，但 `WinInfo.Multiplier` 已经保存了 `Odds * LineCount`。这里需要按策划确认最终赔付口径。

## 5. 后端待实现项

### 5.1 先修正基础一致性

1. 确认盘面到底是 `5x6` 还是 `3x5`，然后统一 `const.go`、注释、proto 注释、README 和测试描述。
2. 补齐符号表，特别是轮带中的 `9`。
3. 明确 `wild` 是否参与自身赔付，`treasure` 是否参与 ways 赔付。
4. 修正 ways 赔付倍数：确认 `stepMultiplier` 使用 `Odds`、`LineCount` 还是 `Odds * LineCount`。
5. 移除或重命名旧玩法字段，例如 `colorMul`、`权能觉醒` 注释。

### 5.2 实现元素反应

Axure 明确列出三种反应：

- 蒸发：水火
- 超载：雷火
- 感电：水雷

建议后端新增结构化结果，至少包含：

- 反应类型
- 参与符号/位置
- 反应倍率或额外赢分
- 反应前赢分
- 反应后赢分

待策划确认：

- 反应基于中奖符号、全盘符号，还是消除后的收集结果。
- 同一局可否触发多个反应。
- 同一符号可否参与多个反应。
- 多个反应之间是相加、相乘、按优先级覆盖，还是按顺序结算。

### 5.3 实现最终倍乘

Axure 有独立页面 `最终倍乘`，建议把它作为独立结算步骤，不混入基础 ways 判奖：

```text
base_win = ways_win
reaction_win = apply_reaction(base_win)
step_win = reaction_win * final_multiplier
```

需要确认：

- 最终倍乘触发条件。
- 倍率池。
- 是否只在普通模式触发，还是普通/免费都触发。
- 是否作用于本步赢分、本回合累计赢分，还是免费总赢分。

### 5.4 实现免费游戏

当前代码已有免费次数状态和触发配置：

- `scatter_min`
- `free_times`
- `per_scatter_add_times`

待完善：

- 免费触发是否由 `treasure` 数量决定。
- 免费触发是在每步结算、回合结束，还是整个消除完成后判断。
- 免费模式是否有独立轮带、独立反应参数、独立最终倍乘参数。
- 免费模式是否允许重触发。
- `奖励模式` 是否是免费内的二级状态机。

### 5.5 实现三大奖

Axure 有 `三大奖` 页面，后端建议按赢分/投注倍数输出枚举：

- `NONE`
- `BIG_WIN`
- `MEGA_WIN`
- `SUPER_MEGA_WIN`

需要确认阈值口径：按单步赢分、单个 round 累计赢分，还是免费总赢分计算。

### 5.6 RTP 和配置

正式开发前需要把配置拆为可调参数：

- 普通/免费轮带权重
- 符号赔付表
- scatter/free 参数
- 元素映射和反应参数
- 最终倍乘参数
- 三大奖阈值
- 最大赔付封顶

RTP 测试至少输出：

- 普通 RTP
- 免费 RTP
- 总 RTP
- 普通中奖率
- 免费中奖率
- 免费触发率
- 平均免费次数
- 最大赢分分布

## 6. 协议建议

现有 `ys_BetOrderResponse` 能支撑基础盘面展示，但新玩法建议扩展：

- `reactionEvents`：元素反应事件列表。
- `reactionMul` 或 `reactionWin`：反应结算影响。
- `finalMul`：最终倍乘。
- `bigWinType`：三大奖类型。
- `rewardMode`：奖励模式状态。
- `roundMul`：当前 round 累计倍率。

如果前端只需要展示最终结果，也应在 `WinDetails` 中保留这些字段，便于订单回放、对账和问题排查。

## 7. 建议实施顺序

1. 统一盘面、符号、proto 注释和配置，先消除旧项目残留。
2. 修正基础 ways 赔付，补单测覆盖 wild、scatter、消除下落、免费触发。
3. 接入元素反应，先做纯函数测试，再接入 spin 流程。
4. 接入最终倍乘和三大奖。
5. 完善免费模式与奖励模式状态。
6. 跑 RTP，按策划目标调权重和倍率。
7. 联调前端回包字段和订单回放。

## 8. 待策划确认清单

1. 盘面规格：`5x6` 还是 `3x5`。
2. 完整符号列表，特别是符号 `9`。
3. 每个符号的元素归属。
4. `wild` 和 `treasure` 的判奖/替代/触发规则。
5. ways 赔付公式。
6. 三种元素反应的触发、叠加和倍率规则。
7. 最终倍乘触发条件和倍率池。
8. 免费触发、重触发、奖励模式规则。
9. 三大奖阈值。
10. 最大赔付封顶口径。

## 9. 参考文件

- `game/ys/const.go`
- `game/ys/bet_order_spin.go`
- `game/ys/bet_order_helper.go`
- `game/ys/bet_order_scene.go`
- `game/ys/bet_order_configs.go`
- `game/ys/game_json.go`
- `game/ys/pb/ys.proto`
- `game/ys/rtp_test.go`
