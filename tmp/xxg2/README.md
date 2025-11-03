# XXG2（吸血鬼）游戏文档

## 🎮 基本信息

| 项目 | 说明 | 项目 | 说明 |
|------|------|------|------|
| 游戏ID | 18891 | 网格 | 4×5 |
| 玩法 | Ways | 最小中奖 | 3列 |
| 基础倍数 | 20倍 | 免费触发 | 3个Treasure |

### 符号说明

| 符号 | 倍率(3/4/5列) | 说明 |
|------|--------------|------|
| 1-4 (J/Q/K/A) | 2/3/5 | 低倍符号 |
| 5-6 (十字架/酒杯) | 4-7/10-15 | 中倍符号 |
| **7-9 (小孩/女人/老头)** | 12-40 | **Wind符号**（可转Wild） |
| **10 (Wild)** | 20/30/50 | 百搭 |
| **11 (Treasure)** | - | 夺宝（触发免费） |

---

## 🦇 核心机制

### Wind转换机制

**基础模式**（1-2个Treasure）：
- Treasure射线到Wind符号 → 转为Wild
- bat记录射线轨迹：`{x:列, y:行, nx:目标列, ny:目标行}`

**免费模式**（免费游戏中）：
- 蝙蝠从上轮位置随机移动一格（8方向）
- 移到人符号(7/8/9) → 转为Wild
- 最多5个蝙蝠同时移动 (可配置)

**免费触发**：
- ≥3个Treasure
- 免费次数：10 + (夺宝数-3)×2
- 免费中每个Treasure +1次 (注意：每次spin不限制，累加)

---

## 🔄 baseSpin 流程

```
1. initSpinSymbol()    - 生成符号
2. loadStepData()      - 加载网格，扫描treasure
3. collectBat()        - Wind转换（基础=射线，免费=移动）
4. findWinInfos()      - 查找Ways中奖
5. processWinInfos()   - 计算倍率
6. updateBonusAmount() - 计算奖金
7. updateResult()      - 更新状态（基础/免费）
```

---

## 📁 文件组织

```
xxg2/
├── xxg2_bet_order.go      - betOrder主逻辑
├── xxg2_spin.go           - baseSpin核心逻辑
├── xxg2_order_step.go     - 中奖计算、订单处理
├── xxg2_spin_helper.go    - 辅助函数、坐标转换
├── xxg2_order_scene.go    - 场景数据持久化
├── xxg2_order_next_step.go - 免费模式初始化
├── xxg2_order_mdb.go      - 数据库操作
├── xxg2_types.go          - 类型定义
├── xxg2_const.go          - 常量定义
├── xxg2_configs.go        - 配置加载
├── xxg2_configs_json.go   - JSON配置
├── xxg2_exported.go       - 对外接口
├── xxg2_helpers.go        - 蝙蝠数据说明
└── rtp_test.go            - RTP测试
```

---

## ⚙️ 配置

**倍率表** (`pay_table`)：符号1-10的3/4/5列倍率   
**免费触发** (`free_game_trigger_scatter`): 3   
**免费次数** (`free_game_init_times`): 10   
**额外次数** (`extra_scatter_extra_time`): 2   
**最大蝙蝠** (`max_bat_positions`): 5   

---

## 🛠️ 开发

### RTP测试
```bash
cd game/xxg2
go test -v -run TestRtp
```

### 配置修改
- 倍率：`xxg2_configs_json.go` 的 `pay_table`
- 免费次数：`free_game_init_times`
- 蝙蝠数量：`xxg2_configs.go` 的 `MaxBatPositions`

---

## 🎯 与xxg的区别

| 特性 | xxg | xxg2                   |
|------|-----|------------------------|
| 数据源 | Redis预设 | RealData动态生成           |
| Bat生成 | 预设读取 | 实时计算                   |
| 代码风格 | 传统 | 现代（参考mahjong+mahjong2） |

---

## 📊 坐标转换说明

### 需要转换的数据

| 字段 | 转换规则 |
|------|---------|
| **bat** | 交换X/Y（服务器X=行/Y=列 → 客户端x=列/y=行） |
| **winResults.WinPositions** | 行序反转（[0,1,2,3] → [3,2,1,0]） |

### 不转换的数据

- **symbolGrid**：服务器和客户端格式一致
- **winGrid**：从symbolGrid派生，格式一致

### 转换函数

```go
// game/xxg2/xxg2_spin_helper.go
reverseBats()        - 交换bat的X/Y坐标
reverseWinResults()  - 反转WinPositions行序
reverseGridRows()    - 通用网格行序反转
```

---

**游戏ID**：18891 | **总代码**：~1500行 |
