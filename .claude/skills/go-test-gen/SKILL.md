---
name: go-test-gen
description: 自动生成 Go 单元测试。用于为现有函数生成测试用例，提高代码覆盖率。
---

## 使用场景

当需要为 Go 函数生成单元测试时使用此 Skill。

---

## 执行步骤

### 1. 识别目标函数

找到需要测试的函数文件。

### 2. 分析函数逻辑

- 输入参数类型
- 返回值类型
- 边界条件
- 错误处理

### 3. 生成测试用例

创建 `xxx_test.go` 文件：

```go
package xxx_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {
            name:    "正常情况",
            input:   InputType{Field: "value"},
            want:    OutputType{Result: "expected"},
            wantErr: false,
        },
        {
            name:    "边界情况-空值",
            input:   InputType{},
            want:    OutputType{},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FunctionName(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.want, got)
            }
        })
    }
}
```

### 4. 运行测试

```bash
go test ./internal/xxx/... -v -cover
```

---

## 测试用例模板

### 正常流程测试
- 正常输入返回预期结果

### 边界条件测试
- 空值输入
- 最大值/最小值
- 特殊字符

### 错误处理测试
- 无效输入返回错误
- 依赖服务不可用

---

## 检查清单

- [ ] 测试文件创建成功
- [ ] 测试用例覆盖正常流程
- [ ] 测试用例覆盖边界条件
- [ ] 测试用例覆盖错误处理
- [ ] 运行测试通过
- [ ] 覆盖率 > 80%