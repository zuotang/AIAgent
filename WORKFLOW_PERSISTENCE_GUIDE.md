# 工作流持久化功能测试指南

## ✅ 已实现的完整API列表

### 1. 节点管理
- `GET /api/workflow/nodes` - 获取所有可用节点

### 2. 工作流校验和执行
- `POST /api/workflow/validate` - 校验工作流
- `POST /api/workflow/execute` - 执行工作流

### 3. 工作流持久化（新增）
- `POST /api/workflow/save` - 保存工作流
- `GET /api/workflow/list` - 获取工作流列表
- `GET /api/workflow/:id` - 获取单个工作流
- `DELETE /api/workflow/:id` - 删除工作流

### 4. 执行历史（新增）
- `GET /api/workflow/trace/:id` - 获取执行追踪

## 🚀 启动服务器

```bash
cd cmd/api
./api.exe
```

## 📝 测试示例

### 1. 保存工作流

```bash
curl -X POST http://localhost:8080/api/workflow/save \
  -H "Content-Type: application/json" \
  -H "X-User-ID: user123" \
  -d '{
    "id": "wf-test-001",
    "name": "我的第一个工作流",
    "description": "这是一个测试工作流",
    "workflow": {
      "version": "1.0",
      "meta": {
        "id": "wf-test-001",
        "name": "我的第一个工作流"
      },
      "nodes": {
        "input": {
          "id": "input",
          "type": "Input.Text",
          "version": "1.0",
          "params": {
            "text": "Hello World"
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
          "to": { "node": "output", "port": "text" },
          "type": "data"
        }
      ]
    }
  }'
```

**预期响应**:
```json
{
  "success": true,
  "id": "wf-test-001"
}
```

### 2. 获取工作流列表

```bash
curl -H "X-User-ID: user123" \
  "http://localhost:8080/api/workflow/list?page=1&limit=20"
```

**预期响应**:
```json
{
  "workflows": [
    {
      "id": "wf-test-001",
      "name": "我的第一个工作流",
      "description": "这是一个测试工作流",
      "created_at": "2024-02-06T14:30:00Z",
      "updated_at": "2024-02-06T14:30:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "limit": 20
}
```

### 3. 获取单个工作流

```bash
curl http://localhost:8080/api/workflow/wf-test-001
```

**预期响应**:
```json
{
  "id": "wf-test-001",
  "name": "我的第一个工作流",
  "description": "这是一个测试工作流",
  "workflow": {
    "version": "1.0",
    "meta": { ... },
    "nodes": { ... },
    "edges": [ ... ]
  },
  "created_at": "2024-02-06T14:30:00Z",
  "updated_at": "2024-02-06T14:30:00Z"
}
```

### 4. 执行工作流（会自动保存执行记录）

```bash
curl -X POST http://localhost:8080/api/workflow/execute \
  -H "Content-Type: application/json" \
  -H "X-User-ID: user123" \
  -d '{
    "workflow": {
      "version": "1.0",
      "meta": {
        "id": "wf-test-001",
        "name": "测试"
      },
      "nodes": {
        "input": {
          "id": "input",
          "type": "Input.Text",
          "version": "1.0",
          "params": { "text": "Hello" }
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
          "to": { "node": "output", "port": "text" },
          "type": "data"
        }
      ]
    },
    "async": false
  }'
```

**预期响应**:
```json
{
  "success": true,
  "trace": {
    "workflow_id": "wf-test-001",
    "workflow_name": "测试",
    "status": "success",
    "duration": "123ms",
    "nodes": {
      "input": {
        "status": "success",
        "outputs": { "text": "Hello" }
      },
      "output": {
        "status": "success",
        "inputs": { "text": "Hello" }
      }
    }
  }
}
```

### 5. 删除工作流

```bash
curl -X DELETE http://localhost:8080/api/workflow/wf-test-001
```

**预期响应**:
```json
{
  "success": true
}
```

## 🗄️ 数据库

工作流数据保存在 SQLite 数据库中：
- 数据库文件: `memory.db` (配置文件中指定)
- 表1: `workflows` - 工作流定义
- 表2: `workflow_executions` - 执行历史

### 查看数据库内容

```bash
# 安装 sqlite3
# Windows: 下载 sqlite3.exe
# Linux: sudo apt install sqlite3

# 查看工作流表
sqlite3 memory.db "SELECT id, name, created_at FROM workflows;"

# 查看执行记录表
sqlite3 memory.db "SELECT id, workflow_id, status, started_at FROM workflow_executions;"
```

## 🎯 前端集成示例

### Vue3 + Axios

```javascript
// 保存工作流
async function saveWorkflow(workflow, name, description) {
  const response = await axios.post('/api/workflow/save', {
    id: workflow.meta.id,
    name: name,
    description: description,
    workflow: workflow
  }, {
    headers: {
      'X-User-ID': 'current-user-id'
    }
  })
  return response.data
}

// 获取工作流列表
async function listWorkflows(page = 1, limit = 20) {
  const response = await axios.get('/api/workflow/list', {
    params: { page, limit },
    headers: {
      'X-User-ID': 'current-user-id'
    }
  })
  return response.data
}

// 加载工作流
async function loadWorkflow(id) {
  const response = await axios.get(`/api/workflow/${id}`)
  return response.data.workflow
}

// 删除工作流
async function deleteWorkflow(id) {
  const response = await axios.delete(`/api/workflow/${id}`)
  return response.data
}

// 执行工作流
async function executeWorkflow(workflow) {
  const response = await axios.post('/api/workflow/execute', {
    workflow: workflow,
    async: false
  }, {
    headers: {
      'X-User-ID': 'current-user-id'
    }
  })
  return response.data
}
```

## ⚠️ 注意事项

### 1. 用户ID
- 通过 `X-User-ID` 请求头传递
- 如果不传，默认使用 "default"
- 用于隔离不同用户的工作流

### 2. 工作流ID
- 必须唯一
- 建议格式: `wf-{timestamp}` 或 `wf-{uuid}`
- 保存时如果ID已存在会更新

### 3. 执行记录
- 每次执行都会自动保存
- 记录ID格式: `{workflow_id}-{timestamp_nano}`
- 包含完整的执行追踪信息

### 4. 数据库备份
```bash
# 备份数据库
cp memory.db memory.db.backup

# 恢复数据库
cp memory.db.backup memory.db
```

## 🔍 故障排查

### 问题1: "Not Found" 错误
- 确认API服务器已启动
- 检查URL路径是否正确
- 查看服务器日志

### 问题2: 保存失败
- 检查工作流JSON格式是否正确
- 确认数据库文件有写权限
- 查看服务器错误日志

### 问题3: 列表为空
- 确认使用了正确的 `X-User-ID`
- 检查数据库中是否有数据
- 尝试不带 `X-User-ID` 查询所有数据

## 📊 完整功能对比

| 功能 | 之前 | 现在 |
|------|------|------|
| 获取节点列表 | ✅ | ✅ |
| 校验工作流 | ✅ | ✅ |
| 执行工作流 | ✅ | ✅ |
| 保存工作流 | ❌ | ✅ |
| 列出工作流 | ❌ | ✅ |
| 获取工作流 | ❌ | ✅ |
| 删除工作流 | ❌ | ✅ |
| 执行历史 | ❌ | ✅ |
| 数据持久化 | ❌ | ✅ |

## 🎉 总结

现在你的工作流系统已经具备完整的持久化功能：
- ✅ 8个完整的API端点
- ✅ SQLite数据库存储
- ✅ 工作流CRUD操作
- ✅ 执行历史记录
- ✅ 用户隔离
- ✅ 分页查询

可以开始前端集成了！
