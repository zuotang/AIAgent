# Agent-Langchain 项目文档

## 项目概述

Agent-Langchain 是一个基于 Go 语言开发的智能代理系统，结合了大语言模型（LLM）、向量数据库和结构化存储技术，实现了一个具有记忆能力的聊天代理。该系统能够：

- 与用户进行自然语言交互
- 提取和存储对话中的关键信息
- 基于历史记忆生成更连贯的回应
- 提供详细的调试功能

## 技术架构

### 核心组件

1. **聊天接口**：基于 Ollama 客户端实现与 LLM 的交互
2. **记忆管理系统**：
   - SQLite：存储结构化记忆
   - Qdrant：存储语义记忆（向量嵌入）
3. **调试系统**：提供详细的运行时日志和 API 交互信息

### 技术栈

- **编程语言**：Go
- **LLM 接口**：Ollama API
- **结构化存储**：SQLite
- **向量存储**：Qdrant
- **HTTP 客户端**：标准库 `net/http`
- **JSON 处理**：标准库 `encoding/json`

## 目录结构

```
.
├── chat.exe             # 编译后的可执行文件
├── cmd/
│   ├── chat/            # 聊天程序入口
│   │   └── main.go      # 主程序文件
│   └── ingest/          # 数据导入工具
│       └── main.go
├── data/
│   └── fortune.txt      # 示例数据
├── go.mod               # Go 模块文件
├── go.sum               # Go 依赖校验文件
├── internal/
│   ├── memory/          # 记忆管理模块
│   │   ├── sqlite.go    # SQLite 存储实现
│   │   └── types.go     # 数据类型定义
│   ├── models/          # LLM 模型接口
│   │   └── ollama.go    # Ollama 客户端实现
│   ├── profile/         # 用户配置文件
│   │   └── profile.go
│   └── rag/             # 检索增强生成
│       └── qdrant.go    # Qdrant 向量存储实现
└── memory.db            # SQLite 数据库文件
```

## 核心功能模块

### 1. 对话管理

- **用户输入处理**：读取用户输入并进行基本处理
- **LLM 交互**：通过 Ollama API 发送请求并接收响应
- **上下文管理**：维护对话历史，确保 LLM 能够参考之前的交互

### 2. 记忆管理系统

- **SQLite 存储**：
  - 存储结构化的用户信息和提取的记忆
  - 支持配置文件的加载和保存
  - 实现记忆的增删改查操作

- **Qdrant 向量存储**：
  - 存储文本的向量嵌入
  - 支持基于相似度的记忆检索
  - 实现向量数据的批量插入和更新

### 3. 记忆提取与学习

- **关键信息提取**：从对话中提取重要信息
- **语义嵌入**：将文本转换为向量表示
- **记忆存储**：将提取的信息分别存储到 SQLite 和 Qdrant
- **记忆检索**：根据当前对话内容检索相关记忆

### 4. 调试系统

- **命令行标志**：通过 `-debug` 开启调试模式
- **详细日志**：
  - 显示发送给 LLM 的内容
  - 显示 LLM 的思考过程
  - 显示 Ollama API 的请求和响应
  - 显示记忆提取和存储的详细信息

## 配置与使用

### 命令行参数

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `-ollama` | Ollama API 地址 | http://localhost:11434 |
| `-qdrant` | Qdrant API 地址 | http://localhost:6334 |
| `-debug` | 启用调试输出 | false |
| `-model` | 聊天模型名称 | llama3 |
| `-emb` | 嵌入模型名称 | llama3 |
| `-win` | 短期记忆回合数 | 8 |

### 构建项目

```bash
go build -o chat.exe ./cmd/chat
```

### 运行项目

#### 正常模式

```bash
./chat.exe
```

#### 调试模式

```bash
./chat.exe -debug
```

### 工作流程

1. **初始化**：加载配置，初始化 Ollama 客户端和存储系统
2. **用户输入**：读取用户输入的 UID 和对话内容
3. **记忆检索**：从 SQLite 和 Qdrant 检索相关记忆
4. **LLM 交互**：将用户输入和检索到的记忆发送给 LLM
5. **记忆提取**：从 LLM 的响应中提取关键信息
6. **记忆存储**：将提取的信息存储到 SQLite 和 Qdrant
7. **循环**：重复步骤 2-6，直到用户退出

## 代码结构与核心 API

### 1. Ollama 客户端

**文件**：`internal/models/ollama.go`

**核心结构**：

```go
type Client struct {
    BaseURL  string
    ChatModel string
    EmbModel  string
    HTTP      *http.Client
    Debug     bool
}
```

**核心方法**：
- `New(baseURL, chatModel, embModel string) *Client`：创建新的 Ollama 客户端
- `SetDebug(debug bool)`：设置调试模式
- `Chat(ctx context.Context, messages []ChatMessage) (string, error)`：发送聊天请求
- `Embed(ctx context.Context, text string) ([]float64, error)`：获取文本的向量嵌入

### 2. SQLite 存储

**文件**：`internal/memory/sqlite.go`

**核心结构**：

```go
type Store struct {
    db *sql.DB
}
```

**核心方法**：
- `New(dbPath string) (*Store, error)`：创建新的 SQLite 存储
- `Close() error`：关闭存储
- `LoadProfile(profileType, key string) (*Profile, error)`：加载配置文件
- `SaveProfile(profile *Profile) error`：保存配置文件
- `UpsertExtractedMemories(memories []ExtractedMemory) error`：更新或插入提取的记忆

### 3. Qdrant 存储

**文件**：`internal/rag/qdrant.go`

**核心结构**：

```go
type QdrantStore struct {
    BaseURL    string
    Collection string
    Embedder   *models.Client
    HTTP       *http.Client
}
```

**核心方法**：
- `NewQdrantStore(baseURL, collection string, embedder *models.Client) *QdrantStore`：创建新的 Qdrant 存储
- `EnsureCollection() error`：确保集合存在
- `SimilaritySearch(query string, topK int) ([]string, error)`：基于相似度搜索
- `UpsertTexts(texts []string) error`：插入或更新文本

## 部署与依赖

### 依赖服务

1. **Ollama**：
   - 安装：https://ollama.com/download
   - 启动：`ollama serve`

2. **Qdrant**：
   - 安装：https://qdrant.tech/documentation/quick-start/
   - 启动：`qdrant`

### Go 依赖

项目使用 Go 模块管理依赖，主要依赖包括：

- 标准库：`net/http`, `encoding/json`, `database/sql` 等
- SQLite 驱动：`modernc.org/sqlite`
- LangChain Go：`github.com/tmc/langchaingo`

## 扩展与开发

### 添加新功能

1. **添加新模型**：
   - 在 `internal/models/` 目录下创建新的模型实现
   - 实现 `Chat` 和 `Embed` 方法

2. **添加新存储**：
   - 在 `internal/memory/` 或 `internal/rag/` 目录下创建新的存储实现
   - 实现相应的存储和检索方法

3. **添加新功能模块**：
   - 在 `internal/` 目录下创建新的功能模块
   - 在 `main.go` 中集成新模块

### 调试技巧

- 使用 `-debug` 标志开启详细日志
- 检查 Ollama 和 Qdrant 的日志
- 使用 `go run ./cmd/chat -debug` 直接运行项目，无需编译

## 示例输出

### 正常模式

```
请输入你的 UID（第一行输入将作为用户ID）：
123
用户: 你好，我是张三
AI: 你好张三！很高兴认识你。我是一个人工智能助手，可以帮助你回答问题、提供信息或进行聊天。有什么我可以帮助你的吗？
用户:
```

### 调试模式

```
请输入你的 UID（第一行输入将作为用户ID）：
123
用户: 你好，我是张三
[DEBUG] 发送给 LLM 的内容:
你是一个专业的个人记忆提取助手，需要从用户和AI的对话历史中提取关键信息，将其转化为结构化的记忆。

对话历史:
用户: 你好，我是张三
AI: 你好张三！很高兴认识你。我是一个人工智能助手，可以帮助你回答问题、提供信息或进行聊天。有什么我可以帮助你的吗？

请按照以下JSON格式输出提取的记忆：
{
  "memories": [
    {
      "type": "person",
      "key": "name",
      "value": "张三",
      "confidence": 1.0
    }
  ]
}
[DEBUG] 发送到 Ollama API 的请求:
{"model":"llama3","messages":[{"role":"user","content":"你是一个专业的个人记忆提取助手..."}],"stream":false}
[DEBUG] 从 Ollama API 收到的响应:
{"model":"llama3","created_at":"2024-01-01T00:00:00.000Z","message":{"role":"assistant","content":"{\"memories\":[{\"type\":\"person\",\"key\":\"name\",\"value\":\"张三\",\"confidence\":1.0}]}"},"done":true,"total_duration":1000000000,"load_duration":500000000,"prompt_eval_duration":300000000,"eval_duration":200000000,"eval_count":10}
[DEBUG] 提取到 1 条记忆
[DEBUG] 成功写入 SQLite 结构化记忆
[DEBUG] 准备写入 Qdrant 的语义记忆数量: 1
[DEBUG] 思考完成，准备输出最终答案。
AI: 你好张三！很高兴认识你。我是一个人工智能助手，可以帮助你回答问题、提供信息或进行聊天。有什么我可以帮助你的吗？
用户:
```

## 未来展望

1. **多模型支持**：集成更多 LLM 提供商
2. **高级记忆管理**：实现记忆的优先级和过期机制
3. **个性化设置**：允许用户自定义模型参数和记忆存储策略
4. **API 接口**：提供 HTTP API 接口，支持集成到其他系统
5. **多语言支持**：增强对多语言对话的处理能力

## 许可证

本项目采用 MIT 许可证。

## 联系方式

如有问题或建议，请通过 GitHub Issues 提交。