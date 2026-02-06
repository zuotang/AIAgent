package context

import (
	"context"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// PackNode Context.Pack 节点
// 输入: messages, kb_docs, memory_items, tool_result (all optional)
// 输出: context_pack
type PackNode struct{}

// Run 执行节点
func (n *PackNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	// 打包所有输入到 context_pack
	contextPack := make(map[string]any)

	if messages, ok := inputs["messages"]; ok {
		contextPack["messages"] = messages
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

	// 返回输出
	return map[string]any{
		"context_pack": contextPack,
	}, nil
}

// Spec 返回节点规范
func (n *PackNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Context.Pack",
		Version: "1.0",
		Inputs: []types.PortSpec{
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
