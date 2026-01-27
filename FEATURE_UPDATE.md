# 功能更新总结

## 新增功能

### 1. ✅ Qdrant API Key 认证支持

**问题**：向量数据库需要密码/API Key 认证

**解决方案**：
- 在配置文件中添加 `qdrant.api_key` 字段
- 修改 `QdrantStore` 结构，添加 API Key 支持
- 在所有 HTTP 请求中自动添加 `api-key` 认证头

**配置示例**：
```yaml
qdrant:
  base_url: http://127.0.0.1:6333
  api_key: "your-qdrant-api-key"  # 新增字段
  collection: memories
  top_k: 6
```

**代码修改**：
- `internal/config/config.go`: 添加 `APIKey` 字段到 `QdrantConfig`
- `internal/rag/qdrant.go`:
  - 修改 `QdrantStore` 结构添加 `APIKey` 字段
  - 修改 `doWithRetry` 函数，在请求中添加认证头
  - 更新 `NewQdrantStore` 和 `NewStoreFromOllama` 函数签名
- `cmd/chat/main.go`: 传递 API Key 到 Qdrant Store

### 2. ✅ DeepSeek 自定义基础 URL

**问题**：DeepSeek 需要支持自定义基础 URL（因为聊天走 DeepSeek，向量走 Ollama）

**解决方案**：
- 在配置文件中添加 `deepseek.base_url` 字段
- 修改 `DeepSeekClient` 构造函数接受 baseURL 参数
- 设置默认值为 `https://api.deepseek.com/v1`

**配置示例**：
```yaml
deepseek:
  base_url: https://api.deepseek.com/v1  # 新增字段，可自定义
  api_key: "sk-xxxxx"
  chat_model: deepseek-chat
```

**代码修改**：
- `internal/config/config.go`:
  - 添加 `BaseURL` 字段到 `DeepSeekConfig`
  - 在 `setDefaults` 中设置默认值
- `internal/models/deepseek.go`: 修改 `NewDeepSeek` 函数接受 `baseURL` 参数
- `cmd/chat/main.go`: 传递 BaseURL 到 DeepSeek 客户端

## 使用场景

### 场景 1: 生产环境 Qdrant 认证

```yaml
provider: deepseek

qdrant:
  base_url: https://your-qdrant-cloud.com
  api_key: "prod-qdrant-key-xxxxx"
  collection: memories
```

### 场景 2: 使用代理访问 DeepSeek

```yaml
provider: deepseek

deepseek:
  base_url: https://your-proxy.com/v1
  api_key: "sk-xxxxx"
  chat_model: deepseek-chat
```

### 场景 3: 混合架构（DeepSeek 聊天 + Ollama Embedding）

```yaml
provider: deepseek

ollama:
  base_url: http://127.0.0.1:11434
  embed_model: nomic-embed-text

deepseek:
  base_url: https://api.deepseek.com/v1
  api_key: "sk-xxxxx"
  chat_model: deepseek-chat

qdrant:
  base_url: http://127.0.0.1:6333
  api_key: ""  # 本地开发无需认证
  collection: memories
```

## 运行时日志

程序启动时会显示详细的配置信息：

```
使用 DeepSeek API (base_url: https://api.deepseek.com/v1, model: deepseek-chat)
使用 Ollama 进行 Embedding (base_url: http://127.0.0.1:11434, model: nomic-embed-text)
使用 Qdrant (base_url: http://127.0.0.1:6333, 已启用 API Key 认证)
```

## 向后兼容性

✅ **完全向后兼容**

- 旧的配置文件仍然可以正常工作
- 新字段都是可选的，有合理的默认值
- 如果不配置 `qdrant.api_key`，不会发送认证头
- 如果不配置 `deepseek.base_url`，使用默认值

## 文件修改清单

### 核心代码
- ✅ `internal/config/config.go`: 配置结构更新
- ✅ `internal/models/deepseek.go`: DeepSeek 客户端支持自定义 URL
- ✅ `internal/rag/qdrant.go`: Qdrant 客户端支持 API Key 认证
- ✅ `cmd/chat/main.go`: 主程序集成新配置

### 配置文件
- ✅ `config.example.yaml`: 更新示例配置
- ✅ `config.deepseek.example.yaml`: 更新 DeepSeek 示例配置

### 文档
- ✅ `CONFIG.md`: 更新配置说明
- ✅ `CONFIG_UPDATE.md`: 新增配置更新说明
- ✅ `FEATURE_UPDATE.md`: 本文档

## 测试结果

✅ **编译成功**
```bash
go build -o chat.exe ./cmd/chat
# 无错误
```

## 下一步建议

1. **测试 Qdrant 认证**：
   ```bash
   # 启动带认证的 Qdrant
   docker run -p 6333:6333 qdrant/qdrant --api-key your-test-key

   # 配置并测试
   ./chat.exe -config config.yaml
   ```

2. **测试 DeepSeek 自定义 URL**：
   ```yaml
   deepseek:
     base_url: https://your-custom-endpoint.com/v1
     api_key: "sk-xxxxx"
   ```

3. **更新部署文档**：
   - 添加 Qdrant 认证配置说明
   - 添加代理配置示例

## 安全提示

⚠️ **重要**：
- 不要将包含真实 API Key 的配置文件提交到 Git
- `config.yaml` 已在 `.gitignore` 中
- 生产环境建议使用环境变量或密钥管理服务
- Qdrant API Key 应该定期轮换
