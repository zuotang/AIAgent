package memory

import (
	"context"
	"fmt"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// QueryNode Memory.Query 节点 — 结构化记忆查询
type QueryNode struct{}

// Run 执行节点
func (n *QueryNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	if rc.MemoryStore == nil {
		return nil, fmt.Errorf("MemoryStore not configured")
	}

	// 读取参数
	userID := "default"
	if u, ok := params["user_id"].(string); ok && u != "" {
		userID = u
	}

	agentID := uint(1)
	if a, ok := params["agent_id"].(float64); ok {
		agentID = uint(a)
	}

	limit := 50
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	text, err := rc.MemoryStore.RenderStructuredMemory(ctx, userID, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("memory query failed: %w", err)
	}

	return map[string]any{
		"text": text,
	}, nil
}

// Spec 返回节点规范
func (n *QueryNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Memory.Query",
		Version: "1.0",
		Inputs:  []types.PortSpec{},
		Outputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Runner: n,
	}
}
