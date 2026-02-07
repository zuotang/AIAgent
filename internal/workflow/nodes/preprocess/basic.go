package preprocess

import (
	"context"
	"strings"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// BasicNode Preprocess.Basic 节点
// 输入: text
// 输出: text, need_kb, mem_action
type BasicNode struct{}

func (n *BasicNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	text, _ := inputs["text"].(string)
	cleaned := strings.TrimSpace(text)

	needKB := "no"
	lower := strings.ToLower(cleaned)
	if strings.Contains(lower, "什么") || strings.Contains(lower, "怎么") || strings.Contains(lower, "资料") || strings.Contains(lower, "知识") || strings.Contains(lower, "what") || strings.Contains(lower, "how") {
		needKB = "yes"
	}

	memAction := "none"

	return map[string]any{
		"text":      cleaned,
		"need_kb":   needKB,
		"mem_action": memAction,
	}, nil
}

func (n *BasicNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Preprocess.Basic",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
			{Name: "need_kb", Type: types.PortTypeText, Required: true},
			{Name: "mem_action", Type: types.PortTypeText, Required: true},
		},
		Runner: n,
	}
}
