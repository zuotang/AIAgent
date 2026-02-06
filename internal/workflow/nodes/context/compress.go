package context

import (
	"context"
	"fmt"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// CompressNode Context.Compress 节点
// 输入: context_pack
// 输出: context_pack (compressed)
// 依赖: LLMClient
type CompressNode struct{}

// Run 执行节点
func (n *CompressNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	// 获取 LLM 客户端
	if rc.LLMClient == nil {
		return nil, fmt.Errorf("LLMClient is required")
	}

	// 从输入获取 context_pack
	contextPack, ok := inputs["context_pack"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("context_pack input is required")
	}

	// TODO: 实现上下文压缩逻辑
	// 这里应该使用 LLM 来压缩上下文
	// 目前先返回原始 context_pack

	// 示例：可以提取 messages 并让 LLM 总结
	if messages, ok := contextPack["messages"]; ok {
		if msgList, ok := messages.([]any); ok && len(msgList) > 0 {
			// 构建压缩提示
			compressPrompt := []any{
				map[string]any{
					"role":    "system",
					"content": "Please summarize the following conversation concisely.",
				},
			}
			compressPrompt = append(compressPrompt, msgList...)

			// 调用 LLM 压缩
			summary, err := rc.LLMClient.Chat(ctx, compressPrompt)
			if err != nil {
				return nil, fmt.Errorf("context compression failed: %w", err)
			}

			// 更新 context_pack
			contextPack["messages"] = []any{
				map[string]any{
					"role":    "assistant",
					"content": summary,
				},
			}
		}
	}

	// 返回压缩后的 context_pack
	return map[string]any{
		"context_pack": contextPack,
	}, nil
}

// Spec 返回节点规范
func (n *CompressNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Context.Compress",
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
