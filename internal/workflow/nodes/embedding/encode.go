package embedding

import (
	"context"
	"fmt"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// EncodeNode Embedding.Encode 节点 — 文本向量化
type EncodeNode struct{}

// Run 执行节点
func (n *EncodeNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	if rc.EmbedClient == nil {
		return nil, fmt.Errorf("EmbedClient not configured")
	}

	text, ok := inputs["text"].(string)
	if !ok || text == "" {
		return nil, fmt.Errorf("text input is required")
	}

	embedding, err := rc.EmbedClient.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("embedding failed: %w", err)
	}

	return map[string]any{
		"embedding": embedding,
	}, nil
}

// Spec 返回节点规范
func (n *EncodeNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Embedding.Encode",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "embedding", Type: types.PortTypeEmbedding, Required: true},
		},
		Runner: n,
	}
}
