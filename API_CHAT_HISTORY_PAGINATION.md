# Chat History Pagination API

## Overview

聊天记录API使用基于游标的分页方式，通过消息ID作为分页标记。这种方式可以避免在分页过程中新消息插入导致的数据混乱。

## Endpoint

**GET** `/api/chat/history`

## Query Parameters

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `user_id` | string | 否 | "api_user" | 用户ID |
| `agent_id` | number | 否 | 1 | Agent ID |
| `limit` | number | 否 | 20 | 每页返回的消息数量（最大100） |
| `before_id` | number | 否 | 0 | 游标：获取此消息ID之前的消息，0表示从最新消息开始 |

## Response Structure

```json
{
  "messages": [
    {
      "id": 123,
      "user_id": "user123",
      "agent_id": 1,
      "role": "user",
      "content": "Hello",
      "session_id": "session_001",
      "created_at": "2024-01-01T12:00:00Z"
    },
    {
      "id": 122,
      "user_id": "user123",
      "agent_id": 1,
      "role": "assistant",
      "content": "Hi there!",
      "session_id": "session_001",
      "created_at": "2024-01-01T11:59:00Z"
    }
  ],
  "pagination": {
    "total": 500,
    "limit": 20,
    "has_more": true,
    "next_cursor": 102
  }
}
```

### Response Fields

#### messages (array)
聊天消息数组，按ID倒序排列（最新的消息在前）

- `id`: 消息ID
- `user_id`: 用户ID
- `agent_id`: Agent ID
- `role`: 角色（"user" 或 "assistant"）
- `content`: 消息内容
- `session_id`: 会话ID
- `created_at`: 创建时间

#### pagination (object)
分页元数据

- `total`: 总消息数
- `limit`: 当前页的限制数量
- `has_more`: 是否还有更多数据
- `next_cursor`: 下一页的游标（最后一条消息的ID），如果为null表示没有更多数据

## Usage Examples

### 1. 获取最新的20条消息（第一页）

```bash
curl "http://localhost:8080/api/chat/history?user_id=user123&agent_id=1&limit=20"
```

响应：
```json
{
  "messages": [
    {"id": 120, "role": "user", "content": "最新消息", ...},
    {"id": 119, "role": "assistant", "content": "...", ...},
    ...
    {"id": 101, "role": "user", "content": "...", ...}
  ],
  "pagination": {
    "total": 500,
    "limit": 20,
    "has_more": true,
    "next_cursor": 101
  }
}
```

### 2. 获取下一页（使用上一页的 next_cursor）

```bash
curl "http://localhost:8080/api/chat/history?user_id=user123&agent_id=1&limit=20&before_id=101"
```

响应：
```json
{
  "messages": [
    {"id": 100, "role": "assistant", "content": "...", ...},
    {"id": 99, "role": "user", "content": "...", ...},
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

### 3. 最后一页（没有更多数据）

```bash
curl "http://localhost:8080/api/chat/history?user_id=user123&agent_id=1&limit=20&before_id=15"
```

响应：
```json
{
  "messages": [
    {"id": 14, "role": "assistant", "content": "...", ...},
    {"id": 13, "role": "user", "content": "...", ...},
    ...
    {"id": 1, "role": "user", "content": "最早的消息", ...}
  ],
  "pagination": {
    "total": 500,
    "limit": 20,
    "has_more": false,
    "next_cursor": null
  }
}
```

## JavaScript Example

```javascript
class ChatHistoryPaginator {
  constructor(baseUrl, userId, agentId, limit = 20) {
    this.baseUrl = baseUrl;
    this.userId = userId;
    this.agentId = agentId;
    this.limit = limit;
    this.nextCursor = null;
  }

  async fetchPage(beforeId = null) {
    const params = new URLSearchParams({
      user_id: this.userId,
      agent_id: this.agentId,
      limit: this.limit
    });

    if (beforeId) {
      params.append('before_id', beforeId);
    }

    const response = await fetch(`${this.baseUrl}/api/chat/history?${params}`);
    const data = await response.json();

    this.nextCursor = data.pagination.next_cursor;
    return data;
  }

  async fetchFirstPage() {
    return this.fetchPage();
  }

  async fetchNextPage() {
    if (!this.nextCursor) {
      return null; // 没有更多数据
    }
    return this.fetchPage(this.nextCursor);
  }

  hasMore() {
    return this.nextCursor !== null;
  }
}

// 使用示例
const paginator = new ChatHistoryPaginator('http://localhost:8080', 'user123', 1, 20);

// 获取第一页
const firstPage = await paginator.fetchFirstPage();
console.log('第一页:', firstPage.messages);
console.log('总数:', firstPage.pagination.total);

// 获取下一页
if (paginator.hasMore()) {
  const secondPage = await paginator.fetchNextPage();
  console.log('第二页:', secondPage.messages);
}
```

## Python Example

```python
import requests
from typing import Optional, Dict, List

class ChatHistoryPaginator:
    def __init__(self, base_url: str, user_id: str, agent_id: int, limit: int = 20):
        self.base_url = base_url
        self.user_id = user_id
        self.agent_id = agent_id
        self.limit = limit
        self.next_cursor: Optional[int] = None

    def fetch_page(self, before_id: Optional[int] = None) -> Dict:
        params = {
            'user_id': self.user_id,
            'agent_id': self.agent_id,
            'limit': self.limit
        }

        if before_id:
            params['before_id'] = before_id

        response = requests.get(f'{self.base_url}/api/chat/history', params=params)
        response.raise_for_status()
        data = response.json()

        self.next_cursor = data['pagination']['next_cursor']
        return data

    def fetch_first_page(self) -> Dict:
        return self.fetch_page()

    def fetch_next_page(self) -> Optional[Dict]:
        if not self.next_cursor:
            return None
        return self.fetch_page(self.next_cursor)

    def has_more(self) -> bool:
        return self.next_cursor is not None

# 使用示例
paginator = ChatHistoryPaginator('http://localhost:8080', 'user123', 1, 20)

# 获取第一页
first_page = paginator.fetch_first_page()
print(f"第一页: {len(first_page['messages'])} 条消息")
print(f"总数: {first_page['pagination']['total']}")

# 获取所有消息
all_messages = []
all_messages.extend(first_page['messages'])

while paginator.has_more():
    next_page = paginator.fetch_next_page()
    all_messages.extend(next_page['messages'])
    print(f"已加载 {len(all_messages)} 条消息")

print(f"总共加载了 {len(all_messages)} 条消息")
```

## 优势

### 1. 避免数据重复或遗漏
使用消息ID作为游标，即使在分页过程中有新消息插入，也不会导致数据重复或遗漏。

**传统offset分页的问题：**
```
第1页: offset=0, limit=20  → 获取消息 ID 100-81
[新消息插入: ID 101, 102, 103]
第2页: offset=20, limit=20 → 获取消息 ID 83-64 (遗漏了 ID 80-78)
```

**游标分页的优势：**
```
第1页: before_id=0       → 获取消息 ID 100-81, next_cursor=81
[新消息插入: ID 101, 102, 103]
第2页: before_id=81      → 获取消息 ID 80-61 (正确，没有遗漏)
```

### 2. 性能更好
基于索引的ID查询比offset查询更高效，特别是在大数据集上。

### 3. 倒序排列
消息按ID倒序返回（最新的在前），符合聊天应用的常见需求。

## Notes

- 消息按ID倒序排列（最新的消息在前）
- `before_id=0` 或不传 `before_id` 表示从最新消息开始
- `limit` 最大值为100，默认为20
- `next_cursor` 为null时表示没有更多数据
- 总数（`total`）会实时更新，反映当前的消息总数
