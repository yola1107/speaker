# CLAUDE.md

本项目指南用于指导 Claude Code 协助开发。

## 项目简介

**Kratos v2 微服务**，用于游戏服务器平台。使用定制版 Kratos 框架 (`github.com/yola1107/kratos/v2`)，支持多协议实时通信。

## 常用命令

```bash
make init    # 安装工具
make all     # 生成所有代码（api + config + wire）
make build   # 构建
./bin/server -conf ./configs  # 运行
go test ./tmp/xxg2/... -v     # 测试游戏模块
```

## 项目结构

```
cmd/server/        # 入口（main.go, wire.go, wire_gen.go）
api/speaker/v1/   # Proto 定义及生成代码
internal/
  ├── service/    # 服务层（实现 gRPC/HTTP 端点）
  ├── biz/        # 业务逻辑层
  ├── data/       # 数据访问层
  ├── server/     # 服务器配置（http, grpc, tcp, ws, gnet）
  └── conf/       # 配置结构
configs/          # 运行时配置（config.yaml）
tmp/              # 游戏模块（xxg2, hbtr2, mahjong4, sgz, xslm2, jqs, champ）
test/             # 测试客户端
```

## 架构

**整洁架构** + **Google Wire** 依赖注入：

```
cmd/server → internal/server → internal/service → internal/biz → internal/data
```

每层在主文件定义 `ProviderSet`，Wire 生成 `wire_gen.go` 自动组装依赖。

## 传输协议

| 协议 | 端口 | 说明 |
|-----|------|-----|
| HTTP | 8000 | REST API |
| gRPC | 9000 | gRPC 服务 |
| TCP | 3101 | 自定义 TCP 协议 |
| WebSocket | 3102 | 浏览器客户端 |
| Gnet | 3103 | 高性能 TCP |

## 技术栈

- **Go** 1.24.5
- **Kratos** v2（定制版）
- **Google Wire** 依赖注入
- **XORM** MySQL ORM
- **go-redis v9** 缓存
- **RabbitMQ** 消息队列
- **gnet** 高性能网络
- **Protocol Buffers** API 定义

## 编码规范

### 命名约定

| 类型 | 规范 | 示例 |
|------|------|------|
| 导出结构体/接口 | PascalCase | `SpeakerUsecase`, `SpeakerRepo` |
| 私有结构体 | camelCase | `betOrderService` |
| 导出函数/方法 | PascalCase | `CreateSpeaker` |
| 私有函数/方法 | camelCase | `doBetOrder` |
| 常量 | camelCase | `ErrUserNotFound` |

### 注释规范

- 导出函数必须有注释：`// FunctionName does something`
- 接口需说明用途
- 复杂逻辑添加行内注释

### 错误处理

- 使用 Kratos `errors` 包定义项目级别错误
- 在 `internal/biz/` 层定义错误变量

```go
var (
    ErrUserNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)
```

### 依赖注入

- 构造函数命名：`NewXxx`
- 每层定义 `ProviderSet` 供 Wire 使用
- 依赖通过参数传入，避免全局变量

### 日志规范

- 使用 `log.Helper` 包装 logger
- 关键操作记录日志

```go
log.Infof("CreateSpeaker: %v", g.Hello)
```

## Git 工作流

### 分支策略

| 分支类型 | 命名 | 用途 |
|---------|------|------|
| 主分支 | `main` | 生产代码，保持稳定 |
| 功能分支 | `feature/xxx` | 新功能开发 |
| 修复分支 | `fix/xxx` | Bug 修复 |
| 重构分支 | `refactor/xxx` | 代码重构 |

### 提交信息格式

```
<type>: <subject>
```

类型：`feat` | `fix` | `refactor` | `docs` | `test` | `chore`

**示例：**
- `feat: 添加用户登录功能`
- `fix: 修复下注金额计算错误`
- `refactor: 重构订单服务`

### PR 流程

1. 从 `main` 创建功能分支
2. 完成开发并自测通过
3. 提交 PR，描述变更内容
4. Code Review 通过后合并

## 规则
1. 在编写任何代码前，先描述你的方法并等待批准
2. 如果我给出的需求模糊，请在编写代码前提出澄清问题
3. 完成任何代码编写后，列出边缘案例并建议覆盖它们的测试用例
4. 如果任务需要修改超过 3 个文件，先停止并将其拆分成更小的任务
5. 出现 bug 时，先编写能重现该 bug 的测试，再修复直到测试通过
6. 每次我纠正你时，反思你做错了什么，并制定永不再犯的计划
