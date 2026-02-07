package vector

import (
	"context"
	"fmt"
	"strings"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// UpsertNode Vector.Upsert 节点 — 向量写入
type UpsertNode struct{}

// Run 执行节点
func (n *UpsertNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	if rc.QdrantClient == nil {
		return nil, fmt.Errorf("QdrantClient not configured")
	}

	text, ok := inputs["text"].(string)
	if !ok || text == "" {
		return nil, fmt.Errorf("text input is required")
	}

	// 读取参数
	collection := "knowledge"
	if c, ok := params["collection"].(string); ok && c != "" {
		collection = c
	}

	userID := "default"
	if u, ok := params["user_id"].(string); ok && u != "" {
		userID = u
	}

	agentID := uint(1)
	if a, ok := params["agent_id"].(float64); ok {
		agentID = uint(a)
	}

	fileName := ""
	if f, ok := params["file_name"].(string); ok {
		fileName = f
	}

	// 按行拆分文本
	lines := strings.Split(text, "\n")
	texts := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			texts = append(texts, trimmed)
		}
	}

	if len(texts) == 0 {
		return map[string]any{
			"result": map[string]any{"success": true, "count": 0},
		}, nil
	}

	// 根据 collection 选择写入方法
	var err error
	switch collection {
	case "knowledge":
		err = rc.QdrantClient.UpsertKnowledgeTexts(ctx, agentID, texts, fileName)
	case "memory":
		err = rc.QdrantClient.UpsertMemoryTexts(ctx, userID, agentID, texts)
	default:
		return nil, fmt.Errorf("unknown collection: %s (use 'knowledge' or 'memory')", collection)
	}

	if err != nil {
		return nil, fmt.Errorf("vector upsert failed: %w", err)
	}

	return map[string]any{
		"result": map[string]any{"success": true, "count": len(texts)},
	}, nil
}

// Spec 返回节点规范
func (n *UpsertNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Vector.Upsert",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "result", Type: types.PortTypeJSON, Required: true},
		},
		Runner: n,
	}
}
