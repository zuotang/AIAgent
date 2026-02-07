package context

import (
	"context"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// SummaryNode Context.Summary 节点
// 输入: context_pack
// 输出: context_pack
type SummaryNode struct{}

func (n *SummaryNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	contextPack, ok := inputs["context_pack"].(map[string]any)
	if !ok {
		return nil, nil
	}
	return map[string]any{"context_pack": contextPack}, nil
}

func (n *SummaryNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Context.Summary",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "context_pack", Type: types.PortTypeContextPack, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "context_pack", Type: types.PortTypeContextPack, Required: true},
		},
		Runner: n,
	}
}
