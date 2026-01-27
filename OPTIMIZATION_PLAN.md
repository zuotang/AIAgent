# AI Agent 系统优化方案

基于市面上成熟AI Agent框架的最佳实践，对当前项目进行架构优化。

---

## 一、架构优化：从单体对话到Agent系统

### 问题诊断
当前系统本质上是一个**增强型聊天机器人**，缺少真正的Agent能力：
- ❌ 无工具调用能力（Tool/Function Calling）
- ❌ 无任务规划与执行能力
- ❌ 无反思与自我修正机制
- ❌ 无多Agent协作能力

### 1.1 添加工具系统（Tool System）

**参考框架**: LangChain Tools, OpenAI Function Calling, Semantic Kernel Plugins

#### 实现方案

**创建 `internal/tools/` 模块**:

```go
// internal/tools/tool.go
package tools

import "context"

// Tool 定义工具接口
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, input string) (string, error)
    Schema() ToolSchema  // JSON Schema for function calling
}

// ToolSchema 定义工具的输入参数schema
type ToolSchema struct {
    Type       string                 `json:"type"`
    Properties map[string]PropertyDef `json:"properties"`
    Required   []string              `json:"required"`
}

type PropertyDef struct {
    Type        string `json:"type"`
    Description string `json:"description"`
}

// ToolRegistry 工具注册中心
type ToolRegistry struct {
    tools map[string]Tool
}

func NewRegistry() *ToolRegistry {
    return &ToolRegistry{tools: make(map[string]Tool)}
}

func (r *ToolRegistry) Register(tool Tool) {
    r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
    t, ok := r.tools[name]
    return t, ok
}

func (r *ToolRegistry) List() []Tool {
    tools := make([]Tool, 0, len(r.tools))
    for _, t := range r.tools {
        tools = append(tools, t)
    }
    return tools
}
```

**内置工具示例**:

```go
// internal/tools/memory_search.go
type MemorySearchTool struct {
    qdrant *rag.QdrantStore
    sqlite *memory.SQLiteMemory
}

func (t *MemorySearchTool) Name() string {
    return "memory_search"
}

func (t *MemorySearchTool) Description() string {
    return "Search user memories by semantic similarity"
}

func (t *MemorySearchTool) Execute(ctx context.Context, query string) (string, error) {
    docs, err := t.qdrant.SimilaritySearch(ctx, userID, query, 5)
    // ... format results
    return formattedResults, err
}

// internal/tools/calculator.go
type CalculatorTool struct{}

func (t *CalculatorTool) Name() string { return "calculator" }
func (t *CalculatorTool) Description() string {
    return "Perform mathematical calculations. Input: math expression like '2+3*4'"
}
func (t *CalculatorTool) Execute(ctx context.Context, expr string) (string, error) {
    // Use go-eval or similar library
    result, err := evaluate(expr)
    return fmt.Sprintf("%.2f", result), err
}

// internal/tools/web_search.go (可选)
type WebSearchTool struct {
    apiKey string
}
// ... 集成 Google Search API 或 Bing Search API
```

#### 集成到对话流程

```go
// cmd/chat/main.go

func runAgentLoop(cfg *config.Config, userID, input string) {
    // 1. 注册工具
    registry := tools.NewRegistry()
    registry.Register(&tools.MemorySearchTool{qdrant: store, sqlite: mem})
    registry.Register(&tools.CalculatorTool{})

    // 2. 构建带工具描述的系统提示
    systemPrompt := buildSystemPromptWithTools(registry)

    // 3. LLM 生成（可能包含工具调用）
    response, toolCalls := llmClient.ChatWithTools(ctx, messages, registry.List())

    // 4. 执行工具调用
    if len(toolCalls) > 0 {
        for _, call := range toolCalls {
            tool, ok := registry.Get(call.ToolName)
            if !ok {
                continue
            }
            result, err := tool.Execute(ctx, call.Arguments)
            // 将工具结果反馈给LLM
            messages = append(messages, ToolResultMessage(call.ID, result, err))
        }

        // 5. LLM 基于工具结果生成最终响应
        finalResponse, _ := llmClient.Chat(ctx, messages)
        fmt.Println(finalResponse)
    }
}
```

**优势**:
- ✅ Agent可以主动调用工具解决问题
- ✅ 可扩展：轻松添加新工具（天气、新闻、数据库查询等）
- ✅ 符合OpenAI Function Calling标准

---

### 1.2 任务规划与执行（Planning & Execution）

**参考框架**: LangGraph, BabyAGI, AutoGPT

#### 实现ReAct模式（Reason + Act）

```go
// internal/agent/react.go
package agent

type ReActAgent struct {
    llm         models.LLMClient
    tools       *tools.ToolRegistry
    maxIterations int
}

func (a *ReActAgent) Run(ctx context.Context, task string) (string, error) {
    thought := task
    for i := 0; i < a.maxIterations; i++ {
        // Step 1: Reasoning - LLM决定下一步行动
        action, err := a.planNextAction(ctx, thought)
        if action.Type == "finish" {
            return action.Answer, nil
        }

        // Step 2: Action - 执行工具调用
        tool, ok := a.tools.Get(action.ToolName)
        if !ok {
            thought = fmt.Sprintf("Tool %s not found. Try another approach.", action.ToolName)
            continue
        }

        observation, err := tool.Execute(ctx, action.Input)
        if err != nil {
            thought = fmt.Sprintf("Tool execution failed: %v. Try another approach.", err)
            continue
        }

        // Step 3: Observation - 整合结果，继续思考
        thought = fmt.Sprintf("Previous action: %s\nObservation: %s", action.ToolName, observation)
    }

    return "Max iterations reached", nil
}

type Action struct {
    Type     string // "tool" or "finish"
    ToolName string
    Input    string
    Answer   string
}

func (a *ReActAgent) planNextAction(ctx context.Context, thought string) (Action, error) {
    prompt := fmt.Sprintf(`You are a reasoning agent. Based on the current situation, decide:
1. If you need to use a tool, respond: {"type":"tool","tool":"tool_name","input":"..."}
2. If you have the final answer, respond: {"type":"finish","answer":"..."}

Available tools:
%s

Current situation:
%s`, a.tools.DescribeAll(), thought)

    response, err := a.llm.Chat(ctx, []models.ChatMessage{{Role: "user", Content: prompt}})
    // Parse JSON response into Action
    return parseAction(response), err
}
```

**使用场景**:
```
用户: "帮我计算一下我今年的平均睡眠时长"

Agent思考流程:
1. [Thought] 需要先查询用户的睡眠记录
2. [Action] 调用 memory_search("睡眠时长")
3. [Observation] 找到5条记录：7h, 6.5h, 8h, 7h, 6h
4. [Thought] 需要计算平均值
5. [Action] 调用 calculator("(7+6.5+8+7+6)/5")
6. [Observation] 结果: 6.9
7. [Finish] "你今年的平均睡眠时长是6.9小时"
```

---

### 1.3 多Agent协作（Multi-Agent System）

**参考框架**: CrewAI, AutoGen

#### 场景：专家角色分工

```go
// internal/agent/crew.go
package agent

type Agent struct {
    Name        string
    Role        string
    Goal        string
    LLM         models.LLMClient
    Tools       []tools.Tool
    Memory      *memory.AgentMemory
}

type Crew struct {
    agents []*Agent
    tasks  []Task
}

type Task struct {
    Description string
    AssignedTo  string  // Agent name
    Dependencies []string // Task IDs that must complete first
}

func (c *Crew) Execute(ctx context.Context) []TaskResult {
    // 按依赖关系执行任务
    results := make([]TaskResult, 0)

    for _, task := range c.tasks {
        agent := c.getAgent(task.AssignedTo)
        result := agent.ExecuteTask(ctx, task, c.getCompletedTaskResults(task.Dependencies))
        results = append(results, result)
    }

    return results
}
```

**使用示例**:
```go
// 创建一个研究团队
crew := agent.Crew{
    agents: []*agent.Agent{
        {
            Name: "Researcher",
            Role: "Information Gatherer",
            Goal: "Find accurate, up-to-date information",
            Tools: []tools.Tool{webSearchTool, memorySearchTool},
        },
        {
            Name: "Analyst",
            Role: "Data Analyzer",
            Goal: "Synthesize information and draw insights",
            Tools: []tools.Tool{calculatorTool, chartGeneratorTool},
        },
        {
            Name: "Writer",
            Role: "Content Creator",
            Goal: "Write clear, engaging summaries",
            Tools: []tools.Tool{},
        },
    },
    tasks: []agent.Task{
        {Description: "Research topic X", AssignedTo: "Researcher"},
        {Description: "Analyze research findings", AssignedTo: "Analyst", Dependencies: []string{"task-1"}},
        {Description: "Write final report", AssignedTo: "Writer", Dependencies: []string{"task-2"}},
    },
}

results := crew.Execute(ctx)
```

---

## 二、记忆系统优化

### 问题诊断
当前记忆系统缺少：
- ❌ 工作记忆（Working Memory）概念不清晰
- ❌ 记忆遗忘机制（无法控制记忆增长）
- ❌ 知识图谱（实体关系）
- ❌ 记忆重要性评分与优先级

### 2.1 分层记忆架构

**参考框架**: MemGPT, Generative Agents (Stanford)

```
┌─────────────────────────────────────┐
│       工作记忆 (Working Memory)      │  当前对话上下文 (8轮)
│  - 立即可访问                        │  SQLite: conversation_buffer
│  - 容量限制                          │
└─────────────────────────────────────┘
              ↓ 提取重要信息
┌─────────────────────────────────────┐
│       短期记忆 (Short-term)         │  最近7天的记忆
│  - 最近会话                          │  SQLite: 带 created_at 索引
│  - 高访问频率                        │  Qdrant: 带 timestamp filter
└─────────────────────────────────────┘
              ↓ 整合 & 提炼
┌─────────────────────────────────────┐
│       长期记忆 (Long-term)          │  超过7天，高重要性
│  - 稳定的事实                        │  SQLite: importance_score > 0.8
│  - 核心偏好                          │  Qdrant: 定期重新向量化
└─────────────────────────────────────┘
```

#### 数据库Schema更新

```sql
-- 添加时间维度和重要性评分
ALTER TABLE memories ADD COLUMN created_at DATETIME DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE memories ADD COLUMN last_accessed_at DATETIME DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE memories ADD COLUMN access_count INTEGER DEFAULT 0;
ALTER TABLE memories ADD COLUMN importance_score REAL DEFAULT 0.5;

-- 创建会话缓冲表（工作记忆）
CREATE TABLE conversation_buffer (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,  -- "user" or "assistant"
    content TEXT NOT NULL,
    turn_index INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_buffer_session ON conversation_buffer(user_id, session_id, turn_index);
```

#### 实现记忆分层查询

```go
// internal/memory/layered.go
package memory

type LayeredMemory struct {
    sqlite *SQLiteMemory
    qdrant *rag.QdrantStore
}

// GetContextForPrompt 根据记忆层级构建上下文
func (m *LayeredMemory) GetContextForPrompt(ctx context.Context, userID, sessionID string) (string, error) {
    var parts []string

    // 1. 工作记忆：当前会话的最后8轮对话
    workingMemory := m.getWorkingMemory(ctx, userID, sessionID, 8)
    parts = append(parts, "## Current Conversation:\n"+workingMemory)

    // 2. 短期记忆：最近7天的重要记忆
    shortTerm := m.getShortTermMemory(ctx, userID, 7*24*time.Hour)
    if shortTerm != "" {
        parts = append(parts, "## Recent Memories (last 7 days):\n"+shortTerm)
    }

    // 3. 长期记忆：高重要性记忆 + 语义检索
    longTerm := m.getLongTermMemory(ctx, userID, currentQuery, 0.8)
    if longTerm != "" {
        parts = append(parts, "## Core Knowledge:\n"+longTerm)
    }

    return strings.Join(parts, "\n\n"), nil
}

func (m *LayeredMemory) getShortTermMemory(ctx context.Context, userID string, within time.Duration) string {
    query := `
        SELECT mtype, mkey, mvalue, importance_score
        FROM memories
        WHERE user_id = ? AND created_at > datetime('now', ?)
        ORDER BY importance_score DESC, created_at DESC
        LIMIT 20
    `
    rows, _ := m.sqlite.db.QueryContext(ctx, query, userID, fmt.Sprintf("-%d seconds", int(within.Seconds())))
    // ... 格式化结果
}

func (m *LayeredMemory) getLongTermMemory(ctx context.Context, userID, query string, minImportance float64) string {
    // 结合结构化查询和语义搜索
    structured := m.sqlite.GetHighImportanceMemories(ctx, userID, minImportance)
    semantic := m.qdrant.SimilaritySearch(ctx, userID, query, 5)
    // ... 合并去重
}
```

---

### 2.2 记忆重要性评分与遗忘

**参考**: Ebbinghaus遗忘曲线 + LLM评分

```go
// internal/memory/importance.go

// CalculateImportance 使用LLM评估记忆重要性
func CalculateImportance(ctx context.Context, llm models.LLMClient, memory ExtractedMemory) (float64, error) {
    prompt := fmt.Sprintf(`Rate the importance of this memory for future conversations (0-1 scale):

Memory: %s = %s (type: %s, owner: %s)

Consider:
- Is this core identity information? (high importance)
- Is this a stable preference? (high importance)
- Is this temporary/contextual? (low importance)
- How likely is this needed in future conversations?

Respond with ONLY a number between 0 and 1.`, memory.Key, memory.Value, memory.Type, memory.Owner)

    response, err := llm.Chat(ctx, []models.ChatMessage{{Role: "user", Content: prompt}})
    score, _ := strconv.ParseFloat(strings.TrimSpace(response), 64)

    if score < 0 {
        score = 0
    } else if score > 1 {
        score = 1
    }

    return score, err
}

// DecayImportance 随时间衰减重要性（遗忘曲线）
func DecayImportance(originalScore float64, daysSinceLastAccess int) float64 {
    // 使用指数衰减：score * e^(-decay_rate * days)
    decayRate := 0.05  // 每20天衰减到原来的1/e
    return originalScore * math.Exp(-decayRate*float64(daysSinceLastAccess))
}

// PruneOldMemories 定期清理低重要性的旧记忆
func (m *SQLiteMemory) PruneOldMemories(ctx context.Context, userID string) error {
    // 删除：超过30天 && 访问次数<3 && 重要性<0.3
    query := `
        DELETE FROM memories
        WHERE user_id = ?
          AND created_at < datetime('now', '-30 days')
          AND access_count < 3
          AND importance_score < 0.3
    `
    _, err := m.db.ExecContext(ctx, query, userID)
    return err
}
```

#### 在存储时计算重要性

```go
// cmd/chat/main.go - 修改内存提取流程

// 提取记忆后，计算重要性评分
for i := range extracted {
    score, err := memory.CalculateImportance(ctx, llmClient, extracted[i])
    if err == nil {
        extracted[i].Importance = score
    } else {
        extracted[i].Importance = 0.5  // 默认中等重要性
    }
}

// 只存储重要性 >= 0.3 的记忆
filtered := filterByImportance(extracted, 0.3)
```

---

### 2.3 知识图谱集成（可选高级功能）

**参考框架**: Neo4j + LangChain Graph Memory

```
当前: 扁平化键值对存储
  name = "Alice"
  favorite_color = "blue"
  job = "engineer"

知识图谱: 实体-关系图
  (Alice)-[:HAS_PREFERENCE]->(blue)
  (Alice)-[:WORKS_AS]->(Engineer)
  (Alice)-[:LIVES_IN]->(San Francisco)
  (Alice)-[:KNOWS]->(Bob)
```

**实现思路** (使用轻量级方案):

```go
// internal/memory/graph.go
package memory

type Entity struct {
    ID   string
    Type string  // "person", "place", "concept"
    Name string
}

type Relation struct {
    From       string  // Entity ID
    To         string  // Entity ID
    Type       string  // "likes", "works_at", "located_in"
    Confidence float64
}

type MemoryGraph struct {
    entities  map[string]Entity
    relations []Relation
}

// 从对话中提取实体和关系
func ExtractGraphMemory(ctx context.Context, llm models.LLMClient, conversation string) (*MemoryGraph, error) {
    prompt := `Extract entities and relationships from this conversation in JSON format:

Conversation:
%s

Output format:
{
  "entities": [{"id": "e1", "type": "person", "name": "Alice"}],
  "relations": [{"from": "e1", "to": "e2", "type": "likes", "confidence": 0.9}]
}
`
    // ... LLM提取 + JSON解析
}

// 查询路径：Alice喜欢什么？
func (g *MemoryGraph) Query(entityID, relationType string) []Entity {
    // ... 图遍历
}
```

**存储方案**:
- 轻量级：SQLite存储图结构（entities表 + relations表）
- 高级：集成Neo4j或其他图数据库

---

## 三、代码组织优化

### 问题诊断
- ❌ `cmd/chat/main.go` 超过850行，职责过重
- ❌ 缺少清晰的分层架构
- ❌ 业务逻辑和基础设施代码混杂

### 3.1 模块化重构

#### 目标架构

```
cmd/chat/
  └── main.go                    # 入口 + CLI解析 (50行)

internal/
  ├── agent/                     # Agent核心逻辑
  │   ├── agent.go               # Agent接口定义
  │   ├── react.go               # ReAct实现
  │   └── conversational.go      # 对话式Agent
  │
  ├── tools/                     # 工具系统
  │   ├── registry.go            # 工具注册
  │   ├── memory_search.go
  │   ├── calculator.go
  │   └── web_search.go
  │
  ├── memory/                    # 记忆系统
  │   ├── interface.go           # Memory接口
  │   ├── sqlite.go              # SQLite实现
  │   ├── layered.go             # 分层记忆
  │   ├── importance.go          # 重要性评分
  │   └── graph.go               # 知识图谱（可选）
  │
  ├── models/                    # LLM客户端
  │   ├── interface.go           # LLMClient接口
  │   ├── ollama.go
  │   └── deepseek.go
  │
  ├── rag/                       # 向量存储
  │   ├── interface.go           # VectorStore接口
  │   └── qdrant.go
  │
  ├── orchestrator/              # 编排层（新增）
  │   ├── conversation.go        # 对话流程编排
  │   ├── memory_pipeline.go     # 记忆提取流程
  │   └── context_builder.go     # 上下文构建
  │
  └── config/
      └── config.go
```

#### 重构步骤

**Step 1: 抽取Agent接口**

```go
// internal/agent/agent.go
package agent

type Agent interface {
    // Run 执行Agent任务
    Run(ctx context.Context, input AgentInput) (AgentOutput, error)

    // AddTool 添加工具
    AddTool(tool tools.Tool)

    // SetMemory 设置记忆系统
    SetMemory(mem memory.Memory)
}

type AgentInput struct {
    UserID    string
    SessionID string
    Message   string
    Context   map[string]interface{}
}

type AgentOutput struct {
    Response     string
    ToolCalls    []tools.ToolCall
    ThoughtTrace []string  // 思考过程
    Citations    []string  // 引用来源
}
```

**Step 2: 实现对话式Agent**

```go
// internal/agent/conversational.go
package agent

type ConversationalAgent struct {
    llm         models.LLMClient
    memory      memory.Memory
    tools       *tools.ToolRegistry
    config      AgentConfig
}

func NewConversationalAgent(llm models.LLMClient, cfg AgentConfig) *ConversationalAgent {
    return &ConversationalAgent{
        llm:    llm,
        tools:  tools.NewRegistry(),
        config: cfg,
    }
}

func (a *ConversationalAgent) Run(ctx context.Context, input AgentInput) (AgentOutput, error) {
    // 1. 构建上下文
    context := a.memory.GetContextForPrompt(ctx, input.UserID, input.SessionID)

    // 2. 构建消息
    messages := a.buildMessages(context, input.Message)

    // 3. LLM生成
    response, err := a.llm.Chat(ctx, messages)
    if err != nil {
        return AgentOutput{}, err
    }

    // 4. 提取记忆
    go a.extractAndStoreMemories(ctx, input, response)

    return AgentOutput{Response: response}, nil
}
```

**Step 3: 创建编排层**

```go
// internal/orchestrator/conversation.go
package orchestrator

type ConversationOrchestrator struct {
    agent  agent.Agent
    memory memory.Memory
}

func (o *ConversationOrchestrator) HandleUserInput(ctx context.Context, userID, input string) (string, error) {
    // 1. 准备输入
    agentInput := agent.AgentInput{
        UserID:    userID,
        SessionID: generateSessionID(),
        Message:   input,
    }

    // 2. 执行Agent
    output, err := o.agent.Run(ctx, agentInput)
    if err != nil {
        return "", err
    }

    // 3. 存储对话历史
    o.memory.SaveConversationTurn(ctx, userID, input, output.Response)

    return output.Response, nil
}
```

**Step 4: 简化main.go**

```go
// cmd/chat/main.go
package main

func main() {
    // 1. 加载配置
    cfg := config.Load("config.yaml")

    // 2. 初始化组件
    llm := initLLM(cfg)
    mem := initMemory(cfg)
    store := initVectorStore(cfg)

    // 3. 创建Agent
    agent := agent.NewConversationalAgent(llm, agent.AgentConfig{
        MaxTokens: 2048,
        Temperature: 0.7,
    })
    agent.SetMemory(mem)

    // 注册工具
    agent.AddTool(&tools.MemorySearchTool{Qdrant: store})
    agent.AddTool(&tools.CalculatorTool{})

    // 4. 创建编排器
    orchestrator := orchestrator.NewConversationOrchestrator(agent, mem)

    // 5. 启动交互循环
    runInteractiveLoop(orchestrator)
}

func runInteractiveLoop(orch *orchestrator.ConversationOrchestrator) {
    scanner := bufio.NewScanner(os.Stdin)
    for {
        fmt.Print("You: ")
        if !scanner.Scan() {
            break
        }

        input := scanner.Text()
        response, err := orch.HandleUserInput(context.Background(), "user123", input)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            continue
        }

        fmt.Printf("Assistant: %s\n\n", response)
    }
}
```

---

### 3.2 接口驱动设计

**统一所有外部依赖为接口**:

```go
// internal/memory/interface.go
package memory

type Memory interface {
    // 获取上下文
    GetContextForPrompt(ctx context.Context, userID, sessionID string) (string, error)

    // 存储记忆
    StoreMemories(ctx context.Context, userID string, memories []ExtractedMemory) error

    // 查询记忆
    SearchMemories(ctx context.Context, userID, query string, limit int) ([]ExtractedMemory, error)

    // 会话管理
    SaveConversationTurn(ctx context.Context, userID, userMsg, assistantMsg string) error
    GetConversationHistory(ctx context.Context, userID, sessionID string, limit int) ([]ConversationTurn, error)
}

// internal/rag/interface.go
package rag

type VectorStore interface {
    UpsertEmbeddings(ctx context.Context, userID string, docs []Document) error
    SimilaritySearch(ctx context.Context, userID, query string, topK int) ([]Document, error)
    DeleteByUserID(ctx context.Context, userID string) error
}
```

**好处**:
- ✅ 易于单元测试（mock接口）
- ✅ 可替换实现（SQLite → PostgreSQL; Qdrant → Pinecone）
- ✅ 清晰的依赖边界

---

## 四、功能增强

### 4.1 流式响应（Streaming）

**参考**: OpenAI Streaming API

```go
// internal/models/interface.go
type LLMClient interface {
    Chat(ctx context.Context, msgs []ChatMessage, model ...string) (string, error)

    // 新增流式接口
    ChatStream(ctx context.Context, msgs []ChatMessage, model ...string) (<-chan string, error)
}

// internal/models/ollama.go
func (c *OllamaClient) ChatStream(ctx context.Context, msgs []ChatMessage, model ...string) (<-chan string, error) {
    stream := make(chan string, 100)

    req := &ChatRequest{
        Model:    c.selectModel(model),
        Messages: msgs,
        Stream:   true,  // 启用流式模式
    }

    resp, err := c.doRequest(ctx, "/api/chat", req)
    if err != nil {
        close(stream)
        return stream, err
    }

    go func() {
        defer close(stream)
        defer resp.Body.Close()

        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            var chunk ChatResponse
            json.Unmarshal(scanner.Bytes(), &chunk)

            if chunk.Message.Content != "" {
                stream <- chunk.Message.Content
            }

            if chunk.Done {
                break
            }
        }
    }()

    return stream, nil
}
```

**使用示例**:

```go
stream, err := llm.ChatStream(ctx, messages)
if err != nil {
    log.Fatal(err)
}

fmt.Print("Assistant: ")
for token := range stream {
    fmt.Print(token)  // 实时打印
}
fmt.Println()
```

---

### 4.2 并发控制与资源管理

**问题**: 当前代码缺少：
- 并发请求限流
- 超时控制
- 资源池管理

```go
// internal/ratelimit/limiter.go
package ratelimit

import (
    "context"
    "golang.org/x/time/rate"
)

type RateLimiter struct {
    limiter *rate.Limiter
}

func NewRateLimiter(rps float64) *RateLimiter {
    return &RateLimiter{
        limiter: rate.NewLimiter(rate.Limit(rps), int(rps)),
    }
}

func (r *RateLimiter) Wait(ctx context.Context) error {
    return r.limiter.Wait(ctx)
}

// internal/models/ollama.go - 添加限流
type OllamaClient struct {
    // ...
    limiter *ratelimit.RateLimiter
}

func (c *OllamaClient) Chat(ctx context.Context, msgs []ChatMessage, model ...string) (string, error) {
    // 等待限流许可
    if err := c.limiter.Wait(ctx); err != nil {
        return "", err
    }

    // ... 原有逻辑
}
```

---

### 4.3 可观测性（Observability）

**参考**: LangSmith, Phoenix Tracing

```go
// internal/telemetry/tracer.go
package telemetry

import (
    "context"
    "time"
)

type Span struct {
    Name      string
    StartTime time.Time
    EndTime   time.Time
    Metadata  map[string]interface{}
    Parent    *Span
}

type Tracer struct {
    spans []*Span
}

func (t *Tracer) StartSpan(ctx context.Context, name string) (*Span, context.Context) {
    span := &Span{
        Name:      name,
        StartTime: time.Now(),
        Metadata:  make(map[string]interface{}),
    }

    t.spans = append(t.spans, span)

    // 将span存入context
    ctx = context.WithValue(ctx, "current_span", span)
    return span, ctx
}

func (t *Tracer) EndSpan(span *Span) {
    span.EndTime = time.Now()
}

// 使用示例
span, ctx := tracer.StartSpan(ctx, "llm.chat")
defer tracer.EndSpan(span)

span.Metadata["model"] = "gemma3:12b"
span.Metadata["input_tokens"] = 150
response, err := llm.Chat(ctx, messages)
span.Metadata["output_tokens"] = 420
```

**输出trace日志**:

```
[TRACE] conversation.run (2.3s)
  ├─ memory.get_context (0.05s)
  │  ├─ sqlite.query (0.02s)
  │  └─ qdrant.search (0.03s)
  ├─ llm.chat (2.1s)
  │  └─ ollama.api_call (2.0s)
  └─ memory.extract (0.15s)
     └─ llm.chat (0.1s)
```

---

## 五、用户体验优化

### 5.1 会话管理

**问题**: 当前缺少session概念，每次启动都是新会话

```go
// internal/session/manager.go
package session

type Session struct {
    ID        string
    UserID    string
    StartTime time.Time
    LastActive time.Time
    Metadata  map[string]string
}

type SessionManager struct {
    store map[string]*Session
}

func (m *SessionManager) CreateSession(userID string) *Session {
    sess := &Session{
        ID:         uuid.New().String(),
        UserID:     userID,
        StartTime:  time.Now(),
        LastActive: time.Now(),
        Metadata:   make(map[string]string),
    }
    m.store[sess.ID] = sess
    return sess
}

func (m *SessionManager) GetSession(sessionID string) (*Session, bool) {
    sess, ok := m.store[sessionID]
    if ok {
        sess.LastActive = time.Now()
    }
    return sess, ok
}

// 自动清理过期会话
func (m *SessionManager) CleanupStale(maxIdleTime time.Duration) {
    now := time.Now()
    for id, sess := range m.store {
        if now.Sub(sess.LastActive) > maxIdleTime {
            delete(m.store, id)
        }
    }
}
```

**CLI支持会话恢复**:

```bash
# 创建新会话
$ ./chat.exe --new-session

# 恢复会话
$ ./chat.exe --session abc123

# 列出所有会话
$ ./chat.exe --list-sessions
Session abc123: Started 2h ago, last active 10m ago
Session def456: Started 1d ago, last active 3h ago
```

---

### 5.2 人类反馈循环（Human-in-the-loop）

**参考**: RLHF, Constitutional AI

```go
// internal/feedback/collector.go
package feedback

type FeedbackType string

const (
    ThumbsUp   FeedbackType = "thumbs_up"
    ThumbsDown FeedbackType = "thumbs_down"
    Report     FeedbackType = "report"
)

type Feedback struct {
    MessageID  string
    UserID     string
    Type       FeedbackType
    Comment    string
    Timestamp  time.Time
}

type FeedbackCollector struct {
    db *sql.DB
}

func (c *FeedbackCollector) Record(feedback Feedback) error {
    query := `
        INSERT INTO feedback (message_id, user_id, type, comment, timestamp)
        VALUES (?, ?, ?, ?, ?)
    `
    _, err := c.db.Exec(query, feedback.MessageID, feedback.UserID, feedback.Type, feedback.Comment, feedback.Timestamp)
    return err
}
```

**交互示例**:

```
Assistant: 你今年的平均睡眠时长是6.9小时

[👍 Helpful] [👎 Not Helpful] [🚩 Report Issue]

You: 👎