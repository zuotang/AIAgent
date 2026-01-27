# 记忆系统架构分析报告

## 一、当前架构概述

### 1. 短时记忆（Short-term Memory）

**实现方式**：`WindowMemory` 滑动窗口
```go
type WindowMemory struct {
    N     int      // 窗口大小（默认 8 轮）
    Turns []Turn   // 对话轮次
}
```

**特点**：
- ✅ 简单的 FIFO 队列
- ✅ 保持最近 N 轮完整对话
- ✅ 用于维持对话上下文连贯性

**使用场景**：
- 每次生成回复时，将窗口内容作为上下文
- 格式化为 `[TURN N] [USER] ... [ASSISTANT] ...`

---

### 2. 长时记忆（Long-term Memory）

#### 2.1 SQLite 结构化记忆

**数据结构**：
```sql
CREATE TABLE memories (
  id INTEGER PRIMARY KEY,
  user_id TEXT,
  mtype TEXT,      -- identity/preference/goal/tool/constraint/fact/activity/duration
  mkey TEXT,       -- 标准化的 key
  mvalue TEXT,     -- 值
  confidence REAL, -- 置信度 0-1
  owner TEXT,      -- user/agent
  updated_at DATETIME,
  UNIQUE(user_id, mtype, mkey, owner)
)
```

**召回策略**：
- 按 `updated_at DESC` 排序
- 取最近 30 条记录
- 格式：`owner: type.key = value (conf=0.85)`

#### 2.2 Qdrant 语义记忆

**存储内容**：
- 向量化的记忆文本
- 元数据：`user_id`, `text`, `timestamp`

**召回策略**：
- 基于当前用户输入的语义相似度
- Top-K 检索（默认 6 条）
- 实际使用时限制为 5 条
- 每条截断到 220 字符

---

### 3. 记忆提取流程

```
用户输入 + 助手回复
    ↓
LLM 记忆提取器（专门的 prompt）
    ↓
提取结构化记忆（JSON 格式）
    ↓
归一化 + 清洗 + 过滤
    ↓
并行写入 SQLite + Qdrant
```

**提取规则**：
- 只提取稳定、可复用的信息
- 过滤敏感信息（密码、身份证等）
- 置信度阈值：≥ 0.65
- 区分 user 和 agent 的记忆

---

## 二、架构评估

### ✅ 优点

1. **双存储架构合理**
   - SQLite：适合结构化查询（按类型、key 查找）
   - Qdrant：适合语义检索（模糊匹配、相关性）
   - 符合业界最佳实践（如 LangChain、LlamaIndex）

2. **记忆分层清晰**
   - 短时记忆：对话连贯性
   - 长时记忆：持久化知识
   - 职责分明，易于理解

3. **安全性考虑周到**
   - 敏感信息过滤
   - 置信度阈值
   - Owner 区分（避免混淆用户和 agent 信息）

4. **自动化记忆提取**
   - 无需手动标注
   - LLM 自动识别关键信息
   - 符合现代 AI Agent 设计

### ⚠️ 问题与不足

#### 1. 短时记忆问题

**问题 1.1：固定窗口大小不灵活**
```go
// 当前实现
if len(m.Turns) > m.N {
    m.Turns = m.Turns[len(m.Turns)-m.N:]  // 简单截断
}
```

**问题**：
- 不考虑对话的语义完整性
- 可能在话题中间截断
- 不考虑 token 限制

**业界做法**：
- 动态窗口：根据 token 数量调整
- 语义分段：保持话题完整性
- 摘要压缩：旧对话压缩成摘要

**问题 1.2：缺少重要性权重**
- 所有对话轮次权重相同
- 无法突出重要信息

---

#### 2. 长时记忆问题

**问题 2.1：SQLite 召回策略过于简单**
```go
// 当前：只按时间排序
ORDER BY updated_at DESC LIMIT 30
```

**问题**：
- 不考虑相关性
- 可能召回无关的旧记忆
- 30 条固定数量可能过多或过少

**改进建议**：
- 按类型分组召回（identity 优先级高）
- 结合置信度排序
- 根据当前对话主题过滤

**问题 2.2：Qdrant 召回限制不合理**
```go
// 配置 top_k = 6，但实际只用 5 条
maxRecall := 5  // 硬编码
for i, d := range recalledDocs {
    if i >= maxRecall { break }
}
```

**问题**：
- 为什么配置 6 但只用 5？
- 硬编码的 5 缺乏灵活性
- 220 字符截断可能破坏语义

**问题 2.3：双存储冗余**
```go
// 同一条记忆同时写入两个地方
mem.UpsertExtractedMemories(...)  // SQLite
store.UpsertTexts(...)             // Qdrant
```

**问题**：
- 数据冗余
- 同步问题（一个成功一个失败）
- 没有事务保证

---

#### 3. 记忆提取问题

**问题 3.1：每轮都提取，成本高**
```go
// 每次对话后都调用 LLM 提取
extracted, err = extractMemories(...)
```

**问题**：
- 增加延迟
- 增加 API 成本
- 可能提取大量重复信息

**改进建议**：
- 批量提取（每 N 轮提取一次）
- 增量提取（只提取新信息）
- 异步提取（不阻塞用户）

**问题 3.2：提取 prompt 过于复杂**
```go
sys := `你是"记忆提取器"。只输出严格 JSON，不要任何额外文字。
目标：从对话中提取未来能提升个性化与效率的长期信息。
...（200+ 行 prompt）`
```

**问题**：
- Prompt 过长，消耗 token
- 规则复杂，LLM 可能理解偏差
- 难以维护和调试

**问题 3.3：归一化逻辑硬编码**
```go
switch m.Type {
case "identity":
    if m.Owner == "agent" && (m.Key == "assistant_name" || ...) {
        m.Key = "name"
    }
}
```

**问题**：
- 规则硬编码，难以扩展
- 应该由配置或数据库驱动

---

#### 4. 记忆召回问题

**问题 4.1：召回顺序不合理**
```go
// 当前顺序
1. 短期对话窗口
2. 结构化长期记忆（SQLite）
3. 语义长期记忆（Qdrant）
4. 用户输入
```

**问题**：
- 结构化记忆和语义记忆分开展示
- 可能有重复内容
- 没有去重和融合

**改进建议**：
- 融合召回：合并相似记忆
- 重排序：按相关性重新排序
- 去重：避免重复信息

**问题 4.2：缺少记忆过期机制**
```sql
-- 没有过期时间字段
-- 旧记忆永久保留
```

**问题**：
- 过时信息可能误导
- 数据库无限增长
- 没有遗忘机制

**改进建议**：
- 添加 `expires_at` 字段
- 定期清理低置信度记忆
- 实现记忆衰减（confidence 随时间降低）

---

#### 5. 性能问题

**问题 5.1：每次都查询数据库**
```go
// 每轮对话都查询
structuredText, _ = mem.RenderStructuredMemory(...)
recalledDocs, err = store.SimilaritySearch(...)
```

**问题**：
- 没有缓存
- 重复查询相同内容
- 增加延迟

**改进建议**：
- 添加内存缓存（LRU）
- 批量预加载
- 增量更新

**问题 5.2：串行处理**
```go
// 串行查询
structuredText, _ = mem.RenderStructuredMemory(...)  // 1
recalledDocs, err = store.SimilaritySearch(...)      // 2
```

**改进建议**：
- 并行查询 SQLite 和 Qdrant
- 使用 goroutine + channel

---

## 三、业界对比

### 1. LangChain Memory

**特点**：
- 多种记忆类型：ConversationBufferMemory, ConversationSummaryMemory
- 自动摘要压缩
- 支持多种后端（Redis, MongoDB, Postgres）

**你的实现对比**：
- ✅ 类似 ConversationBufferMemory（窗口记忆）
- ❌ 缺少摘要压缩
- ✅ 双后端（SQLite + Qdrant）

### 2. MemGPT

**特点**：
- 分层记忆：工作记忆、短期记忆、长期记忆
- 主动记忆管理（agent 决定何时存储/召回）
- 记忆分页（类似操作系统虚拟内存）

**你的实现对比**：
- ✅ 有分层（短期 + 长期）
- ❌ 被动记忆管理（每轮都提取）
- ❌ 没有分页机制

### 3. AutoGPT Memory

**特点**：
- 向量数据库 + 传统数据库
- 记忆重要性评分
- 定期记忆整理

**你的实现对比**：
- ✅ 双数据库架构
- ✅ 有置信度（类似重要性）
- ❌ 没有定期整理

---

## 四、优化建议（按优先级）

### 🔴 高优先级（核心问题）

#### 1. 修复 Qdrant 召回配置不一致
```yaml
# config.yaml
qdrant:
  top_k: 6  # 配置

# main.go
maxRecall := 5  # 硬编码，不一致
```

**建议**：
```go
// 使用配置值
maxRecall := cfg.Qdrant.TopK
// 或者移除 maxRecall，直接使用所有召回结果
```

#### 2. 添加记忆去重
```go
// 当前：SQLite 和 Qdrant 可能返回重复内容
// 建议：添加去重逻辑
func deduplicateMemories(structured, semantic []string) []string {
    seen := make(map[string]bool)
    result := []string{}
    for _, m := range append(structured, semantic...) {
        key := normalizeMemory(m)
        if !seen[key] {
            seen[key] = true
            result = append(result, m)
        }
    }
    return result
}
```

#### 3. 优化 SQLite 召回策略
```sql
-- 当前
SELECT * FROM memories WHERE user_id=? ORDER BY updated_at DESC LIMIT 30

-- 建议：按类型分组，优先召回重要类型
SELECT * FROM memories
WHERE user_id=?
ORDER BY
  CASE mtype
    WHEN 'identity' THEN 1
    WHEN 'preference' THEN 2
    WHEN 'goal' THEN 3
    ELSE 4
  END,
  confidence DESC,
  updated_at DESC
LIMIT 30
```

#### 4. 添加事务保证
```go
// 当前：分别写入，可能不一致
mem.UpsertExtractedMemories(...)
store.UpsertTexts(...)

// 建议：添加补偿机制
if err := mem.UpsertExtractedMemories(...); err != nil {
    return err
}
if err := store.UpsertTexts(...); err != nil {
    // 回滚 SQLite 或记录失败
    log.Error("Qdrant write failed, SQLite may be inconsistent")
}
```

### 🟡 中优先级（性能优化）

#### 5. 并行查询记忆
```go
// 当前：串行
structuredText, _ = mem.RenderStructuredMemory(...)
recalledDocs, err = store.SimilaritySearch(...)

// 建议：并行
type MemoryResult struct {
    Structured string
    Semantic   []rag.Doc
    Err        error
}

ch := make(chan MemoryResult, 2)

go func() {
    text, err := mem.RenderStructuredMemory(...)
    ch <- MemoryResult{Structured: text, Err: err}
}()

go func() {
    docs, err := store.SimilaritySearch(...)
    ch <- MemoryResult{Semantic: docs, Err: err}
}()

// 收集结果
for i := 0; i < 2; i++ {
    result := <-ch
    // 处理结果
}
```

#### 6. 添加记忆缓存
```go
type MemoryCache struct {
    cache *lru.Cache
    ttl   time.Duration
}

func (c *MemoryCache) Get(userID string) (*CachedMemory, bool) {
    // 检查缓存
}

func (c *MemoryCache) Set(userID string, memory *CachedMemory) {
    // 设置缓存
}
```

#### 7. 异步记忆提取
```go
// 当前：同步提取，阻塞用户
extracted, err = extractMemories(...)

// 建议：异步提取
go func() {
    extracted, err := extractMemories(...)
    if err == nil {
        mem.UpsertExtractedMemories(...)
        store.UpsertTexts(...)
    }
}()
// 立即返回给用户，后台提取
```

### 🟢 低优先级（功能增强）

#### 8. 添加记忆过期机制
```sql
ALTER TABLE memories ADD COLUMN expires_at DATETIME;
ALTER TABLE memories ADD COLUMN access_count INTEGER DEFAULT 0;

-- 查询时过滤过期记忆
SELECT * FROM memories
WHERE user_id=?
  AND (expires_at IS NULL OR expires_at > datetime('now'))
ORDER BY updated_at DESC
```

#### 9. 实现记忆衰减
```go
// 定期降低旧记忆的置信度
func decayMemories(db *sql.DB) {
    db.Exec(`
        UPDATE memories
        SET confidence = confidence * 0.95
        WHERE updated_at < datetime('now', '-30 days')
          AND confidence > 0.1
    `)
}
```

#### 10. 添加记忆统计
```go
type MemoryStats struct {
    TotalMemories   int
    ByType          map[string]int
    AvgConfidence   float64
    OldestMemory    time.Time
    NewestMemory    time.Time
}

func (s *Store) GetStats(userID string) (*MemoryStats, error) {
    // 统计记忆信息
}
```

---

## 五、推荐的最终架构

```
┌─────────────────────────────────────────────────────────┐
│                    用户输入                              │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│              记忆召回（并行）                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ 短期记忆     │  │ SQLite       │  │ Qdrant       │  │
│  │ (窗口 8 轮)  │  │ (结构化)     │  │ (语义)       │  │
│  │              │  │ 按类型+置信度│  │ Top-K 相似度 │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│              记忆融合 + 去重 + 重排序                    │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│              LLM 生成回复                                │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│         记忆提取（异步，批量）                           │
│  ┌──────────────────────────────────────────────────┐  │
│  │ 1. LLM 提取关键信息                              │  │
│  │ 2. 归一化 + 清洗 + 去重                          │  │
│  │ 3. 事务写入 SQLite + Qdrant                      │  │
│  │ 4. 更新缓存                                      │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

---

## 六、总结

### 当前实现评分

| 维度 | 评分 | 说明 |
|------|------|------|
| 架构设计 | 8/10 | 双存储架构合理，分层清晰 |
| 代码质量 | 7/10 | 实现清晰，但有硬编码和冗余 |
| 性能 | 6/10 | 串行查询，无缓存，有优化空间 |
| 可维护性 | 7/10 | 结构清晰，但配置分散 |
| 安全性 | 8/10 | 敏感信息过滤完善 |
| 可扩展性 | 6/10 | 部分逻辑硬编码，扩展受限 |
| **总分** | **7/10** | **良好，但有明显优化空间** |

### 核心建议

1. **立即修复**：配置不一致、缺少去重
2. **性能优化**：并行查询、添加缓存
3. **功能增强**：记忆过期、异步提取

你的实现已经达到了**生产可用**的水平，但还有很大的优化空间。建议优先处理高优先级问题，然后逐步优化性能和功能。
