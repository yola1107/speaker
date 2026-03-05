---
name: code-refactor
description: Go 代码重构优化。用于优化代码结构、提高可读性、减少重复代码。
---

## 使用场景

- 代码重复较多
- 函数过长（> 50 行）
- 嵌套过深（> 3 层）
- 命名不清晰
- 缺少错误处理

---

## 重构检查清单

### 1. 重复代码

**问题**：相同逻辑出现在多处

**解决**：提取公共函数

```go
// 重构前
func ProcessOrder(o *Order) { /* 100行 */ }
func ProcessRefund(o *Order) { /* 95行重复 */ }

// 重构后
func processCommon(o *Order) { /* 公共逻辑 */ }
func ProcessOrder(o *Order) { processCommon(o); /* 订单特有 */ }
func ProcessRefund(o *Order) { processCommon(o); /* 退款特有 */ }
```

### 2. 长函数

**问题**：函数超过 50 行

**解决**：按职责拆分

```go
// 重构前
func HandleRequest(req *Request) (*Response, error) {
    // 验证 20 行
    // 处理 30 行
    // 响应 20 行
}

// 重构后
func HandleRequest(req *Request) (*Response, error) {
    if err := validateRequest(req); err != nil {
        return nil, err
    }
    result := processRequest(req)
    return buildResponse(result), nil
}
```

### 3. 嵌套过深

**问题**：if 嵌套超过 3 层

**解决**：提前返回

```go
// 重构前
func Process(u *User) error {
    if u != nil {
        if u.Active {
            if u.Role == "admin" {
                // 处理
            }
        }
    }
}

// 重构后
func Process(u *User) error {
    if u == nil || !u.Active || u.Role != "admin" {
        return ErrInvalidUser
    }
    // 处理
}
```

### 4. 魔法数字

**问题**：硬编码的数字

**解决**：定义常量

```go
// 重构前
if age > 18 { }

// 重构后
const AdultAge = 18
if age > AdultAge { }
```

---

## 重构流程

1. **识别问题** - 扫描代码发现坏味道
2. **编写测试** - 确保重构不破坏功能
3. **小步重构** - 每次只改一处
4. **运行测试** - 验证重构正确
5. **提交代码** - 独立提交

---

## 检查清单

- [ ] 无重复代码
- [ ] 函数 < 50 行
- [ ] 嵌套 < 3 层
- [ ] 命名清晰
- [ ] 有单元测试
- [ ] 测试通过