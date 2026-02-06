package transform

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// TextToMessagesNode Transform.TextToMessages 节点
// 将 text 转换为 messages 格式
type TextToMessagesNode struct{}

func (n *TextToMessagesNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	text, ok := inputs["text"].(string)
	if !ok {
		return nil, fmt.Errorf("text input is required")
	}

	// 获取角色（默认 user）
	role := "user"
	if r, ok := params["role"].(string); ok && r != "" {
		role = r
	}

	// 转换为 messages 格式
	messages := []any{
		map[string]any{
			"role":    role,
			"content": text,
		},
	}

	return map[string]any{
		"messages": messages,
	}, nil
}

func (n *TextToMessagesNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Transform.TextToMessages",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
		},
		Runner: n,
	}
}

// MessagesToTextNode Transform.MessagesToText 节点
// 将 messages 转换为 text
type MessagesToTextNode struct{}

func (n *MessagesToTextNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	messages, ok := inputs["messages"]
	if !ok {
		return nil, fmt.Errorf("messages input is required")
	}

	// 如果是字符串，直接返回
	if text, ok := messages.(string); ok {
		return map[string]any{
			"text": text,
		}, nil
	}

	// 如果是消息列表，提取最后一条消息的内容
	if msgList, ok := messages.([]any); ok && len(msgList) > 0 {
		lastMsg := msgList[len(msgList)-1]
		if msgMap, ok := lastMsg.(map[string]any); ok {
			if content, ok := msgMap["content"].(string); ok {
				return map[string]any{
					"text": content,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("invalid messages format")
}

func (n *MessagesToTextNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Transform.MessagesToText",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Runner: n,
	}
}

// JSONToTextNode Transform.JSONToText 节点
// 将 JSON 转换为 text
type JSONToTextNode struct{}

func (n *JSONToTextNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	jsonData, ok := inputs["json"]
	if !ok {
		return nil, fmt.Errorf("json input is required")
	}

	// 序列化为 JSON 字符串
	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return map[string]any{
		"text": string(jsonBytes),
	}, nil
}

func (n *JSONToTextNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Transform.JSONToText",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "json", Type: types.PortTypeJSON, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Runner: n,
	}
}

// TextToJSONNode Transform.TextToJSON 节点
// 将 text 转换为 JSON
type TextToJSONNode struct{}

func (n *TextToJSONNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	text, ok := inputs["text"].(string)
	if !ok {
		return nil, fmt.Errorf("text input is required")
	}

	// 解析 JSON
	var jsonData any
	if err := json.Unmarshal([]byte(text), &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return map[string]any{
		"json": jsonData,
	}, nil
}

func (n *TextToJSONNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Transform.TextToJSON",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "json", Type: types.PortTypeJSON, Required: true},
		},
		Runner: n,
	}
}
