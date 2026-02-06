# 工作流节点完善 - 更新总结

## 📊 更新概览

### 新增节点数量
- **之前**: 14 个节点
- **现在**: 18 个节点
- **新增**: 4 个 LLM 提供商节点

### 节点分类

| 类别 | 节点数 | 节点列表 |
|------|--------|----------|
| **LLM** | 6 | Generate, JSON, Ollama, DeepSeek, Anthropic, Chat |
| **Context** | 2 | Pack, Compress |
| **Tool** | 2 | Time.Now, Calc |
| **IO** | 4 | Input.Text, Output.Text, Input.JSON, Output.JSON |
| **Transform** | 4 | TextToMessages, MessagesToText, JSONToText, TextToJSON |
| **总计** | **18** | - |

---

## 🆕 新增 LLM 节点详解

### 1. LLM.Ollama（本地模型）

**特点**:
- ✅ 支持本地 Ollama 运行的所有模型
- ✅ 无需 API Key
- ✅ 完全私有化部署
- ✅ 支持自定义 base_url

**参数**:
```json
{
  "model": "qwen2.5:7b",           // 模型名称
  "base_url": "http://localhost:11434",  // Ollama 地址
  "temperature": 0.7,              // 温度（0-2）
  "max_retries": 3                 // 最大重试次数
}
```

**适用场景**:
- 本地开发测试
- 私有化部署
- 无网络环境
- 成本敏感场景

---

### 2. LLM.DeepSeek（DeepSeek API）

**特点**:
- ✅ 支持 DeepSeek 对话和代码模型
- ✅ 性价比高
- ✅ 中文能力强
- ⚠️ 需要 API Key

**参数**:
```json
{
  "model": "deepseek-chat",        // 模型名称
  "api_key": "sk-xxxxx",           // API 密钥（必需）
  "base_url": "https://api.deepseek.com/v1",
  "temperature": 0.7,
  "max_retries": 3
}
```

**适用场景**:
- 生产环境
- 需要稳定 API 服务
- 中文对话场景
- 代码生成场景

**获取 API Key**: https://platform.deepseek.com/

---

### 3. LLM.Anthropic（Claude API）

**特点**:
- ✅ 支持 Claude 3 系列模型
- ✅ 推理能力强
- ✅ 上下文窗口大
- ⚠️ 需要 API Key
- ⚠️ 价格较高

**参数**:
```json
{
  "model": "claude-3-sonnet-20240229",  // 模型名称
  "api_key": "sk-ant-xxxxx",            // API 密钥（必需）
  "base_url": "https://api.anthropic.com/v1",
  "temperature": 0.7,
  "max_retries": 3
}
```

**适用场景**:
- 复杂推理任务
- 长文本处理
- 高质量输出要求

**获取 API Key**: https://console.anthropic.com/

---

### 4. LLM.Chat（统一接口）

**特点**:
- ✅ 统一接口，支持所有提供商
- ✅ 通过 `provider` 参数切换
- ✅ 便于迁移和测试

**参数**:
```json
{
  "provider": "ollama",            // 提供商（ollama/deepseek/anthropic）
  "model": "qwen2.5:7b",
  "api_key": "",                   // 根据提供商决定是否必需
  "base_url": "",
  "temperature": 0.7,
  "max_retries": 3
}
```

**适用场景**:
- 需要灵活切换提供商
- 多环境部署（开发用 Ollama，生产用 DeepSeek）
- A/B 测试不同模型

---

## 🎯 节点合理性分析

### ✅ 优点

#### 1. 提供商隔离
每个提供商有独立节点，参数清晰：
```
LLM.Ollama    → 本地模型，无需 API Key
LLM.DeepSeek  → 云端 API，需要 API Key
LLM.Anthropic → 云端 API，需要 API Key
```

#### 2. 参数灵活
支持所有常用参数：
- ✅ `model` - 模型选择
- ✅ `temperature` - 温度控制
- ✅ `max_retries` - 重试机制
- ✅ `base_url` - 自定义 API 地址
- ✅ `api_key` - API 密钥

#### 3. 前端友好
前端可以根据节点类型自动生成表单：

```vue
<!-- LLM.Ollama 表单 -->
<select v-model="params.model">
  <option value="qwen2.5:7b">Qwen 2.5 (7B)</option>
  <option value="llama2">Llama 2</option>
  <option value="mistral">Mistral</option>
</select>

<!-- LLM.DeepSeek 表单 -->
<input type="password" v-model="params.api_key"
       placeholder="sk-xxxxx" required />
```

#### 4. 错误处理
支持自动重试：
```go
for i := 0; i < maxRetries; i++ {
    response, err = client.Chat(ctx, msgs, model)
    if err == nil {
        return response, nil
    }
    // 指数退避重试
    time.Sleep(time.Second * time.Duration(i+1))
}
```

---

### ⚠️ 需要注意的问题

#### 1. API Key 安全

**问题**: API Key 不应该硬编码在工作流 JSON 中

**解决方案**:

**方案 A**: 环境变量
```json
{
  "api_key": "${env.DEEPSEEK_API_KEY}"
}
```

**方案 B**: 后端配置
```go
// 从配置文件或环境变量读取
apiKey := config.Get("deepseek.api_key")
```

**方案 C**: 用户级密钥管理
```sql
CREATE TABLE user_api_keys (
  user_id TEXT,
  provider TEXT,
  api_key_encrypted TEXT
);
```

#### 2. 参数校验

**问题**: 前端需要校验参数

**解决方案**: 在 API 层添加校验
```go
func validateLLMParams(nodeType string, params map[string]any) error {
    switch nodeType {
    case "LLM.DeepSeek", "LLM.Anthropic":
        if params["api_key"] == "" {
            return fmt.Errorf("api_key is required")
        }
    }

    if temp, ok := params["temperature"].(float64); ok {
        if temp < 0 || temp > 2 {
            return fmt.Errorf("temperature must be between 0 and 2")
        }
    }

    return nil
}
```

#### 3. 成本控制

**问题**: 云端 API 调用有成本

**解决方案**: 添加使用限制
```go
type UsageLimit struct {
    MaxTokensPerDay   int
    MaxRequestsPerDay int
    CurrentUsage      int
}

func checkUsageLimit(userID string) error {
    usage := getUserUsage(userID)
    if usage.CurrentUsage >= usage.MaxRequestsPerDay {
        return fmt.Errorf("daily limit exceeded")
    }
    return nil
}
```

---

## 📝 前端集成示例

### 1. 节点选择器

```vue
<template>
  <div class="llm-node-selector">
    <h3>选择 LLM 提供商</h3>

    <div class="provider-cards">
      <!-- Ollama -->
      <div class="card" @click="selectNode('LLM.Ollama')">
        <div class="icon">🏠</div>
        <h4>Ollama</h4>
        <p>本地模型，免费</p>
        <span class="badge">推荐开发</span>
      </div>

      <!-- DeepSeek -->
      <div class="card" @click="selectNode('LLM.DeepSeek')">
        <div class="icon">🚀</div>
        <h4>DeepSeek</h4>
        <p>云端 API，性价比高</p>
        <span class="badge">推荐生产</span>
      </div>

      <!-- Anthropic -->
      <div class="card" @click="selectNode('LLM.Anthropic')">
        <div class="icon">🧠</div>
        <h4>Claude</h4>
        <p>推理能力强</p>
        <span class="badge">高级</span>
      </div>
    </div>
  </div>
</template>
```

### 2. 参数表单（动态生成）

```vue
<template>
  <div class="node-params-form">
    <div v-for="param in nodeParams" :key="param.name" class="param-field">
      <label>
        {{ param.label }}
        <span v-if="param.required" class="required">*</span>
      </label>

      <!-- 文本输入 -->
      <input v-if="param.type === 'string'"
             v-model="params[param.name]"
             :type="param.secret ? 'password' : 'text'"
             :placeholder="param.placeholder"
             :required="param.required" />

      <!-- 数字输入 -->
      <input v-else-if="param.type === 'number'"
             v-model.number="params[param.name]"
             type="number"
             :min="param.min"
             :max="param.max"
             :step="param.step" />

      <!-- 选择器 -->
      <select v-else-if="param.type === 'select'"
              v-model="params[param.name]">
        <option v-for="opt in param.options" :key="opt.value"
                :value="opt.value">
          {{ opt.label }}
        </option>
      </select>

      <small v-if="param.description">{{ param.description }}</small>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps(['nodeType'])
const params = ref({})

const nodeParams = computed(() => {
  const paramDefs = {
    'LLM.Ollama': [
      {
        name: 'model',
        type: 'select',
        label: '模型',
        required: false,
        options: [
          { value: 'qwen2.5:7b', label: 'Qwen 2.5 (7B)' },
          { value: 'llama2', label: 'Llama 2' },
          { value: 'mistral', label: 'Mistral' }
        ]
      },
      {
        name: 'base_url',
        type: 'string',
        label: 'Base URL',
        required: false,
        placeholder: 'http://localhost:11434'
      },
      {
        name: 'temperature',
        type: 'number',
        label: 'Temperature',
        required: false,
        min: 0,
        max: 2,
        step: 0.1,
        description: '控制输出随机性（0-2）'
      },
      {
        name: 'max_retries',
        type: 'number',
        label: '最大重试次数',
        required: false,
        min: 1,
        max: 10
      }
    ],
    'LLM.DeepSeek': [
      {
        name: 'model',
        type: 'select',
        label: '模型',
        required: false,
        options: [
          { value: 'deepseek-chat', label: 'DeepSeek Chat' },
          { value: 'deepseek-coder', label: 'DeepSeek Coder' }
        ]
      },
      {
        name: 'api_key',
        type: 'string',
        label: 'API Key',
        required: true,
        secret: true,
        placeholder: 'sk-xxxxx',
        description: '从 platform.deepseek.com 获取'
      },
      {
        name: 'temperature',
        type: 'number',
        label: 'Temperature',
        required: false,
        min: 0,
        max: 2,
        step: 0.1
      }
    ]
  }

  return paramDefs[props.nodeType] || []
})
</script>
```

---

## 🚀 使用示例

### 示例 1: 本地开发（Ollama）

```json
{
  "version": "1.0",
  "meta": {"id": "local-dev", "name": "本地开发测试"},
  "nodes": {
    "input": {
      "type": "Input.Text",
      "params": {"text": "你好"}
    },
    "transform": {
      "type": "Transform.TextToMessages",
      "params": {}
    },
    "llm": {
      "type": "LLM.Ollama",
      "params": {
        "model": "qwen2.5:7b",
        "temperature": 0.7
      }
    },
    "output": {
      "type": "Output.Text",
      "params": {}
    }
  },
  "edges": [...]
}
```

### 示例 2: 生产环境（DeepSeek）

```json
{
  "nodes": {
    "llm": {
      "type": "LLM.DeepSeek",
      "params": {
        "model": "deepseek-chat",
        "api_key": "${env.DEEPSEEK_API_KEY}",
        "temperature": 0.7,
        "max_retries": 3
      }
    }
  }
}
```

### 示例 3: 多提供商对比

```json
{
  "nodes": {
    "llm1": {
      "type": "LLM.Ollama",
      "params": {"model": "qwen2.5:7b"}
    },
    "llm2": {
      "type": "LLM.DeepSeek",
      "params": {
        "model": "deepseek-chat",
        "api_key": "${env.DEEPSEEK_API_KEY}"
      }
    },
    "llm3": {
      "type": "LLM.Anthropic",
      "params": {
        "model": "claude-3-sonnet-20240229",
        "api_key": "${env.ANTHROPIC_API_KEY}"
      }
    }
  }
}
```

---

## 📊 总结

### ✅ 已完成
1. **4 个新 LLM 节点** - Ollama, DeepSeek, Anthropic, Chat
2. **完整参数支持** - model, temperature, max_retries, base_url, api_key
3. **错误重试机制** - 指数退避重试
4. **参数文档** - NODE_PARAMETERS_GUIDE.md

### 🎯 节点合理性
- ✅ **提供商隔离** - 每个提供商独立节点
- ✅ **参数灵活** - 支持所有常用参数
- ✅ **前端友好** - 可自动生成表单
- ✅ **安全考虑** - API Key 管理建议

### 📝 下一步
1. 实现 API Key 安全管理
2. 添加参数校验
3. 实现成本控制
4. 前端表单自动生成

---

**节点总数**: 18 个（+4 个 LLM 节点）
**文档**: NODE_PARAMETERS_GUIDE.md
**状态**: ✅ 完成
