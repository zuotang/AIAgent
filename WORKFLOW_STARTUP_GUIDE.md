# 工作流系统启动指南

## 快速启动

### 1. 启动API服务器

```bash
# 方法1: 使用默认配置
cd cmd/api
go run main.go

# 方法2: 指定配置文件
go run main.go -config=config.com.yaml

# 方法3: 编译后运行
go build -o api.exe
./api.exe
```

### 2. 验证服务启动

```bash
# 检查健康状态
curl http://localhost:8080/health

# 获取可用节点列表
curl http://localhost:8080/api/workflow/nodes
```

### 3. 测试工作流执行

```bash
# 使用示例工作流
cd cmd/workflow_example
go run main.go
```

## 前端对接

### 1. 获取可用节点

```javascript
const response = await fetch('http://localhost:8080/api/workflow/nodes')
const data = await response.json()
console.log(data.nodes)
```

### 2. 校验工作流

```javascript
const workflow = {
  version: "1.0",
  meta: { id: "wf-001", name: "测试" },
  nodes: { ... },
  edges: [ ... ]
}

const response = await fetch('http://localhost:8080/api/workflow/validate', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(workflow)
})

const result = await response.json()
console.log(result.valid, result.errors)
```

### 3. 执行工作流

```javascript
const response = await fetch('http://localhost:8080/api/workflow/execute', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    workflow: workflow,
    async: false
  })
})

const result = await response.json()
console.log(result.success, result.trace)
```

## 当前可用的API端点

### 工作流相关
- `GET /api/workflow/nodes` - 获取所有可用节点
- `POST /api/workflow/validate` - 校验工作流
- `POST /api/workflow/execute` - 执行工作流

### 其���服务
- `GET /health` - 健康检查
- `POST /api/chat` - 聊天接口
- `POST /api/knowledge/ingest/file` - 知识库录入
- 更多接口见 cmd/api/main.go

## 配置要求

### 1. 确保Ollama运行（如果使用本地模型）

```bash
# 检查Ollama是否运行
curl http://localhost:11434/api/tags

# 如果没有运行，启动Ollama
ollama serve
```

### 2. 配置文件

确保 `config.com.yaml` 配置正确：

```yaml
base:
  provider: ollama  # 或 deepseek, anthropic
  debug: true
  timeout: 120

llm:
  ollama:
    base_url: http://localhost:11434
    chat_model: qwen2.5:7b
    temperature: 0.7

services:
  api:
    port: 8080
```

## 故障排查

### 问题1: 端口被占用

```bash
# Windows
netstat -ano | findstr :8080
taskkill /PID <PID> /F

# Linux/Mac
lsof -i :8080
kill -9 <PID>
```

### 问题2: Ollama连接失败

```bash
# 检查Ollama状态
curl http://localhost:11434/api/tags

# 重启Ollama
# Windows: 从任务管理器重启
# Linux/Mac: systemctl restart ollama
```

### 问题3: 工作流执行失败

检查日志输出，常见原因：
- LLM客户端未正确初始化
- 节点参数缺失或错误
- 端口类型不匹配
- 网络连接问题

## 开发模式

### 热重载开发

```bash
# 安装air（Go热重载工具）
go install github.com/cosmtrek/air@latest

# 在cmd/api目录下运行
air
```

### 查看日志

```bash
# API服务器会输出详细日志
# 包括：
# - 服务启动信息
# - 路由注册信息
# - 请求处理日志
# - 错误信息
```

## 下一步

1. ✅ API服务器已启动
2. ✅ 工作流端点已注册
3. ⏳ 前端集成（参考 FRONTEND_API_GUIDE.md）
4. ⏳ 添加工作流持久化（数据库）
5. ⏳ 添加WebSocket实时推送

## 示例工作流

### 简单对话工作流

```json
{
  "version": "1.0",
  "meta": {
    "id": "wf-simple-chat",
    "name": "简单对话"
  },
  "nodes": {
    "input": {
      "id": "input",
      "type": "Input.Text",
      "version": "1.0",
      "params": {
        "text": "你好，请介绍一下自己"
      }
    },
    "transform": {
      "id": "transform",
      "type": "Transform.TextToMessages",
      "version": "1.0",
      "params": {
        "role": "user"
      }
    },
    "llm": {
      "id": "llm",
      "type": "LLM.Ollama",
      "version": "1.0",
      "params": {
        "model": "qwen2.5:7b",
        "temperature": 0.7
      }
    },
    "output": {
      "id": "output",
      "type": "Output.Text",
      "version": "1.0",
      "params": {}
    }
  },
  "edges": [
    {
      "id": "e1",
      "from": { "node": "input", "port": "text" },
      "to": { "node": "transform", "port": "text" },
      "type": "data"
    },
    {
      "id": "e2",
      "from": { "node": "transform", "port": "messages" },
      "to": { "node": "llm", "port": "messages" },
      "type": "data"
    },
    {
      "id": "e3",
      "from": { "node": "llm", "port": "messages" },
      "to": { "node": "output", "port": "text" },
      "type": "data"
    }
  ]
}
```

保存为 `simple_chat.json`，然后：

```bash
curl -X POST http://localhost:8080/api/workflow/execute \
  -H "Content-Type: application/json" \
  -d @simple_chat.json
```

## 技术支持

如有问题，请查看：
- `FRONTEND_API_GUIDE.md` - 前端对接文档
- `NODE_PARAMETERS_GUIDE.md` - 节点参数指南
- `internal/workflow/README.md` - 工作流引擎文档
