package context

import (
	"context"
	"fmt"
	_ "strings"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// WindowCheckNode Context.WindowCheck 节点
// 输入: context_pack
// 输出: over (text "yes"/"no")
type WindowCheckNode struct{}

func (n *WindowCheckNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	contextPack, ok := inputs["context_pack"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("context_pack input is required")
	}

	maxChars := 4000
	if v, ok := params["max_chars"].(float64); ok && v > 0 {
		maxChars = int(v)
	}

	total := 0
	if messages, ok := contextPack["messages"].([]any); ok {
		for _, msg := range messages {
			if m, ok := msg.(map[string]any); ok {
				if content, ok := m["content"].(string); ok {
					total += len(content)
				}
			}
		}
	}

	over := "no"
	if total > maxChars {
		over = "yes"
	}

	return map[string]any{"over": over}, nil
}

func (n *WindowCheckNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Context.WindowCheck",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "context_pack", Type: types.PortTypeContextPack, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "over", Type: types.PortTypeText, Required: true},
		},
		Runner: n,
	}
}
