package memory

import (
	"context"
	"fmt"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// ChatHistoryNode Memory.ChatHistory 节点 — 聊天历史查询
type ChatHistoryNode struct{}

// Run 执行节点
func (n *ChatHistoryNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
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

	limit := 20
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	chatMessages, err := rc.MemoryStore.GetChatHistory(ctx, userID, agentID, limit, 0)
	if err != nil {
		return nil, fmt.Errorf("chat history query failed: %w", err)
	}

	// 转换为 []any 格式的 messages
	messages := make([]any, 0, len(chatMessages))
	for _, msg := range chatMessages {
		messages = append(messages, map[string]any{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	return map[string]any{
		"messages": messages,
	}, nil
}

// Spec 返回节点规范
func (n *ChatHistoryNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Memory.ChatHistory",
		Version: "1.0",
		Inputs:  []types.PortSpec{},
		Outputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
		},
		Runner: n,
	}
}
