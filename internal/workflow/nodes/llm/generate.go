package llm

import (
	"context"
	"fmt"
	"strings"

	"agent-langchain/internal/models"
	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// GenerateNode LLM.Generate 节点
// 输入: messages ([]ChatMessage) 或 context_pack
// 输出: messages (string)
type GenerateNode struct{}

// Run 执行节点
func (n *GenerateNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
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

	var model string
	if m, ok := params["model"]; ok {
		model, _ = m.(string)
	}
	if model == "" {
		model = "qwen3:4b"
	}

	provider, _ := params["provider"].(string)
	baseURL, _ := params["base_url"].(string)
	apiKey, _ := params["api_key"].(string)
	temperature, _ := params["temperature"].(float64)

	var response string
	var err error

	if provider == "" {
		if rc.LLMClient == nil {
			return nil, fmt.Errorf("LLMClient is required")
		}
		response, err = rc.LLMClient.Chat(ctx, messages, model)
	} else {
		client, err := selectLLMClient(provider, baseURL, apiKey, model, temperature)
		if err != nil {
			return nil, err
		}
		chatMsgs := convertToModelMessages(messages)
		response, err = client.Chat(ctx, chatMsgs, model)
	}
	if err != nil {
		return nil, fmt.Errorf("LLM chat failed: %w", err)
	}

	return map[string]any{
		"messages": response,
	}, nil
}

// Spec 返回节点规范
func (n *GenerateNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "LLM.Generate",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "messages", Type: types.PortTypeMessages, Required: false},
			{Name: "context_pack", Type: types.PortTypeContextPack, Required: false},
		},
		Outputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
		},
		Runner: n,
	}
}

func selectLLMClient(provider, baseURL, apiKey, model string, temperature float64) (models.LLMClient, error) {
	switch strings.ToLower(provider) {
	case "ollama":
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		if model == "" {
			model = "qwen3:4b"
		}
		client := models.New(baseURL, model, "")
		if temperature > 0 {
			client.Temperature = temperature
		}
		return client, nil
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
		return models.NewDeepSeek(baseURL, apiKey, model), nil
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
		return models.NewAnthropic(baseURL, model, ""), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}
