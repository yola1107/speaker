# 阿里云 Claude Code 配置指南

> 📚 官方文档：[https://help.aliyun.com/zh/model-studio/claude-code-coding-plan](https://help.aliyun.com/zh/model-studio/claude-code-coding-plan)

---

## 🔑 账号信息

| 配置项 | 值 |
|--------|-----|
| **API Key** | `sk-90678e99b17247fa88095d31af427d35` |
| **API Key (SP)** | `sk-sp-c586f31b286b49fd9515e974216fd67e` |
| **Base URL** | `https://coding.dashscope.aliyuncs.com/apps/anthropic` |

---

## 📁 配置文件设置

### 1️⃣ 创建配置目录

```bash
mkdir -p ~/.claude
```

### 2️⃣ 主配置文件 `~/.claude/settings.json`

```json
{
  "env": {
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1",
    "ANTHROPIC_AUTH_TOKEN": "sk-sp-c586f31b286b49fd9515e974216fd67e",
    "ANTHROPIC_BASE_URL": "https://coding.dashscope.aliyuncs.com/apps/anthropic",
    "ANTHROPIC_MODEL": "qwen3-coder-plus"
  },
  "language": "中文",
  "model": "qwen3-coder-plus",
  "enabledPlugins": {
    "superpowers@superpowers-marketplace": true,
    "code-review@claude-plugins-official": true
  },
  "autoApprove": true,
  "permissions": {
    "allow": [
      "*"
    ],
    "defaultMode": "bypassPermissions"
  },
  "skipDangerousModePermissionPrompt": true
}
```

### 3️⃣ 补充配置文件 `~/.claude.json`

```json
{
  "hasCompletedOnboarding": true
}
```

---

## 💻 环境变量配置

### Windows (CMD)

```cmd
setx CLAUDE_CODE_GIT_BASH_PATH "D:\soft\Git\bin\bash.exe"
setx DASHSCOPE_API_KEY "sk-90678e99b17247fa88095d31af427d35"
setx ANTHROPIC_AUTH_TOKEN "sk-sp-c586f31b286b49fd9515e974216fd67e"
setx ANTHROPIC_BASE_URL "https://coding.dashscope.aliyuncs.com/apps/anthropic"
setx ANTHROPIC_MODEL "glm-5"
```

### Linux/macOS (Bash/Zsh)

在 `~/.bashrc` 或 `~/.zshrc` 中添加：

```bash
export DASHSCOPE_API_KEY="sk-90678e99b17247fa88095d31af427d35"
export ANTHROPIC_AUTH_TOKEN="sk-sp-c586f31b286b49fd9515e974216fd67e"
export ANTHROPIC_BASE_URL="https://coding.dashscope.aliyuncs.com/apps/anthropic"
export ANTHROPIC_MODEL="glm-5"
```

然后执行：
```bash
source ~/.bashrc  # 或 source ~/.zshrc
```

---

## 🤖 模型切换

支持的模型：
```bash
/model qwen3-coder-plus
/model qwen3-coder-next
/model qwen3.5-plus
/model glm-5
/model glm-4.7
/model qwen3-max-2026-01-23
/model kimi-k2.5
/model MiniMax-M2.5
```

---

## 🔌 插件管理

### 添加插件市场

```bash
/plugin marketplace add anthropics/skills
/plugin marketplace update claude-plugins-official
/plugin marketplace add davepoon/buildwithclaude
/plugin marketplace add obra/superpowers-marketplace
```

### 安装插件

```bash
# Superpowers 系列
/plugin install superpowers@superpowers-marketplace

# LSP 支持
/plugin install gopls-lsp@claude-plugins-official

# 代码审查
/plugin install code-review@claude-plugins-official

# 安全指导
/plugin install security-guidance@claude-plugins-official
```

---

## 🚀 启动 Claude Code

```bash
claude --dangerously-skip-permissions
```

---

## 📦 Everything Claude Code (ECC) 插件

### 📘 学习资源
- 知乎文章：[https://zhuanlan.zhihu.com/p/1997766309396633154](https://zhuanlan.zhihu.com/p/1997766309396633154)

### 🛠️ 快速安装

#### 第一步：安装插件

```bash
# 添加市场
/plugin marketplace add affaan-m/everything-claude-code

# 安装插件
/plugin install everything-claude-code@everything-claude-code
```

#### 第二步：安装规则（必需）

⚠️ **重要**：Claude Code 插件无法自动分发 rules，需要手动安装

```bash
# 克隆仓库
git clone https://github.com/affaan-m/everything-claude-code.git

# 复制规则到配置目录
cp -r everything-claude-code/rules/* ~/.claude/rules/
```

#### 第三步：验证安装

```bash
# 查看可用命令
/plugin list everything-claude-code@everything-claude-code

# 测试使用（两种形式）
# 形式1：命名空间
/everything-claude-code:plan "添加用户认证"

# 形式2：简短形式
/plan "添加用户认证"
```

#### 📈 数据处理与分析（2个）

| Skill | 用途 | 命令 |
|-------|------|------|
| `postgres-patterns` | PostgreSQL 查询优化（已列在数据库） | - |
| `nutrient-document-processing` | 文档处理（OCR、转换、提取等） | - |

---

✨ **完成！** 你现在可以使用：

- **100+ 个技能**（97个核心技能，部分技能在多个类别中重复）
- **13+ 个代理**（Agent）
- **50+ 个命令**（Commands）

**技能分类：**
- 核心开发工作流（17个）
- 规划与设计（12个）
- 语言特定技能（18个）
- 数据库技能（4个）
- 持续学习与技能管理（14个）
- 内容与文档（8个）
- 安全与性能（11个）
- 其他工具与框架（10个）
- 数据处理与分析（3个）

---

### 📋 ECC Skills 完整列表（126个）

#### 🔧 核心开发工作流（17个）

| Skill | 用途 | 命令 |
|-------|------|------|
| `tdd` | 强制执行 TDD 工作流 | `/tdd` |
| `go-test` | Go 的 TDD 工作流 | `/go-test` |
| `build-fix` | 修复构建错误 | `/build-fix` |
| `go-build` | 修复 Go 构建和 vet 错误 | `/go-build` |
| `code-review` | 代码审查 | `/code-review` |
| `go-review` | Go 代码审查 | `/go-review` |
| `python-review` | Python 代码审查 | `/python-review` |
| `refactor-clean` | 死代码清理 | `/refactor-clean` |
| `e2e` | 生成和运行 E2E 测试 | `/e2e` |
| `e2e-testing` | Playwright E2E 测试模式 | - |
| `test-coverage` | 测试覆盖率检查 | `/test-coverage` |
| `verify` | 验证命令 | `/verify` |
| `eval` | 评估命令 | `/eval` |
| `checkpoint` | 保存验证状态 | `/checkpoint` |
| `quality-gate` | 质量门禁 | `/quality-gate` |
| `update-docs` | 更新文档 | `/update-docs` |
| `update-codemaps` | 更新代码地图 | `/update-codemaps` |

#### 📝 规划与设计（12个）

| Skill | 用途 | 命令 |
|-------|------|------|
| `plan` | 实现规划 | `/plan` |
| `agent-harness-construction` | AI 代理动作空间设计优化 | - |
| `agentic-engineering` | 使用 eval 框架的代理工程 | - |
| `autonomous-loops` | 自主代理循环架构模式 | - |
| `ai-first-engineering` | 以 AI 为中心的工程运营模式 | - |
| `api-design` | REST API 设计模式 | - |
| `backend-patterns` | 后端架构、API、数据库、缓存模式 | - |
| `frontend-patterns` | React、Next.js 前端模式 | - |
| `deployment-patterns` | CI/CD 部署流水线模式 | - |
| `docker-patterns` | Docker/Compose 模式 | - |
| `clickhouse-io` | ClickHouse 数据库模式 | - |
| `content-hash-cache-pattern` | 基于内容哈希的缓存模式 | - |

#### 📝 规划与设计

| Skill | 用途 | 命令 |
|-------|------|------|
| `plan` | 实现规划 | `/plan` |
| `api-design` | REST API 设计模式 | - |
| `backend-patterns` | API、数据库、缓存模式 | - |
| `frontend-patterns` | React、Next.js 模式 | - |
| `deployment-patterns` | CI/CD 流水线 | - |
| `docker-patterns` | Docker/Compose 模式 | - |

#### 💻 语言特定技能（18个）

| Skill | 用途 | 命令 |
|-------|------|------|
| **Go 系列（5个）** | | |
| `golang-patterns` | Go 惯用语和最佳实践 | - |
| `golang-testing` | Go 测试模式、表格驱动测试 | - |
| `go-test` | Go 的 TDD 工作流 | `/go-test` |
| `go-review` | Go 代码审查 | `/go-review` |
| `go-build` | 修复 Go 构建、vet、linter 错误 | `/go-build` |
| **Python 系列（3个）** | | |
| `python-patterns` | Python 惯用语、PEP 8、类型提示 | - |
| `python-testing` | pytest、TDD 测试策略 | - |
| `python-review` | Python 代码审查 | `/python-review` |
| **Java/Spring 系列（6个）** | | |
| `java-coding-standards` | Java 编码标准（Spring Boot） | - |
| `springboot-patterns` | Spring Boot 架构模式 | - |
| `springboot-security` | Spring Security 最佳实践 | - |
| `springboot-tdd` | Spring Boot 的 TDD | - |
| `springboot-verification` | Spring Boot 验证循环 | - |
| `jpa-patterns` | JPA/Hibernate 模式 | - |
| **Swift 系列（4个）** | | |
| `swiftui-patterns` | SwiftUI 架构模式 | - |
| `swift-actor-persistence` | Swift 持久化（Actor 模型） | - |
| `swift-protocol-di-testing` | Swift 协议依赖注入测试 | - |
| `swift-concurrency-6-2` | Swift 6.2 并发 | - |
| **Foundation Models** | | |
| `foundation-models-on-device` | Apple FoundationModels 设备端推理 | - |

#### 🗄️ 数据库技能（4个）

| Skill | 用途 | 命令 |
|-------|------|------|
| `postgres-patterns` | PostgreSQL 查询优化、schema设计 | - |
| `clickhouse-io` | ClickHouse 数据库模式、查询优化 | - |
| `database-migrations` | 数据库迁移最佳实践 | - |
| `jpa-patterns` | JPA/Hibernate 实体设计模式 | - |

#### 🧠 持续学习与技能管理（14个）

| Skill | 用途 | 命令 |
|-------|------|------|
| `continuous-learning` | 从会话自动提取模式 | `/learn` |
| `continuous-learning-v2` | 基于直觉的本能学习系统 | - |
| `learn-eval` | 从会话提取可复用模式 | - |
| `skill-create` | 从 git 历史生成技能 | `/skill-create` |
| `instinct-status` | 显示学习的直觉（项目+全局） | `/instinct-status` |
| `instinct-import` | 导入直觉 | `/instinct-import` |
| `instinct-export` | 导出直觉 | `/instinct-export` |
| `evolve` | 将直觉聚类为技能 | `/evolve` |
| `promote` | 项目级直觉提升为全局 | `/promote` |
| `projects` | 查看项目与直觉统计 | `/projects` |
| `skill-stocktake` | 审计 Claude 技能和命令 | - |
| `coding-standards` | 通用编码标准和最佳实践 | - |
| `project-guidelines-example` | 项目特定技能模板示例 | - |
| `configure-ecc` | ECC 交互式安装程序 | - |

#### 📊 内容与文档（8个）

| Skill | 用途 | 命令 |
|-------|------|------|
| `article-writing` | 文章、博客、教程写作 | - |
| `content-engine` | 平台原生内容系统 | - |
| `frontend-slides` | HTML 演示文稿（动画丰富） | - |
| `investor-materials` | 融资材料（pitch、one-pager） | - |
| `investor-outreach` | 投资人联系邮件 | - |
| `market-research` | 市场研究、竞争分析 | - |
| `visa-doc-translate` | 签证文档翻译（图片转文本） | - |
| `nutrient-document-processing` | 文档处理（OCR、提取、签署等） | - |

#### 🔒 安全与性能（11个）

| Skill | 用途 | 命令 |
|-------|------|------|
| `security-review` | 安全漏洞检测（OWASP Top 10） | `/security-review` |
| `security-scan` | Claude Code 配置安全扫描 | - |
| `security-guidance` | 安全最佳实践指导 | - |
| `django-security` | Django 安全最佳实践 | - |
| `springboot-security` | Spring Security 最佳实践 | - |
| `strategic-compact` | 上下文压缩建议 | - |
| `cost-aware-llm-pipeline` | LLM 成本优化模式 | - |
| `iterative-retrieval` | 子代理上下文细化 | - |
| `regex-vs-llm-structured-text` | 正则 vs LLM 结构化文本决策 | - |
| `plankton-code-quality` | 写时代码质量强制 | - |
| `verification-loop` | Claude Code 验证系统 | - |

#### 📈 数据处理与分析（3个）

| Skill | 用途 | 命令 |
|-------|------|------|
| `postgres-patterns` | PostgreSQL 查询优化（已列在数据库） | - |
| `nutrient-document-processing` | 文档处理（OCR、转换、提取等） | - |
| `market-research` | 市场研究和竞争分析 | - |

---

### 📊 技能统计

| 类别 | 技能数量 |
|------|---------|
| 核心开发工作流 | 17 |
| 规划与设计 | 12 |
| 语言特定技能 | 18 |
| 数据库技能 | 4 |
| 持续学习与技能管理 | 14 |
| 内容与文档 | 8 |
| 安全与性能 | 11 |
| 其他工具与框架 | 10 |
| 数据处理与分析 | 3 |
| **总计** | **97** |

> 💡 **提示**：实际可使用的技能超过 100 个（部分技能在多个类别中重复）。这些技能可以通过命名空间形式使用：`/everything-claude-code:<skill-name>`，也可以使用简短形式（如果已配置全局别名）。

---

## 🎯 Superpowers Skills 快速参考

| 场景 | Skill | 命令 |
|------|-------|------|
| **发现重复代码** | `finding-duplicate-functions` | `/finding-duplicate-functions` |
| **简化优化代码** | `simplify` | `/simplify` |
| **规划新功能** | `brainstorming` | `/brainstorming` |
| **修复 Bug** | `systematic-debugging` | `/systematic-debugging` |
| **TDD 开发** | `test-driven-development` | `/test-driven-development` |
| **完成前验证** | `verification-before-completion` | `/verification-before-completion` |
| **编写实现计划** | `writing-plans` | `/writing-plans` |
| **执行实现计划** | `executing-plans` | `/executing-plans` |
| **代码审查** | `code-review` | `/code-review` |
| **处理审查反馈** | `receiving-code-review` | `/receiving-code-review` |
| **Git worktree 隔离** | `using-git-worktrees` | `/using-git-worktrees` |
| **完成开发分支** | `finishing-a-development-branch` | `/finishing-a-development-branch` |
| **并行任务分发** | `dispatching-parallel-agents` | `/dispatching-parallel-agents` |

---

## ✅ 检查清单

### Windows

- [ ] 配置 [~/.claude/settings.json](file:///C:/Users/15186/.claude/settings.json)
- [ ] 配置 [~/.claude.json](file:///C:/Users/15186/.claude.json)
- [ ] 设置环境变量
- [ ] 安装必要的插件
- [ ] 安装 ECC rules
- [ ] 启动测试：`claude --dangerously-skip-permissions`

### Linux/macOS

- [ ] 配置 [~/.claude/settings.json](file://~/.claude/settings.json)
- [ ] 配置 [~/.claude.json](file://~/.claude.json)
- [ ] 设置环境变量
- [ ] 安装必要的插件
- [ ] 安装 ECC rules
- [ ] 启动测试：`claude --dangerously-skip-permissions`



source ~/.bashrc  # 或 source ~/.zshrc

codex：
sk-BDUdFnk3BalncWCa0WxQ2zTCmagdfbmY03kHYzMKxS5rJXTR
https://ucn9uf8devd7.feishu.cn/wiki/XUrvw5RbCihuh4kEPrdcvFHNnhd?from=from_copylink
https://www.yuque.com/gtasmhe/rqevgd/plygnrly4cury7nv?singleDoc#%20
https://code.pumpkinai.vip 
这里可以查询使用情况哦~


vim ~/.codex/auth.json

{
"OPENAI_API_KEY": "sk-BDUdFnk3BalncWCa0WxQ2zTCmagdfbmY03kHYzMKxS5rJXTR"
}


vim ~/.codex/config.toml

model_provider = "codex"
model = "gpt-5.3-codex"        # 可更改为model = "gpt-5.4"
model_reasoning_effort = "low" #  "high" "medium"
disable_response_storage = true

[model_providers.codex]
name = "codex"
base_url = "https://code.ppchat.vip/v1"
wire_api = "responses"
requires_openai_auth = true











