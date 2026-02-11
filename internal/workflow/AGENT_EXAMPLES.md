# Agent Workflow 示例说明

本目录包含三个完整的 Agent workflow 示例，展示了如何使用 Agent.Create 和 Agent.Chat 节点。

## 示例文件

### 1. agent_workflow_example.json - 基础多 Agent 协作

**场景**: 内容创作与审核

**流程**:
1. 输入一个写作主题
2. 创建"作家" Agent（高温度，更有创造性）
3. 创建"编辑" Agent（低温度，更严谨）
4. 作家 Agent 生成初稿
5. 编辑 Agent 审核并改进
6. 输出最终结果

**特点**:
- 展示基本的 Agent 创建和使用
- 展示不同 temperature 参数的效果
- 简单的串行工作流

**测试方法**:
```bash
curl -X POST http://localhost:8080/api/workflow/execute \
  -H "Content-Type: application/json" \
  -d @internal/workflow/agent_workflow_example.json
```

### 2. parallel_agents_example.json - 并行 Agent 对比

**场景**: 多风格翻译对比

**流程**:
1. 输入一段英文文本
2. 创建三个不同风格的翻译 Agent：
   - 正式学术风格（temperature: 0.3）
   - 通俗口语风格（temperature: 0.7）
   - 文学优雅风格（temperature: 0.9）
3. 三个 Agent 并行翻译同一文本
4. 将结果格式化为 JSON
5. 输出对比结果

**特点**:
- 展示多个 Agent 并行工作
- 展示不同 prompt 和 temperature 的效果
- 使用 Transform 节点处理输出

**测试方法**:
```bash
curl -X POST http://localhost:8080/api/workflow/execute \
  -H "Content-Type: application/json" \
  -d @internal/workflow/parallel_agents_example.json
```

### 3. customer_service_example.json - 智能客服系统

**场景**: 意图识别与专业 Agent 路由

**流程**:
1. 用户输入问题
2. 意图分类 Agent 识别问题类型（订单/产品/支付/投诉）
3. 使用 Flow.Switch 节点根据意图路由
4. 路由到对应的专业 Agent：
   - 订单服务专员
   - 产品顾问
   - 财务客服
   - 客户关系专员
5. 专业 Agent 处理问题
6. 输出回复

**特点**:
- 展示条件分支（Flow.Switch）
- 展示多个专业 Agent 的协作
- 实际应用场景示例
- 展示如何根据上下文动态选择 Agent

**测试方法**:
```bash
curl -X POST http://localhost:8080/api/workflow/execute \
  -H "Content-Type: application/json" \
  -d @internal/workflow/customer_service_example.json
```

## 修改输入

你可以修改 JSON 文件中的 `input` 节点的 `params.text` 字段来测试不同的输入：

```json
{
  "input": {
    "id": "input",
    "type": "Input.Text",
    "version": "1.0",
    "params": {
      "text": "你的输入文本"
    }
  }
}
```

## 使用不同的 LLM 提供商

你可以修改 Agent.Create 节点的参数来使用不同的 LLM 提供商：

### 使用 DeepSeek:
```json
{
  "type": "Agent.Create",
  "version": "1.0",
  "params": {
    "prompt": "你的提示词",
    "provider": "deepseek",
    "api_key": "your-api-key",
    "model": "deepseek-chat",
    "temperature": 0.7
  }
}
```

### 使用 Anthropic:
```json
{
  "type": "Agent.Create",
  "version": "1.0",
  "params": {
    "prompt": "你的提示词",
    "provider": "anthropic",
    "api_key": "your-api-key",
    "model": "claude-3-sonnet-20240229",
    "temperature": 0.7
  }
}
```

## 调试技巧

1. **查看执行追踪**: 使用 `/api/workflow/trace/:id` 端点查看详细的执行过程
2. **流式输出**: 使用 `/api/workflow/execute/stream` 端点实时查看执行进度
3. **验证 workflow**: 使用 `/api/workflow/validate` 端点在执行前验证 workflow 结构

## 常见问题

### Q: Agent 创建失败
A: 检查 provider 参数是否正确，以及是否需要 API key

### Q: 输出为空
A: 检查数据流连接是否正确，确保所有必需的输入端口都已连接

### Q: 执行超时
A: 可以在配置文件中调整 timeout 参数，或者简化 workflow 结构

## 扩展建议

1. **添加记忆功能**: 结合 Memory 节点，让 Agent 记住对话历史
2. **添加知识库**: 结合 KB 节点，让 Agent 基于知识库回答问题
3. **添加工具调用**: 结合 Tool 节点，让 Agent 能够调用外部工具
4. **循环处理**: 使用 Flow.Loop 节点实现迭代改进
