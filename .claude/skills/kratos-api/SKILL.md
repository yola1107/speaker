---
name: kratos-api
description: 创建 Kratos 微服务 API（Proto + Service + Biz + Data）
---

## 使用场景

当需要为 Kratos v2 微服务添加新的 API 端点时使用此 skill。

## 前置条件

- 确认 API 需求已明确
- 了解请求/响应数据结构

## 执行步骤

### 1. 定义 Proto

在 `api/speaker/v1/` 目录创建或修改 `.proto` 文件：

```protobuf
service Speaker {
  rpc NewAPI (NewAPIRequest) returns (NewAPIReply);
}

message NewAPIRequest {
  string field = 1;
}

message NewAPIReply {
  string message = 1;
}
```

### 2. 生成代码

```bash
make api
```

### 3. 实现 Service 层

在 `internal/service/speaker.go` 添加：

```go
func (s *SpeakerService) NewAPI(ctx context.Context, in *v1.NewAPIRequest) (*v1.NewAPIReply, error) {
    // 调用 biz 层
    return &v1.NewAPIReply{Message: "ok"}, nil
}
```

### 4. 实现 Biz 层

在 `internal/biz/speaker.go` 添加业务逻辑。

### 5. 实现 Data 层（如需数据库）

在 `internal/data/` 添加数据访问。

### 6. 更新依赖注入

```bash
make all
```

### 7. 测试

```bash
go test ./... -v
```

## 检查清单

- [ ] Proto 定义符合规范
- [ ] 代码生成成功
- [ ] Service 层实现
- [ ] Biz 层实现
- [ ] 测试通过