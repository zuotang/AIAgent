package llm

import (
	"context"
	"fmt"
	"time"

	"agent-langchain/internal/models"
	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// OllamaNode Ollama LLM 节点
// 支持本地 Ollama 模型
type OllamaNode struct{}

func (n *OllamaNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	// 获取参数
	baseURL, _ := params["base_url"].(string)
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	model, _ := params["model"].(string)
	if model == "" {
		model = "qwen2.5:7b"
	}

	temperature, _ := params["temperature"].(float64)
	maxRetries := getIntParam(params, "max_retries", 1)

	// 获取 messages
	messages, err := extractMessages(inputs)
	if err != nil {
		return nil, err
	}

	// 创建客户端
	client := models.New(baseURL, model, "")
	if temperature > 0 {
		client.Temperature = temperature
	}

	// 转换消息
	chatMsgs := convertToModelMessages(messages)

	// 调用 LLM（支持重试）
	response, err := callLLMWithRetry(ctx, client, chatMsgs, model, maxRetries)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"messages": response,
	}, nil
}

func (n *OllamaNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "LLM.Ollama",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: false},
			{Name: "context_pack", Type: types.PortTypeContextPack, Required: false},
		},
		Outputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
		},
		Runner: n,
	}
}

// DeepSeekNode DeepSeek LLM 节点
type DeepSeekNode struct{}

func (n *DeepSeekNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	// 获取参数
	baseURL, _ := params["base_url"].(string)
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}

	model, _ := params["model"].(string)
	if model == "" {
		model = "deepseek-chat"
	}

	apiKey, _ := params["api_key"].(string)
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required for DeepSeek")
	}

	maxRetries := getIntParam(params, "max_retries", 1)

	// 获取 messages
	messages, err := extractMessages(inputs)
	if err != nil {
		return nil, err
	}

	// 创建客户端
	client := models.NewDeepSeek(baseURL, apiKey, model)

	// 转换消息
	chatMsgs := convertToModelMessages(messages)

	// 调用 LLM
	response, err := callLLMWithRetry(ctx, client, chatMsgs, model, maxRetries)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"messages": response,
	}, nil
}

func (n *DeepSeekNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "LLM.DeepSeek",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: false},
			{Name: "context_pack", Type: types.PortTypeContextPack, Required: false},
		},
		Outputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
		},
		Runner: n,
	}
}

// AnthropicNode Anthropic Claude 节点
type AnthropicNode struct{}

func (n *AnthropicNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	// 获取参数
	baseURL, _ := params["base_url"].(string)
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	model, _ := params["model"].(string)
	if model == "" {
		model = "claude-3-sonnet-20240229"
	}

	apiKey, _ := params["api_key"].(string)
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required for Anthropic")
	}

	maxRetries := getIntParam(params, "max_retries", 1)

	// 获取 messages
	messages, err := extractMessages(inputs)
	if err != nil {
		return nil, err
	}

	// 创建客户端
	client := models.NewAnthropic(baseURL, model, "")
	// TODO: 设置 API Key

	// 转换消息
	chatMsgs := convertToModelMessages(messages)

	// 调用 LLM
	response, err := callLLMWithRetry(ctx, client, chatMsgs, model, maxRetries)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"messages": response,
	}, nil
}

func (n *AnthropicNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "LLM.Anthropic",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: false},
			{Name: "context_pack", Type: types.PortTypeContextPack, Required: false},
		},
		Outputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
		},
		Runner: n,
	}
}

// 辅助函数

// extractMessages 从输入中提取 messages
func extractMessages(inputs map[string]any) ([]any, error) {
	var messages []any

	if contextPack, ok := inputs["context_pack"]; ok {
		if pack, ok := contextPack.(map[string]any); ok {
			if msgs, ok := pack["messages"]; ok {
				messages, _ = msgs.([]any)
			}
		}
	} else if msgs, ok := inputs["messages"]; ok {
		messages, _ = msgs.([]any)
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("messages input is required")
	}

	return messages, nil
}

// convertToModelMessages 转换为模型消息格式
func convertToModelMessages(messages []any) []models.ChatMessage {
	chatMsgs := make([]models.ChatMessage, 0, len(messages))

	for _, msg := range messages {
		switch m := msg.(type) {
		case models.ChatMessage:
			chatMsgs = append(chatMsgs, m)
		case map[string]any:
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			chatMsgs = append(chatMsgs, models.ChatMessage{
				Role:    role,
				Content: content,
			})
		case string:
			chatMsgs = append(chatMsgs, models.ChatMessage{
				Role:    "user",
				Content: m,
			})
		}
	}

	return chatMsgs
}

// callLLMWithRetry 调用 LLM 并支持重试
func callLLMWithRetry(ctx context.Context, client models.LLMClient, msgs []models.ChatMessage, model string, maxRetries int) (string, error) {
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var response string
	var err error

	for i := 0; i < maxRetries; i++ {
		response, err = client.Chat(ctx, msgs, model)
		if err == nil {
			return response, nil
		}

		// 如果不是最后一次重试，等待后重试
		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Second * time.Duration(i+1)):
				// 指数退避
			}
		}
	}

	return "", fmt.Errorf("LLM call failed after %d retries: %w", maxRetries, err)
}

// getIntParam 获取整数参数
func getIntParam(params map[string]any, key string, defaultValue int) int {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			// 尝试解析字符串
			var i int
			fmt.Sscanf(v, "%d", &i)
			return i
		}
	}
	return defaultValue
}
