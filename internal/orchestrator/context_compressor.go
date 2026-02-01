package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"agent-langchain/internal/memory"
	"agent-langchain/internal/models"
)

// CompressContextIncremental 增量压缩上下文
// 基于上次压缩的结果和新的对话内容进行压缩
func CompressContextIncremental(
	ctx context.Context,
	llm models.LLMClient,
	store *memory.Store,
	userID string,
	newConversation string,
	lastMessageID uint,
	model string,
	maxLength int,
) (string, error) {
	// 获取上次压缩的上下文
	lastCompressed, err := store.GetCompressedContext(ctx, userID)
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
		systemPrompt = fmt.Sprintf(`你是上下文压缩器。将对话历史压缩为简短摘要（保持但不超过%d字）。

压缩原则：
1. 保留关键信息：人物、事件、环境、动作、重要细节
2. 去除冗余：重复内容、客套话、无关信息
3. 使用简洁语言：用最少的字表达最多的信息
4. 保持时序：按对话顺序组织信息
5. 突出重点：强调重要的转折和关键点

输出格式：
直接输出压缩后的摘要，不要添加任何解释或标记。`, maxLength)
	} else {
		// 增量压缩
		systemPrompt = fmt.Sprintf(`你是上下文压缩器。基于之前的摘要和新的对话，生成更新后的摘要（保持但不超过%d字）。

压缩原则：
1. 整合信息：将新对话的关键信息整合到之前的摘要中
2. 保持连贯：确保新旧信息的逻辑连贯性
3. 去除冗余：如果新对话重复了旧信息，只保留一份
4. 突出新内容：重点关注新对话中的重要信息
5. 控制长度：如果超长，优先保留最新和最重要的信息

输出格式：
直接输出更新后的摘要，不要添加任何解释或标记。`, maxLength)
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

// CompressContext 使用小型 LLM 压缩对话上下文（旧版本，保留兼容性）
// 将长对话历史压缩为简短摘要，节省 tokens
func CompressContext(
	ctx context.Context,
	llm models.LLMClient,
	conversationHistory string,
	model string,
	maxLength int,
) (string, error) {
	// 如果上下文为空或很短，直接返回
	if conversationHistory == "" || len(conversationHistory) < 100 {
		return conversationHistory, nil
	}

	// 压缩提示词
	systemPrompt := fmt.Sprintf(`你是上下文压缩器。将对话历史压缩为简短摘要（保持但不超过%d字）。

压缩原则：
1. 保留关键信息：人物、事件、环境、动作重要细节
2. 去除冗余：重复内容、客套话、无关信息
3. 使用简洁语言：用最少的字表达最多的信息
4. 保持时序：按对话顺序组织信息
5. 突出重点：强调重要的转折和关键点

输出格式：
直接输出压缩后的摘要，不要添加任何解释或标记。`, maxLength)

	userPrompt := fmt.Sprintf(`请将以下对话历史压缩为简短摘要：

%s

压缩后的摘要：`, conversationHistory)

	msgs := []models.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// 使用小型模型进行压缩
	compressed, err := llm.Chat(ctx, msgs, model)
	if err != nil {
		return "", fmt.Errorf("context compression failed: %w", err)
	}

	compressed = strings.TrimSpace(compressed)

	// 如果压缩后反而更长，返回原文
	if len(compressed) > len(conversationHistory) {
		return conversationHistory, nil
	}

	return compressed, nil
}

// ShouldCompressContext 判断是否需要压缩上下文
// 当上下文超过一定长度时才压缩
func ShouldCompressContext(conversationHistory string, threshold int) bool {
	return len(conversationHistory) > threshold
}
