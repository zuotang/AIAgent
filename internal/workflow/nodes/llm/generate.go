package llm

import (
	"context"
	"fmt"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// GenerateNode LLM.Generate 节点
// 输入: messages ([]ChatMessage) 或 context_pack
// 输出: messages (string)
type GenerateNode struct{}

// Run 执行节点
func (n *GenerateNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	// 获取 LLM 客户端
	if rc.LLMClient == nil {
		return nil, fmt.Errorf("LLMClient is required")
	}

	// 从输入获取 messages
	var messages []any
	if contextPack, ok := inputs["context_pack"]; ok {
		// 从 context_pack 中提取 messages
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

	// 获取模型参数（可选）
	var model string
	if m, ok := params["model"]; ok {
		model, _ = m.(string)
	}

	// 调用 LLM
	response, err := rc.LLMClient.Chat(ctx, messages, model)
	if err != nil {
		return nil, fmt.Errorf("LLM chat failed: %w", err)
	}

	// 返回输出
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
			{Name: "messages", Type: types.PortTypeMessages, Required: false},
			{Name: "context_pack", Type: types.PortTypeContextPack, Required: false},
		},
		Outputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
		},
		Runner: n,
	}
}
