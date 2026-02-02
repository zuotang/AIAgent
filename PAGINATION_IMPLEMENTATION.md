# 聊天记录分页功能实现总结

## 概述

实现了基于游标（Cursor-based）的聊天记录分页功能，使用消息ID作为分页标记，避免在分页过程中新消息插入导致的数据混乱。

## 实现的功能

### 1. 数据库层 (internal/memory/sqlite.go)

新增方法：

- **GetChatHistoryWithCursor**: 基于游标的分页查询
  - 参数：`beforeID` (获取此ID之前的消息，0表示从最新开始)
  - 返回：按ID倒序排列的消息列表

- **GetChatHistoryCount**: 获取聊天记录总数
  - 用于返回分页元数据中的总数

### 2. 编排器层 (internal/orchestrator/orchestrator.go)

更新了 `Orchestrator` 接口，新增方法：

- `GetChatHistoryWithCursor(ctx, userID, agentID, beforeID, limit)`
- `GetChatHistoryCount(ctx, userID, agentID)`

### 3. API层 (internal/api/chat.go)

更新了 `GetChatHistory` 端点：

**新增数据结构：**

```go
// 分页元数据
type PaginationMeta struct {
    Total      int64  `json:"total"`       // 总记录数
    Limit      int    `json:"limit"`       // 每页数量
    HasMore    bool   `json:"has_more"`    // 是否有更多数据
    NextCursor *uint  `json:"next_cursor"` // 下一页游标
}

// 聊天记录响应
type ChatHistoryResponse struct {
    Messages   []memory.ChatMessage `json:"messages"`
    Pagination PaginationMeta       `json:"pagination"`
}
```

**查询参数：**

- `user_id`: 用户ID（默认："api_user"）
- `agent_id`: Agent ID（默认：1）
- `limit`: 每页数量（默认：20，最大：100）
- `before_id`: 游标，获取此ID之前的消息（默认：0，表示从最新开始）

## API使用示例

### 获取第一页（最新的20条消息）

```bash
GET /api/chat/history?user_id=user123&agent_id=1&limit=20
```

响应：
```json
{
  "messages": [
    {"id": 100, "role": "user", "content": "最新消息", ...},
    {"id": 99, "role": "assistant", "content": "...", ...},
    ...
    {"id": 81, "role": "user", "content": "...", ...}
  ],
  "pagination": {
    "total": 500,
    "limit": 20,
    "has_more": true,
    "next_cursor": 81
  }
}
```

### 获取下一页

```bash
GET /api/chat/history?user_id=user123&agent_id=1&limit=20&before_id=81
```

响应：
```json
{
  "messages": [
    {"id": 80, "role": "assistant", "content": "...", ...},
    ...
    {"id": 61, "role": "user", "content": "...", ...}
  ],
  "pagination": {
    "total": 500,
    "limit": 20,
    "has_more": true,
    "next_cursor": 61
  }
}
```

### 最后一页

```json
{
  "messages": [...],
  "pagination": {
    "total": 500,
    "limit": 20,
    "has_more": false,
    "next_cursor": null
  }
}
```

## 技术优势

### 1. 避免数据重复或遗漏

**传统offset分页的问题：**
```
时刻T1: 获取 offset=0, limit=20  → 消息 ID 100-81
[新消息插入: ID 101, 102, 103]
时刻T2: 获取 offset=20, limit=20 → 消息 ID 83-64 (遗漏了 80-78)
```

**游标分页的优势：**
```
时刻T1: before_id=0       → 消息 ID 100-81, next_cursor=81
[新消息插入: ID 101, 102, 103]
时刻T2: before_id=81      → 消息 ID 80-61 (正确，无遗漏)
```

### 2. 性能更好

- 基于索引的ID查询比offset查询更高效
- 特别是在大数据集上，offset越大性能越差
- 游标分页性能稳定，不受数据量影响

### 3. 符合聊天应用需求

- 消息按ID倒序返回（最新的在前）
- 用户通常从最新消息开始浏览
- 向上滚动加载历史消息

## 测试

创建了单元测试 `internal/api/chat_pagination_test.go`：

- 测试第一页获取
- 测试后续页获取
- 测试分页元数据正确性
- 测试 `has_more` 和 `next_cursor` 逻辑

所有测试通过 ✅

## 文档

创建了详细的API文档：

- `API_CHAT_HISTORY_PAGINATION.md`: 完整的API使用指南
  - 查询参数说明
  - 响应结构说明
  - JavaScript/Python示例代码
  - 优势分析

## 向后兼容性

- 保留了原有的 `GetChatHistory` 方法（基于offset）
- 新的游标分页通过 `before_id` 参数启用
- 不传 `before_id` 时默认从最新消息开始

## 构建状态

✅ 编译成功
✅ 测试通过
✅ 文档完整

## 使用建议

1. **前端实现无限滚动**：使用 `next_cursor` 实现向上滚动加载更多
2. **缓存策略**：可以在前端缓存已加载的消息，避免重复请求
3. **错误处理**：检查 `has_more` 避免不必要的请求
4. **性能优化**：根据实际需求调整 `limit` 大小（建议20-50）

## 相关文件

- `internal/memory/sqlite.go`: 数据库查询方法
- `internal/orchestrator/orchestrator.go`: 编排器接口和实现
- `internal/api/chat.go`: API处理器
- `internal/api/chat_pagination_test.go`: 单元测试
- `API_CHAT_HISTORY_PAGINATION.md`: API文档
