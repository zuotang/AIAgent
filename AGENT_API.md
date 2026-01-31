# Agent 和 Prompt API 文档

## 概述

本文档描述了新增的 Agent（智能助手）和 Prompt（提示词）管理 API。

## 功能特性

- **提示词管理**：创建、查询、更新、删除系统提示词
- **Agent 管理**：创建、查询、更新、删除不同角色的 Agent
- **动态切换**：聊天时可以通过 `agent_id` 参数切换不同的 Agent
- **关联查询**：Agent 自动关联对应的 Prompt

## 数据模型

### Prompt（提示词）

```json
{
  "id": 1,
  "name": "默认助手",
  "content": "你是一个友好、专业的 AI 助手...",
  "description": "通用 AI 助手提示词",
  "category": "assistant",
  "is_default": true,
  "created_at": "2026-01-31T10:00:00Z",
  "updated_at": "2026-01-31T10:00:00Z"
}
```

**字段说明**：
- `id`: 提示词 ID（自动生成）
- `name`: 提示词名称（唯一）
- `content`: 提示词内容
- `description`: 描述
- `category`: 分类（assistant, translator, coder 等）
- `is_default`: 是否为默认提示词
- `created_at`: 创建时间
- `updated_at`: 更新时间

### Agent（智能助手）

```json
{
  "id": 1,
  "name": "翻译助手",
  "description": "专业的翻译助手",
  "prompt_id": 2,
  "prompt": {
    "id": 2,
    "name": "翻译助手",
    "content": "你是一个专业的翻译助手...",
    "category": "translator"
  },
  "avatar": "https://example.com/avatar.png",
  "config": "{\"temperature\": 0.7, \"model\": \"gpt-4\"}",
  "is_active": true,
  "created_at": "2026-01-31T10:00:00Z",
  "updated_at": "2026-01-31T10:00:00Z"
}
```

**字段说明**：
- `id`: Agent ID（自动生成）
- `name`: Agent 名称（唯一）
- `description`: 描述
- `prompt_id`: 关联的提示词 ID
- `prompt`: 关联的提示词对象（查询时自动加载）
- `avatar`: 头像 URL
- `config`: JSON 配置（temperature, model 等）
- `is_active`: 是否激活
- `created_at`: 创建时间
- `updated_at`: 更新时间

## API 端点

### 提示词 API

#### 1. 创建提示词

```http
POST /api/prompts
Content-Type: application/json

{
  "name": "代码助手",
  "content": "你是一个专业的编程助手...",
  "description": "编程辅助提示词",
  "category": "coder",
  "is_default": false
}
```

**响应**：
```json
{
  "id": 3,
  "name": "代码助手",
  "content": "你是一个专业的编程助手...",
  "description": "编程辅助提示词",
  "category": "coder",
  "is_default": false,
  "created_at": "2026-01-31T10:00:00Z",
  "updated_at": "2026-01-31T10:00:00Z"
}
```

#### 2. 获取提示词列表

```http
GET /api/prompts?category=assistant&limit=20&offset=0
```

**查询参数**：
- `category`: 分类过滤（可选）
- `limit`: 每页数量（默认 20）
- `offset`: 偏移量（默认 0）

**响应**：
```json
{
  "prompts": [
    {
      "id": 1,
      "name": "默认助手",
      "content": "...",
      "category": "assistant"
    }
  ],
  "total": 1
}
```

#### 3. 获取单个提示词

```http
GET /api/prompts/1
```

**响应**：
```json
{
  "id": 1,
  "name": "默认助手",
  "content": "...",
  "category": "assistant"
}
```

#### 4. 获取默认提示词

```http
GET /api/prompts/default
```

#### 5. 更新提示词

```http
PUT /api/prompts/1
Content-Type: application/json

{
  "content": "更新后的提示词内容...",
  "description": "更新后的描述"
}
```

#### 6. 删除提示词

```http
DELETE /api/prompts/1
```

**响应**：
```json
{
  "message": "Prompt deleted successfully"
}
```

### Agent API

#### 1. 创建 Agent

```http
POST /api/agents
Content-Type: application/json

{
  "name": "翻译助手",
  "description": "专业的翻译助手",
  "prompt_id": 2,
  "avatar": "https://example.com/avatar.png",
  "config": "{\"temperature\": 0.7}",
  "is_active": true
}
```

**响应**：
```json
{
  "id": 1,
  "name": "翻译助手",
  "description": "专业的翻译助手",
  "prompt_id": 2,
  "prompt": {
    "id": 2,
    "name": "翻译助手",
    "content": "..."
  },
  "avatar": "https://example.com/avatar.png",
  "config": "{\"temperature\": 0.7}",
  "is_active": true,
  "created_at": "2026-01-31T10:00:00Z",
  "updated_at": "2026-01-31T10:00:00Z"
}
```

#### 2. 获取 Agent 列表

```http
GET /api/agents?is_active=true&limit=20&offset=0
```

**查询参数**：
- `is_active`: 是否激活（可选，true/false）
- `limit`: 每页数量（默认 20）
- `offset`: 偏移量（默认 0）

**响应**：
```json
{
  "agents": [
    {
      "id": 1,
      "name": "翻译助手",
      "prompt": {
        "id": 2,
        "name": "翻译助手"
      }
    }
  ],
  "total": 1
}
```

#### 3. 获取激活的 Agent

```http
GET /api/agents/active
```

**响应**：
```json
{
  "agents": [
    {
      "id": 1,
      "name": "翻译助手",
      "is_active": true
    }
  ],
  "total": 1
}
```

#### 4. 获取单个 Agent

```http
GET /api/agents/1
```

**响应**：
```json
{
  "id": 1,
  "name": "翻译助手",
  "prompt": {
    "id": 2,
    "name": "翻译助手",
    "content": "..."
  }
}
```

#### 5. 更新 Agent

```http
PUT /api/agents/1
Content-Type: application/json

{
  "description": "更新后的描述",
  "is_active": false
}
```

#### 6. 删除 Agent

```http
DELETE /api/agents/1
```

**响应**：
```json
{
  "message": "Agent deleted successfully"
}
```

### 聊天 API（支持 Agent）

#### 使用指定 Agent 聊天

```http
POST /api/chat
Content-Type: application/json

{
  "user_id": "user123",
  "message": "帮我翻译这段文字",
  "agent_id": 1
}
```

**说明**：
- 如果提供 `agent_id`，系统会使用该 Agent 关联的提示词
- 如果不提供 `agent_id`，使用默认提示词

**响应**：
```json
{
  "response": "当然，请提供需要翻译的文字...",
  "debug_info": {
    "llm_input": "..."
  }
}
```

## 使用场景

### 场景 1：创建多角色助手

1. **创建提示词**：
```bash
# 创建翻译助手提示词
curl -X POST http://localhost:8080/api/prompts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "翻译助手",
    "content": "你是一个专业的翻译助手...",
    "category": "translator"
  }'

# 创建代码助手提示词
curl -X POST http://localhost:8080/api/prompts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "代码助手",
    "content": "你是一个专业的编程助手...",
    "category": "coder"
  }'
```

2. **创建 Agent**：
```bash
# 创建翻译 Agent
curl -X POST http://localhost:8080/api/agents \
  -H "Content-Type: application/json" \
  -d '{
    "name": "小译",
    "description": "专业翻译助手",
    "prompt_id": 2,
    "avatar": "https://example.com/translator.png"
  }'

# 创建代码 Agent
curl -X POST http://localhost:8080/api/agents \
  -H "Content-Type: application/json" \
  -d '{
    "name": "小码",
    "description": "编程助手",
    "prompt_id": 3,
    "avatar": "https://example.com/coder.png"
  }'
```

3. **使用不同 Agent 聊天**：
```bash
# 使用翻译助手
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "message": "Translate: Hello World",
    "agent_id": 1
  }'

# 使用代码助手
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "message": "写一个快速排序算法",
    "agent_id": 2
  }'
```

### 场景 2：动态切换 Agent

```javascript
// 前端示例
const agents = await fetch('/api/agents/active').then(r => r.json());

// 用户选择 Agent
const selectedAgent = agents.agents[0];

// 发送消息
const response = await fetch('/api/chat', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    user_id: 'user123',
    message: '你好',
    agent_id: selectedAgent.id
  })
});
```

### 场景 3：管理提示词

```bash
# 列出所有提示词
curl http://localhost:8080/api/prompts

# 按分类筛选
curl http://localhost:8080/api/prompts?category=translator

# 更新提示词
curl -X PUT http://localhost:8080/api/prompts/1 \
  -H "Content-Type: application/json" \
  -d '{
    "content": "更新后的提示词内容..."
  }'

# 删除提示词
curl -X DELETE http://localhost:8080/api/prompts/1
```

## 默认数据

系统初始化时会自动创建以下默认提示词：

1. **默认助手** (category: assistant, is_default: true)
   - 通用 AI 助手

2. **翻译助手** (category: translator)
   - 专业翻译

3. **代码助手** (category: coder)
   - 编程辅助

## 错误处理

所有 API 在出错时返回标准错误格式：

```json
{
  "error": "错误描述信息"
}
```

**常见错误**：
- `400 Bad Request`: 请求参数错误
- `404 Not Found`: 资源不存在
- `500 Internal Server Error`: 服务器内部错误

## 注意事项

1. **唯一性约束**：
   - Prompt 的 `name` 必须唯一
   - Agent 的 `name` 必须唯一

2. **外键约束**：
   - Agent 的 `prompt_id` 必须引用存在的 Prompt
   - 删除 Prompt 前需要先删除关联的 Agent

3. **软删除**：
   - 删除操作是软删除，数据不会真正删除
   - 可以通过数据库直接恢复

4. **性能优化**：
   - Agent 查询自动预加载关联的 Prompt
   - 使用索引优化查询性能

## 相关文件

- `internal/memory/types.go` - 数据模型定义
- `internal/memory/sqlite.go` - 数据库操作
- `internal/api/prompt.go` - 提示词 API
- `internal/api/agent.go` - Agent API
- `internal/api/chat.go` - 聊天 API（支持 agent_id）
- `cmd/api/main.go` - 路由注册
