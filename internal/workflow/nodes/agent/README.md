# Agent 节点

Agent 节点允许你在 workflow 中动态创建和使用多个 LLM Agent 实例。每个 Agent 可以有自己的 prompt、模型配置和参数。

## 节点类型

### 1. Agent.Create

创建一个新的 LLM Agent 实例。

**输入端口：**
- `in` (flow, 可选): 控制流输入

**输出端口：**
- `agent` (llm_config): Agent 配置对象

**参数：**
- `prompt` (string, 必需*): 系统提示词文本
- `prompt_id` (number, 可选): 从数据库加载的提示词 ID（暂未实现）
- `provider` (string, 可选): LLM 提供商，支持：
  - `ollama` (默认)
  - `deepseek`
  - `anthropic`
- `base_url` (string, 可选): API 基础 URL
  - ollama 默认: `http://localhost:11434`
  - deepseek 默认: `https://api.deepseek.com/v1`
  - anthropic 默认: `https://api.anthropic.com/v1`
- `api_key` (string, 可选): API 密钥（deepseek 和 anthropic 必需）
- `model` (string, 可选): 模型名称
  - ollama 默认: `qwen3:4b`
  - deepseek 默认: `deepseek-chat`
  - anthropic 默认: `claude-3-sonnet-20240229`
- `temperature` (number, 可选): 温度参数，默认 0.7

**示例：**

```json
{
  "id": "agent1",
  "type": "Agent.Create",
  "version": "1.0",
  "params": {
    "prompt": "你是一个专业的翻译助手，擅长中英文互译。",
    "provider": "ollama",
    "model": "qwen3:4b",
    "temperature": 0.7
  }
}
```

### 2. Agent.Chat

使用创建的 Agent 进行对话。

**输入端口：**
- `in` (flow, 可选): 控制流输入
- `agent` (llm_config, 必需): Agent 配置（来自 Agent.Create）
- `messages` (messages, 可选): 消息列表
- `text` (text, 可选): 文本输入（会自动转换为 user 消息）

**输出端口：**
- `messages` (messages): LLM 响应文本

**示例：**

```json
{
  "id": "chat1",
  "type": "Agent.Chat",
  "version": "1.0"
}
```

## 完整工作流示例

### 示例 1: 创建单个翻译 Agent

```json
{
  "meta": {
    "id": "translator-workflow",
    "name": "翻译工作流",
    "version": "1.0"
  },
  "nodes": {
    "input": {
      "type": "Input.Text",
      "version": "1.0",
      "params": {
        "text": "Hello, how are you?"
      }
    },
    "translator": {
      "type": "Agent.Create",
      "version": "1.0",
      "params": {
        "prompt": "你是一个专业的翻译助手，请将用户输入的英文翻译成中文。",
        "provider": "ollama",
        "model": "qwen3:4b"
      }
    },
    "translate": {
      "type": "Agent.Chat",
      "version": "1.0"
    },
    "output": {
      "type": "Output.Text",
      "version": "1.0"
    }
  },
  "edges": [
    {
      "from": {"node": "input", "port": "text"},
      "to": {"node": "translate", "port": "text"},
      "type": "data"
    },
    {
      "from": {"node": "translator", "port": "agent"},
      "to": {"node": "translate", "port": "agent"},
      "type": "data"
    },
    {
      "from": {"node": "translate", "port": "messages"},
      "to": {"node": "output", "port": "text"},
      "type": "data"
    }
  ]
}
```

### 示例 2: 创建多个 Agent 协作

```json
{
  "meta": {
    "id": "multi-agent-workflow",
    "name": "多 Agent 协作工作流",
    "version": "1.0"
  },
  "nodes": {
    "input": {
      "type": "Input.Text",
      "version": "1.0",
      "params": {
        "text": "写一篇关于人工智能的文章"
      }
    },
    "writer": {
      "type": "Agent.Create",
      "version": "1.0",
      "params": {
        "prompt": "你是一个专业的内容创作者，擅长写作各类文章。",
        "provider": "ollama",
        "model": "qwen3:4b",
        "temperature": 0.8
      }
    },
    "reviewer": {
      "type": "Agent.Create",
      "version": "1.0",
      "params": {
        "prompt": "你是一个严格的审稿人，负责审查文章的质量并提出改进建议。",
        "provider": "ollama",
        "model": "qwen3:4b",
        "temperature": 0.3
      }
    },
    "write": {
      "type": "Agent.Chat",
      "version": "1.0"
    },
    "review": {
      "type": "Agent.Chat",
      "version": "1.0"
    },
    "output": {
      "type": "Output.Text",
      "version": "1.0"
    }
  },
  "edges": [
    {
      "from": {"node": "input", "port": "text"},
      "to": {"node": "write", "port": "text"},
      "type": "data"
    },
    {
      "from": {"node": "writer", "port": "agent"},
      "to": {"node": "write", "port": "agent"},
      "type": "data"
    },
    {
      "from": {"node": "write", "port": "messages"},
      "to": {"node": "review", "port": "text"},
      "type": "data"
    },
    {
      "from": {"node": "reviewer", "port": "agent"},
      "to": {"node": "review", "port": "agent"},
      "type": "data"
    },
    {
      "from": {"node": "review", "port": "messages"},
      "to": {"node": "output", "port": "text"},
      "type": "data"
    }
  ]
}
```

### 示例 3: 使用不同的 LLM 提供商

```json
{
  "meta": {
    "id": "multi-provider-workflow",
    "name": "多提供商工作流",
    "version": "1.0"
  },
  "nodes": {
    "input": {
      "type": "Input.Text",
      "version": "1.0",
      "params": {
        "text": "解释量子计算的基本原理"
      }
    },
    "ollama_agent": {
      "type": "Agent.Create",
      "version": "1.0",
      "params": {
        "prompt": "你是一个物理学专家。",
        "provider": "ollama",
        "model": "qwen3:4b"
      }
    },
    "deepseek_agent": {
      "type": "Agent.Create",
      "version": "1.0",
      "params": {
        "prompt": "你是一个计算机科学专家。",
        "provider": "deepseek",
        "api_key": "your-api-key-here",
        "model": "deepseek-chat"
      }
    },
    "chat1": {
      "type": "Agent.Chat",
      "version": "1.0"
    },
    "chat2": {
      "type": "Agent.Chat",
      "version": "1.0"
    }
  },
  "edges": [
    {
      "from": {"node": "input", "port": "text"},
      "to": {"node": "chat1", "port": "text"},
      "type": "data"
    },
    {
      "from": {"node": "ollama_agent", "port": "agent"},
      "to": {"node": "chat1", "port": "agent"},
      "type": "data"
    },
    {
      "from": {"node": "input", "port": "text"},
      "to": {"node": "chat2", "port": "text"},
      "type": "data"
    },
    {
      "from": {"node": "deepseek_agent", "port": "agent"},
      "to": {"node": "chat2", "port": "agent"},
      "type": "data"
    }
  ]
}
```

## 使用场景

1. **多角色对话**: 创建多个具有不同角色的 Agent，模拟多人对话场景
2. **内容生成与审核**: 一个 Agent 生成内容，另一个 Agent 审核和改进
3. **多语言翻译**: 创建专门的翻译 Agent 处理不同语言对
4. **专业领域咨询**: 为不同专业领域创建专门的 Agent
5. **A/B 测试**: 使用不同的 prompt 或模型参数创建多个 Agent 进行对比

## 注意事项

1. 每个 Agent 都是独立的 LLM 实例，会消耗相应的计算资源
2. 使用 DeepSeek 或 Anthropic 时需要提供有效的 API 密钥
3. `prompt_id` 参数暂未实现，请使用 `prompt` 参数直接传入提示词
4. Agent 配置会在 workflow 执行期间保持，可以被多个 Chat 节点复用
