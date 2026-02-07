package context

import (
	"context"
	"fmt"
	"strings"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// AssembleNode Context.Assemble 节点
// 输入: system_messages, user_messages, messages, evidence(text), memory(text)
// 输出: context_pack
type AssembleNode struct{}

func (n *AssembleNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	contextPack := make(map[string]any)

	var allMessages []any
	if sysMsg, ok := inputs["system_messages"]; ok {
		if msgs, ok := toSlice(sysMsg); ok {
			allMessages = append(allMessages, msgs...)
		}
	}
	if userMsg, ok := inputs["user_messages"]; ok {
		if msgs, ok := toSlice(userMsg); ok {
			allMessages = append(allMessages, msgs...)
		}
	}
	if messages, ok := inputs["messages"]; ok {
		if msgs, ok := toSlice(messages); ok {
			allMessages = append(allMessages, msgs...)
		}
	}

	if evidence, ok := inputs["evidence"].(string); ok && strings.TrimSpace(evidence) != "" {
		allMessages = append(allMessages, map[string]any{"role": "system", "content": evidence})
	}
	if memory, ok := inputs["memory"].(string); ok && strings.TrimSpace(memory) != "" {
		allMessages = append(allMessages, map[string]any{"role": "system", "content": memory})
	}

	if len(allMessages) == 0 {
		return nil, fmt.Errorf("no messages to assemble")
	}

	contextPack["messages"] = allMessages

	return map[string]any{"context_pack": contextPack}, nil
}

func (n *AssembleNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Context.Assemble",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "system_messages", Type: types.PortTypeMessages, Required: false},
			{Name: "user_messages", Type: types.PortTypeMessages, Required: false},
			{Name: "messages", Type: types.PortTypeMessages, Required: false},
			{Name: "evidence", Type: types.PortTypeText, Required: false},
			{Name: "memory", Type: types.PortTypeText, Required: false},
		},
		Outputs: []types.PortSpec{
			{Name: "context_pack", Type: types.PortTypeContextPack, Required: true},
		},
		Runner: n,
	}
}
