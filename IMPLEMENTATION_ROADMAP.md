# Agent系统优化实施路线图

基于 OPTIMIZATION_PLAN.md 的详细方案，这里提供分阶段、可执行的实施路线图。

---

## 实施原则

1. **渐进式迭代**：每个阶段都能独立运行，不破坏现有功能
2. **向后兼容**：保留原有API，逐步迁移
3. **测试驱动**：每个模块都有单元测试
4. **文档同步**：代码变更同步更新文档

---

## 阶段一：代码重构与模块化（2-3天）

### 目标
- 清理 main.go，将职责分离到各个模块
- 建立清晰的分层架构
- 提高代码可维护性

### 具体任务

#### Task 1.1: 创建接口层
```bash
# 创建新目录
mkdir -p internal/agent
mkdir -p internal/orchestrator
mkdir -p internal/tools
mkdir -p internal/session

# 定义核心接口
touch internal/agent/interface.go
touch internal/memory/interface.go
touch internal/rag/interface.go
```

**文件内容**:
- `internal/agent/interface.go`: Agent接口定义
- `internal/memory/interface.go`: Memory接口定义
- `internal/rag/interface.go`: VectorStore接口定义

**验收标准**:
- 所有接口编译通过
- 现有的 Ollama/DeepSeek 实现符合 LLMClient 接口
- 现有的 SQLite 实现符合 Memory 接口
- 现有的 Qdrant 实现符合 VectorStore 接口

---

#### Task 1.2: 抽取编排层
从 `cmd/chat/main.go` 中抽取：
- 对话流程编排 → `internal/orchestrator/conversation.go`
- 记忆提取流程 → `internal/orchestrator/memory_pipeline.go`
- 上下文构建 → `internal/orchestrator/context_builder.go`

**代码结构**:
```go
// internal/orchestrator/conversation.go
type ConversationOrchestrator struct {
    llm    models.LLMClient
    memory memory.Memory
    vector rag.VectorStore
    config OrchestratorConfig
}

func (o *ConversationOrchestrator) HandleUserInput(ctx context.Context, userID, input string) (string, error)
func (o *ConversationOrchestrator) extractMemories(ctx context.Context, conversation []Turn) ([]memory.ExtractedMemory, error)
```

**验收标准**:
- `main.go` 减少到 < 200 行
- 编排逻辑可独立测试
- 现有功能完全正常运行

---

#### Task 1.3: 实现对话式Agent
```go
// internal/agent/conversational.go
type ConversationalAgent struct {
    llm    models.LLMClient
    memory memory.Memory
    tools  *tools.ToolRegistry  // 暂时为空
}

func (a *ConversationalAgent) Run(ctx context.Context, input AgentInput) (AgentOutput, error)
```

**验收标准**:
- Agent 可以独立运行对话
- 与 Orchestrator 集成成功
- 通过集成测试

---

#### Task 1.4: 添加单元测试
为每个新模块添加测试：
```bash
touch internal/agent/conversational_test.go
touch internal/orchestrator/conversation_test.go
touch internal/memory/sqlite_test.go
```

**测试覆盖**:
- Agent.Run() 逻辑
- Orchestrator.HandleUserInput() 流程
- Memory 接口的 mock 实现

**验收标准**:
- 测试覆盖率 > 70%
- 所有测试通过：`go test ./...`

---

## 阶段二：工具系统（3-4天）

### 目标
- 实现工具注册与调用框架
- 添加3-5个内置工具
- 支持 Function Calling

### 具体任务

#### Task 2.1: 工具框架
```bash
mkdir -p internal/tools
touch internal/tools/registry.go
touch internal/tools/interface.go
```

**核心代码**:
```go
// internal/tools/interface.go
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, input string) (string, error)
    Schema() ToolSchema
}

type ToolRegistry struct {
    tools map[string]Tool
}
```

**验收标准**:
- 工具注册机制完成
- 可以动态添加/移除工具
- 工具 Schema 符合 JSON Schema 规范

---

#### Task 2.2: 实现内置工具
```bash
touch internal/tools/memory_search.go      # 记忆搜索
touch internal/tools/calculator.go         # 计算器
touch internal/tools/time.go               # 时间工具
touch internal/tools/weather.go            # 天气查询（可选）
```

**工具列表**:
1. **MemorySearchTool**: 语义搜索用户记忆
2. **CalculatorTool**: 数学计算
3. **TimeTool**: 获取当前时间/日期
4. **WeatherTool**: 查询天气（需API key）

**验收标准**:
- 每个工具可独立运行
- 工具有单元测试
- 集成到 Agent 中可用

---

#### Task 2.3: 集成 Function Calling
修改 LLM 客户端支持工具调用：

```go
// internal/models/interface.go
type LLMClient interface {
    Chat(ctx context.Context, msgs []ChatMessage, model ...string) (string, error)

    // 新增：支持工具调用
    ChatWithTools(ctx context.Context, msgs []ChatMessage, tools []tools.Tool) (Response, error)
}

type Response struct {
    Content   string
    ToolCalls []ToolCall
}

type ToolCall struct {
    ID        string
    ToolName  string
    Arguments string  // JSON string
}
```

**Ollama 实现**:
```go
// internal/models/ollama.go
func (c *OllamaClient) ChatWithTools(ctx context.Context, msgs []ChatMessage, tools []tools.Tool) (Response, error) {
    // 将工具注入系统提示
    systemMsg := buildSystemPromptWithTools(tools)

    // 调用 LLM
    response, err := c.Chat(ctx, append([]ChatMessage{{Role: "system", Content: systemMsg}}, msgs...))

    // 解析工具调用（从 response 中提取 JSON）
    toolCalls := parseToolCalls(response)

    return Response{Content: response, ToolCalls: toolCalls}, err
}
```

**验收标准**:
- Agent 可以根据用户输入决定调用哪个工具
- 工具调用结果反馈给 LLM
- 完整的 ReAct 循环可运行

---

#### Task 2.4: 实现 ReAct Agent
```bash
touch internal/agent/react.go
touch internal/agent/react_test.go
```

**核心逻辑**:
```go
func (a *ReActAgent) Run(ctx context.Context, task string) (string, error) {
    for i := 0; i < a.maxIterations; i++ {
        // 1. Thought: LLM 决定下一步
        action := a.planNextAction(ctx, currentThought)

        if action.Type == "finish" {
            return action.Answer, nil
        }

        // 2. Action: 执行工具
        result := a.executeTool(ctx, action)

        // 3. Observation: 更新思考
        currentThought = fmt.Sprintf("Action: %s\nResult: %s", action.ToolName, result)
    }
}
```

**验收标准**:
- ReAct 循环可以自主调用多个工具
- 最多迭代次数保护（防止死循环）
- 调试模式输出思考过程

---

## 阶段三：记忆系统优化（3-4天）

### 目标
- 实现分层记忆架构
- 添加重要性评分
- 实现记忆遗忘机制

### 具体任务

#### Task 3.1: 数据库Schema升级
```sql
-- 添加新字段
ALTER TABLE memories ADD COLUMN created_at DATETIME DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE memories ADD COLUMN last_accessed_at DATETIME DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE memories ADD COLUMN access_count INTEGER DEFAULT 0;
ALTER TABLE memories ADD COLUMN importance_score REAL DEFAULT 0.5;

-- 创建会话缓冲表
CREATE TABLE conversation_buffer (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    turn_index INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_buffer_session ON conversation_buffer(user_id, session_id, turn_index);
```

**验收标准**:
- Schema 升级脚本无损执行
- 兼容旧数据（旧记忆自动填充默认值）
- 索引创建成功

---

#### Task 3.2: 实现分层记忆
```bash
touch internal/memory/layered.go
touch internal/memory/layered_test.go
```

**核心代码**:
```go
// internal/memory/layered.go
type LayeredMemory struct {
    sqlite *SQLiteMemory
    qdrant *rag.QdrantStore
}

func (m *LayeredMemory) GetContextForPrompt(ctx context.Context, userID, sessionID string) (string, error) {
    // 1. 工作记忆：当前会话
    working := m.getWorkingMemory(ctx, userID, sessionID, 8)

    // 2. 短期记忆：最近7天
    shortTerm := m.getShortTermMemory(ctx, userID, 7*24*time.Hour)

    // 3. 长期记忆：高重要性 + 语义搜索
    longTerm := m.getLongTermMemory(ctx, userID, currentQuery, 0.8)

    return combineMemories(working, shortTerm, longTerm), nil
}
```

**验收标准**:
- 三层记忆清晰分离
- 查询性能测试（应 < 100ms）
- 集成到 Orchestrator

---

#### Task 3.3: 重要性评分系统
```bash
touch internal/memory/importance.go
touch internal/memory/importance_test.go
```

**实现方式**:
1. **规则评分**（快速，作为基线）:
   ```go
   func ruleBasedImportance(mem ExtractedMemory) float64 {
       score := 0.5  // 基础分

       if mem.Type == "identity" {
           score += 0.3
       } else if mem.Type == "preference" {
           score += 0.2
       }

       if mem.Confidence > 0.9 {
           score += 0.1
       }

       return min(score, 1.0)
   }
   ```

2. **LLM评分**（慢但准确，用于关键记忆）:
   ```go
   func llmBasedImportance(ctx context.Context, llm models.LLMClient, mem ExtractedMemory) (float64, error) {
       prompt := fmt.Sprintf("Rate the importance of this memory (0-1): %s = %s", mem.Key, mem.Value)
       // ... LLM调用
   }
   ```

**混合策略**:
- 首次提取：使用规则评分
- 高置信度记忆（> 0.8）：异步 LLM 重新评分
- 每周定期：对访问频繁的记忆重新评分

**验收标准**:
- 评分算法通过单元测试
- LLM 评分可选（通过配置开关）
- 性能影响 < 10%

---

#### Task 3.4: 记忆遗忘机制
```bash
touch internal/memory/pruning.go
touch cmd/prune/main.go  # 定时任务
```

**遗忘策略**:
```go
func (m *SQLiteMemory) PruneOldMemories(ctx context.Context, userID string) (int, error) {
    // 规则：删除同时满足以下条件的记忆
    // 1. 超过30天
    // 2. 访问次数 < 3
    // 3. 重要性 < 0.3
    // 4. 非 identity 类型

    query := `
        DELETE FROM memories
        WHERE user_id = ?
          AND mtype != 'identity'
          AND created_at < datetime('now', '-30 days')
          AND access_count < 3
          AND importance_score < 0.3
    `

    result, err := m.db.ExecContext(ctx, query, userID)
    rowsAffected, _ := result.RowsAffected()
    return int(rowsAffected), err
}
```

**定时任务**:
```go
// cmd/prune/main.go
func main() {
    ticker := time.NewTicker(24 * time.Hour)
    for range ticker.C {
        pruneAllUsers()
    }
}
```

**验收标准**:
- 遗忘算法不误删重要记忆
- 定时任务可正常运行
- 日志记录删除统计

---

## 阶段四：高级功能（4-5天）

### 目标
- 流式响应
- 多Agent协作
- 可观测性

### 具体任务

#### Task 4.1: 流式响应
```bash
touch internal/models/streaming.go
touch internal/orchestrator/streaming.go
```

**实现**:
```go
// internal/models/interface.go
type LLMClient interface {
    Chat(ctx context.Context, msgs []ChatMessage) (string, error)
    ChatStream(ctx context.Context, msgs []ChatMessage) (<-chan string, error)  // 新增
}

// internal/models/ollama.go
func (c *OllamaClient) ChatStream(ctx context.Context, msgs []ChatMessage) (<-chan string, error) {
    stream := make(chan string, 100)

    req := &ChatRequest{
        Model:    c.chatModel,
        Messages: msgs,
        Stream:   true,
    }

    resp, err := c.doRequest(ctx, "/api/chat", req)

    go func() {
        defer close(stream)
        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            var chunk ChatResponse
            json.Unmarshal(scanner.Bytes(), &chunk)
            if chunk.Message.Content != "" {
                stream <- chunk.Message.Content
            }
        }
    }()

    return stream, err
}
```

**CLI 更新**:
```go
// cmd/chat/main.go
stream, err := orchestrator.HandleUserInputStreaming(ctx, userID, input)

fmt.Print("Assistant: ")
for token := range stream {
    fmt.Print(token)
}
fmt.Println()
```

**验收标准**:
- 流式输出实时显示
- 不阻塞后续处理
- 兼容非流式模式

---

#### Task 4.2: 会话管理
```bash
mkdir -p internal/session
touch internal/session/manager.go
touch internal/session/manager_test.go
```

**功能**:
```go
type SessionManager struct {
    store map[string]*Session
}

func (m *SessionManager) CreateSession(userID string) *Session
func (m *SessionManager) GetSession(sessionID string) (*Session, bool)
func (m *SessionManager) ListUserSessions(userID string) []*Session
func (m *SessionManager) CleanupStale(maxIdleTime time.Duration)
```

**CLI 命令**:
```bash
# 创建新会话
./chat.exe --new-session

# 恢复会话
./chat.exe --session <session-id>

# 列出会话
./chat.exe --list-sessions
```

**验收标准**:
- 会话可持久化（保存到 SQLite）
- 自动清理过期会话（超过24小时未活跃）
- CLI 命令正常工作

---

#### Task 4.3: 可观测性
```bash
mkdir -p internal/telemetry
touch internal/telemetry/tracer.go
touch internal/telemetry/logger.go
```

**Trace 输出示例**:
```
[TRACE] conversation.handle_input (2.3s)
  ├─ [0.05s] memory.get_context
  │  ├─ [0.02s] sqlite.query
  │  └─ [0.03s] qdrant.similarity_search
  ├─ [2.1s] llm.chat
  │  └─ [2.0s] ollama.api_call
  └─ [0.15s] memory.extract
     └─ [0.1s] llm.chat
```

**日志结构化**:
```json
{
  "timestamp": "2026-01-27T10:30:00Z",
  "level": "INFO",
  "component": "orchestrator",
  "action": "handle_input",
  "user_id": "user123",
  "duration_ms": 2300,
  "metadata": {
    "memory_hits": 15,
    "tokens_sent": 450,
    "tokens_received": 280
  }
}
```

**验收标准**:
- 所有关键操作都有 trace
- 日志可配置级别（DEBUG/INFO/WARN/ERROR）
- 支持导出到文件

---

#### Task 4.4: 多Agent协作（实验性）
```bash
touch internal/agent/crew.go
touch examples/multi_agent_demo.go
```

**示例场景**:
```go
// 研究助手团队
crew := agent.NewCrew([]*agent.Agent{
    {Name: "Researcher", Role: "信息收集", Tools: []tools.Tool{webSearchTool, memorySearchTool}},
    {Name: "Analyst", Role: "数据分析", Tools: []tools.Tool{calculatorTool}},
    {Name: "Writer", Role: "内容创作", Tools: []tools.Tool{}},
})

result := crew.Execute(ctx, "研究Go语言性能优化技术并写一份报告")
```

**验收标准**:
- 多Agent可协作完成复杂任务
- 任务依赖关系正确处理
- Demo 可运行

---

## 阶段五：测试与文档（2-3天）

### 目标
- 完善测试覆盖
- 更新文档
- 性能优化

### 具体任务

#### Task 5.1: 集成测试
```bash
mkdir -p tests/integration
touch tests/integration/end_to_end_test.go
touch tests/integration/tools_test.go
touch tests/integration/memory_test.go
```

**测试场景**:
1. **完整对话流程**: 用户输入 → Agent响应 → 记忆提取 → 存储
2. **工具调用**: Agent 自主调用工具并返回结果
3. **多轮对话**: 会话上下文正确维护
4. **记忆检索**: 分层记忆正确组装
5. **错误处理**: 网络失败、超时等异常场景

**验收标准**:
- 所有集成测试通过
- 测试覆盖率 > 80%
- CI/CD 集成（GitHub Actions）

---

#### Task 5.2: 性能基准测试
```bash
touch tests/benchmark/llm_bench_test.go
touch tests/benchmark/memory_bench_test.go
```

**基准测试**:
```go
func BenchmarkMemoryRetrieval(b *testing.B) {
    for i := 0; i < b.N; i++ {
        mem.GetContextForPrompt(ctx, "user123", "session456")
    }
}

func BenchmarkToolExecution(b *testing.B) {
    for i := 0; i < b.N; i++ {
        tool.Execute(ctx, "2+2")
    }
}
```

**性能目标**:
- 记忆检索: < 100ms (p95)
- 工具执行: < 500ms (p95)
- 完整对话: < 3s (不含 LLM 推理)

**验收标准**:
- 基准测试建立
- 性能回归检测

---

#### Task 5.3: 文档更新
更新以下文档：
- `README.md`: 快速开始指南
- `ARCHITECTURE.md`: 架构设计文档
- `API.md`: API 参考文档
- `TOOLS.md`: 工具开发指南
- `MEMORY.md`: 记忆系统详解
- `DEPLOYMENT.md`: 部署指南

**文档要求**:
- 清晰的架构图（使用 Mermaid）
- 代码示例
- 配置参考
- 常见问题 FAQ

---

## 阶段六：生产就绪（可选，2-3天）

### 目标
- Docker 容器化
- 配置管理优化
- 监控告警

### 具体任务

#### Task 6.1: Docker 容器化
```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o chat ./cmd/chat

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/chat .
COPY config.example.yaml config.yaml
CMD ["./chat"]
```

```yaml
# docker-compose.yml
version: '3.8'
services:
  chat:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - qdrant
    environment:
      - CONFIG_PATH=/app/config.yaml
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./memory.db:/app/memory.db

  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"
      - "6334:6334"
    volumes:
      - qdrant_data:/qdrant/storage

  ollama:
    image: ollama/ollama:latest
    ports:
      - "11434:11434"
    volumes:
      - ollama_data:/root/.ollama

volumes:
  qdrant_data:
  ollama_data:
```

**验收标准**:
- Docker 镜像构建成功
- Docker Compose 一键启动
- 服务间通信正常

---

#### Task 6.2: HTTP API（可选）
```bash
mkdir -p cmd/server
touch cmd/server/main.go
touch internal/api/handlers.go
touch internal/api/middleware.go
```

**API 端点**:
```
POST /api/v1/chat
POST /api/v1/sessions
GET  /api/v1/sessions/:id
GET  /api/v1/memories/:user_id
DELETE /api/v1/memories/:user_id/:memory_id
```

**验收标准**:
- RESTful API 可用
- JWT 认证
- API 文档（Swagger）

---

#### Task 6.3: 监控告警
```bash
mkdir -p internal/metrics
touch internal/metrics/prometheus.go
```

**指标收集**:
- LLM 调用次数、延迟、错误率
- 工具调用统计
- 记忆存储/检索延迟
- 活跃用户数、会话数

**Prometheus 集成**:
```go
import "github.com/prometheus/client_golang/prometheus"

var (
    llmCallDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "llm_call_duration_seconds",
            Help: "LLM call duration",
        },
        []string{"model", "provider"},
    )
)
```

**验收标准**:
- Prometheus exporter 暴露指标
- Grafana 仪表板配置

---

## 时间估算与资源分配

| 阶段 | 预计时间 | 优先级 | 依赖 |
|------|---------|--------|------|
| 阶段一：代码重构 | 2-3天 | P0（必须） | 无 |
| 阶段二：工具系统 | 3-4天 | P0（必须） | 阶段一 |
| 阶段三：记忆优化 | 3-4天 | P1（重要） | 阶段一 |
| 阶段四：高级功能 | 4-5天 | P1（重要） | 阶段二 |
| 阶段五：测试文档 | 2-3天 | P0（必须） | 所有 |
| 阶段六：生产就绪 | 2-3天 | P2（可选） | 所有 |

**总计**: 16-22 天（约 3-4 周）

---

## 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| LLM 工具调用不准确 | 高 | 使用更强的模型（如 deepseek-chat）；优化 prompt |
| 记忆数据库迁移失败 | 高 | 编写迁移脚本并充分测试；备份原数据库 |
| 性能回归 | 中 | 建立基准测试；每次变更运行性能测试 |
| 第三方依赖不稳定 | 中 | 添加重试机制；使用稳定版本依赖 |
| 开发周期超期 | 低 | 分阶段交付；P2任务可延后 |

---

## 成功指标

1. **功能完整性**:
   - ✅ Agent 可以自主调用至少 3 个工具
   - ✅ 记忆系统支持分层查询
   - ✅ 流式响应可用
   - ✅ 会话管理可持久化

2. **性能**:
   - ✅ 记忆检索 < 100ms (p95)
   - ✅ 完整对话响应 < 3s（不含 LLM）
   - ✅ 支持并发用户 > 10

3. **代码质量**:
   - ✅ 单元测试覆盖率 > 80%
   - ✅ 所有集成测试通过
   - ✅ 无严重代码异味（通过 golangci-lint）

4. **文档完整性**:
   - ✅ 架构文档清晰
   - ✅ API 文档完整
   - ✅ 部署文档可操作

---

## 下一步行动

### 立即开始（Phase 1）:
```bash
# 1. 创建新分支
git checkout -b refactor/modular-architecture

# 2. 创建目录结构
mkdir -p internal/{agent,orchestrator,tools,session}

# 3. 定义接口
touch internal/agent/interface.go
touch internal/memory/interface.go
touch internal/rag/interface.go

# 4. 开始重构 main.go
# ... 编码 ...

# 5. 运行测试
go test ./...

# 6. 提交
git commit -m "Phase 1: Modular architecture refactoring"
```

### 与团队沟通:
- 与团队 review OPTIMIZATION_PLAN.md 和 IMPLEMENTATION_ROADMAP.md
- 确认优先级和时间表
- 分配任务

### 持续改进:
- 每完成一个阶段，进行代码审查
- 收集用户反馈，调整优先级
- 定期更新文档

---

## 参考资源

### 开源框架
- **LangChain**: https://github.com/langchain-ai/langchain
- **LangGraph**: https://github.com/langchain-ai/langgraph
- **AutoGPT**: https://github.com/Significant-Gravitas/AutoGPT
- **CrewAI**: https://github.com/joaomdmoura/crewAI
- **Semantic Kernel**: https://github.com/microsoft/semantic-kernel

### 论文与博客
- **ReAct**: https://arxiv.org/abs/2210.03629
- **Generative Agents**: https://arxiv.org/abs/2304.03442
- **MemGPT**: https://arxiv.org/abs/2310.08560
- **Tool使用**: https://openai.com/blog/function-calling-and-other-api-updates

### Go 库
- **LangChain Go**: https://github.com/tmc/langchaingo
- **Qdrant Go SDK**: https://github.com/qdrant/go-client
- **Ollama Go**: https://github.com/ollama/ollama/tree/main/api

---

**祝开发顺利！** 🚀