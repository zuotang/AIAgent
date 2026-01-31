# 配置更新说明

## 最新更新 (2026-01-31)

### 配置系统重大重构

本次更新对配置系统进行了重大重构，将 `embedding`、`extractor` 和 `classifier` 从原有配置中独立出来，使配置更加灵活和模块化。

#### 1. Embedding 配置独立化

**变更前**：
```yaml
llm:
  ollama:
    embed_model: nomic-embed-text
```

**变更后**：
```yaml
embedding:
  provider: ollama
  base_url: http://127.0.0.1:11434
  model: nomic-embed-text
  api_key: ""
  batch_size: 10
  dimensions: 0
```

**优势**：
- 可以使用独立的 embedding 服务（如 OpenAI embeddings）
- 支持不同的 base_url 和 API key
- 增加了批量处理和维度配置

#### 2. 记忆提取器配置独立化

**变更前**：
```yaml
memory:
  extractor_model: ""
```

**变更后**：
```yaml
extractor:
  provider: ollama
  base_url: http://127.0.0.1:11434
  model: ""
  api_key: ""
  temperature: 0.1
  max_retries: 3
```

**优势**：
- 可以使用不同的服务提供商
- 独立的 base_url 配置
- 支持温度和重试次数配置

#### 3. 分类器配置独立化

**变更前**：
```yaml
memory:
  classifier_model: "qwen2.5:0.5b"
```

**变更后**：
```yaml
classifier:
  provider: ollama
  base_url: http://127.0.0.1:11434
  model: qwen2.5:0.5b
  api_key: ""
  temperature: 0.0
  timeout: 10
```

#### 4. 新增性能配置节

```yaml
performance:
  max_concurrent_requests: 10
  request_timeout: 120
  enable_cache: true
  cache_ttl: 3600
```

#### 5. 记忆配置增强

新增配置项：
```yaml
memory:
  min_confidence: 0.65  # 最小置信度阈值
  max_memories_per_extraction: 20  # 每次提取的最大记忆数
```

#### 6. API 服务配置增强

新增配置项：
```yaml
services:
  api:
    host: 0.0.0.0  # 监听地址
    cors_enabled: true  # CORS 支持
```

### 使用场景示例

#### 场景 1: 全本地部署
```yaml
base:
  provider: ollama

embedding:
  provider: ollama
  base_url: http://127.0.0.1:11434
  model: nomic-embed-text

extractor:
  provider: ollama
  model: gemma3:4b  # 使用小模型

classifier:
  provider: ollama
  model: qwen2.5:0.5b
```

#### 场景 2: 混合云服务
```yaml
base:
  provider: deepseek

llm:
  deepseek:
    api_key: "your-key"

embedding:
  provider: ollama  # 本地 embedding
  base_url: http://127.0.0.1:11434
  model: nomic-embed-text

extractor:
  provider: ollama  # 本地提取器
  model: qwen2.5:7b
```

#### 场景 3: 使用专业 Embedding 服务
```yaml
embedding:
  provider: openai
  base_url: https://api.openai.com/v1
  model: text-embedding-3-small
  api_key: "your-openai-key"
  dimensions: 1536
```

### 迁移指南

详细的迁移指南请参考 `CONFIG_MIGRATION.md`

---

## 历史更新

### 新增功能 (之前版本)

### 1. Qdrant API Key 支持

现在可以为 Qdrant 向量数据库配置 API Key 认证：

```yaml
qdrant:
  base_url: http://127.0.0.1:6333
  api_key: "your-qdrant-api-key"  # 可选，如果 Qdrant 启用了认证
  collection: memories
  top_k: 6
```

**使用场景**：
- 生产环境中的 Qdrant 实例通常启用了认证
- 云端托管的 Qdrant 服务需要 API Key

**注意**：
- 如果 Qdrant 没有启用认证，可以留空或不填写此字段
- API Key 会通过 HTTP Header `api-key` 发送

### 2. DeepSeek 自定义基础 URL

现在可以为 DeepSeek 配置自定义的基础 URL：

```yaml
deepseek:
  base_url: https://api.deepseek.com/v1  # 可自定义
  api_key: "sk-xxxxx"
  chat_model: deepseek-chat
```

**使用场景**：
- 使用代理服务
- 使用自建的 DeepSeek 兼容服务
- 使用不同区域的 API 端点

**默认值**：
- 如果不配置，默认使用 `https://api.deepseek.com/v1`

## 配置示例

### 完整的 DeepSeek + Qdrant 认证配置

```yaml
provider: deepseek

ollama:
  base_url: http://127.0.0.1:11434
  embed_model: nomic-embed-text

deepseek:
  base_url: https://api.deepseek.com/v1
  api_key: "sk-your-deepseek-key"
  chat_model: deepseek-chat

qdrant:
  base_url: https://your-qdrant-instance.com
  api_key: "your-qdrant-api-key"
  collection: memories
  top_k: 6
```

### 使用代理的 DeepSeek 配置

```yaml
deepseek:
  base_url: https://your-proxy.com/v1  # 自定义代理地址
  api_key: "sk-xxxxx"
  chat_model: deepseek-chat
```

## 运行时日志

程序启动时会显示配置信息：

```
使用 DeepSeek API (base_url: https://api.deepseek.com/v1, model: deepseek-chat)
使用 Ollama 进行 Embedding (base_url: http://127.0.0.1:11434, model: nomic-embed-text)
使用 Qdrant (base_url: http://127.0.0.1:6333, 已启用 API Key 认证)
```

## 迁移指南

### 从旧配置迁移

如果你使用的是旧版本的配置文件，需要添加以下字段：

1. **DeepSeek 配置**：
```yaml
deepseek:
  base_url: https://api.deepseek.com/v1  # 新增
  api_key: "sk-xxxxx"
  chat_model: deepseek-chat
```

2. **Qdrant 配置**：
```yaml
qdrant:
  base_url: http://127.0.0.1:6333
  api_key: ""  # 新增（可选）
  collection: memories
  top_k: 6
```

### 向后兼容性

- 如果不配置 `deepseek.base_url`，会自动使用默认值
- 如果不配置 `qdrant.api_key`，不会发送认证头
- 旧的配置文件仍然可以正常工作

## 安全建议

1. **保护 API Key**：
   - 不要将包含真实 API Key 的配置文件提交到 Git
   - 使用环境变量或密钥管理服务

2. **使用 HTTPS**：
   - 生产环境中的 Qdrant 应该使用 HTTPS
   - DeepSeek API 默认使用 HTTPS

3. **最小权限原则**：
   - Qdrant API Key 应该只授予必要的权限
   - 定期轮换 API Key
