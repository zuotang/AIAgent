# Agent 维度记忆隔离 - 实施进度

## ✅ 已完成的修改

### 1. 数据模型层 (100%)
- ✅ `Memory` 表：添加 `AgentID uint` 字段
- ✅ `ChatMessage` 表：添加 `AgentID uint` 字段
- ✅ `CompressedContext` 表：添加 `AgentID uint` 字段

### 2. 存储层 (100%)
#### SQLite (internal/memory/sqlite.go)
- ✅ `SaveChatMessage(ctx, userID, role, content, sessionID, agentID)`
- ✅ `GetChatHistory(ctx, userID, agentID, limit, offset)`
- ✅ `GetChatHistoryAfterID(ctx, userID, agentID, afterID, limit)`
- ✅ `GetChatSessions(ctx, userID, agentID)`
- ✅ `UpsertExtractedMemories(ctx, userID, agentID, memories)`
- ✅ `RenderStructuredMemory(ctx, userID, agentID, limit)`
- ✅ `GetTopAccessedMemories(ctx, userID, agentID, limit)`
- ✅ `IncrementAccessCount(ctx, userID, agentID, mtype, mkey, owner)`
- ✅ `GetMemoriesByType(ctx, userID, agentID, mtype)`
- ✅ `DeleteMemory(ctx, userID, agentID, mtype, mkey, owner)`
- ✅ `GetMemoryCount(ctx, userID, agentID)`
- ✅ `ExportMemories(ctx, userID, agentID)`

#### CompressedContext (internal/memory/compressed_context.go)
- ✅ `GetCompressedContext(ctx, userID, agentID)`
- ✅ `UpsertCompressedContext(ctx, cc)` - 使用 cc.AgentID
- ✅ `ClearCompressedContext(ctx, userID, agentID)`

#### Qdrant (internal/rag/qdrant.go)
- ✅ `SimilaritySearch(ctx, userID, agentID, query, topK)`
- ✅ `SimilaritySearchFromCollection(ctx, userID, agentID, query, topK, collection)`
- ✅ `UpsertTexts(ctx, userID, agentID, texts, fileName)`
- ✅ `ListFiles(ctx, userID, agentID)`

## ⏳ 待修改的部分

### 3. Orchestrator 层 (0%)
需要修改所有调用存储层的地方，添加 `agentID` 参数。

**文件**: `internal/orchestrator/orchestrator.go`

需要修改的方法：
1. `ProcessMessage` - 添加 agentID 参数
2. `ProcessMessageStream` - 添加 agentID 参数
3. `GetChatHistory` - 添加 agentID 参数
4. `GetChatHistoryAfterID` - 添加 agentID 参数
5. `GetChatSessions` - 添加 agentID 参数

需要修改的调用点（约 15+ 处）：
- `o.memStore.GetChatHistory(ctx, userID, limit, offset)` → 添加 agentID
- `o.memStore.GetChatHistoryAfterID(ctx, userID, afterID, limit)` → 添加 agentID
- `o.memStore.GetChatSessions(ctx, userID)` → 添加 agentID
- `o.memStore.GetCompressedContext(ctx, userID)` → 添加 agentID
- `o.memStore.SaveChatMessage(ctx, userID, role, content, sessionID)` → 添加 agentID
- `o.memStore.UpsertExtractedMemories(ctx, userID, memories)` → 添加 agentID
- `o.vectorStore.SimilaritySearch(ctx, userID, query, topK)` → 添加 agentID

**文件**: `internal/orchestrator/context_compressor.go`
- `store.GetCompressedContext(ctx, userID)` → 添加 agentID

**文件**: `internal/orchestrator/memory_extractor.go`
- 可能需要修改记忆提取逻辑

### 4. API 层 (0%)
**文件**: `internal/api/chat.go`

需要修改的方法：
1. `HandleChat` - 从请求中获取 agentID，传递给 orchestrator
2. `HandleChatStream` - 从请求中获取 agentID，传递给 orchestrator
3. `GetChatHistory` - 添加 agentID 参数
4. `GetChatSessions` - 添加 agentID 参数

当前 API 请求已经包含 `agent_id` 字段，只需要传递下去。

### 5. CMD 层 (0%)
**文件**: `cmd/chat/main.go`

CLI 工具可能需要添加 `-agent` 参数来指定使用哪个 agent。

## 🔧 实施建议

### 方案 A：手动修改（推荐）
逐个文件修改，确保每个调用点都正确添加 agentID 参数。

### 方案 B：使用脚本批量修改
创建一个 sed/awk 脚本来批量替换函数调用。

## 📝 下一步行动

1. **修改 Orchestrator 接口**：
   ```go
   type Orchestrator interface {
       ProcessMessage(ctx, userID, agentID, userText, conversationHistory, systemPrompt) (Output, error)
       ProcessMessageStream(ctx, userID, agentID, userText, conversationHistory, systemPrompt, callback) (Output, error)
       GetChatHistory(ctx, userID, agentID, limit, offset) ([]ChatMessage, error)
       GetChatHistoryAfterID(ctx, userID, agentID, afterID, limit) ([]ChatMessage, error)
       GetChatSessions(ctx, userID, agentID) ([]ChatSession, error)
   }
   ```

2. **修改 Orchestrator 实现**：
   - 在所有调用存储层的地方添加 agentID 参数

3. **修改 API 层**：
   - 从请求中提取 agentID
   - 传递给 orchestrator

4. **测试**：
   - 创建两个不同的 agent
   - 用同一个 user_id 分别和两个 agent 聊天
   - 验证记忆是否正确隔离

## 🎯 预期效果

完成后，系统将支持：
- ✅ 同一用户可以和多个 agent 聊天
- ✅ 每个 agent 有独立的记忆
- ✅ 对话历史按 agent 隔离
- ✅ 向量搜索只返回对应 agent 的记忆
- ✅ 压缩上下文按 agent 隔离

## 📊 数据库迁移

GORM 会自动添加新字段：
```sql
ALTER TABLE memories ADD COLUMN agent_id INTEGER NOT NULL DEFAULT 1;
ALTER TABLE chat_history ADD COLUMN agent_id INTEGER NOT NULL DEFAULT 1;
ALTER TABLE compressed_contexts ADD COLUMN agent_id INTEGER NOT NULL DEFAULT 1;
```

旧数据会自动使用默认值 1（默认 agent）。
