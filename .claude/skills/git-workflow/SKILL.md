---
name: git-workflow
description: Git 工作流标准化。用于规范分支管理、提交信息、PR 流程。
---

## 分支策略

### 分支命名

| 类型 | 命名格式 | 示例 |
|------|----------|------|
| 功能 | `feature/xxx` | `feature/user-auth` |
| 修复 | `fix/xxx` | `fix/login-bug` |
| 重构 | `refactor/xxx` | `refactor/order-service` |
| 发布 | `release/vX.X.X` | `release/v1.2.0` |

### 分支流程

```
main (生产)
  └── feature/xxx (开发)
        └── commit → commit → PR → merge
```

---

## 提交规范

### 格式

```
<type>: <subject>

<body>

Co-Authored-By: Claude <noreply@anthropic.com>
```

### Type 类型

| Type | 说明 | 示例 |
|------|------|------|
| `feat` | 新功能 | `feat: 添加用户登录` |
| `fix` | Bug 修复 | `fix: 修复金额计算错误` |
| `refactor` | 重构 | `refactor: 重构订单服务` |
| `docs` | 文档 | `docs: 更新 README` |
| `test` | 测试 | `test: 添加单元测试` |
| `chore` | 构建/工具 | `chore: 更新依赖` |

### 示例

```
feat: 添加用户认证功能

- 实现 JWT 验证
- 添加登录接口
- 添加权限中间件

Co-Authored-By: Claude <noreply@anthropic.com>
```

---

## 工作流程

### 1. 开始新功能

```bash
# 从 main 创建分支
git checkout main
git pull origin main
git checkout -b feature/xxx

# 开发...
```

### 2. 提交代码

```bash
# 查看变更
git status
git diff

# 暂存文件
git add <files>

# 提交
git commit -m "feat: xxx"
```

### 3. 推送分支

```bash
git push -u origin feature/xxx
```

### 4. 创建 PR

```bash
gh pr create --title "feat: xxx" --body "## 变更内容\n- xxx"
```

### 5. 合并

```bash
# PR 通过后
gh pr merge --squash
git checkout main
git pull origin main
```

---

## 常用命令

```bash
# 查看 PR 列表
gh pr list

# 查看 PR 详情
gh pr view <number>

# 合并 PR
gh pr merge <number> --squash

# 查看分支
git branch -a

# 删除已合并分支
git branch -d feature/xxx
```

---

## 检查清单

- [ ] 分支命名正确
- [ ] 提交信息符合规范
- [ ] 代码已自测
- [ ] 创建 PR 描述清晰
- [ ] Code Review 通过