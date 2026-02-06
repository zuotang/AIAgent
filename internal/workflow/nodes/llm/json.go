package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// JSONNode LLM.JSON 节点
// 输入: messages
// 参数: schema (JSON schema)
// 输出: json
type JSONNode struct{}

// Run 执行节点
func (n *JSONNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	// 获取 LLM 客户端
	if rc.LLMClient == nil {
		return nil, fmt.Errorf("LLMClient is required")
	}

	// 从输入获取 messages
	messages, ok := inputs["messages"].([]any)
	if !ok || len(messages) == 0 {
		return nil, fmt.Errorf("messages input is required")
	}

	// 获取 schema 参数（可选）
	var schemaStr string
	if schema, ok := params["schema"]; ok {
		schemaStr, _ = schema.(string)
	}

	// 构建提示词，要求返回 JSON
	prompt := "Please respond with valid JSON only."
	if schemaStr != "" {
		prompt = fmt.Sprintf("Please respond with valid JSON matching this schema: %s", schemaStr)
	}

	// 添加 JSON 提示到 messages
	messagesWithPrompt := append(messages, map[string]any{
		"role":    "user",
		"content": prompt,
	})

	// 调用 LLM
	response, err := rc.LLMClient.Chat(ctx, messagesWithPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM chat failed: %w", err)
	}

	// 解析 JSON
	var jsonData any
	if err := json.Unmarshal([]byte(response), &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// 返回输出
	return map[string]any{
		"json": jsonData,
	}, nil
}

// Spec 返回节点规范
func (n *JSONNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "LLM.JSON",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "json", Type: types.PortTypeJSON, Required: true},
		},
		Runner: n,
	}
}
