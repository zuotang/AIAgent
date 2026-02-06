# Workflow DSL 执行引擎

ComfyUI 风格的工作流 DSL 执行引擎，支持声明式工作流定义、节点注册、拓扑排序执行、类型校验和执行追踪。

## 目录结构

```
internal/workflow/
├── types/          # 端口类型和通用结构
│   └── types.go
├── dsl/            # JSON 结构和加载/校验
│   ├── workflow.go
│   └── validate.go
├── registry/       # 节点规范注册中心
│   └── registry.go
├── engine/         # 执行器和追踪
│   ├── executor.go
│   ├── trace.go
│   └── executor_test.go
└── nodes/          # 内置节点实现
    ├── register.go
    ├── llm/
    │   ├── generate.go
    │   └── json.go
    ├── context/
    │   ├── pack.go
    │   └── compress.go
    └── tool/
        └── tools.go
```

## 工作流 JSON 格式

```json
{
  "version": "1.0",
  "meta": {
    "id": "wf-001",
    "name": "demo"
  },
  "nodes": {
    "node-1": {
      "id": "node-1",
      "type": "LLM.Generate",
      "version": "1.0",
      "params": {}
    },
    "node-2": {
      "id": "node-2",
      "type": "Memory.Extract",
      "version": "1.0",
      "params": {}
    }
  },
  "edges": [
    {
      "id": "e1",
      "from": {
        "node": "node-1",
        "port": "messages"
      },
      "to": {
        "node": "node-2",
        "port": "messages"
      },
      "type": "data"
    }
  ],
  "ui": {
    "nodes": {
      "node-1": {
        "x": 100,
        "y": 200
      }
    }
  }
}
```

## 端口类型系统

支持以下端口类型：

- `messages` - LLM 消息列表
- `text` - 纯文本
- `json` - JSON 数据
- `embedding` - 向量嵌入
- `vector_query` - 向量查询请求
- `vector_results` - 向量查询结果
- `memory_items` - 记忆项列表
- `kb_docs` - 知识库文档
- `tool_call` - 工具调用请求
- `tool_result` - 工具执行结果
- `context_pack` - 打包的上下文
- `llm_config` - LLM 配置
- `flow` - 控制流信号

## 内置节点

### LLM 相关
- `LLM.Generate` - LLM 生成
- `LLM.JSON` - LLM JSON 输出

### Context 相关
- `Context.Pack` - 打包上下文
- `Context.Compress` - 压缩上下文

### Tool 相关
- `Tool.Time.Now` - 获取当前时间
- `Tool.Calc` - 计算器

### TODO 节点
- `Memory.Extract` - 提取记忆
- `Memory.Longterm.Get/Put` - 长期记忆读写
- `Embedding.Encode` - 编码向量
- `Vector.Query.Build` - 构建向量查询
- `Memory.Vector.Query/Upsert` - 向量记忆查询/插入
- `KB.Query` - 知识库查询
- `Tool.Dispatch` - 工具分发
- `Tool.Execute` - 工具执行

## 使用示例

### 1. 创建注册中心并注册节点

```go
import (
    "agent-langchain/internal/workflow/registry"
    "agent-langchain/internal/workflow/nodes"
)

// 创建注册中心
reg := registry.NewRegistry()

// 注册内置节点
if err := nodes.RegisterBuiltinNodes(reg); err != nil {
    log.Fatal(err)
}
```

### 2. 加载工作流

```go
import "agent-langchain/internal/workflow/dsl"

// 从文件加载
wf, err := dsl.Load("workflow.json")
if err != nil {
    log.Fatal(err)
}

// 或从 JSON 字节解析
wf, err := dsl.Unmarshal(jsonData)
if err != nil {
    log.Fatal(err)
}
```

### 3. 创建执行器并执行

```go
import (
    "context"
    "agent-langchain/internal/workflow/engine"
    "agent-langchain/internal/models"
)

// 创建执行器
executor := engine.NewExecutor(reg)

// 创建运行时上下文（注入依赖）
rc := &registry.RunContext{
    LLMClient:    models.New("http://localhost:11434", "qwen2.5:7b", ""),
    EmbedClient:  nil, // TODO: 注入 Embedding 客户端
    MemoryStore:  nil, // TODO: 注入 Memory 存储
    QdrantClient: nil, // TODO: 注入 Qdrant 客户端
    ToolRegistry: nil, // TODO: 注入 Tool 注册表
}

// 执行工作流
trace, err := executor.Execute(context.Background(), wf, rc)
if err != nil {
    log.Printf("Execution failed: %v", err)
}

// 查看执行结果
fmt.Printf("Workflow Status: %s\n", trace.Status)
fmt.Printf("Duration: %v\n", trace.Duration)

for nodeID, nodeTrace := range trace.Nodes {
    fmt.Printf("Node %s: %s (took %v)\n",
        nodeID, nodeTrace.Status, nodeTrace.Duration)
    if nodeTrace.Error != "" {
        fmt.Printf("  Error: %s\n", nodeTrace.Error)
    }
}
```

### 4. 自定义节点

```go
import (
    "context"
    "agent-langchain/internal/workflow/registry"
    "agent-langchain/internal/workflow/types"
)

// 定义自定义节点
type MyCustomNode struct{}

// 实现 NodeRunner 接口
func (n *MyCustomNode) Run(ctx context.Context, rc *registry.RunContext,
    inputs map[string]any, params map[string]any) (map[string]any, error) {

    // 从输入获取数据
    text, _ := inputs["text"].(string)

    // 执行自定义逻辑
    result := "Processed: " + text

    // 返回输出
    return map[string]any{
        "text": result,
    }, nil
}

// 定义节点规范
func (n *MyCustomNode) Spec() *registry.NodeSpec {
    return &registry.NodeSpec{
        Type:    "Custom.MyNode",
        Version: "1.0",
        Inputs: []types.PortSpec{
            {Name: "text", Type: types.PortTypeText, Required: true},
        },
        Outputs: []types.PortSpec{
            {Name: "text", Type: types.PortTypeText, Required: true},
        },
        Runner: n,
    }
}

// 注册自定义节点
reg.Register((&MyCustomNode{}).Spec())
```

## 特性

### 1. 类型安全
- 端口类型严格校验
- 连接时自动检查类型匹配

### 2. 拓扑排序
- 自动按依赖关系排序执行
- 检测循环依赖

### 3. 错误处理
- Fail-fast 模式（默认）
- 详细的错误追踪

### 4. Context 支持
- 支持 context.Context 取消
- 超时控制

### 5. 执行追踪
- 记录每个节点的输入/输出
- 记录执行时间和状态
- 错误信息追踪

## 测试

运行测试：

```bash
cd internal/workflow/engine
go test -v
```

测试覆盖：
- ✅ 合法工作流执行
- ✅ 端口类型不匹配校验
- ✅ 节点错误处理和追踪

## TODO

1. 实现剩余内置节点（Memory, Vector, KB, Tool 等）
2. 添加缓存机制
3. 支持 flow 边控制流
4. 添加节点版本迁移支持
5. 实现 ParamsSchema 校验
6. 添加更多测试用例
7. 性能优化（并行执行独立节点）
8. 添加工作流可视化导出

## 架构设计

### 依赖注入
所有外部依赖（LLM, Memory, Qdrant, Tools）通过 `RunContext` 注入，保持代码可测试性。

### 接口抽象
- `NodeRunner` - 节点执行接口
- `LLMClient` - LLM 客户端接口
- `EmbedClient` - Embedding 客户端接口
- 其他依赖接口

### 可扩展性
- 通过 Registry 注册自定义节点
- 支持节点版本管理
- 灵活的端口类型系统

## 许可

与主项目相同
