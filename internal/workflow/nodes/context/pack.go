package context

import (
	"context"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// PackNode Context.Pack 节点
// 输入: system_messages, user_messages, messages, kb_docs, memory_items, tool_result (all optional)
// 输出: context_pack
// 多个 messages 输入按 system -> user -> messages 顺序合并
type PackNode struct{}

// Run 执行节点
func (n *PackNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	contextPack := make(map[string]any)

	// 按顺序合并多个 messages 输入: system_messages -> user_messages -> messages
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

	if len(allMessages) > 0 {
		contextPack["messages"] = allMessages
	}

	if kbDocs, ok := inputs["kb_docs"]; ok {
		contextPack["kb_docs"] = kbDocs
	}
	if memoryItems, ok := inputs["memory_items"]; ok {
		contextPack["memory_items"] = memoryItems
	}
	if toolResult, ok := inputs["tool_result"]; ok {
		contextPack["tool_result"] = toolResult
	}

	return map[string]any{
		"context_pack": contextPack,
	}, nil
}

// toSlice 将输入转为 []any
func toSlice(v any) ([]any, bool) {
	switch val := v.(type) {
	case []any:
		return val, true
	case string:
		// 单条文本当作一条 user message
		return []any{map[string]any{"role": "user", "content": val}}, true
	default:
		return nil, false
	}
}

// Spec 返回节点规范
func (n *PackNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Context.Pack",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "system_messages", Type: types.PortTypeMessages, Required: false},
			{Name: "user_messages", Type: types.PortTypeMessages, Required: false},
			{Name: "messages", Type: types.PortTypeMessages, Required: false},
			{Name: "kb_docs", Type: types.PortTypeKBDocs, Required: false},
			{Name: "memory_items", Type: types.PortTypeMemoryItems, Required: false},
			{Name: "tool_result", Type: types.PortTypeToolResult, Required: false},
		},
		Outputs: []types.PortSpec{
			{Name: "context_pack", Type: types.PortTypeContextPack, Required: true},
		},
		Runner: n,
	}
}
