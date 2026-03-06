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
    "ANTHROPIC_MODEL": "glm-5"
  },
  "model": "glm-5",
  "enabledPlugins": {
    "superpowers@superpowers-marketplace": true,
    "code-review@claude-plugins-official": true
  },
  "permissions": {
    "allow": [
      "*"
    ],
    "defaultMode": "bypassPermissions"
  },
  "language": "中文",
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

✨ **完成！** 你现在可以使用 13 个代理、43 个技能和 31 个命令。

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

- [ ] 配置 `~/.claude/settings.json`
- [ ] 配置 `~/.claude.json`
- [ ] 设置环境变量
- [ ] 安装必要的插件
- [ ] 安装 ECC rules
- [ ] 启动测试：`claude --dangerously-skip-permissions`
