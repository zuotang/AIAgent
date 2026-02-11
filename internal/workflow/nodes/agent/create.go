package agent

import (
	"context"
	"fmt"

	"agent-langchain/internal/models"
	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// CreateNode Agent.Create 节点
// 功能：动态创建一个 LLM Agent 实例
// 输入: 无（或可选的 flow 输入）
// 输出: agent (LLM Agent 实例)
// 参数:
//   - prompt: 系统提示词（字符串）
//   - prompt_id: 提示词 ID（从数据库加载，可选）
//   - provider: LLM 提供商 (ollama/deepseek/anthropic)
//   - base_url: API 基础 URL（可选）
//   - api_key: API 密钥（可选）
//   - model: 模型名称（可选）
//   - temperature: 温度参数（可选，默认 0.7）
type CreateNode struct{}

// Run 执行节点
func (n *CreateNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	// 1. 获取 prompt
	var prompt string

	// 优先使用 prompt 参数
	if p, ok := params["prompt"].(string); ok && p != "" {
		prompt = p
	} else if _, ok := params["prompt_id"]; ok {
		// 从数据库加载 prompt
		if rc.MemoryStore == nil {
			return nil, fmt.Errorf("memory store is required to load prompt by ID")
		}

		// 这里需要扩展 MemoryStore 接口来支持 GetPrompt
		// 暂时返回错误，提示用户使用 prompt 参数
		return nil, fmt.Errorf("prompt_id is not yet supported, please use prompt parameter directly")
	} else {
		return nil, fmt.Errorf("either prompt or prompt_id parameter is required")
	}

	// 2. 获取 LLM 配置
	provider, _ := params["provider"].(string)
	if provider == "" {
		provider = "ollama" // 默认使用 ollama
	}

	baseURL, _ := params["base_url"].(string)
	apiKey, _ := params["api_key"].(string)
	model, _ := params["model"].(string)

	temperature := 0.7
	if t, ok := params["temperature"].(float64); ok {
		temperature = t
	}

	// 3. 创建 LLM 客户端
	var llmClient models.LLMClient

	switch provider {
	case "ollama":
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		if model == "" {
			model = "qwen3:4b"
		}
		client := models.New(baseURL, model, "")
		client.Temperature = temperature
		llmClient = client

	case "deepseek":
		if baseURL == "" {
			baseURL = "https://api.deepseek.com/v1"
		}
		if model == "" {
			model = "deepseek-chat"
		}
		if apiKey == "" {
			return nil, fmt.Errorf("api_key is required for DeepSeek provider")
		}
		client := models.NewDeepSeek(baseURL, apiKey, model)
		llmClient = client

	case "anthropic":
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		if model == "" {
			model = "claude-3-sonnet-20240229"
		}
		if apiKey == "" {
			return nil, fmt.Errorf("api_key is required for Anthropic provider")
		}
		client := models.NewAnthropic(baseURL, model, "")
		llmClient = client

	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: ollama, deepseek, anthropic)", provider)
	}

	// 4. 创建 Agent 配置
	agentConfig := map[string]any{
		"llm_client":  llmClient,
		"prompt":      prompt,
		"provider":    provider,
		"model":       model,
		"temperature": temperature,
	}

	return map[string]any{
		"agent": agentConfig,
	}, nil
}

// Spec 返回节点规范
func (n *CreateNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Agent.Create",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
		},
		Outputs: []types.PortSpec{
			{Name: "agent", Type: types.PortTypeLLMConfig, Required: true},
		},
		Runner: n,
	}
}
