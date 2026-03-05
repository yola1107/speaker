---
name: multi-model-dev
description: 多模型协作开发工作流。用于需要不同模型负责不同任务的场景，如文案+后端+前端开发。
---

## 多模型协作工作流

使用此 Skill 时，按以下步骤切换模型执行任务。

---

## 阶段 1：文案创作

**模型**：`glm-5` 或 `kimi-k2.5`（文本生成能力强）

```bash
/model glm-5
```

**任务**：
- 产品文案
- 用户文档
- 界面文案

---

## 阶段 2：后端开发

**模型**：`qwen3-max-2026-01-23`（深度思考能力强）

```bash
/model qwen3-max-2026-01-23
```

**任务**：
- API 设计
- 业务逻辑实现
- 数据库设计

---

## 阶段 3：前端开发

**模型**：`MiniMax-M2.5`（视觉理解 + 文本生成）

```bash
/model MiniMax-M2.5
```

**任务**：
- HTML/CSS 页面
- 用户界面设计
- 前端交互逻辑

---

## 阶段 4：代码审查

**模型**：`qwen3-max-2026-01-23`（深度分析）

```bash
/model qwen3-max-2026-01-23
/code-review
```

---

## 快速模型切换

| 任务类型 | 模型 | 命令 |
|----------|------|------|
| 文案/文档 | glm-5 | `/model glm-5` |
| 快速任务 | glm-4-flash | `/model glm-4-flash` |
| 复杂开发 | qwen3-max | `/model qwen3-max-2026-01-23` |
| 前端/UI | MiniMax-M2.5 | `/model MiniMax-M2.5` |

---

## 示例工作流

```
# 1. 文案
/model glm-5
> 帮我写一个在线书城的产品介绍

# 2. 后端
/model qwen3-max-2026-01-23
> 根据上面的文案，用 Kratos 框架实现后端 API

# 3. 前端
/model MiniMax-M2.5
> 根据文案和 API，创建精美的 HTML 首页
```