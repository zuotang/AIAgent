# 工作流前端对接文档

## 一、可用节点列表

### 1. LLM 节点

#### LLM.Ollama
- **输入端口**: `messages` (messages类型)
- **输出端口**: `messages` (messages类型)
- **参数**:
  - `model` (string, 可选, 默认: "qwen2.5:7b")
  - `base_url` (string, 可选, 默认: "http://localhost:11434")
  - `temperature` (number, 可选, 默认: 0)
  - `max_retries` (number, 可选, 默认: 1)

#### LLM.DeepSeek
- **输入端口**: `messages` (messages类型)
- **输出端口**: `messages` (messages类型)
- **参数**:
  - `model` (string, 可选, 默认: "deepseek-chat")
  - `api_key` (string, **必需**)
  - `base_url` (string, 可选, 默认: "https://api.deepseek.com/v1")
  - `temperature` (number, 可选, 默认: 0)
  - `max_retries` (number, 可选, 默认: 1)

#### LLM.Anthropic
- **输入端口**: `messages` (messages类型)
- **输出端口**: `messages` (messages类型)
- **参数**:
  - `model` (string, 可选, 默认: "claude-3-sonnet-20240229")
  - `api_key` (string, **必需**)
  - `base_url` (string, 可选, 默认: "https://api.anthropic.com/v1")
  - `temperature` (number, 可选, 默认: 0)
  - `max_retries` (number, 可选, 默认: 1)

#### LLM.Chat (统一接口)
- **输入端口**: `messages` (messages类型)
- **输出端口**: `messages` (messages类型)
- **参数**:
  - `provider` (string, 可选, 默认: "ollama", 可选值: "ollama", "deepseek", "anthropic")
  - `model` (string, 可选)
  - `api_key` (string, 条件必需 - DeepSeek/Anthropic需要)
  - `base_url` (string, 可选)
  - `temperature` (number, 可选, 默认: 0)
  - `max_retries` (number, 可选, 默认: 1)

#### LLM.Generate (原始节点)
- **输入端口**: `messages` 或 `context_pack`
- **输出端口**: `messages` (messages类型)
- **参数**:
  - `model` (string, 可选)

#### LLM.JSON
- **输入端口**: `messages` (messages类型)
- **输出端口**: `json` (json类型)
- **参数**:
  - `model` (string, 可选)

### 2. Context 节点

#### Context.Pack
- **输入端口**: `messages` (messages类型)
- **输出端口**: `context_pack` (context_pack类型)
- **参数**: 无

#### Context.Compress
- **输入端口**: `context_pack` (context_pack类型)
- **输出端口**: `context_pack` (context_pack类型)
- **参数**: 无

### 3. Tool 节点

#### Tool.Time.Now
- **输入端口**: 无
- **输出端口**: `text` (text类型)
- **参数**:
  - `format` (string, 可选, 默认: "2006-01-02 15:04:05")

#### Tool.Calc
- **输入端口**: `text` (text类型) - 数学表达式
- **输出端口**: `text` (text类型) - 计算结果
- **参数**: 无

### 4. IO 节点

#### Input.Text
- **输入端口**: 无
- **输出端口**: `text` (text类型)
- **参数**:
  - `text` (string, **必需**) - 输入文本内容

#### Output.Text
- **输入端口**: `text` (text类型)
- **输出端口**: 无
- **参数**: 无

#### Input.JSON
- **输入端口**: 无
- **输出端口**: `json` (json类型)
- **参数**:
  - `json` (object, **必需**) - JSON数据

#### Output.JSON
- **输入端口**: `json` (json类型)
- **输出端口**: 无
- **参数**: 无

### 5. Transform 节点

#### Transform.TextToMessages
- **输入端口**: `text` (text类型)
- **输出端口**: `messages` (messages类型)
- **参数**:
  - `role` (string, 可选, 默认: "user", 可选值: "user", "assistant", "system")

#### Transform.MessagesToText
- **输入端口**: `messages` (messages类型)
- **输出端口**: `text` (text类型)
- **参数**: 无

#### Transform.JSONToText
- **输入端口**: `json` (json类型)
- **输出端口**: `text` (text类型)
- **参数**: 无

#### Transform.TextToJSON
- **输入端口**: `text` (text类型)
- **输出端口**: `json` (json类型)
- **参数**: 无

---

## 二、端口类型系统

工作流支持以下端口类型，连接时必须类型匹配：

- `messages` - LLM消息列表
- `text` - 纯文本字符串
- `json` - JSON数据对象
- `context_pack` - 打包的上下文
- `embedding` - 向量嵌入
- `vector_query` - 向量查询请求
- `vector_results` - 向量查询结果
- `memory_items` - 记忆项列表
- `kb_docs` - 知识库文档
- `tool_call` - 工具调用请求
- `tool_result` - 工具执行结果
- `llm_config` - LLM配置
- `flow` - 控制流信号

---

## 三、API 接口定义

### 1. 获取所有可用节点

**请求**:
```http
GET /api/workflow/nodes
```

**响应**:
```json
{
  "nodes": [
    {
      "type": "LLM.Ollama",
      "version": "1.0",
      "category": "LLM",
      "name": "Ollama",
      "description": "使用本地Ollama运行的开源模型",
      "inputs": [
        {
          "name": "messages",
          "type": "messages",
          "required": true,
          "description": "输入消息列表"
        }
      ],
      "outputs": [
        {
          "name": "messages",
          "type": "messages",
          "required": true,
          "description": "输出消息"
        }
      ],
      "params": [
        {
          "name": "model",
          "type": "string",
          "required": false,
          "default": "qwen2.5:7b",
          "description": "模型名称",
          "options": ["qwen2.5:7b", "llama2", "mistral", "codellama"]
        },
        {
          "name": "temperature",
          "type": "number",
          "required": false,
          "default": 0,
          "description": "温度参数(0-2)",
          "min": 0,
          "max": 2
        }
      ]
    }
  ]
}
```

### 2. 校验工作流

**请求**:
```http
POST /api/workflow/validate
Content-Type: application/json

{
  "version": "1.0",
  "meta": {
    "id": "wf-001",
    "name": "测试工作流"
  },
  "nodes": {
    "node-1": {
      "id": "node-1",
      "type": "Input.Text",
      "version": "1.0",
      "params": {
        "text": "Hello"
      }
    },
    "node-2": {
      "id": "node-2",
      "type": "Transform.TextToMessages",
      "version": "1.0",
      "params": {
        "role": "user"
      }
    }
  },
  "edges": [
    {
      "id": "e1",
      "from": {
        "node": "node-1",
        "port": "text"
      },
      "to": {
        "node": "node-2",
        "port": "text"
      },
      "type": "data"
    }
  ]
}
```

**响应**:
```json
{
  "valid": true,
  "errors": []
}
```

或错误情况:
```json
{
  "valid": false,
  "errors": [
    "节点 node-3 的输入端口 messages 未连接",
    "端口类型不匹配: text -> messages"
  ]
}
```

### 3. 执行工作流

**请求**:
```http
POST /api/workflow/execute
Content-Type: application/json

{
  "workflow": {
    "version": "1.0",
    "meta": {
      "id": "wf-001",
      "name": "测试工作流"
    },
    "nodes": { ... },
    "edges": [ ... ]
  },
  "async": false
}
```

**同步响应** (async: false):
```json
{
  "success": true,
  "trace": {
    "workflow_id": "wf-001",
    "status": "success",
    "duration": "1.234s",
    "started_at": "2024-02-06T10:00:00Z",
    "completed_at": "2024-02-06T10:00:01Z",
    "nodes": {
      "node-1": {
        "status": "success",
        "duration": "0.1s",
        "inputs": {},
        "outputs": {
          "text": "Hello"
        },
        "error": ""
      },
      "node-2": {
        "status": "success",
        "duration": "0.2s",
        "inputs": {
          "text": "Hello"
        },
        "outputs": {
          "messages": [
            {"role": "user", "content": "Hello"}
          ]
        },
        "error": ""
      }
    }
  }
}
```

**异步响应** (async: true):
```json
{
  "trace_id": "trace-12345",
  "status": "running"
}
```

### 4. 获取执行追踪

**请求**:
```http
GET /api/workflow/trace/:trace_id
```

**响应**:
```json
{
  "trace_id": "trace-12345",
  "workflow_id": "wf-001",
  "status": "success",
  "duration": "1.234s",
  "started_at": "2024-02-06T10:00:00Z",
  "completed_at": "2024-02-06T10:00:01Z",
  "nodes": { ... }
}
```

### 5. 保存工作流

**请求**:
```http
POST /api/workflow/save
Content-Type: application/json

{
  "id": "wf-001",
  "name": "我的工作流",
  "description": "这是一个测试工作流",
  "workflow": { ... }
}
```

**响应**:
```json
{
  "success": true,
  "id": "wf-001"
}
```

### 6. 获取工作流列表

**请求**:
```http
GET /api/workflow/list?page=1&limit=20
```

**响应**:
```json
{
  "workflows": [
    {
      "id": "wf-001",
      "name": "我的工作流",
      "description": "这是一个测试工作流",
      "created_at": "2024-02-06T10:00:00Z",
      "updated_at": "2024-02-06T10:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "limit": 20
}
```

### 7. 获取单个工作流

**请求**:
```http
GET /api/workflow/:id
```

**响应**:
```json
{
  "id": "wf-001",
  "name": "我的工作流",
  "description": "这是一个测试工作流",
  "workflow": { ... },
  "created_at": "2024-02-06T10:00:00Z",
  "updated_at": "2024-02-06T10:00:00Z"
}
```

### 8. 删除工作流

**请求**:
```http
DELETE /api/workflow/:id
```

**响应**:
```json
{
  "success": true
}
```

---

## 四、工作流 JSON 格式

```json
{
  "version": "1.0",
  "meta": {
    "id": "wf-001",
    "name": "示例工作流",
    "description": "这是一个示例"
  },
  "nodes": {
    "node-1": {
      "id": "node-1",
      "type": "Input.Text",
      "version": "1.0",
      "params": {
        "text": "你好，世界！"
      }
    },
    "node-2": {
      "id": "node-2",
      "type": "Transform.TextToMessages",
      "version": "1.0",
      "params": {
        "role": "user"
      }
    },
    "node-3": {
      "id": "node-3",
      "type": "LLM.Ollama",
      "version": "1.0",
      "params": {
        "model": "qwen2.5:7b",
        "temperature": 0.7
      }
    },
    "node-4": {
      "id": "node-4",
      "type": "Output.Text",
      "version": "1.0",
      "params": {}
    }
  },
  "edges": [
    {
      "id": "e1",
      "from": { "node": "node-1", "port": "text" },
      "to": { "node": "node-2", "port": "text" },
      "type": "data"
    },
    {
      "id": "e2",
      "from": { "node": "node-2", "port": "messages" },
      "to": { "node": "node-3", "port": "messages" },
      "type": "data"
    },
    {
      "id": "e3",
      "from": { "node": "node-3", "port": "messages" },
      "to": { "node": "node-4", "port": "text" },
      "type": "data"
    }
  ],
  "ui": {
    "nodes": {
      "node-1": { "x": 100, "y": 100 },
      "node-2": { "x": 300, "y": 100 },
      "node-3": { "x": 500, "y": 100 },
      "node-4": { "x": 700, "y": 100 }
    }
  }
}
```

---

## 五、Vue3 集成示例

### 1. 获取并显示可用节点

```vue
<script setup>
import { ref, onMounted } from 'vue'

const availableNodes = ref([])

onMounted(async () => {
  const res = await fetch('/api/workflow/nodes')
  const data = await res.json()
  availableNodes.value = data.nodes
})
</script>

<template>
  <div class="node-palette">
    <h3>可用节点</h3>
    <div v-for="node in availableNodes" :key="node.type" class="node-item">
      <div class="node-header">
        <span class="node-category">{{ node.category }}</span>
        <span class="node-name">{{ node.name }}</span>
      </div>
      <p class="node-desc">{{ node.description }}</p>
    </div>
  </div>
</template>
```

### 2. 构建工作流并执行

```vue
<script setup>
import { ref } from 'vue'
import { VueFlow, useVueFlow } from '@vue-flow/core'

const { nodes, edges } = useVueFlow()
const executing = ref(false)
const trace = ref(null)

// 导出工作流JSON
function exportWorkflow() {
  return {
    version: "1.0",
    meta: {
      id: `wf-${Date.now()}`,
      name: "我的工作流"
    },
    nodes: Object.fromEntries(
      nodes.value.map(n => [n.id, {
        id: n.id,
        type: n.data.nodeType,
        version: n.data.version || "1.0",
        params: n.data.params || {}
      }])
    ),
    edges: edges.value.map(e => ({
      id: e.id,
      from: { node: e.source, port: e.sourceHandle },
      to: { node: e.target, port: e.targetHandle },
      type: "data"
    })),
    ui: {
      nodes: Object.fromEntries(
        nodes.value.map(n => [n.id, {
          x: n.position.x,
          y: n.position.y
        }])
      )
    }
  }
}

// 执行工作流
async function executeWorkflow() {
  executing.value = true

  try {
    const workflow = exportWorkflow()

    const res = await fetch('/api/workflow/execute', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ workflow, async: false })
    })

    const result = await res.json()

    if (result.success) {
      trace.value = result.trace
      console.log('执行成功', result.trace)
    } else {
      console.error('执行失败', result.error)
    }
  } finally {
    executing.value = false
  }
}
</script>

<template>
  <div class="workflow-editor">
    <VueFlow v-model:nodes="nodes" v-model:edges="edges" />

    <button @click="executeWorkflow" :disabled="executing">
      {{ executing ? '执行中...' : '执行工作流' }}
    </button>

    <div v-if="trace" class="trace-panel">
      <h3>执行结果</h3>
      <p>状态: {{ trace.status }}</p>
      <p>耗时: {{ trace.duration }}</p>
    </div>
  </div>
</template>
```

### 3. 节点参数表单

```vue
<script setup>
import { ref } from 'vue'

const props = defineProps({
  nodeType: String,
  nodeSpec: Object
})

const params = ref({})

// 根据节点类型初始化默认参数
function initParams() {
  props.nodeSpec.params.forEach(param => {
    if (param.default !== undefined) {
      params.value[param.name] = param.default
    }
  })
}

initParams()
</script>

<template>
  <div class="node-params-form">
    <div v-for="param in nodeSpec.params" :key="param.name" class="param-field">
      <label>{{ param.description }}</label>

      <!-- 字符串输入 -->
      <input
        v-if="param.type === 'string' && !param.options"
        v-model="params[param.name]"
        type="text"
        :placeholder="param.default"
      />

      <!-- 下拉选择 -->
      <select
        v-else-if="param.type === 'string' && param.options"
        v-model="params[param.name]"
      >
        <option v-for="opt in param.options" :key="opt" :value="opt">
          {{ opt }}
        </option>
      </select>

      <!-- 数字输入 -->
      <input
        v-else-if="param.type === 'number'"
        v-model.number="params[param.name]"
        type="number"
        :min="param.min"
        :max="param.max"
        :step="param.step || 0.1"
      />

      <!-- 布尔值 -->
      <input
        v-else-if="param.type === 'boolean'"
        v-model="params[param.name]"
        type="checkbox"
      />
    </div>
  </div>
</template>
```

---

## 六、注意事项

### 1. 类型匹配
- 连接节点时必须确保端口类型匹配
- 使用 Transform 节点进行类型转换

### 2. 必需参数
- 某些节点有必需参数（如 `Input.Text` 的 `text` 参数）
- 执行前需要校验所有必需参数已填写

### 3. API Key 安全
- 不要在前端硬编码 API Key
- 建议使用环境变量或后端配置管理

### 4. 错误处理
- 执行失败时检查 `trace.nodes` 中的错误信息
- 每个节点都有独立的状态和错误信息

### 5. 异步执行
- 长时间运行的工作流建议使用异步模式
- 通过轮询 `/api/workflow/trace/:id` 获取进度

---

## 七、常见工作流示例

### 示例1: 简单对话
```
Input.Text → Transform.TextToMessages → LLM.Ollama → Output.Text
```

### 示例2: 带时间戳的对话
```
Tool.Time.Now → Transform.TextToMessages → Context.Pack → LLM.Ollama → Output.Text
```

### 示例3: JSON输出
```
Input.Text → Transform.TextToMessages → LLM.JSON → Output.JSON
```

---

## 八、API 状态码

- `200` - 成功
- `400` - 请求参数错误
- `404` - 资源不存在
- `500` - 服务器内部错误
