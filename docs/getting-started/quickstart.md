# 快速开始指南

## 前置要求

1. **Go 1.25+** 已安装
2. **Ollama** 服务（用于 embedding，即使使用 DeepSeek 也需要）
3. **Qdrant** 向量数据库

## 安装步骤

### 1. 启动依赖服务

#### 启动 Qdrant
```bash
docker-compose up -d
```

#### 启动 Ollama
```bash
ollama serve
```

#### 拉取所需模型
```bash
# Embedding 模型（必需）
ollama pull nomic-embed-text

# 如果使用 Ollama 作为聊天模型
ollama pull gemma3:12b
```

### 2. 配置项目

#### 选项 A: 使用 Ollama（本地）

```bash
# 复制配置文件
cp config.example.yaml config.yaml

# 配置已包含默认值，可以直接使用
```

#### 选项 B: 使用 DeepSeek（云端 API）

```bash
# 复制 DeepSeek 配置文件
cp config.deepseek.example.yaml config.yaml

# 编辑配置文件，填入你的 API Key
nano config.yaml
```

在 `config.yaml` 中修改：
```yaml
deepseek:
  api_key: "sk-your-actual-api-key-here"
```

### 3. 编译并运行

```bash
# 编译
go build -o chat.exe ./cmd/chat

# 运行
./chat.exe
```

## 使用示例

### 基本对话

```
请输入你的 UID（第一行输入将作为用户ID）：
user123

AI Agent（user=user123，输入 exit 退出）

你：你好，我是张三
AI：你好张三！很高兴认识你...

你：我喜欢蓝色
AI：好的，我记住了你喜欢蓝色...

你：exit
```

### 调试模式

在配置文件中启用调试模式：

```yaml
debug: true
```

然后运行：
```bash
./chat.exe
```

调试模式会显示：
- 发送给 LLM 的完整内容
- LLM 的思考过程
- 记忆提取和存储的详细信息

### 使用不同配置文件

```bash
# 开发环境
./chat.exe -config config.dev.yaml

# 生产环境
./chat.exe -config config.prod.yaml
```

## 配置选项

### 使用 Ollama

```yaml
provider: ollama

ollama:
  base_url: http://127.0.0.1:11434
  chat_model: gemma3:12b
  embed_model: nomic-embed-text
```

### 使用 DeepSeek

```yaml
provider: deepseek

ollama:
  base_url: http://127.0.0.1:11434  # 仍需要用于 embedding
  embed_model: nomic-embed-text

deepseek:
  api_key: "sk-xxxxx"
  chat_model: deepseek-chat
```

## 常见问题

### Q: 如何切换模型？

编辑 `config.yaml`：
```yaml
ollama:
  chat_model: llama3  # 改为其他模型
```

然后重新运行程序。

### Q: 如何增加记忆窗口大小？

编辑 `config.yaml`：
```yaml
memory:
  window_size: 16  # 默认是 8
```

### Q: 如何使用不同的数据库？

编辑 `config.yaml`：
```yaml
database:
  path: my_memory.db
```

### Q: DeepSeek API 调用失败？

检查：
1. API Key 是否正确
2. 网络连接是否正常
3. API 配额是否充足

### Q: Ollama 连接失败？

确保 Ollama 服务正在运行：
```bash
ollama serve
```

### Q: Qdrant 连接失败？

确保 Qdrant 容器正在运行：
```bash
docker-compose up -d
docker ps  # 检查容器状态
```

## 进阶使用

### 环境变量

```bash
# 使用环境变量传递敏感信息
export DEEPSEEK_KEY="sk-xxxxx"
./chat.exe -deepseek-key $DEEPSEEK_KEY
```

### 多用户支持

系统支持多用户，每个用户的记忆是隔离的：
```
请输入你的 UID：user1
# user1 的对话和记忆

# 重新运行
请输入你的 UID：user2
# user2 的对话和记忆（与 user1 完全独立）
```

### 记忆管理

记忆存储在两个地方：
1. **SQLite** (`memory.db`): 结构化记忆
2. **Qdrant**: 语义记忆（向量）

可以通过删除数据库文件来清空记忆：
```bash
rm memory.db
# Qdrant 数据在 Docker volume 中，需要删除 volume
docker-compose down -v
docker-compose up -d
```

## 更多文档

- **CONFIG.md**: 详细的配置文件说明
- **DEEPSEEK.md**: DeepSeek API 集成指南
- **CLAUDE.md**: 项目架构和开发指南
- **IMPLEMENTATION.md**: 实现细节总结

## 获取帮助

如有问题，请查看：
1. 配置文件是否正确
2. 依赖服务是否运行
3. 使用 `-debug` 模式查看详细日志
