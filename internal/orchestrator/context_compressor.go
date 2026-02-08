package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"agent-langchain/internal/memory"
	"agent-langchain/internal/models"
	"agent-langchain/internal/utils"
)

// CompressContextIncremental 增量压缩上下文
// 基于上次压缩的结果和新的对话内容进行压缩
func CompressContextIncremental(
	ctx context.Context,
	llm models.LLMClient,
	store *memory.Store,
	userID string,
	agentID uint,
	newConversation string,
	lastMessageID uint,
	model string,
	maxLength int,
) (string, error) {
	// 获取上次压缩的上下文
	lastCompressed, err := store.GetCompressedContext(ctx, userID, agentID)
	var previousSummary string
	if err != nil {
		// 第一次压缩，没有历史
		previousSummary = ""
	} else {
		previousSummary = lastCompressed.CompressedText
	}

	// 如果新对话为空，返回上次的压缩结果
	if newConversation == "" {
		return previousSummary, nil
	}

	// 构建压缩提示词
	var systemPrompt string
	if previousSummary == "" {
		// 第一次压缩
		systemPrompt = fmt.Sprintf(`你是对话的上下文压缩器。将对话历史压缩为“可继续扮演”的简短摘要（不超过%d字）。
你需要保留当前场景，正在做的事情，男女主角位置，最重要的对话，以及其他重要信息。
格式如下
场景:
正在做的事情:
男主位置:
女主位置:
男主姿势:
女主姿势:
最重要的对话:
其他重要信息:
最后对话的意思：
`, maxLength)
	} else {
		// 增量压缩
		systemPrompt = fmt.Sprintf(`你是对话的上下文压缩器。基于“之前摘要 + 新对话”，生成更新后的摘要（不超过%d字）。
你需要保留当前场景，正在做的事情，男女主角位置，最重要的对话，以及其他重要信息。
格式如下
场景:
正在做的事情:
男主位置:
女主位置:
男主姿势:
女主姿势:
最重要的对话:
其他重要信息:
最后对话的意思：
`, maxLength)
	}

	var userPrompt string
	if previousSummary == "" {
		userPrompt = fmt.Sprintf(`请将以下对话历史压缩为简短摘要：

%s

压缩后的摘要：`, newConversation)
	} else {
		userPrompt = fmt.Sprintf(`之前的摘要：
%s

新的对话：
%s

请生成更新后的摘要：`, previousSummary, newConversation)
	}

	msgs := []models.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// 使用小型模型进行压缩
	compressed, err := llm.Chat(ctx, msgs, model)
	if err != nil {
		return "", fmt.Errorf("incremental compression failed: %w", err)
	}

	compressed = strings.TrimSpace(compressed)

	// 保存压缩结果和最后处理的消息ID
	compressedCtx := &memory.CompressedContext{
		UserID:          userID,
		AgentID:         agentID, // 必须设置 AgentID，否则查询时找不到
		CompressedText:  compressed,
		LastMessageID:   lastMessageID,
		UncompressedLen: 0, // 重置未压缩长度
	}
	if err := store.UpsertCompressedContext(ctx, compressedCtx); err != nil {
		// 保存失败不影响返回结果
		fmt.Printf("Warning: failed to save compressed context: %v\n", err)
	}

	return compressed, nil
}

// ShouldCompressContext 判断是否需要压缩上下文
// 当上下文超过一定长度时才压缩
func ShouldCompressContext(conversationHistory string, threshold int) bool {
	return len(conversationHistory) > threshold
}

// ShouldCompressContextByTokens 判断是否需要基于 token 使用率压缩
// thresholdPercent 表示总上下文占用率触发压缩（例如 60 表示占用 60%）
func ShouldCompressContextByTokens(stats utils.ContextStats, thresholdPercent float64) bool {
	if stats.ModelLimit <= 0 {
		return false
	}
	if thresholdPercent <= 0 {
		thresholdPercent = 60.0
	}
	return stats.UsagePercent >= thresholdPercent
}
