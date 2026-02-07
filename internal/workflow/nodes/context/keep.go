package context

import (
	"context"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// KeepRecentNode Context.KeepRecent 节点
type KeepRecentNode struct{}

func (n *KeepRecentNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	contextPack, ok := inputs["context_pack"].(map[string]any)
	if !ok {
		return nil, nil
	}
	return map[string]any{"context_pack": contextPack}, nil
}

func (n *KeepRecentNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Context.KeepRecent",
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

// KeepCitationsNode Context.KeepCitations 节点
type KeepCitationsNode struct{}

func (n *KeepCitationsNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	contextPack, ok := inputs["context_pack"].(map[string]any)
	if !ok {
		return nil, nil
	}
	return map[string]any{"context_pack": contextPack}, nil
}

func (n *KeepCitationsNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Context.KeepCitations",
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
