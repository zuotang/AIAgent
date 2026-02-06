# 工作流节点参数配置指南

## LLM 节点参数

### 1. LLM.Ollama（本地 Ollama 模型）

**用途**: 使用本地 Ollama 运行的开源模型

**参数**:

| 参数名 | 类型 | 必需 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 否 | `qwen2.5:7b` | 模型名称（如 `llama2`, `mistral`, `qwen2.5:7b`） |
| `base_url` | string | 否 | `http://localhost:11434` | Ollama API 地址 |
| `temperature` | number | 否 | `0` | 温度参数（0-2），控制输出随机性 |
| `max_retries` | number | 否 | `1` | 最大重试次数 |

**示例配置**:
```json
{
  "model": "qwen2.5:7b",
  "base_url": "http://localhost:11434",
  "temperature": 0.7,
  "max_retries": 3
}
```

**支持的模型**:
- `qwen2.5:7b` - 通义千问 2.5（7B 参数）
- `llama2` - Meta Llama 2
- `mistral` - Mistral AI
- `codellama` - Code Llama（代码生成）
- 更多模型见 [Ollama 模型库](https://ollama.com/library)

---

### 2. LLM.DeepSeek（DeepSeek API）

**用途**: 使用 DeepSeek 云端 API

**参数**:

| 参数名 | 类型 | 必需 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 否 | `deepseek-chat` | 模型名称 |
| `api_key` | string | **是** | - | DeepSeek API 密钥 |
| `base_url` | string | 否 | `https://api.deepseek.com/v1` | API 地址 |
| `temperature` | number | 否 | `0` | 温度参数（0-2） |
| `max_retries` | number | 否 | `1` | 最大重试次数 |

**示例配置**:
```json
{
  "model": "deepseek-chat",
  "api_key": "sk-xxxxxxxxxxxxx",
  "temperature": 0.7,
  "max_retries": 3
}
```

**支持的模型**:
- `deepseek-chat` - DeepSeek 对话模型
- `deepseek-coder` - DeepSeek 代码模型

**获取 API Key**: [DeepSeek 平台](https://platform.deepseek.com/)

---

### 3. LLM.Anthropic（Claude API）

**用途**: 使用 Anthropic Claude API

**参数**:

| 参数名 | 类型 | 必需 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 否 | `claude-3-sonnet-20240229` | 模型名称 |
| `api_key` | string | **是** | - | Anthropic API 密钥 |
| `base_url` | string | 否 | `https://api.anthropic.com/v1` | API 地址 |
| `temperature` | number | 否 | `0` | 温度参数（0-1） |
| `max_retries` | number | 否 | `1` | 最大重试次数 |

**示例配置**:
```json
{
  "model": "claude-3-sonnet-20240229",
  "api_key": "sk-ant-xxxxxxxxxxxxx",
  "temperature": 0.7,
  "max_retries": 3
}
```

**支持的模型**:
- `claude-3-opus-20240229` - Claude 3 Opus（最强）
- `claude-3-sonnet-20240229` - Claude 3 Sonnet（平衡）
- `claude-3-haiku-20240307` - Claude 3 Haiku（最快）

**获取 API Key**: [Anthropic Console](https://console.anthropic.com/)

---

### 4. LLM.Chat（统一接口）

**用途**: 统一的 LLM 接口，支持多种提供商

**参数**:

| 参数名 | 类型 | 必需 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `provider` | string | 否 | `ollama` | 提供商（`ollama`, `deepseek`, `anthropic`） |
| `model` | string | 否 | 根据提供商 | 模型名称 |
| `api_key` | string | 条件 | - | API 密钥（DeepSeek/Anthropic 必需） |
| `base_url` | string | 否 | 根据提供商 | API 地址 |
| `temperature` | number | 否 | `0` | 温度参数 |
| `max_retries` | number | 否 | `1` | 最大重试次数 |

**示例配置**:
```json
{
  "provider": "deepseek",
  "model": "deepseek-chat",
  "api_key": "sk-xxxxxxxxxxxxx",
  "temperature": 0.7,
  "max_retries": 3
}
```

---

## 其他节点参数

### Context.Pack

**参数**: 无

**说明**: 将多个输入打包成 context_pack

---

### Context.Compress

**参数**: 无

**说明**: 使用 LLM 压缩上下文

---

### Tool.Time.Now

**参数**:

| 参数名 | 类型 | 必需 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `format` | string | 否 | `2006-01-02 15:04:05` | 时间格式 |

**示例**:
```json
{
  "format": "2006-01-02 15:04:05"
}
```

---

### Tool.Calc

**参数**: 无

**说明**: 执行数学计算

---

### Input.Text

**参数**:

| 参数名 | 类型 | 必需 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `text` | string | 是 | - | 输入文本 |

**示例**:
```json
{
  "text": "Hello, world!"
}
```

---

### Transform.TextToMessages

**参数**:

| 参数名 | 类型 | 必需 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `role` | string | 否 | `user` | 消息角色（`user`, `assistant`, `system`） |

**示例**:
```json
{
  "role": "user"
}
```

---

## 前端表单配置

### 参数类型映射

| 参数类型 | 前端组件 | 说明 |
|----------|----------|------|
| `string` | `<input type="text">` | 文本输入框 |
| `number` | `<input type="number">` | 数字输入框 |
| `boolean` | `<input type="checkbox">` | 复选框 |
| `select` | `<select>` | 下拉选择 |
| `textarea` | `<textarea>` | 多行文本 |
| `password` | `<input type="password">` | 密码输入框 |

### 示例：LLM.Ollama 参数表单

```vue
<template>
  <div class="node-params">
    <div class="param-group">
      <label>模型</label>
      <select v-model="params.model">
        <option value="qwen2.5:7b">Qwen 2.5 (7B)</option>
        <option value="llama2">Llama 2</option>
        <option value="mistral">Mistral</option>
        <option value="codellama">Code Llama</option>
      </select>
    </div>

    <div class="param-group">
      <label>Base URL</label>
      <input type="text" v-model="params.base_url"
             placeholder="http://localhost:11434" />
    </div>

    <div class="param-group">
      <label>Temperature (0-2)</label>
      <input type="number" v-model.number="params.temperature"
             min="0" max="2" step="0.1" />
    </div>

    <div class="param-group">
      <label>最大重试次数</label>
      <input type="number" v-model.number="params.max_retries"
             min="1" max="10" />
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const params = ref({
  model: 'qwen2.5:7b',
  base_url: 'http://localhost:11434',
  temperature: 0.7,
  max_retries: 3
})
</script>
```

---

## 参数校验规则

### 1. 必需参数校验

```javascript
function validateParams(nodeType, params) {
  const requiredParams = {
    'LLM.DeepSeek': ['api_key'],
    'LLM.Anthropic': ['api_key'],
    'Input.Text': ['text']
  }

  const required = requiredParams[nodeType] || []

  for (const param of required) {
    if (!params[param]) {
      throw new Error(`参数 ${param} 是必需的`)
    }
  }
}
```

### 2. 参数范围校验

```javascript
function validateParamRanges(params) {
  if (params.temperature !== undefined) {
    if (params.temperature < 0 || params.temperature > 2) {
      throw new Error('temperature 必须在 0-2 之间')
    }
  }

  if (params.max_retries !== undefined) {
    if (params.max_retries < 1 || params.max_retries > 10) {
      throw new Error('max_retries 必须在 1-10 之间')
    }
  }
}
```

---

## 安全建议

### 1. API Key 管理

**❌ 不要**:
```json
{
  "api_key": "sk-xxxxxxxxxxxxx"  // 不要硬编码在工作流中
}
```

**✅ 推荐**:
```json
{
  "api_key": "${env.DEEPSEEK_API_KEY}"  // 使用环境变量
}
```

或者在后端配置：
```go
// 从环境变量读取
apiKey := os.Getenv("DEEPSEEK_API_KEY")
```

### 2. 敏感参数加密

在数据库中存储工作流时，加密敏感参数：

```go
func encryptSensitiveParams(params map[string]any) {
    sensitiveKeys := []string{"api_key", "password", "token"}

    for _, key := range sensitiveKeys {
        if val, ok := params[key].(string); ok {
            params[key] = encrypt(val)
        }
    }
}
```

---

## 总结

- ✅ 支持 3 种 LLM 提供商（Ollama, DeepSeek, Anthropic）
- ✅ 每个节点都有清晰的参数定义
- ✅ 支持常用参数（model, temperature, max_retries, base_url, api_key）
- ✅ 前端可以根据参数定义自动生成表单
- ✅ 提供参数校验和安全建议
