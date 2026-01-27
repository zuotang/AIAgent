# DeepSeek API 集成说明

## 概述

本项目现已支持使用 DeepSeek 官方 API 作为 LLM 提供商。您可以选择使用 Ollama（本地部署）或 DeepSeek API（云端服务）。

## 使用方法

### 1. 使用 Ollama（默认）

```bash
./chat.exe -provider ollama -chat gemma3:12b
```

### 2. 使用 DeepSeek API

```bash
./chat.exe -provider deepseek -deepseek-key YOUR_API_KEY -chat deepseek-chat
```

## 命令行参数

### 新增参数

- `-provider`: LLM 提供商，可选值：`ollama`（默认）或 `deepseek`
- `-deepseek-key`: DeepSeek API 密钥（当 provider=deepseek 时必需）

### 其他参数

- `-chat`: 聊天模型名称
  - Ollama: `gemma3:12b`, `llama3`, 等
  - DeepSeek: `deepseek-chat`, `deepseek-coder`, 等
- `-extractor`: 记忆提取模型（默认与 chat 模型相同）
- `-embed`: Embedding 模型（始终使用 Ollama，因为 DeepSeek 不支持 embedding）
- `-debug`: 启用调试输出

## 重要说明

1. **Embedding 功能**: DeepSeek API 不支持 embedding，因此即使使用 DeepSeek 作为聊天模型，embedding 功能仍然需要 Ollama 服务。

2. **混合使用**: 您可以使用 DeepSeek 进行聊天和记忆提取，同时使用 Ollama 进行 embedding：
   ```bash
   ./chat.exe -provider deepseek -deepseek-key YOUR_KEY -ollama http://localhost:11434 -embed nomic-embed-text
   ```

3. **API 密钥安全**: 建议通过环境变量传递 API 密钥：
   ```bash
   export DEEPSEEK_KEY="your-api-key"
   ./chat.exe -provider deepseek -deepseek-key $DEEPSEEK_KEY
   ```

## 完整示例

### 使用 DeepSeek + Ollama Embedding

```bash
# 确保 Ollama 服务正在运行（用于 embedding）
ollama serve

# 确保 Qdrant 服务正在运行
docker-compose up -d

# 运行聊天程序
./chat.exe \
  -provider deepseek \
  -deepseek-key sk-xxxxxxxxxxxxx \
  -chat deepseek-chat \
  -ollama http://localhost:11434 \
  -embed nomic-embed-text \
  -qdrant http://localhost:6333 \
  -debug
```

## DeepSeek 模型选项

- `deepseek-chat`: 通用对话模型（推荐）
- `deepseek-coder`: 代码专用模型

更多模型信息请参考 [DeepSeek API 文档](https://platform.deepseek.com/docs)

## 故障排查

### 错误：DeepSeek API key is required
确保使用 `-deepseek-key` 参数提供有效的 API 密钥。

### 错误：ollama embedding failed
即使使用 DeepSeek，也需要 Ollama 服务用于 embedding。确保：
1. Ollama 服务正在运行
2. 已安装 embedding 模型：`ollama pull nomic-embed-text`

### 错误：deepseek chat http 401
API 密钥无效或已过期，请检查您的 DeepSeek API 密钥。
