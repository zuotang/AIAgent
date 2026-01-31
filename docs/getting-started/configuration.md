# 配置文件使用指南

## 快速开始

### 1. 创建配置文件

从示例文件复制一份配置文件：

```bash
# 使用示例配置文件
cp config.example.yaml config.yaml
```

### 2. 编辑配置文件

打开 `config.yaml` 并根据需要修改配置。新的配置文件采用分层结构，分为 base、llm、storage、services 和 memory 五个部分。

### 3. 运行程序

```bash
# 使用默认配置文件 (config.yaml)
./chat.exe

# 使用指定的配置文件
./chat.exe -config my-config.yaml
```

## 配置文件结构

新的配置文件采用分层结构，更加清晰和易于管理：

### 基础配置（base）

```yaml
# 基础配置
base:
  provider: ollama  # LLM 提供商: ollama 或 deepseek
  debug: false      # 调试模式
  timeout: 60       # 超时时间（秒）
```

### LLM 配置（llm）

```yaml
# LLM 配置
llm:
  # Ollama 配置
  ollama:
    base_url: http://127.0.0.1:11434  # Ollama 服务地址
    chat_model: gemma3:12b             # 聊天模型
    embed_model: nomic-embed-text      # Embedding 模型
  
  # DeepSeek 配置
  deepseek:
    base_url: https://api.deepseek.com/v1  # DeepSeek API 基础 URL
    api_key: ""  # 在这里填入你的 DeepSeek API Key
    chat_model: deepseek-chat             # 模型名称
  
  # RAG 配置
  rag:
    collection: "knowledge"      # 知识库集合名称
    chunk_size: 1000        # 分块大小
    chunk_overlap: 200       # 分块重叠大小
    chunking_strategy: "tokens"  # 分块策略: tokens, semantic
```

### 存储配置（storage）

```yaml
# 存储配置
storage:
  # SQLite 数据库配置
  database:
    path: memory.db  # SQLite 数据库文件路径
  
  # Qdrant 向量数据库配置
  qdrant:
    base_url: http://127.0.0.1:6333  # Qdrant 服务地址
    api_key: ""  # Qdrant API Key（可选，如果 Qdrant 启用了认证）
    collection: memories  # 记忆集合名称
    top_k: 6  # 语义检索返回的最大结果数
```

### 服务配置（services）

```yaml
# 服务配置
services:
  # API 服务配置
  api:
    port: 8080  # API 服务端口
```

### 记忆配置（memory）

```yaml
# 记忆配置
memory:
  window_size: 8            # 短期记忆窗口大小（对话轮数）
  extractor_model: ""        # 记忆提取模型（留空则使用 chat_model）
  enable_smart_trigger: true  # 启用智能触发（节省 70-80% token）
  trigger_method: "conservative"  # 触发方法: keyword, llm, conservative
  classifier_model: "qwen2.5:0.5b"  # LLM 分类器模型（仅 llm 方法使用）
  min_message_length: 10      # 最小消息长度
  include_history_context: false  # 提取时包含历史上下文（false 避免重复提取）
```

## 命令行选项

```bash
# 使用默认配置文件
./chat.exe

# 使用指定配置文件
./chat.exe -config my-config.yaml

# 显示记忆访问统计
./chat.exe -stats

# 查看指定用户的统计
./chat.exe -stats -user username
```

**所有其他配置必须在配置文件中设置。**

## 配置示例

### 示例 1: 使用 Ollama（本地部署）

```yaml
# 基础配置
base:
  provider: ollama
  debug: false
  timeout: 60

# LLM 配置
llm:
  ollama:
    base_url: http://127.0.0.1:11434
    chat_model: gemma3:12b
    embed_model: nomic-embed-text
  deepseek:
    base_url: https://api.deepseek.com/v1
    api_key: ""
    chat_model: deepseek-chat
  rag:
    collection: "knowledge"
    chunk_size: 1000
    chunk_overlap: 200
    chunking_strategy: "tokens"

# 存储配置
storage:
  database:
    path: memory.db
  qdrant:
    base_url: http://127.0.0.1:6333
    api_key: ""
    collection: memories
    top_k: 6

# 服务配置
services:
  api:
    port: 8080

# 记忆配置
memory:
  window_size: 8
  extractor_model: ""
  enable_smart_trigger: true
  trigger_method: "conservative"
  classifier_model: "qwen2.5:0.5b"
  min_message_length: 10
  include_history_context: false
```

### 示例 2: 使用 DeepSeek（云端 API）

```yaml
# 基础配置
base:
  provider: deepseek
  debug: false
  timeout: 60

# LLM 配置
llm:
  ollama:
    base_url: http://127.0.0.1:11434  # 仍需 Ollama 用于 embedding
    chat_model: gemma3:12b
    embed_model: nomic-embed-text
  deepseek:
    base_url: https://api.deepseek.com/v1
    api_key: "sk-your-api-key-here"  # 填入你的 DeepSeek API Key
    chat_model: deepseek-chat
  rag:
    collection: "knowledge"
    chunk_size: 1000
    chunk_overlap: 200
    chunking_strategy: "tokens"

# 存储配置
storage:
  database:
    path: memory.db
  qdrant:
    base_url: http://127.0.0.1:6333
    api_key: ""
    collection: memories
    top_k: 6

# 服务配置
services:
  api:
    port: 8080

# 记忆配置
memory:
  window_size: 8
  extractor_model: ""
  enable_smart_trigger: true
  trigger_method: "conservative"
  classifier_model: "qwen2.5:0.5b"
  min_message_length: 10
  include_history_context: false
```

### 示例 3: 开发环境（启用调试）

```yaml
# 基础配置
base:
  provider: ollama
  debug: true  # 启用调试输出
  timeout: 120  # 更长的超时时间

# LLM 配置
llm:
  ollama:
    base_url: http://127.0.0.1:11434
    chat_model: gemma3:12b
    embed_model: nomic-embed-text
  deepseek:
    base_url: https://api.deepseek.com/v1
    api_key: ""
    chat_model: deepseek-chat
  rag:
    collection: "dev_knowledge"
    chunk_size: 1000
    chunk_overlap: 200
    chunking_strategy: "tokens"

# 存储配置
storage:
  database:
    path: dev_memory.db  # 开发环境数据库
  qdrant:
    base_url: http://127.0.0.1:6333
    api_key: ""
    collection: dev_memories  # 开发环境集合
    top_k: 6

# 服务配置
services:
  api:
    port: 8080

# 记忆配置
memory:
  window_size: 4  # 更小的窗口用于测试
  extractor_model: ""
  enable_smart_trigger: true
  trigger_method: "conservative"
  classifier_model: "qwen2.5:0.5b"
  min_message_length: 10
  include_history_context: false
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
