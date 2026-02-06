# 工作流系统 - 快速开始指南

## 🚀 5 分钟快速开始

### 1. 测试现有功能

```bash
# 运行测试
cd internal/workflow/engine
go test -v

# 应该看到：
# ✅ TestExecutor_ValidWorkflow
# ✅ TestExecutor_PortTypeMismatch
# ✅ TestExecutor_NodeError
# PASS
```

### 2. 运行示例程序

```bash
# 编译示例
cd cmd/workflow_example
go build

# 运行（需要 Ollama 运行在 localhost:11434）
./workflow_example
```

### 3. 查看可用节点

当前系统有 **14 个内置节点**：

| 类别 | 节点 | 说明 |
|------|------|------|
| LLM | LLM.Generate | LLM 生成 |
| LLM | LLM.JSON | LLM JSON 输出 |
| Context | Context.Pack | 打包上下文 |
| Context | Context.Compress | 压缩上下文 |
| Tool | Tool.Time.Now | 获取当前时间 |
| Tool | Tool.Calc | 计算器 |
| IO | Input.Text | 文本输入 |
| IO | Output.Text | 文本输出 |
| IO | Input.JSON | JSON 输入 |
| IO | Output.JSON | JSON 输出 |
| Transform | Transform.TextToMessages | 文本转消息 |
| Transform | Transform.MessagesToText | 消息转文本 |
| Transform | Transform.JSONToText | JSON 转文本 |
| Transform | Transform.TextToJSON | 文本转 JSON |

---

## 📝 创建你的第一个工作流

### 示例：简单的 LLM 对话

```json
{
  "version": "1.0",
  "meta": {
    "id": "my-first-workflow",
    "name": "My First Workflow"
  },
  "nodes": {
    "input": {
      "id": "input",
      "type": "Input.Text",
      "version": "1.0",
      "params": {
        "text": "Hello, how are you?"
      }
    },
    "transform": {
      "id": "transform",
      "type": "Transform.TextToMessages",
      "version": "1.0",
      "params": {}
    },
    "llm": {
      "id": "llm",
      "type": "LLM.Generate",
      "version": "1.0",
      "params": {
        "model": "qwen2.5:7b"
      }
    },
    "output": {
      "id": "output",
      "type": "Output.Text",
      "version": "1.0",
      "params": {}
    }
  },
  "edges": [
    {
      "id": "e1",
      "from": {"node": "input", "port": "text"},
      "to": {"node": "transform", "port": "text"},
      "type": "data"
    },
    {
      "id": "e2",
      "from": {"node": "transform", "port": "messages"},
      "to": {"node": "llm", "port": "messages"},
      "type": "data"
    },
    {
      "id": "e3",
      "from": {"node": "llm", "port": "messages"},
      "to": {"node": "output", "port": "messages"},
      "type": "data"
    }
  ]
}
```

保存为 `my_workflow.json`，然后运行：

```go
package main

import (
    "context"
    "fmt"
    "log"

    "agent-langchain/internal/models"
    "agent-langchain/internal/workflow/dsl"
    "agent-langchain/internal/workflow/engine"
    "agent-langchain/internal/workflow/nodes"
    "agent-langchain/internal/workflow/registry"
)

func main() {
    // 1. 创建注册中心
    reg := registry.NewRegistry()
    nodes.RegisterBuiltinNodes(reg)

    // 2. 加载工作流
    wf, err := dsl.Load("my_workflow.json")
    if err != nil {
        log.Fatal(err)
    }

    // 3. 创建执行器
    executor := engine.NewExecutor(reg)

    // 4. 创建运行时上下文
    llmClient := models.New("http://localhost:11434", "qwen2.5:7b", "")
    rc := &registry.RunContext{
        LLMClient: registry.NewLLMClientAdapter(llmClient),
        Cache:     make(map[string]any),
    }

    // 5. 执行
    trace, err := executor.Execute(context.Background(), wf, rc)
    if err != nil {
        log.Fatal(err)
    }

    // 6. 获取输出
    if output, ok := rc.Cache["output"].(string); ok {
        fmt.Println("输出:", output)
    }

    fmt.Printf("状态: %s, 耗时: %v\n", trace.Status, trace.Duration)
}
```

---

## 🔧 下一步：实现 API 层

### 需要实现的文件

创建 `internal/api/workflow.go`：

```go
package api

import (
    "github.com/labstack/echo/v4"
    "agent-langchain/internal/workflow/registry"
    "agent-langchain/internal/workflow/engine"
    "agent-langchain/internal/models"
)

type WorkflowAPI struct {
    registry *registry.Registry
    executor *engine.Executor
    llmClient models.LLMClient
}

func NewWorkflowAPI(llmClient models.LLMClient) *WorkflowAPI {
    reg := registry.NewRegistry()
    nodes.RegisterBuiltinNodes(reg)

    return &WorkflowAPI{
        registry:  reg,
        executor:  engine.NewExecutor(reg),
        llmClient: llmClient,
    }
}

func (api *WorkflowAPI) RegisterRoutes(e *echo.Echo) {
    g := e.Group("/api/workflow")

    g.GET("/nodes", api.GetNodes)
    g.POST("/validate", api.ValidateWorkflow)
    g.POST("/execute", api.ExecuteWorkflow)
    g.GET("/trace/:id", api.GetTrace)
}

// TODO: 实现各个方法
```

### 在 main.go 中注册

```go
func main() {
    e := echo.New()

    // 创建 LLM 客户端
    llmClient := models.New("http://localhost:11434", "qwen2.5:7b", "")

    // 注册工作流 API
    workflowAPI := api.NewWorkflowAPI(llmClient)
    workflowAPI.RegisterRoutes(e)

    e.Start(":8080")
}
```

---

## 📚 更多资源

- **完整分析报告**: `WORKFLOW_ANALYSIS_REPORT.md`
- **前后端集成**: `WORKFLOW_FRONTEND_INTEGRATION.md`
- **实现总结**: `WORKFLOW_IMPLEMENTATION.md`
- **使用文档**: `internal/workflow/README.md`

---

## ❓ 常见问题

### Q: 如何添加自定义节点？

```go
type MyCustomNode struct{}

func (n *MyCustomNode) Run(ctx context.Context, rc *registry.RunContext,
    inputs map[string]any, params map[string]any) (map[string]any, error) {
    // 你的逻辑
    return map[string]any{"output": "result"}, nil
}

func (n *MyCustomNode) Spec() *registry.NodeSpec {
    return &registry.NodeSpec{
        Type:    "Custom.MyNode",
        Version: "1.0",
        Inputs:  []types.PortSpec{...},
        Outputs: []types.PortSpec{...},
        Runner:  n,
    }
}

// 注册
reg.Register((&MyCustomNode{}).Spec())
```

### Q: 如何调试工作流？

```go
// 查看执行追踪
for nodeID, nodeTrace := range trace.Nodes {
    fmt.Printf("Node %s:\n", nodeID)
    fmt.Printf("  Status: %s\n", nodeTrace.Status)
    fmt.Printf("  Duration: %v\n", nodeTrace.Duration)
    fmt.Printf("  Inputs: %v\n", nodeTrace.Inputs)
    fmt.Printf("  Outputs: %v\n", nodeTrace.Outputs)
    if nodeTrace.Error != "" {
        fmt.Printf("  Error: %s\n", nodeTrace.Error)
    }
}
```

### Q: 如何处理长时间运行的工作流？

使用异步执行：

```go
// 异步执行
go func() {
    trace, err := executor.Execute(ctx, wf, rc)
    // 保存 trace 到数据库
}()
```

---

## 🎯 下一步计划

1. ✅ 核心引擎 - 已完成
2. ✅ 基础节点 - 已完成（14 个）
3. ⏳ API 层 - 待实现
4. ⏳ 前端集成 - 待实现
5. ⏳ 工作流持久化 - 待实现

**预计完成时间**: 4-6 周

---

祝你开发顺利！🚀
