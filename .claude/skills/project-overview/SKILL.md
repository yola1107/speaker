---
name: project-overview
description: 项目概览和 Skills 使用指南。用于了解项目中的所有自定义 Skills 和使用方法。
---

## 项目信息

**项目名称**：Speaker - Kratos v2 微服务

**技术栈**：Go 1.24.5 + Kratos + Wire + MySQL + Redis

---

## 自定义 Skills 列表

| Skill | 用途 | 命令 |
|-------|------|------|
| `multi-model-dev` | 多模型协作开发 | `/multi-model-dev` |
| `kratos-api` | Kratos API 开发 | `/kratos-api` |
| `go-test-gen` | Go 单元测试生成 | `/go-test-gen` |
| `code-refactor` | 代码重构优化 | `/code-refactor` |
| `git-workflow` | Git 工作流 | `/git-workflow` |
| `project-overview` | 项目概览（当前） | `/project-overview` |

---

## Skills 使用流程

### 开发新功能

```
1. /multi-model-dev    # 选择合适的模型
2. /kratos-api         # 按步骤开发 API
3. /go-test-gen        # 生成测试
4. /git-workflow       # 提交代码
```

### 重构代码

```
1. /code-refactor      # 分析并重构
2. /go-test-gen        # 确保测试通过
```

### 修复 Bug

```
1. /systematic-debugging  # 系统化调试
2. /go-test-gen           # 写测试用例
3. /git-workflow          # 提交修复
```

---

## 插件 Skills（来自 Superpowers）

| Skill | 用途 |
|-------|------|
| `/brainstorming` | 头脑风暴 |
| `/systematic-debugging` | 系统化调试 |
| `/test-driven-development` | TDD 开发 |
| `/simplify` | 简化代码 |
| `/code-review` | 代码审查 |
| `/finding-duplicate-functions` | 查找重复代码 |

---

## 快速开始

```bash
# 查看所有 Skills
ls .claude/skills/*/

# 调用 Skill
/<skill-name>
```