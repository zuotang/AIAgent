package utils

import (
	"fmt"
	"unicode"
)

// EstimateTokens 估算文本的token数量
// 简单规则：
// - 英文单词 ≈ 1.3 tokens
// - 中文字符 ≈ 2 tokens
// - 标点符号 ≈ 1 token
// 注意：这是粗略估算，实际token数可能有10-20%的误差
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	words := 0
	chineseChars := 0
	punctuation := 0
	inWord := false

	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			// 中文字符
			chineseChars++
			inWord = false
		} else if unicode.IsSpace(r) {
			// 空格
			if inWord {
				words++
				inWord = false
			}
		} else if unicode.IsPunct(r) {
			// 标点符号
			punctuation++
			if inWord {
				words++
				inWord = false
			}
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			// 字母或数字
			inWord = true
		}
	}

	// 处理最后一个单词
	if inWord {
		words++
	}

	// 计算总token数
	// 中文: 每个字符约2 tokens
	// 英文: 每个单词约1.3 tokens
	// 标点: 每个约1 token
	totalTokens := int(float64(words)*1.3 + float64(chineseChars)*2.0 + float64(punctuation)*0.5)

	return totalTokens
}

// GetModelContextLimit 获取模型的上下文限制
func GetModelContextLimit(model string) int {
	limits := map[string]int{
		// Ollama models
		"gemma3:12b":      8192,
		"gemma2:9b":       8192,
		"qwen2.5:7b":      32768,
		"qwen2.5:14b":     32768,
		"llama3.1:8b":     128000,
		"llama3.2:3b":     128000,
		"mistral:7b":      8192,
		"mixtral:8x7b":    32768,

		// DeepSeek models
		"deepseek-chat":   64000,
		"deepseek-coder":  64000,

		// Default
		"default":         4096,
	}

	if limit, ok := limits[model]; ok {
		return limit
	}

	// 如果找不到，返回默认值
	return limits["default"]
}

// ContextStats 上下文统计信息
type ContextStats struct {
	SystemPromptTokens int
	MemoryTokens       int
	ConversationTokens int
	UserInputTokens    int
	TotalTokens        int
	ModelLimit         int
	UsagePercent       float64
}

// CalculateContextStats 计算上下文统计
func CalculateContextStats(systemPrompt, memory, conversation, userInput, model string) ContextStats {
	stats := ContextStats{
		SystemPromptTokens: EstimateTokens(systemPrompt),
		MemoryTokens:       EstimateTokens(memory),
		ConversationTokens: EstimateTokens(conversation),
		UserInputTokens:    EstimateTokens(userInput),
		ModelLimit:         GetModelContextLimit(model),
	}

	stats.TotalTokens = stats.SystemPromptTokens + stats.MemoryTokens +
		stats.ConversationTokens + stats.UserInputTokens

	if stats.ModelLimit > 0 {
		stats.UsagePercent = float64(stats.TotalTokens) / float64(stats.ModelLimit) * 100
	}

	return stats
}

// FormatContextStats 格式化上下文统计信息
func FormatContextStats(stats ContextStats) string {
	result := "\n[Context Window Stats]\n"
	result += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
	result += formatLine("System Prompt", stats.SystemPromptTokens)
	result += formatLine("Memory", stats.MemoryTokens)
	result += formatLine("Conversation", stats.ConversationTokens)
	result += formatLine("User Input", stats.UserInputTokens)
	result += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
	result += formatLine("Total", stats.TotalTokens)
	result += formatLine("Model Limit", stats.ModelLimit)
	result += formatPercent("Usage", stats.UsagePercent)
	result += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"

	// 警告信息
	if stats.UsagePercent > 90 {
		result += "⚠️  WARNING: Context window usage > 90%!\n"
		result += "   Consider reducing conversation history.\n"
	} else if stats.UsagePercent > 80 {
		result += "⚠️  CAUTION: Context window usage > 80%\n"
	}

	return result
}

func formatLine(label string, value int) string {
	return fmt.Sprintf("  %-20s: %6d tokens\n", label, value)
}

func formatPercent(label string, value float64) string {
	return fmt.Sprintf("  %-20s: %6.1f%%\n", label, value)
}
