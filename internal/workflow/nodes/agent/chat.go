package agent

import (
	"context"
	"fmt"

	"agent-langchain/internal/models"
	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// ChatNode Agent.Chat 节点
// 功能：使用创建的 Agent 进行对话
// 输入:
//   - agent: Agent 配置（来自 Agent.Create）
//   - messages: 消息列表或文本
// 输出:
//   - messages: LLM 响应文本
type ChatNode struct{}

// Run 执行节点
func (n *ChatNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	// 1. 获取 Agent 配置
	agentConfig, ok := inputs["agent"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("agent input is required and must be from Agent.Create node")
	}

	llmClient, ok := agentConfig["llm_client"].(models.LLMClient)
	if !ok {
		return nil, fmt.Errorf("invalid agent configuration: llm_client not found")
	}

	systemPrompt, _ := agentConfig["prompt"].(string)
	model, _ := agentConfig["model"].(string)

	// 2. 获取输入消息
	var messages []any

	// 支持从 messages 输入或 text 输入
	if msgs, ok := inputs["messages"]; ok {
		switch v := msgs.(type) {
		case []any:
			messages = v
		case string:
			// 如果是字符串，转换为消息格式
			messages = []any{
				map[string]any{"role": "user", "content": v},
			}
		default:
			return nil, fmt.Errorf("unsupported messages type: %T", msgs)
		}
	} else if text, ok := inputs["text"].(string); ok {
		messages = []any{
			map[string]any{"role": "user", "content": text},
		}
	} else {
		return nil, fmt.Errorf("either messages or text input is required")
	}

	// 3. 如果有系统提示词，添加到消息列表开头
	if systemPrompt != "" {
		messages = append([]any{
			map[string]any{"role": "system", "content": systemPrompt},
		}, messages...)
	}

	// 4. 调用 LLM
	// 将 []any 转换为 []models.ChatMessage
	chatMessages := make([]models.ChatMessage, 0, len(messages))
	for _, msg := range messages {
		if msgMap, ok := msg.(map[string]any); ok {
			role, _ := msgMap["role"].(string)
			content, _ := msgMap["content"].(string)
			chatMessages = append(chatMessages, models.ChatMessage{
				Role:    role,
				Content: content,
			})
		}
	}

	response, err := llmClient.Chat(ctx, chatMessages, model)
	if err != nil {
		return nil, fmt.Errorf("agent chat failed: %w", err)
	}

	// 返回两种格式的输出
	return map[string]any{
		"messages": []map[string]any{
			{
				"role":    "assistant",
				"content": response,
			},
		},
		"text": response,
	}, nil
}

// Spec 返回节点规范
func (n *ChatNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Agent.Chat",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "agent", Type: types.PortTypeLLMConfig, Required: true},
			{Name: "messages", Type: types.PortTypeMessages, Required: false},
			{Name: "text", Type: types.PortTypeText, Required: false},
		},
		Outputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Runner: n,
	}
}
