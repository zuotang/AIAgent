# 智能路由系统 (Intelligent Routing System)

## 概述

智能路由系统是一个基于小模型的快速分类器，用于决定每次用户查询是否需要检索记忆库或知识库。这个系统通过在主 LLM 调用之前进行快速分类（<100ms），显著减少不必要的检索操作，降低延迟和成本。

## 架构设计

### 工作流程

```
用户输入
  ↓
快速分类器（小模型，<100ms）
  ↓
分类结果：
  ├─ MEMORY：需要个人记忆（历史对话、偏好、个人信息）
  ├─ KNOWLEDGE：需要知识库（事实性信息、文档内容）
  ├─ BOTH：两者都需要
  └─ NONE：都不需要（简单闲聊）
  ↓
条件性检索：
  - MEMORY → 检索记忆库 → 注入上下文
  - KNOWLEDGE → 检索知识库 → 注入上下文
  - BOTH → 并行检索两者 → 合并上下文
  - NONE → 跳过检索
  ↓
主 LLM 生成响应
```

### 核心组件

1. **分类器 (Classifier)**
   - 使用小模型（如 `qwen2.5:0.5b`）进行快速分类
   - 超时时间：100ms（可配置）
   - 温度：0.0（确定性分类）

2. **记忆检索 (Memory Retrieval)**
   - SQLite：结构化记忆（key-value pairs）
   - Qdrant：语义记忆（向量搜索）
   - 并行检索，减少延迟

3. **知识库检索 (Knowledge Retrieval)**
   - Qdrant 向量搜索
   - Top-K 结果（默认 3 条）
   - 与记忆检索使用相同的集合

## 配置说明

### config.yaml 配置

```yaml
# 分类器配置（用于智能路由）
classifier:
  provider: ollama          # 分类器提供商
  base_url: http://127.0.0.1:11434
  model: qwen2.5:0.5b      # 小模型，快速分类
  api_key: ""
  temperature: 0.0          # 确定性分类
  timeout: 10               # 超时时间（秒）

# 知识库智能路由配置
knowledge:
  enable_routing: true      # 启用智能路由
  top_k: 3                  # 知识库检索返回的最大结果数
  classifier_timeout: 100   # 分类器超时（毫秒）

# 记忆配置
memory:
  window_size: 8
  enable_extractor: true
  enable_smart_trigger: true
  trigger_method: conservative
  min_message_length: 10
  include_history_context: false
  min_confidence: 0.65
  max_memories_per_extraction: 20
```

### 配置项说明

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `knowledge.enable_routing` | 是否启用智能路由 | `true` |
| `knowledge.top_k` | 知识库检索返回的最大结果数 | `3` |
| `knowledge.classifier_timeout` | 分类器超时（毫秒） | `100` |
| `classifier.model` | 分类器模型（建议使用小模型） | `qwen2.5:0.5b` |
| `classifier.temperature` | 分类温度（建议 0.0） | `0.0` |

## 实现细节

### 代码结构

**核心文件：**
- `internal/orchestrator/orchestrator.go`：主编排逻辑
- `internal/config/config.go`：配置结构定义
- `config.yaml`：配置文件

**关键函数：**

1. **`classifyQueryType(ctx, userText) string`**
   - 使用小模型快速分类查询类型
   - 返回：`MEMORY`, `KNOWLEDGE`, `BOTH`, `NONE`
   - 超时：100ms（可配置）

2. **`retrieveKnowledge(ctx, userID, agentID, query) ([]rag.Doc, error)`**
   - 从 Qdrant 检索知识库内容
   - 使用向量相似度搜索
   - 返回 Top-K 结果

3. **`formatKnowledge(docs []rag.Doc) string`**
   - 格式化知识库内容为文本
   - 用于注入到 LLM 上下文

### ProcessMessage 流程

```go
// 1. 智能路由分类
queryType := o.classifyQueryType(ctx, userText)

// 2. 条件性检索
switch queryType {
case "BOTH":
    // 并行检索记忆和知识库
case "MEMORY":
    // 只检索记忆
case "KNOWLEDGE":
    // 只检索知识库
case "NONE":
    // 跳过检索
}

// 3. 合并上下文
combinedContext := memoryText + "\n\n" + knowledgeText

// 4. 构建 Agent 输入
input := agent.Input{
    Memory: combinedContext,
    // ...
}

// 5. 执行主 LLM
output := o.agent.Run(ctx, input)
```

## 分类提示词

分类器使用以下提示词来判断查询类型：

```
分析用户的查询，判断需要什么类型的上下文来回答。

上下文类型：
1. MEMORY（记忆）：用户的个人信息、偏好、历史对话、过往互动
   - 例如："我之前说过什么"、"你还记得我的名字吗"、"我喜欢什么"
2. KNOWLEDGE（知识库）：事实性信息、文档内容、技术知识、通用知识
   - 例如："什么是机器学习"、"如何使用这个API"、"解释一下原理"
3. BOTH（两者都需要）：既需要个人记忆又需要知识库
   - 例如："根据我的技能水平，推荐学习资料"
4. NONE（都不需要）：简单闲聊、问候、确认等
   - 例如："你好"、"谢谢"、"好的"

用户查询：{userText}

只回答以下之一：MEMORY、KNOWLEDGE、BOTH、NONE
```

## 性能优化

### 延迟优化

1. **快速分类**：使用小模型（0.5B 参数），分类时间 <100ms
2. **并行检索**：当需要两者时，并行检索记忆和知识库
3. **条件性检索**：只在需要时才检索，避免不必要的操作

### 成本优化

1. **减少检索次数**：通过智能分类，避免每次都检索
2. **减少 Token 使用**：只注入必要的上下文
3. **小模型分类**：分类器使用小模型，成本极低

### 预期效果

- **延迟减少**：简单查询跳过检索，延迟降低 50-70%
- **成本降低**：减少不必要的检索和 Token 使用，成本降低 30-50%
- **准确性提升**：只注入相关上下文，减少噪音

## 使用示例

### 示例 1：简单闲聊（NONE）

**用户输入：** "你好"

**分类结果：** `NONE`

**行为：** 跳过检索，直接调用主 LLM

**响应时间：** ~500ms（仅主 LLM）

### 示例 2：个人记忆查询（MEMORY）

**用户输入：** "我之前告诉过你我的名字吗？"

**分类结果：** `MEMORY`

**行为：** 检索记忆库（SQLite + Qdrant）

**响应时间：** ~800ms（分类 + 检索 + 主 LLM）

### 示例 3：知识库查询（KNOWLEDGE）

**用户输入：** "什么是 Transformer 架构？"

**分类结果：** `KNOWLEDGE`

**行为：** 检索知识库（Qdrant）

**响应时间：** ~700ms（分类 + 检索 + 主 LLM）

### 示例 4：混合查询（BOTH）

**用户输入：** "根据我的编程水平，推荐一些 Go 语言学习资料"

**分类结果：** `BOTH`

**行为：** 并行检索记忆库和知识库

**响应时间：** ~900ms（分类 + 并行检索 + 主 LLM）

## 调试和监控

### 启用调试模式

在 `config.yaml` 中设置：

```yaml
base:
  debug: true
```

### 调试日志示例

```
[DEBUG] 查询分类结果: MEMORY
[DEBUG] 记忆已加载
[DEBUG] 上下文占用率: 45.2% (Total=3621, Limit=8000)
```

### 监控指标

建议监控以下指标：

1. **分类分布**：MEMORY / KNOWLEDGE / BOTH / NONE 的比例
2. **分类延迟**：分类器响应时间
3. **检索延迟**：记忆/知识库检索时间
4. **总延迟**：端到端响应时间
5. **分类准确性**：人工评估分类是否正确

## 未来改进

### 短期改进

1. **添加 type 过滤**：在 Qdrant 搜索时过滤 `type=knowledge`
2. **缓存分类结果**：对相似查询缓存分类结果
3. **自适应超时**：根据历史性能动态调整超时

### 长期改进

1. **多级分类**：更细粒度的分类（如：技术知识、通用知识、个人偏好等）
2. **学习优化**：根据用户反馈优化分类提示词
3. **混合策略**：结合关键词和 LLM 分类的混合策略
4. **A/B 测试**：对比不同分类策略的效果

## 故障排查

### 问题 1：分类器超时

**症状：** 日志显示 "查询分类失败"

**原因：** 分类器模型响应慢或服务不可用

**解决方案：**
1. 检查分类器服务是否运行
2. 增加 `classifier_timeout` 配置
3. 使用更快的小模型

### 问题 2：分类不准确

**症状：** 应该检索知识库但分类为 NONE

**原因：** 分类提示词不够清晰或模型能力不足

**解决方案：**
1. 优化分类提示词
2. 使用更强的分类模型
3. 添加更多示例到提示词

### 问题 3：检索结果为空

**症状：** 分类为 KNOWLEDGE 但没有检索到内容

**原因：** 知识库为空或查询向量化失败

**解决方案：**
1. 检查知识库是否已导入数据
2. 检查 embedding 服务是否正常
3. 查看 Qdrant 日志

## 相关文档

- [CLAUDE.md](../CLAUDE.md)：项目整体架构
- [config.example.yaml](../config.example.yaml)：配置示例
- [internal/orchestrator/orchestrator.go](../internal/orchestrator/orchestrator.go)：实现代码

## 更新日志

### 2026-02-03
- ✅ 实现智能路由系统
- ✅ 添加 `classifyQueryType` 分类器
- ✅ 添加 `retrieveKnowledge` 知识库检索
- ✅ 修改 `ProcessMessage` 和 `ProcessMessageStream` 流程
- ✅ 去除关键词触发机制，完全使用 LLM 分类
- ✅ 更新配置结构和默认值
