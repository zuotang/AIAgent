package memory

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// SaveNode Memory.Save 节点 — 将提取的记忆写入 SQLite + 可选写入 Qdrant
type SaveNode struct{}

// Run 执行节点
func (n *SaveNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	if rc.MemoryStore == nil {
		return nil, fmt.Errorf("MemoryStore not configured")
	}

	// 解析输入的 memories JSON
	memoriesRaw, ok := inputs["memories"]
	if !ok {
		return nil, fmt.Errorf("memories input is required")
	}

	var memories []registry.ExtractedMemoryItem

	switch v := memoriesRaw.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &memories); err != nil {
			return nil, fmt.Errorf("parse memories JSON failed: %w", err)
		}
	case []any:
		// 从 []any 转换（可能来自上游节点直接传递）
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal memories failed: %w", err)
		}
		if err := json.Unmarshal(data, &memories); err != nil {
			return nil, fmt.Errorf("parse memories failed: %w", err)
		}
	default:
		// 尝试 JSON 序列化再反序列化
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("unsupported memories input type: %T", v)
		}
		if err := json.Unmarshal(data, &memories); err != nil {
			return nil, fmt.Errorf("parse memories failed: %w", err)
		}
	}

	if len(memories) == 0 {
		return map[string]any{
			"result": map[string]any{
				"success":       true,
				"sqlite_count":  0,
				"vector_count":  0,
			},
		}, nil
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

	alsoVector := true
	if v, ok := params["also_vector"].(bool); ok {
		alsoVector = v
	}

	// 写入 SQLite
	if err := rc.MemoryStore.UpsertExtractedMemories(ctx, userID, agentID, memories); err != nil {
		return nil, fmt.Errorf("upsert memories to SQLite failed: %w", err)
	}

	// 可选写入 Qdrant
	vectorCount := 0
	if alsoVector && rc.QdrantClient != nil {
		var vectorTexts []string
		seen := make(map[string]struct{})

		for _, m := range memories {
			if !m.AlsoVector {
				continue
			}

			fp := fmt.Sprintf("%s:%s:%s:%s", m.Owner, m.Type, m.Key, m.Value)
			if _, ok := seen[fp]; ok {
				continue
			}
			seen[fp] = struct{}{}

			vt := fmt.Sprintf("%s | %s:%s", m.Owner, m.Key, m.Value)
			if m.Text != "" {
				vt = fmt.Sprintf("%s | %s", m.Owner, m.Text)
			}
			if len(vt) > 240 {
				vt = vt[:240] + "..."
			}
			vectorTexts = append(vectorTexts, vt)
		}

		if len(vectorTexts) > 0 {
			if err := rc.QdrantClient.UpsertMemoryTexts(ctx, userID, agentID, vectorTexts); err != nil {
				return nil, fmt.Errorf("upsert memories to Qdrant failed: %w", err)
			}
			vectorCount = len(vectorTexts)
		}
	}

	return map[string]any{
		"result": map[string]any{
			"success":      true,
			"sqlite_count": len(memories),
			"vector_count": vectorCount,
		},
	}, nil
}

// Spec 返回节点规范
func (n *SaveNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Memory.Save",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "memories", Type: types.PortTypeJSON, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "result", Type: types.PortTypeJSON, Required: true},
		},
		Runner: n,
	}
}
