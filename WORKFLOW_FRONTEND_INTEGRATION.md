# AI Agent 工作流系统 - 完整分析与前后端集成方案

## 一、当前系统分析

### ✅ 已实现的核心功能
1. **工作流执行引擎** - 拓扑排序、类型校验、执行追踪
2. **节点系统** - 6 个内置节点（LLM、Context、Tool）
3. **类型系统** - 13 种端口类型，严格类型校验
4. **测试** - 3 个单元测试全部通过

### ❌ 缺失的关键部分

#### 1. API 层（最关键）
**问题**: 前端无法与后端交互

**需要的 API**:
```
GET  /api/workflow/nodes          # 获取所有可用节点
GET  /api/workflow/nodes/:type    # 获取节点详情
POST /api/workflow/validate       # 校验工作流
POST /api/workflow/execute        # 执行工作流（同步/异步）
GET  /api/workflow/trace/:id      # 获取执行追踪
POST /api/workflow/save           # 保存工作流
GET  /api/workflow/list           # 列出工作流
GET  /api/workflow/:id            # 获取工作流
DELETE /api/workflow/:id          # 删除工作流
```

#### 2. 实时状态推送
**问题**: 执行过程中前端无法获取实时状态

**解决方案**:
- WebSocket 推送节点执行状态
- SSE (Server-Sent Events) 推送进度
- 轮询 trace API（简单但不优雅）

#### 3. 工作流持久化
**问题**: 工作流无法保存和管理

**需要的数据库表**:
```sql
CREATE TABLE workflows (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT,
  workflow_json TEXT NOT NULL,
  created_at DATETIME,
  updated_at DATETIME
);

CREATE TABLE workflow_executions (
  id TEXT PRIMARY KEY,
  workflow_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  status TEXT NOT NULL,
  trace_json TEXT,
  started_at DATETIME,
  completed_at DATETIME
);
```

#### 4. 更多实用节点
**当前**: 只有 6 个基础节点
**需要**:
- 条件分支节点（If/Else）
- 循环节点（Loop）
- 数据转换节点（Map/Filter/Reduce）
- HTTP 请求节点
- 文件操作节点
- 数据库查询节点

#### 5. 节点参数校验
**问题**: 节点参数没有 JSON Schema 校验

**解决方案**:
```go
type NodeSpec struct {
    // ...
    ParamsSchema *jsonschema.Schema // JSON Schema 校验
}
```

## 二、节点联动合理性分析

### 当前节点连接示例

```
Tool.Time.Now (text)
    ↓
Context.Pack (messages → context_pack)
    ↓
LLM.Generate (context_pack → messages)
```

### ⚠️ 存在的问题

#### 1. 类型不匹配
```
Tool.Time.Now 输出: text
Context.Pack 输入: messages (期望 LLM 消息格式)
```

**问题**: `text` 类型不能直接连接到 `messages` 类型

**解决方案**:
- 添加类型转换节点
- 或者让 Context.Pack 支持 text 输入

#### 2. 缺少数据转换节点
**需要**:
- `Text.ToMessages` - 将 text 转换为 messages
- `JSON.ToText` - 将 JSON 转换为 text
- `Messages.ToText` - 将 messages 转换为 text

#### 3. 缺少输入/输出节点
**需要**:
- `Input.Text` - 接收用户文本输入
- `Input.JSON` - 接收用户 JSON 输入
- `Output.Text` - 输出文本结果
- `Output.JSON` - 输出 JSON 结果

### 建议的节点分类

```
1. 输入/输出节点
   - Input.Text, Input.JSON, Input.File
   - Output.Text, Output.JSON, Output.File

2. LLM 节点
   - LLM.Generate, LLM.JSON, LLM.Stream

3. 数据转换节点
   - Transform.TextToMessages
   - Transform.JSONToText
   - Transform.MessagesToText

4. 控制流节点
   - Flow.If (条件分支)
   - Flow.Loop (循环)
   - Flow.Switch (多路分支)

5. Context 节点
   - Context.Pack, Context.Compress

6. Memory 节点
   - Memory.Extract, Memory.Get, Memory.Put

7. Vector 节点
   - Vector.Embed, Vector.Query, Vector.Upsert

8. Tool 节点
   - Tool.Time, Tool.Calc, Tool.HTTP, Tool.File

9. KB 节点
   - KB.Query, KB.Upsert
```

## 三、前端集成方案

### 1. 获取可用节点

**前端代码**:
```typescript
// 获取所有节点
const response = await fetch('/api/workflow/nodes');
const { nodes } = await response.json();

// nodes 结构:
interface Node {
  type: string;          // "LLM.Generate"
  version: string;       // "1.0"
  category: string;      // "LLM"
  name: string;          // "Generate"
  description: string;   // "使用 LLM 生成文本响应"
  inputs: Port[];
  outputs: Port[];
  params: Param[];
  icon: string;          // "🤖"
  color: string;         // "#4CAF50"
}

interface Port {
  name: string;          // "messages"
  type: string;          // "messages"
  required: boolean;
  description: string;
}

interface Param {
  name: string;          // "model"
  type: string;          // "string"
  required: boolean;
  default: any;
  description: string;
  options?: any[];       // 枚举选项
}
```

**Vue Flow 集成**:
```vue
<script setup>
import { VueFlow, useVueFlow } from '@vue-flow/core'
import { ref, onMounted } from 'vue'

const nodes = ref([])
const edges = ref([])
const availableNodes = ref([])

// 加载可用节点
onMounted(async () => {
  const res = await fetch('/api/workflow/nodes')
  const data = await res.json()
  availableNodes.value = data.nodes
})

// 添加节点到画布
function addNode(nodeType) {
  const nodeSpec = availableNodes.value.find(n => n.type === nodeType)
  nodes.value.push({
    id: `node-${Date.now()}`,
    type: nodeSpec.type,
    version: nodeSpec.version,
    position: { x: 100, y: 100 },
    data: {
      label: nodeSpec.name,
      params: {}
    }
  })
}

// 连接节点
function onConnect(params) {
  edges.value.push({
    id: `edge-${Date.now()}`,
    source: params.source,
    sourceHandle: params.sourceHandle,
    target: params.target,
    targetHandle: params.targetHandle
  })
}
</script>

<template>
  <div class="workflow-editor">
    <!-- 节点面板 -->
    <div class="node-panel">
      <div v-for="node in availableNodes" :key="node.type">
        <button @click="addNode(node.type)">
          {{ node.icon }} {{ node.name }}
        </button>
      </div>
    </div>

    <!-- 画布 -->
    <VueFlow
      v-model:nodes="nodes"
      v-model:edges="edges"
      @connect="onConnect"
    />
  </div>
</template>
```

### 2. 校验工作流

**前端代码**:
```typescript
async function validateWorkflow() {
  const workflow = exportWorkflow() // 导出 JSON

  const response = await fetch('/api/workflow/validate', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(workflow)
  })

  const result = await response.json()

  if (!result.valid) {
    alert(`工作流校验失败: ${result.error}`)
    return false
  }

  return true
}
```

### 3. 执行工作流

**同步执行**:
```typescript
async function executeWorkflow() {
  const workflow = exportWorkflow()

  const response = await fetch('/api/workflow/execute', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      workflow,
      async: false
    })
  })

  const result = await response.json()

  if (result.success) {
    console.log('执行成功', result.trace)
    showTrace(result.trace)
  } else {
    console.error('执行失败', result.error)
  }
}
```

**异步执行 + 轮询**:
```typescript
async function executeWorkflowAsync() {
  const workflow = exportWorkflow()

  // 1. 提交执行
  const response = await fetch('/api/workflow/execute', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      workflow,
      async: true
    })
  })

  const { trace_id } = await response.json()

  // 2. 轮询状态
  const interval = setInterval(async () => {
    const traceRes = await fetch(`/api/workflow/trace/${trace_id}`)
    const trace = await traceRes.json()

    updateProgress(trace)

    if (trace.status === 'success' || trace.status === 'error') {
      clearInterval(interval)
      showFinalResult(trace)
    }
  }, 1000)
}
```

### 4. 导出工作流 JSON

**Vue Flow 导出**:
```typescript
function exportWorkflow() {
  const { nodes, edges } = useVueFlow()

  return {
    version: "1.0",
    meta: {
      id: `wf-${Date.now()}`,
      name: workflowName.value
    },
    nodes: Object.fromEntries(
      nodes.value.map(node => [
        node.id,
        {
          id: node.id,
          type: node.type,
          version: node.data.version || "1.0",
          params: node.data.params || {}
        }
      ])
    ),
    edges: edges.value.map(edge => ({
      id: edge.id,
      from: {
        node: edge.source,
        port: edge.sourceHandle
      },
      to: {
        node: edge.target,
        port: edge.targetHandle
      },
      type: "data"
    })),
    ui: {
      nodes: Object.fromEntries(
        nodes.value.map(node => [
          node.id,
          {
            x: node.position.x,
            y: node.position.y
          }
        ])
      )
    }
  }
}
```

## 四、完整的前端示例

```vue
<script setup>
import { ref, onMounted } from 'vue'
import { VueFlow, useVueFlow } from '@vue-flow/core'

const { nodes, edges, addNodes, addEdges } = useVueFlow()

const availableNodes = ref([])
const executing = ref(false)
const trace = ref(null)

// 加载可用节点
onMounted(async () => {
  const res = await fetch('/api/workflow/nodes')
  const data = await res.json()
  availableNodes.value = data.nodes
})

// 添加节点
function addNode(nodeType) {
  const spec = availableNodes.value.find(n => n.type === nodeType)
  addNodes({
    id: `node-${Date.now()}`,
    type: 'custom',
    position: { x: Math.random() * 400, y: Math.random() * 400 },
    data: {
      nodeType: spec.type,
      version: spec.version,
      label: spec.name,
      icon: spec.icon,
      color: spec.color,
      inputs: spec.inputs,
      outputs: spec.outputs,
      params: {}
    }
  })
}

// 执行工作流
async function execute() {
  executing.value = true

  const workflow = {
    version: "1.0",
    meta: { id: `wf-${Date.now()}`, name: "Test" },
    nodes: Object.fromEntries(
      nodes.value.map(n => [n.id, {
        id: n.id,
        type: n.data.nodeType,
        version: n.data.version,
        params: n.data.params
      }])
    ),
    edges: edges.value.map(e => ({
      id: e.id,
      from: { node: e.source, port: e.sourceHandle },
      to: { node: e.target, port: e.targetHandle },
      type: "data"
    }))
  }

  try {
    const res = await fetch('/api/workflow/execute', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ workflow, async: false })
    })

    const result = await res.json()
    trace.value = result.trace

    if (result.success) {
      alert('执行成功！')
    } else {
      alert(`执行失败: ${result.error}`)
    }
  } finally {
    executing.value = false
  }
}
</script>

<template>
  <div class="workflow-app">
    <!-- 节点面板 -->
    <aside class="node-panel">
      <h3>可用节点</h3>
      <div v-for="node in availableNodes" :key="node.type" class="node-item">
        <button @click="addNode(node.type)">
          <span>{{ node.icon }}</span>
          <span>{{ node.name }}</span>
        </button>
      </div>
    </aside>

    <!-- 画布 -->
    <main class="canvas">
      <VueFlow />
    </main>

    <!-- 控制面板 -->
    <footer class="controls">
      <button @click="execute" :disabled="executing">
        {{ executing ? '执行中...' : '执行工作流' }}
      </button>
    </footer>

    <!-- 执行结果 -->
    <aside v-if="trace" class="trace-panel">
      <h3>执行追踪</h3>
      <div>状态: {{ trace.status }}</div>
      <div>耗时: {{ trace.duration }}</div>
      <div v-for="(nodeTrace, id) in trace.nodes" :key="id">
        <strong>{{ id }}</strong>: {{ nodeTrace.status }}
      </div>
    </aside>
  </div>
</template>
```

## 五、后端集成到现有系统

### 在 main.go 中注册路由

```go
package main

import (
    "agent-langchain/internal/api"
    "agent-langchain/internal/models"
)

func main() {
    e := echo.New()

    // 创建 LLM 客户端
    llmClient := models.New("http://localhost:11434", "qwen2.5:7b", "")

    // 创建工作流 API
    workflowAPI := api.NewWorkflowAPI(llmClient)
    workflowAPI.RegisterRoutes(e)

    // 启动服务器
    e.Start(":8080")
}
```

## 六、建议的实现优先级

### 第一阶段（立即实现）
1. ✅ 完成 API 层代码（workflow.go）
2. ✅ 添加类型转换节点
3. ✅ 添加输入/输出节点
4. ✅ 前端基础集成

### 第二阶段（1-2 周）
5. 工作流持久化（数据库）
6. WebSocket 实时推送
7. 更多实用节点（HTTP、File、DB）
8. 节点参数 JSON Schema 校验

### 第三阶段（2-4 周）
9. 控制流节点（If/Loop）
10. 工作流版本管理
11. 权限和安全
12. 性能优化

## 七、总结

### 当前状态
- ✅ 核心引擎完整
- ❌ 缺少 API 层
- ❌ 缺少前端集成
- ⚠️ 节点联动需要优化

### 关键缺失
1. **API 层** - 最关键，前端无法使用
2. **类型转换节点** - 节点连接不够灵活
3. **输入/输出节点** - 无法接收用户输入
4. **工作流持久化** - 无法保存和管理

### 下一步行动
1. 实现完整的 API 层
2. 添加必要的转换节点
3. 前端集成测试
4. 数据库持久化
