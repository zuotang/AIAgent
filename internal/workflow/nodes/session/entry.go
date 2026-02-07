package session

import (
	"context"
	"time"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// EntryNode Session.Entry 节点
// 输入: text
// 输出: text, meta(json)
type EntryNode struct{}

func (n *EntryNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	text, _ := inputs["text"].(string)

	meta := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
	}
	if channel, ok := params["channel"].(string); ok && channel != "" {
		meta["channel"] = channel
	}

	return map[string]any{
		"text": text,
		"meta": meta,
	}, nil
}

func (n *EntryNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Session.Entry",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
			{Name: "meta", Type: types.PortTypeJSON, Required: false},
		},
		Runner: n,
	}
}
