package memory

import (
	"context"
	"fmt"
	"strings"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// ReadNode Memory.Read 节点
// 输入: user_id(text), agent_id(text)
// 输出: memory_text, messages
type ReadNode struct{}

func (n *ReadNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	if rc.MemoryStore == nil {
		return nil, fmt.Errorf("MemoryStore is required")
	}

	userID, _ := inputs["user_id"].(string)
	if userID == "" {
		userID = "default"
	}
	agentID := uint(getIntParam(params, "agent_id", 1))
	limit := getIntParam(params, "limit", 20)

	text, err := rc.MemoryStore.RenderStructuredMemory(ctx, userID, agentID, limit)
	if err != nil {
		return nil, err
	}

	messages := []any{
		map[string]any{"role": "system", "content": text},
	}

	return map[string]any{
		"memory_text": text,
		"messages":    messages,
	}, nil
}

func (n *ReadNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Memory.Read",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "user_id", Type: types.PortTypeText, Required: false},
		},
		Outputs: []types.PortSpec{
			{Name: "memory_text", Type: types.PortTypeText, Required: true},
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
		},
		Runner: n,
	}
}

// CandidateNode Memory.Candidate 节点
// 输入: text
// 输出: memory_items
type CandidateNode struct{}

func (n *CandidateNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	text, _ := inputs["text"].(string)
	if strings.TrimSpace(text) == "" {
		return map[string]any{"memory_items": []registry.ExtractedMemoryItem{}}, nil
	}

	// 简化：不做复杂提取，返回空列表
	return map[string]any{"memory_items": []registry.ExtractedMemoryItem{}}, nil
}

func (n *CandidateNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Memory.Candidate",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "memory_items", Type: types.PortTypeMemoryItems, Required: true},
		},
		Runner: n,
	}
}

// GateNode Memory.Gate 节点
// 输入: memory_items
// 输出: memory_items
type GateNode struct{}

func (n *GateNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	items, ok := inputs["memory_items"].([]registry.ExtractedMemoryItem)
	if !ok {
		return map[string]any{"memory_items": []registry.ExtractedMemoryItem{}}, nil
	}
	return map[string]any{"memory_items": items}, nil
}

func (n *GateNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Memory.Gate",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "memory_items", Type: types.PortTypeMemoryItems, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "memory_items", Type: types.PortTypeMemoryItems, Required: true},
		},
		Runner: n,
	}
}

// WriteNode Memory.Write 节点
// 输入: memory_items
// 输出: memory_items
type WriteNode struct{}

func (n *WriteNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	if rc.MemoryStore == nil {
		return map[string]any{"memory_items": []registry.ExtractedMemoryItem{}}, nil
	}

	items, ok := inputs["memory_items"].([]registry.ExtractedMemoryItem)
	if !ok || len(items) == 0 {
		return map[string]any{"memory_items": []registry.ExtractedMemoryItem{}}, nil
	}

	userID, _ := params["user_id"].(string)
	if userID == "" {
		userID = "default"
	}
	agentID := uint(getIntParam(params, "agent_id", 1))

	if err := rc.MemoryStore.UpsertExtractedMemories(ctx, userID, agentID, items); err != nil {
		return nil, err
	}

	return map[string]any{"memory_items": items}, nil
}

func (n *WriteNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Memory.Write",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "memory_items", Type: types.PortTypeMemoryItems, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "memory_items", Type: types.PortTypeMemoryItems, Required: true},
		},
		Runner: n,
	}
}

// InjectNode Context.InjectMemory 节点
// 输入: memory_text
// 输出: messages
type InjectNode struct{}

func (n *InjectNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	text, _ := inputs["memory_text"].(string)
	if strings.TrimSpace(text) == "" {
		text = ""
	}
	messages := []any{
		map[string]any{"role": "system", "content": text},
	}
	return map[string]any{"messages": messages, "memory": text}, nil
}

func (n *InjectNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Context.InjectMemory",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "memory_text", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
			{Name: "memory", Type: types.PortTypeText, Required: false},
		},
		Runner: n,
	}
}

func getIntParam(params map[string]any, key string, defaultValue int) int {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			var i int
			fmt.Sscanf(v, "%d", &i)
			return i
		}
	}
	return defaultValue
}
