# 三层记忆架构

## 概述

实现了由硬到软的三层记忆系统，模拟人类记忆的层次结构。

## 三层架构

### Layer 1: 硬记忆（Hard Memory）- 永久存储
**特点：**
- 永久保存，不会过期
- 存储核心身份信息和关键事实
- 高重要性评分（0.8-1.0）
- 存储在 SQLite 数据库

**内容类型：**
- 身份信息：姓名、年龄、性别、职业
- 核心偏好：重要的喜好、价值观
- 关键事实：不会改变的信息
- 重要关系：亲密关系、重要称呼

**示例：**
```json
{
  "type": "relationship",
  "key": "nickname",
  "value": "小明",
  "layer": 1,
  "importance": 0.9,
  "owner": "user"
}
```

### Layer 2: 中等记忆（Medium Memory）- 半永久存储
**特点：**
- 半永久保存，可能过期
- 存储近期事件和上下文信息
- 中等重要性评分（0.4-0.8）
- 存储在 SQLite + Qdrant 向量库

**内容类型：**
- 近期事件：最近发生��事情
- 对话主题：讨论过的话题
- 情感状态：情绪变化、关系进展
- 一般偏好：非核心的喜好

**示例：**
```json
{
  "type": "event",
  "key": "important_event",
  "value": "今天去了公园",
  "layer": 2,
  "importance": 0.6,
  "owner": "user"
}
```

### Layer 3: 软记忆（Soft Memory）- 临时存储
**特点：**
- 临时保存，会话结束后清除
- 存储当前场景和临时状态
- 低重要性评分（0.0-0.4）
- 存储在内存中的 WindowMemory

**内容类型：**
- 当前场景：时间、地点、环境
- 临时状态：当前心情、正在做的事
- 一次性信息：不需要长期保存的内容
- 对话细节：当前话题的具体内容

**示例：**
```json
{
  "type": "scene",
  "key": "time",
  "value": "晚上8点",
  "layer": 3,
  "importance": 0.2,
  "owner": "user"
}
```

## 数据库结构

### memories 表
```sql
CREATE TABLE memories (
  id INTEGER PRIMARY KEY,
  user_id TEXT NOT NULL,
  mtype TEXT NOT NULL,
  mkey TEXT NOT NULL,
  mvalue TEXT NOT NULL,
  confidence REAL DEFAULT 0.7,
  owner TEXT DEFAULT 'user',
  layer INTEGER DEFAULT 2,        -- 新增：记忆层级
  importance REAL DEFAULT 0.5,    -- 新增：重要性评分
  updated_at DATETIME,
  access_count INTEGER DEFAULT 0,
  last_accessed_at DATETIME,
  deleted_at DATETIME
);
```

## 记忆提取规则

### 重要性评分规则
- **1.0**: 核心身份信息（姓名、年龄等）→ Layer 1
- **0.8-0.9**: 重要偏好、关键事实 → Layer 1
- **0.6-0.7**: 重要事件、主题讨论 → Layer 2
- **0.4-0.5**: 一般事件、情绪状态 → Layer 2
- **0.2-0.3**: 临时场景、当前状态 → Layer 3
- **0.0-0.1**: 一次性信息 → Layer 3

### 自动分层逻辑
```go
if importance >= 0.8 {
    layer = 1  // 硬记忆
} else if importance >= 0.4 {
    layer = 2  // 中等记忆
} else {
    layer = 3  // 软记忆
}
```

## 记忆查询策略

### Layer 1 查询
- 直接从 SQLite 查询 `layer = 1` 的记忆
- 始终包含在上下文中
- 不受时间限制

### Layer 2 查询
- 从 SQLite 查询 `layer = 2` 的记忆
- 从 Qdrant 进行语义检索
- 可能有时间衰减

### Layer 3 查询
- 从 WindowMemory 获取
- 只包含当前会话的对话
- 会话结束后清空

## 记忆格式化

```
【硬记忆 - 核心信息】
- 姓名: 小明
- 年龄: 25岁
- 职业: 程序员

【中等记忆 - 近期上下文】
- 今天去了公园
- 讨论了工作压力
- 心情有些低落

【软记忆 - 当前会话】
[TURN 1]
用户: 你好
助手: 你好！

[TURN 2]
用户: 我是小明
助手: 很高兴认识你，小明！
```

## 使用示例

### 提取记忆
```go
memories, err := ExtractMemories(ctx, llm, history, userText, assistantText, debug, model, includeHistory)
// 返回的记忆已包含 layer 和 importance 字段
```

### 查询记忆
```go
// 查询所有层级
hardMemories := store.GetMemoriesByLayer(ctx, userID, 1)
mediumMemories := store.GetMemoriesByLayer(ctx, userID, 2)
softMemories := windowMem.String()
```

### 格式化记忆
```go
formatted := formatThreeLayerMemories(hardMemories, mediumMemories, softMemories)
```

## 优势

1. **分层存储**：不同重要性的信息存储在不同层级
2. **性能优化**：临时信息不写入数据库，减少 I/O
3. **智能过期**：软记忆自动清除，中等记忆可设置过期
4. **灵活查询**：可以只查询特定层级的记忆
5. **模拟人类**：符合人类记忆的工作方式

## 配置

```yaml
memory:
  layer1_enabled: true   # 启用硬记忆
  layer2_enabled: true   # 启用中等记忆
  layer3_enabled: true   # 启用软记忆
  layer2_ttl: 2592000    # 中等记忆过期时间（秒，默认30天）
  layer3_window: 10      # 软记忆窗口大小（对话轮数）
```

## 实现状态

- ✅ 数据模型更新（ExtractedMemory, Memory）
- ✅ 数据库表结构更新（layer, importance 字段）
- ✅ 记忆提取提示词更新（三层架构说明）
- ✅ normalize 函数更新（自动分层逻辑）
- ⏳ 记忆查询逻辑（待实现）
- ⏳ 记忆格式化（待实现）
- ⏳ 配置选项（待实现）
