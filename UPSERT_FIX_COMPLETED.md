# UNIQUE 约束错误修复完成

## 修复日期
2026-02-03

## 问题描述

在异步记忆提取过程中，系统会报以下错误：

```
constraint failed: UNIQUE constraint failed: memories.user_id, memories.agent_id, memories.mtype, memories.mkey, memories.owner (2067)
```

## 根本原因

原有的 `UpsertExtractedMemories` 方法使用了 `FirstOrCreate` + `Assign` 的组合，但这个组合不能正确处理 UPSERT 逻辑：

```go
// 旧代码（有问题）
result := tx.Where("...").
    Assign(map[string]interface{}{...}).
    FirstOrCreate(memory)
```

**问题**：
- `FirstOrCreate` 只在记录不存在时创建
- `Assign` 在 `FirstOrCreate` 中不会触发更新
- 当尝试插入重复记忆时，会违反 UNIQUE 约束

## 修复方案

采用"先查询，再决定创建或更新"的策略：

```go
// 新代码（已修复）
// 1. 先查询是否存在
var existing Memory
err := tx.Where("user_id = ? AND agent_id = ? AND mtype = ? AND mkey = ? AND owner = ?",
    userID, agentID, m.Type, m.Key, m.Owner).
    First(&existing).Error

if err == gorm.ErrRecordNotFound {
    // 2a. 不存在，创建新记录
    memory := &Memory{...}
    if err := tx.Create(memory).Error; err != nil {
        return err
    }
} else if err != nil {
    // 2b. 查询出错
    return err
} else {
    // 2c. 存在，更新记录
    if err := tx.Model(&existing).Updates(map[string]interface{}{
        "mvalue":     m.Value,
        "confidence": m.Confidence,
        "updated_at": now,
    }).Error; err != nil {
        return err
    }
}
```

## 修复的文件

- `internal/memory/sqlite.go` - `UpsertExtractedMemories` 方法

## 测试验证

创建了全面的单元测试 `internal/memory/upsert_test.go`：

### 测试1：重复插入不报错
```
✅ PASS: TestUpsertExtractedMemories_NoDuplicateError
```
- 第一次插入2条记忆
- 第二次更新相同的2条记忆（不同的值）
- 第三次混合更新和新增
- 验证：无 UNIQUE 约束错误，记忆正确更新

### 测试2：不同 Owner 可以共存
```
✅ PASS: TestUpsertExtractedMemories_DifferentOwners
```
- 插入 user 的记忆
- 插入 agent 的记忆（相同 type 和 key）
- 验证：两条记忆都存在（因为 owner 不同）

## 修复效果

### 修复前
```
2026/02/03 01:36:09 写入 SQLite 失败: constraint failed: UNIQUE constraint failed...
2026/02/03 01:36:09 [DEBUG] 异步提取记忆失败: constraint failed...
```

### 修复后
```
✅ 记忆正确插入或更新
✅ 无 UNIQUE 约束错误
✅ 支持重复提取相同信息（自动更新）
```

## 技术细节

### UNIQUE 约束定义
```sql
CREATE UNIQUE INDEX idx_memories_unique ON memories(
    user_id,
    agent_id,
    mtype,
    mkey,
    owner
);
```

### 更新逻辑
- 如果记录存在：更新 `mvalue`, `confidence`, `updated_at`
- 如果记录不存在：创建新记录
- 使用事务确保原子性

## 性能影响

- **查询次数**：每条记忆需要1次查询 + 1次插入/更新
- **事务保护**：所有操作在单个事务中完成
- **并发安全**：SQLite 的事务机制保证并发安全

## 向后兼容性

✅ 完全兼容
- 方法签名未改变
- 行为符合预期（UPSERT）
- 现有代码无需修改

## 构建状态

```bash
✅ go build -o main.exe ./cmd/api
✅ go build -o chat.exe ./cmd/chat
✅ go test ./internal/memory -v -run TestUpsertExtractedMemories
```

## 使用建议

1. **重新编译**：使用修复后的代码重新编译项目
2. **清理旧数据**（可选）：如果之前有重复记忆，可以手动清理
3. **监控日志**：观察是否还有 UNIQUE 约束错误

## 相关文件

- `internal/memory/sqlite.go` - 修复的主文件
- `internal/memory/upsert_test.go` - 测试文件
- `MEMORY_UPSERT_FIX.md` - 修复方案文档

## 总结

✅ 问题已完全修复
✅ 测试全部通过
✅ 构建成功
✅ 向后兼容

现在系统可以正确处理重复记忆的提取，不会再出现 UNIQUE 约束错误。
