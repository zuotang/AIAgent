# 配置文件使用指南

## 快速开始

### 1. 创建配置文件

从示例文件复制一份配置文件：

```bash
# 使用 Ollama
cp config.example.yaml config.yaml

# 或使用 DeepSeek
cp config.deepseek.example.yaml config.yaml
```

### 2. 编辑配置文件

打开 `config.yaml` 并根据需要修改配置：

```yaml
# 选择 LLM 提供商
provider: ollama  # 或 deepseek

# 如果使用 DeepSeek，填入你的 API Key
deepseek:
  api_key: "sk-your-api-key-here"
```

### 3. 运行程序

```bash
# 使用默认配置文件 (config.yaml)
./chat.exe

# 使用指定的配置文件
./chat.exe -config my-config.yaml

# 命令行参数可以覆盖配置文件
./chat.exe -debug -timeout 120
```

## 配置文件结构

### Provider（LLM 提供商）

```yaml
provider: ollama  # 可选: ollama, deepseek
```

### Ollama 配置

```yaml
ollama:
  base_url: http://127.0.0.1:11434  # Ollama 服务地址
  chat_model: gemma3:12b             # 聊天模型
  embed_model: nomic-embed-text      # Embedding 模型
```

### DeepSeek 配置

```yaml
deepseek:
  base_url: https://api.deepseek.com/v1  # DeepSeek API 基础 URL（可自定义）
  api_key: "sk-xxxxx"                     # DeepSeek API 密钥
  chat_model: deepseek-chat               # 模型名称
```

### Qdrant 配置

```yaml
qdrant:
  base_url: http://127.0.0.1:6333  # Qdrant 服务地址
  api_key: ""                       # Qdrant API Key（可选，用于认证）
  collection: memories              # 集合名称
  top_k: 6                          # 语义检索返回数量
```

### 数据库配置

```yaml
database:
  path: memory.db  # SQLite 数据库文件路径
```

### 记忆配置

```yaml
memory:
  window_size: 8        # 短期记忆窗口大小（对话轮数）
  extractor_model: ""   # 记忆提取模型（留空则使用 chat_model）
```

### 其他配置

```yaml
debug: false   # 调试模式
timeout: 60    # 超时时间（秒）
```

## 命令行选项

只支持一个命令行参数：

```bash
# 使用默认配置文件
./chat.exe

# 使用指定配置文件
./chat.exe -config my-config.yaml
```

**所有其他配置必须在配置文件中设置。**

## 配置示例

### 示例 1: 使用 Ollama（本地部署）

```yaml
provider: ollama

ollama:
  base_url: http://127.0.0.1:11434
  chat_model: gemma3:12b
  embed_model: nomic-embed-text

qdrant:
  base_url: http://127.0.0.1:6333
  collection: memories
  top_k: 6

database:
  path: memory.db

memory:
  window_size: 8
  extractor_model: ""

debug: false
timeout: 60
```

### 示例 2: 使用 DeepSeek（云端 API）

```yaml
provider: deepseek

ollama:
  base_url: http://127.0.0.1:11434  # 仍需 Ollama 用于 embedding
  embed_model: nomic-embed-text

deepseek:
  api_key: "sk-your-api-key-here"
  chat_model: deepseek-chat

qdrant:
  base_url: http://127.0.0.1:6333
  collection: memories
  top_k: 6

database:
  path: memory.db

memory:
  window_size: 8
  extractor_model: ""

debug: false
timeout: 60
```

### 示例 3: 开发环境（启用调试）

```yaml
provider: ollama

ollama:
  base_url: http://127.0.0.1:11434
  chat_model: gemma3:12b
  embed_model: nomic-embed-text

qdrant:
  base_url: http://127.0.0.1:6333
  collection: dev_memories
  top_k: 6

database:
  path: dev_memory.db

memory:
  window_size: 4  # 更小的窗口用于测试
  extractor_model: ""

debug: true  # 启用调试输出
timeout: 120  # 更长的超时时间
```

## 安全建议

1. **不要提交包含 API 密钥的配置文件到 Git**
   - `config.yaml` 已在 `.gitignore` 中
   - 只提交 `config.example.yaml` 示例文件

2. **使用环境变量**
   ```bash
   export DEEPSEEK_KEY="sk-xxxxx"
   ./chat.exe -deepseek-key $DEEPSEEK_KEY
   ```

3. **文件权限**
   ```bash
   chmod 600 config.yaml  # 仅所有者可读写
   ```

## 故障排查

### 配置文件未找到
```
加载配置文件失败: failed to read config file: open config.yaml: no such file or directory
```
**解决方案**: 从示例文件复制一份配置文件
```bash
cp config.example.yaml config.yaml
```

### DeepSeek API 密钥缺失
```
配置验证失败: deepseek api_key is required when provider is 'deepseek'
```
**解决方案**: 在配置文件中填入有效的 API 密钥

### Ollama 连接失败
```
ollama embedding failed: ...
```
**解决方案**: 确保 Ollama 服务正在运行
```bash
ollama serve
```

### Qdrant 连接失败
```
ensure qdrant collection failed: ...
```
**解决方案**: 确保 Qdrant 服务正在运行
```bash
docker-compose up -d
```
