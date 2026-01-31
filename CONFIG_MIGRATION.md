# 配置文件迁移指南

## 从旧版本迁移到新版本

### 主要变更

新版本的配置文件将 `embedding`、`extractor` 和 `classifier` 独立出来，使配置更加灵活和清晰。

### 变更对照表

#### 1. Embedding 配置

**旧配置**：
```yaml
llm:
  ollama:
    embed_model: nomic-embed-text
```

**新配置**：
```yaml
embedding:
  provider: ollama
  base_url: http://127.0.0.1:11434
  model: nomic-embed-text
  api_key: ""
  batch_size: 10
  dimensions: 0
```

#### 2. 记忆提取器配置

**旧配置**：
```yaml
memory:
  extractor_model: ""  # 留空则使用 chat_model
```

**新配置**：
```yaml
extractor:
  provider: ollama  # 或 deepseek
  base_url: http://127.0.0.1:11434
  model: ""  # 留空则使用主 chat_model
  api_key: ""
  temperature: 0.1
  max_retries: 3
```

#### 3. 分类器配置

**旧配置**：
```yaml
memory:
  classifier_model: "qwen2.5:0.5b"
```

**新配置**：
```yaml
classifier:
  provider: ollama
  base_url: http://127.0.0.1:11434
  model: qwen2.5:0.5b
  api_key: ""
  temperature: 0.0
  timeout: 10
```

#### 4. 记忆配置增强

**新增配置项**：
```yaml
memory:
  min_confidence: 0.65  # 最小置信度
  max_memories_per_extraction: 20  # 每次提取的最大记忆数
```

#### 5. API 服务配置增强

**新增配置项**：
```yaml
services:
  api:
    host: 0.0.0.0  # 监听地址
    cors_enabled: true  # 启用 CORS
```

#### 6. 性能配置（全新）

**新增配置节**：
```yaml
performance:
  max_concurrent_requests: 10  # 最大并发请求数
  request_timeout: 120  # 请求超时（秒）
  enable_cache: true    # 启用缓存
  cache_ttl: 3600       # 缓存过期时间（秒）
```

### 迁移步骤

1. **备份现有配置**：
   ```bash
   cp config.yaml config.yaml.backup
   ```

2. **复制新的配置模板**：
   ```bash
   cp config.example.yaml config.yaml
   ```

3. **迁移旧配置值**：
   - 将旧的 `llm.ollama.embed_model` 迁移到 `embedding.model`
   - 将旧的 `memory.extractor_model` 迁移到 `extractor.model`
   - 将旧的 `memory.classifier_model` 迁移到 `classifier.model`
   - 保留其他配置项不变

4. **验证配置**：
   ```bash
   ./chat.exe -config config.yaml
   ```

### 向后兼容性

新版本配置系统具有良好的向后兼容性：

- 如果 `embedding` 配置为空，将使用默认的 Ollama embedding 配置
- 如果 `extractor` 配置为空，将使用主 LLM 配置
- 如果 `classifier` 配置为空，将使用默认的小模型配置
- 所有新增的配置项都有合理的默认值

### 配置优势

新的配置结构带来以下优势：

1. **灵活性**：可以为不同功能使用不同的服务提供商
   - 主对话使用 DeepSeek
   - Embedding 使用 Ollama 本地模型
   - 记忆提取使用更小的模型节省成本

2. **独立性**：每个功能模块都有独立的 base_url 和 api_key
   - 可以使用专门的 embedding 服务（如 OpenAI embeddings）
   - 可以使用不同的 API endpoint

3. **可配置性**：更多的细粒度控制
   - Extractor 的 temperature 和 max_retries
   - Classifier 的 timeout
   - Embedding 的 batch_size 和 dimensions

4. **性能优化**：新增的 performance 配置节
   - 并发控制
   - 缓存策略
   - 超时管理

### 示例配置场景

#### 场景 1：全部使用 Ollama（本地部署）

```yaml
base:
  provider: ollama

llm:
  ollama:
    base_url: http://127.0.0.1:11434
    chat_model: gemma3:12b

embedding:
  provider: ollama
  base_url: http://127.0.0.1:11434
  model: nomic-embed-text

extractor:
  provider: ollama
  base_url: http://127.0.0.1:11434
  model: gemma3:12b

classifier:
  provider: ollama
  base_url: http://127.0.0.1:11434
  model: qwen2.5:0.5b
```

#### 场景 2：混合使用（主对话用 DeepSeek，其他用 Ollama）

```yaml
base:
  provider: deepseek

llm:
  deepseek:
    base_url: https://api.deepseek.com/v1
    api_key: "your-api-key"
    chat_model: deepseek-chat

embedding:
  provider: ollama
  base_url: http://127.0.0.1:11434
  model: nomic-embed-text

extractor:
  provider: ollama
  base_url: http://127.0.0.1:11434
  model: qwen2.5:7b  # 使用更小的模型节省成本

classifier:
  provider: ollama
  base_url: http://127.0.0.1:11434
  model: qwen2.5:0.5b
```

#### 场景 3：使用专门的 Embedding 服务

```yaml
embedding:
  provider: openai
  base_url: https://api.openai.com/v1
  model: text-embedding-3-small
  api_key: "your-openai-api-key"
  dimensions: 1536
```

### 常见问题

**Q: 我的旧配置文件还能用吗？**
A: 可以，但建议迁移到新格式以获得更多功能。系统会使用默认值填充缺失的配置。

**Q: 如何验证配置是否正确？**
A: 运行程序时会在日志中显示加载的配置信息，检查是否符合预期。

**Q: 可以为不同功能使用不同的 Ollama 实例吗？**
A: 可以，只需在各个配置节中指定不同的 base_url。

**Q: 如何禁用某些功能？**
A: 通过 `memory.enable_smart_trigger` 等开关控制功能启用/禁用。

### 技术支持

如有问题，请查看：
- `config.example.yaml` - 完整的配置示例
- `README.md` - 项目文档
- GitHub Issues - 提交问题和建议
