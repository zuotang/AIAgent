package registry

import (
	"context"

	"agent-langchain/internal/models"
)

// LLMClientAdapter 适配器，将 models.LLMClient 适配到工作流的 LLMClient 接口
type LLMClientAdapter struct {
	Client models.LLMClient
}

// Chat 实现工作流的 LLMClient 接口
func (a *LLMClientAdapter) Chat(ctx context.Context, msgs []any, model ...string) (string, error) {
	// 转换 []any 到 []models.ChatMessage
	chatMsgs := make([]models.ChatMessage, 0, len(msgs))
	for _, msg := range msgs {
		switch m := msg.(type) {
		case models.ChatMessage:
			chatMsgs = append(chatMsgs, m)
		case map[string]any:
			// 从 map 构造 ChatMessage
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			chatMsgs = append(chatMsgs, models.ChatMessage{
				Role:    role,
				Content: content,
			})
		case string:
			// 如果是字符串，作为 user 消息
			chatMsgs = append(chatMsgs, models.ChatMessage{
				Role:    "user",
				Content: m,
			})
		}
	}

	return a.Client.Chat(ctx, chatMsgs, model...)
}

// NewLLMClientAdapter 创建 LLM 客户端适配器
func NewLLMClientAdapter(client models.LLMClient) *LLMClientAdapter {
	return &LLMClientAdapter{Client: client}
}
