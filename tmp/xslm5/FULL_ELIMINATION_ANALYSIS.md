# xslm3 全屏消除算分逻辑分析

## 问题分析

### 1. `findWinInfos()` 的逻辑

```go
func (s *betOrderService) findNormalSymbolWinInfo(symbol int64) (*winInfo, bool) {
    exist := false  // 关键：要求必须有基础符号
    // ...
    if currSymbol == symbol {
        exist = true  // 只有找到基础符号，exist才为true
    }
    // ...
    if c >= _minMatchCount && exist {  // 要求exist=true
        return &info, true
    }
    if c == _colCount-1 && exist {  // 要求exist=true
        return &info, true
    }
    return nil, false
}
```

**关键点：**
- `findNormalSymbolWinInfo` **要求必须有基础符号**（`exist = true`）
- 如果某个way只有女性百搭（10-12）和百搭（13），没有基础符号（1-9），`exist` 会是 `false`
- 这个way**不会被找到**，不会加入 `winInfos`

### 2. `fillElimFreeFull()` 的逻辑

```go
func (s *betOrderService) fillElimFreeFull(grid *int64Grid) int {
    for _, w := range s.winInfos {  // 只遍历winInfos中的way
        if w == nil || !infoHasFemaleWild(w.WinGrid) {
            continue
        }
        // 消除这个way中除百搭13之外的所有符号
    }
}
```

**关键点：**
- `fillElimFreeFull` 只遍历 `winInfos` 中的way
- 如果某个way只有女性百搭，不在 `winInfos` 中，**不会被消除**

### 3. 游戏规则（第8条）

```
8.)在免费游戏中，3种女性符号全都可以转变为女性百搭后，
   有女性百搭符号参与的中奖符号（无论是不是女性符号）都会消失
```

**关键点：**
- 全屏情况下，**只要有女性百搭参与的way，都应该被消除**
- 无论是否有基础符号

## 问题场景

### 场景1：有基础符号的way
```
列1: 符号7 + 女性百搭10
列2: 符号7 + 百搭13
列3: 符号7
```
- ✅ `findNormalSymbolWinInfo` 能找到（有基础符号7）
- ✅ 会被算分
- ✅ 会被消除

### 场景2：只有女性百搭的way（问题场景）
```
列1: 女性百搭10 + 百搭13
列2: 女性百搭10
列3: 百搭13
```
- ❌ `findNormalSymbolWinInfo` **找不到**（没有基础符号，exist=false）
- ❌ **不会被算分**
- ❌ **不会被消除**（不在winInfos中）

**但根据游戏规则第8条，这个way应该被消除！**

## `findAllWinInfosForFullElimination` 的功能

### 功能点

1. **补充查找只有女性百搭的way**
   - 跳过已经找到的符号（`existingWinInfos[symbol]`）
   - 只查找那些没有被 `findNormalSymbolWinInfo` 找到的way
   - 这些way只有女性百搭，没有基础符号

2. **确保全屏消除时先算分**
   - 根据mahjong和zdtn2的逻辑，消除前应该先算分
   - 这些只有女性百搭的way也需要算分

3. **符合游戏规则**
   - 游戏规则第8条：有女性百搭参与的way都应该消失
   - 这些way应该被算分和消除

### 是否重复计算？

**答案：不是重复计算**

原因：
1. `findAllWinInfosForFullElimination` 会跳过已经找到的符号：
   ```go
   if existingWinInfos[symbol] {
       continue  // 跳过已经找到的符号
   }
   ```

2. 它只查找那些**只有女性百搭，没有基础符号**的way

3. 这些way在 `findNormalSymbolWinInfo` 中**不会被找到**（因为 `exist = false`）

## 修复前后对比

### 修复前
```
场景：只有女性百搭的way
├─ findWinInfos() → 找不到（exist=false）
├─ updateStepResults() → 不算分
├─ fillElimFreeFull() → 不消除（不在winInfos中）
└─ 结果：❌ 不符合游戏规则
```

### 修复后
```
场景：只有女性百搭的way
├─ findWinInfos() → 找不到（exist=false）
├─ findAllWinInfosForFullElimination() → 找到（补充查找）
├─ updateStepResults() → 算分 ✅
├─ fillElimFreeFull() → 消除 ✅
└─ 结果：✅ 符合游戏规则
```

## 结论

`findAllWinInfosForFullElimination` **不是重复计算**，而是：

1. **补充查找**：查找那些只有女性百搭，没有基础符号的way
2. **确保算分**：这些way在消除前需要先算分
3. **符合规则**：确保所有有女性百搭参与的way都被正确处理

这个函数是**必要的**，因为：
- `findNormalSymbolWinInfo` 要求必须有基础符号
- 但游戏规则要求，全屏情况下只要有女性百搭参与的way都应该被消除
- 所以需要补充查找这些只有女性百搭的way

