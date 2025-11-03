# XSLM2（西施恋美2）游戏文档

## 🎮 基本信息

| 项目 | 说明 | 项目 | 说明 |
|------|------|------|------|
| 游戏ID | 18892 | 网格 | 4×5 |
| 玩法 | Ways | 最小中奖 | 3列 |
| 基础倍数 | 20倍 | 免费触发 | 3个夺宝 |
| 数据来源 | Redis预设数据 | 特色 | 女性符号收集 |

### 符号说明

| 符号 | 说明 |
|------|------|
| 1-6 | 普通符号 |
| **7-9 (女性A/B/C)** | **可收集符号**（免费模式） |
| 10 | Wild女性A |
| 13 | Wild（百搭） |
| **14 (夺宝)** | 触发免费游戏 |

---

## 🌟 核心机制

### 女性符号收集系统 ⭐

**触发条件**：免费游戏模式

**收集规则**：
- 女性A/B/C三种符号独立计数
- 每次中奖的女性符号累加计数
- 计数持续跨多个spin

**全屏消除**：
- 任意女性符号收集满**10个**
- 该符号全部转换为Wild
- 重新计算中奖
- 清空该符号计数（其他保留）

**示例**：
```
初始：[A:0, B:0, C:0]
第1局：中2个A, 1个B → [A:2, B:1, C:0]
第2局：中3个A → [A:5, B:1, C:0]
第3局：中5个A → [A:10, B:1, C:0] ← 触发！
第4局：所有A转Wild，重新计算 → [A:0, B:1, C:0]
```

### 免费游戏

**触发条件**：
- 3个夺宝 = 7次免费
- 4个夺宝 = 10次免费
- 5个夺宝 = 15次免费

**免费模式特性**：
- 激活女性符号收集
- 有女性中奖+有Wild → 继续消除
- 可追加免费次数

---

## 📁 文件组织

```
xslm2/
├── xslm2_bet_order.go        - betOrder主逻辑
├── xslm2_step.go             - 订单步骤处理（含base/free）
├── xslm2_first_step.go       - 首次步骤初始化
├── xslm2_next_step.go        - 下一步骤处理
├── xslm2_scene.go            - 场景数据管理
├── xslm2_mdb.go              - MySQL数据库操作
├── xslm2_rdb.go              - Redis预设数据
├── xslm2_spin.go             - Spin逻辑（含base/free）
├── xslm2_spin_helper.go      - Spin辅助函数
├── xslm2_helpers.go          - 通用辅助函数
├── xslm2_member_login.go     - 用户登录
├── xslm2_types.go            - 类型定义
├── xslm2_const.go            - 常量定义
├── xslm2_exported.go         - 对外接口
└── rtp_test.go               - RTP测试

总计：15个文件，~1500行（优化后）
```

---

## 🔄 游戏流程

### 基础模式

```
1. 根据概率选择期望倍率
2. 从Redis查找匹配的预设数据
3. 加载预设符号到网格
4. 计算Ways中奖
5. 有女性中奖+有Wild → 继续下一step
6. 检查夺宝 → 触发免费
```

### 免费模式

```
1. 加载预设符号（免费滚轴）
2. 计算Ways中奖
3. 统计中奖的女性符号
4. 检查收集计数：
   ├─ =10 → 全屏消除
   └─ <10 → 继续收集
5. 有女性中奖 → 继续下一step
6. 无女性中奖 → 检查夺宝追加次数
```

---

## ⚙️ 预设数据系统

### 数据结构

```
Redis Key: {site}:slot_xslm_data
格式：Hash
  Field: presetID
  Value: JSON {
    "id": 1,
    "kind": 0/1,  // 0=不带免费, 1=带免费
    "total_multiplier": 1000,
    "spin_maps": [
      {"id": 1, "fc": [0,0,0], "mp": [...]},
      {"id": 2, "fc": [2,0,0], "mp": [...]},
      ...
    ]
  }
```

### 选择逻辑

```
1. 随机选择期望倍率（根据概率权重）
2. 查询Redis: {site}:slot_xslm_id:{kind}:{multiplier}
3. 随机选择一个预设ID
4. 加载完整预设数据（包含所有step）
```

---

## 📊 女性符号数据

### 数据持久化

**stepMap结构**：
```go
type stepMap struct {
    ID                  int64     `json:"id"`
    FemaleCountsForFree []int64   `json:"fc"`  // [A计数, B计数, C计数]
    Map                 [20]int64 `json:"mp"`  // 符号网格
}
```

**场景数据**：
```go
type scene struct {
    PresetID uint64  // 当前预设ID
    MapID    uint64  // 当前step ID
}
```

### 收集计数规则

```go
const _femaleSymbolCountForFullElimination = 10  // 触发阈值

// 收集计数存储
nextFemaleCountsForFree [3]int64
  [0] = femaleA计数 (符号7)
  [1] = femaleB计数 (符号8)
  [2] = femaleC计数 (符号9)
```

---

## 🛠️ 开发

### 环境要求
- Go 1.18+
- MySQL（预设数据表）
- Redis（预设数据缓存）

### 测试
```bash
cd game/xslm2
go test -v -run TestRtp
```

---

## 🎯 与xslm的区别

| 特性 | xslm | xslm2 |
|------|------|-------|
| 数据源 | Redis预设 | Redis预设 |
| 代码风格 | 传统 | 现代（21个文件） |
| 模块化 | 一般 | 良好 |

---

## 🆚 与xxg2的区别

| 特性 | xslm2 | xxg2 |
|------|-------|------|
| 数据源 | Redis预设 | RealData动态生成 |
| 特色机制 | 女性符号收集 | 蝙蝠移动转换 |
| 文件数 | 21 | 14 |
| 代码量 | 1422行 | 2101行 |

---

**游戏ID**：18892 | **代码**：1422行 | **质量**：⭐⭐⭐⭐

