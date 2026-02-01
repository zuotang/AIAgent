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
		systemPrompt = fmt.Sprintf(`你是角色扮演对话的上下文压缩器。将对话历史压缩为“可继续扮演”的简短摘要（不超过%d字）。

核心目标：保持互动体验与剧情连贯性，宁可少概括，也不要丢失角色人设与剧情关键点。

保留优先级（从高到低）：
1) 角色人设与关系：称呼、口头禅、性格设定、关系进展、隐含规则
2) 剧情主线与关键事件：触发点、承诺、冲突、伏笔、任务目标
3) 场景与状态：时间地点、氛围、当前行动、重要物品/线索
4) 情感语气：情绪变化、互动基调、重要对白语气

压缩规则：
- 去除重复、寒暄、无关信息
- 合并同义信息，只保留一次
- 保持时间顺序与因果关系
- 保留必要的原话短语（口头禅/关键承诺），其余改写

输出格式：
直接输出压缩后的摘要，不要添加任何解释或标记。`, maxLength)
	} else {
		// 增量压缩
		systemPrompt = fmt.Sprintf(`你是角色扮演对话的上下文压缩器。基于“之前摘要 + 新对话”，生成更新后的摘要（不超过%d字）。

核心目标：保持互动体验与剧情连贯性，宁可少概括，也不要丢失角色人设与剧情关键点。

合并规则：
- 将新对话的重要信息融合进摘要，保持连贯
- 重复信息只保留一份
- 突出新出现的剧情推进、关系变化、情绪转折
- 如果超长，优先保留最新且最关键的信息

保留优先级（从高到低）：
1) 角色人设与关系：称呼、口头禅、性格设定、关系进展、隐含规则
2) 剧情主线与关键事件：触发点、承诺、冲突、伏笔、任务目标
3) 场景与状态：时间地点、氛围、当前行动、重要物品/线索
4) 情感语气：情绪变化、互动基调、重要对白语气

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
	systemPrompt := fmt.Sprintf(`你是角色扮演对话的上下文压缩器。将对话历史压缩为“可继续扮演”的简短摘要（不超过%d字）。

核心目标：保持互动体验与剧情连贯性，宁可少概括，也不要丢失角色人设与剧情关键点。

保留优先级（从高到低）：
1) 角色人设与关系：称呼、口头禅、性格设定、关系进展、隐含规则
2) 剧情主线与关键事件：触发点、承诺、冲突、伏笔、任务目标
3) 场景与状态：时间地点、氛围、当前行动、重要物品/线索
4) 情感语气：情绪变化、互动基调、重要对白语气

压缩规则：
- 去除重复、寒暄、无关信息
- 合并同义信息，只保留一次
- 保持时间顺序与因果关系
- 保留必要的原话短语（口头禅/关键承诺），其余改写

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
