package orchestrator

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"agent-langchain/internal/rag"
	"agent-langchain/internal/utils"
)

// formatMemories 格式化记忆为文本
func (o *orchestrator) formatMemories(structured string, semantic []rag.Doc) string {
	var sb strings.Builder

	sb.WriteString("【角色记忆库（按需加载）】\n")
	if structured != "" && structured != "(暂无长期记忆)" {
		sb.WriteString("【结构化记忆】\n")
		sb.WriteString(structured)
		sb.WriteString("\n")
	}
	if len(semantic) > 0 {
		sb.WriteString("【剧情/互动线索】\n")
	}
	for i, doc := range semantic {
		if i >= o.config.Storage.Qdrant.TopK {
			break
		}
		sb.WriteString("- " + truncate(doc.PageContent, 220) + "\n")
	}

	return sb.String()
}

// formatKnowledge 格式化知识库内容
func (o *orchestrator) formatKnowledge(docs []rag.Doc) string {
	if len(docs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("【知识库】\n")
	for i, doc := range docs {
		if i >= o.config.Knowledge.TopK {
			break
		}
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, truncate(doc.PageContent, 300)))
	}
	return sb.String()
}

// showContextStats 显示上下文统计
func (o *orchestrator) showContextStats(
	systemPrompt string,
	structuredText string,
	semanticDocs []rag.Doc,
	conversation string,
	userInput string,
) {
	memory := structuredText + "\n"
	for i, doc := range semanticDocs {
		if i >= o.config.Storage.Qdrant.TopK {
			break
		}
		memory += doc.PageContent + "\n"
	}

	stats := utils.CalculateContextStats(
		systemPrompt,
		memory,
		conversation,
		userInput,
		o.chatModel,
	)
	log.Print(utils.FormatContextStats(stats))
}

// RemoveThinkingTags 移除LLM响应中的thinking标签及其内容
// 支持多种thinking标签格式：<think>...</think>, <thinking>...</thinking>, <thinking_mode>...</thinking_mode>
func RemoveThinkingTags(text string) string {
	// 定义需要移除的thinking标签模式
	patterns := []string{
		`<think>[\s\S]*?</think>`,           // <think>...</think>
		`<thinking>[\s\S]*?</thinking>`,     // <thinking>...</thinking>
		`<thinking_mode>[\s\S]*?</thinking_mode>`, // <thinking_mode>...</thinking_mode>
	}

	result := text
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, "")
	}

	// 清理多余的空行（连续的换行符）
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")

	// 清理首尾空白
	result = strings.TrimSpace(result)

	return result
}

// removeThinkingTags 内部使用的小写版本，保持向后兼容
func removeThinkingTags(text string) string {
	return RemoveThinkingTags(text)
}
