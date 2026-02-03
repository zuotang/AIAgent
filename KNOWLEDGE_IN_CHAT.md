# 在聊天中触发知识库检索 - 使用指南

## 概述

现在知识库和记忆都存储在 `memories` 集合中，可以在聊天时通过关键词自动触发检索。

## 🔧 修改内容

### 1. 知识录入时添加 agent_id

知识现在会带上 `agent_id` 和 `type: "knowledge"` 标记，存储在 `memories` 集合中。

### 2. 支持的 API 参数

所有知识库 API 现在都支持 `agent_id` 参数：

```json
{
  "user_id": "user123",
  "agent_id": 1,  // 新增：指定 Agent ID
  "text": "知识内容..."
}
```

## 📝 使用方法

### 方法1：录入知识到 memories 集合

```bash
# 录入文本知识
curl -X POST http://localhost:8080/api/knowledge/ingest/text \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "agent_id": 1,
    "text": "Python是一种高级编程语言，由Guido van Rossum于1991年创建。它以简洁易读的语法著称，广泛应用于Web开发、数据分析、人工智能等领域。",
    "file_name": "python_intro.txt"
  }'

# 录入文件
curl -X POST http://localhost:8080/api/knowledge/ingest/file \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "agent_id": 1,
    "path": "/path/to/knowledge.txt",
    "chunk_size": 1000,
    "chunk_overlap": 200
  }'
```

### 方法2：在聊天中触发知识检索

使用触发关键词，系统会自动从 `memories` 集合检索相关知识：

```bash
# 示例1：使用"回忆"关键词
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "agent_id": 1,
    "message": "回忆一下Python的特点"
  }'

# 示例2：使用"还记得"关键词
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "agent_id": 1,
    "message": "你还记得Python是什么时候创建的吗？"
  }'

# 示例3：使用"知识库"关键词（需要添加到配置）
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "agent_id": 1,
    "message": "从知识库中查找Python的信息"
  }'
```

## ⚙️ 配置触发关键词

编辑 `config.yaml` 添加知识检索相关的关键词：

```yaml
memory:
  on_demand_min_length: 8
  on_demand_keywords:
    # 原有的记忆关键词
    - 回忆
    - 记得
    - 还记得
    - 你记得
    - 过去
    - 以前

    # 新增：知识检索关键词
    - 知识库
    - 查询
    - 搜索
    - 查找
    - 告诉我
    - 介绍一下
    - 什么是
    - 如何
    - 怎么
```

## 🔍 检索流程

```
用户消息: "回忆一下Python的特点"
    │
    ├─ 检测到关键词 "回忆" ✅
    ├─ 消息长度 > 8 ✅
    │
    ▼
触发记忆检索
    │
    ├──────────────┬──────────────┐
    │              │              │
    ▼              ▼              ▼
SQLite         Qdrant        继续处理
(结构化记忆)    (语义搜索)
    │              │
    │              ├─ 对话记忆 (type: memory)
    │              └─ 知识内容 (type: knowledge)
    │              │
    └──────┬───────┘
           ▼
    格式化并注入到 LLM
           │
           ▼
    生成带知识上下文的回复
```

## 📊 数据结构

### 知识在 Qdrant 中的存储格式

```json
{
  "id": "random_uuid",
  "vector": [0.1, 0.2, ...],
  "payload": {
    "user_id": "user123",
    "agent_id": 1,
    "type": "knowledge",  // 标记为知识类型
    "text": "Python是一种高级编程语言...",
    "source": "python_intro.txt",
    "ts": "2026-02-03T12:00:00Z"
  }
}
```

### 对话记忆的存储格式

```json
{
  "id": "random_uuid",
  "vector": [0.3, 0.4, ...],
  "payload": {
    "user_id": "user123",
    "agent_id": 1,
    "type": "memory",  // 标记为记忆类型
    "text": "用户喜欢蓝色",
    "ts": "2026-02-03T12:00:00Z"
  }
}
```

## 🎯 实际示例

### 1. 录入Python知识

```bash
curl -X POST http://localhost:8080/api/knowledge/ingest/text \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "student_001",
    "agent_id": 1,
    "text": "Python是一种解释型、面向对象、动态数据类型的高级程序设计语言。Python由Guido van Rossum于1989年底发明，第一个公开发行版发行于1991年。Python语法简洁清晰，特色之一是强制用空白符作为语句缩进。",
    "file_name": "python_basics.txt"
  }'
```

### 2. 在聊天中查询

```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "student_001",
    "agent_id": 1,
    "message": "你还记得Python是谁发明的吗？"
  }'
```

**预期响应**：
```json
{
  "response": "根据我的记忆，Python是由Guido van Rossum发明的。他在1989年底开始开发Python，第一个公开发行版在1991年发布。Python是一种解释型、面向对象的高级编程语言，以其简洁清晰的语法而著称。",
  "debug_info": {
    "llm_input": "..."
  }
}
```

## 🔧 调试

### 查看是否触发记忆检索

启用调试模式（`config.yaml`）：

```yaml
base:
  debug: true
```

查看日志：

```
[DEBUG] 触发按需记忆加载: 还记得
[DEBUG] 按需记忆已加载
[DEBUG] 结构化记忆查询失败: ... (如果有错误)
```

### 验证知识已录入

```bash
# 查询知识库
curl -X POST http://localhost:8080/api/knowledge/query \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "student_001",
    "agent_id": 1,
    "query": "Python",
    "limit": 5
  }'
```

## 💡 最佳实践

### 1. 使用明确的触发词

在消息中明确使用触发关键词：

✅ 好的示例：
- "回忆一下Python的特点"
- "你还记得Python的语法特点吗？"
- "查找关于Python的知识"

❌ 不好的示例：
- "Python" （太短，没有触发词）
- "告诉我" （太短）

### 2. 为不同类型的知识使用不同的 agent_id

```bash
# 编程知识 - Agent 1
curl -X POST http://localhost:8080/api/knowledge/ingest/text \
  -d '{"user_id": "user123", "agent_id": 1, "text": "Python知识..."}'

# 历史知识 - Agent 2
curl -X POST http://localhost:8080/api/knowledge/ingest/text \
  -d '{"user_id": "user123", "agent_id": 2, "text": "历史知识..."}'
```

### 3. 添加自定义触发关键词

根据您的使用场景，在 `config.yaml` 中添加相关关键词：

```yaml
memory:
  on_demand_keywords:
    # 通用
    - 回忆
    - 记得

    # 知识检索
    - 知识库
    - 查询
    - 搜索

    # 学习场景
    - 学过
    - 教过
    - 讲过

    # 技术场景
    - 文档
    - API
    - 代码
```

## 🎉 总结

现在您可以：

1. ✅ 将知识录入到 `memories` 集合
2. ✅ 在聊天中使用关键词自动触发知识检索
3. ✅ 知识和记忆统一管理，按 `user_id` 和 `agent_id` 隔离
4. ✅ 通过 `type` 字段区分知识和记忆

知识会在聊天时自动被检索并注入到 LLM 上下文中，无需额外的 API 调用！
