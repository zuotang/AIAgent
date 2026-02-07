package llm

import (
	"context"
	"fmt"
	"time"

	"agent-langchain/internal/models"
	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// UnifiedLLMNode 统一的 LLM 节点，支持多种提供商
// 输入: messages 或 context_pack
// 输出: messages
// 参数: provider, model, temperature, max_retries, base_url, api_key
type UnifiedLLMNode struct{}

// Run 执行节点
func (n *UnifiedLLMNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	provider, _ := params["provider"].(string)
	if provider == "" {
		provider = "ollama"
	}

	model, _ := params["model"].(string)
	temperature, _ := params["temperature"].(float64)
	maxRetries, _ := params["max_retries"].(float64)
	baseURL, _ := params["base_url"].(string)
	apiKey, _ := params["api_key"].(string)

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

	var llmClient models.LLMClient

	switch provider {
	case "ollama":
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		if model == "" {
			model = "qwen2.5:7b"
		}
		client := models.New(baseURL, model, "")
		if temperature > 0 {
			client.Temperature = temperature
		}
		llmClient = client

	case "deepseek":
		if baseURL == "" {
			baseURL = "https://api.deepseek.com/v1"
		}
		if model == "" {
			model = "deepseek-chat"
		}
		if apiKey == "" {
			return nil, fmt.Errorf("api_key is required for DeepSeek")
		}
		llmClient = models.NewDeepSeek(baseURL, apiKey, model)

	case "anthropic":
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		if model == "" {
			model = "claude-3-sonnet-20240229"
		}
		if apiKey == "" {
			return nil, fmt.Errorf("api_key is required for Anthropic")
		}
		llmClient = models.NewAnthropic(baseURL, model, "")

	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	chatMsgs := convertToModelMessages(messages)

	retries := int(maxRetries)
	if retries <= 0 {
		retries = 1
	}

	var response string
	var err error

	for i := 0; i < retries; i++ {
		response, err = llmClient.Chat(ctx, chatMsgs, model)
		if err == nil {
			break
		}
		if i < retries-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second * time.Duration(i+1)):
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("LLM chat failed after %d retries: %w", retries, err)
	}

	return map[string]any{
		"messages": response,
	}, nil
}

// Spec 返回节点规范
func (n *UnifiedLLMNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "LLM.Chat",
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
