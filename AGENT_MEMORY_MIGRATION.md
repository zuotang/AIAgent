# Agent 维度记忆隔离 - 迁移指南

## 已完成的修改

### 1. 数据模型修改 ✅
- `Memory` 表：添加 `AgentID uint` 字段
- `ChatMessage` 表：添加 `AgentID uint` 字段
- `CompressedContext` 表：添加 `AgentID uint` 字段

### 2. CompressedContext 方法修改 ✅
- `GetCompressedContext(ctx, userID, agentID)` - 添加 agentID 参数
- `UpsertCompressedContext(ctx, cc)` - 使用 cc.AgentID
- `ClearCompressedContext(ctx, userID, agentID)` - 添加 agentID 参数

### 3. SQLite 方法修改（部分完成）
- ✅ `SaveChatMessage(ctx, userID, role, content, sessionID, agentID)`
- ✅ `GetChatHistory(ctx, userID, agentID, limit, offset)`
- ⏳ `GetChatHistoryAfterID` - 需要添加 agentID
- ⏳ `GetChatSessions` - 需要添加 agentID
- ⏳ `UpsertExtractedMemories` - 需要添加 agentID
- ⏳ `RenderStructuredMemory` - 需要添加 agentID
- ⏳ `GetTopAccessedMemories` - 需要添加 agentID
- ⏳ `IncrementAccessCount` - 需要添加 agentID
- ⏳ `GetMemoriesByType` - 需要添加 agentID
- ⏳ `DeleteMemory` - 需要添加 agentID
- ⏳ `GetMemoryCount` - 需要添加 agentID
- ⏳ `ExportMemories` - 需要添加 agentID

## 待修改的部分

### 4. Qdrant 向量存储
需要在 payload 中添加 `agent_id` 字段：
```go
payload := map[string]any{
    "user_id":   userID,
    "agent_id":  agentID,  // 新增
    "content":   t,
    "timestamp": time.Now().Unix(),
}
```

查询时需要同时过滤：
```go
"filter": map[string]any{
    "must": []any{
        map[string]any{
            "key":   "user_id",
            "match": map[string]any{"value": userID},
        },
        map[string]any{
            "key":   "agent_id",
            "match": map[string]any{"value": agentID},
        },
    },
}
```

### 5. Orchestrator 修改
所有调用记忆相关方法的地方都需要传入 agentID

### 6. API 修改
API 请求中已经有 `agent_id` 字段，需要将其传递到所有记忆操作中

## 数据库迁移

运行程序时，GORM 会自动添加新字段：
- `agent_id INTEGER NOT NULL DEFAULT 1`

旧数据会自动使用默认值 1（默认 agent）

## 测试计划

1. 创建两个不同的 agent
2. 用同一个 user_id 分别和两个 agent 聊天
3. 验证记忆是否正确隔离
4. 验证向量搜索是否只返回对应 agent 的记忆
