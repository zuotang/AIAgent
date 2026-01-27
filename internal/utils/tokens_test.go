package utils

import (
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
		margin   int // 允许的误差范围
	}{
		{"empty", "", 0, 0},
		{"simple english", "Hello world", 3, 1},
		{"chinese", "你好世界", 8, 2},
		{"mixed", "Hello 世界", 5, 2},
		{"long english", "The quick brown fox jumps over the lazy dog", 12, 3},
		{"long chinese", "这是一个测试句子，用来验证token计数功能", 40, 10},
		{"with punctuation", "Hello, world! How are you?", 8, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.text)
			if got < tt.expected-tt.margin || got > tt.expected+tt.margin {
				t.Errorf("EstimateTokens(%q) = %d, want %d ± %d", tt.text, got, tt.expected, tt.margin)
			}
		})
	}
}

func TestGetModelContextLimit(t *testing.T) {
	tests := []struct {
		model    string
		expected int
	}{
		{"gemma3:12b", 8192},
		{"qwen2.5:7b", 32768},
		{"deepseek-chat", 64000},
		{"llama3.1:8b", 128000},
		{"unknown-model", 4096}, // default
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := GetModelContextLimit(tt.model)
			if got != tt.expected {
				t.Errorf("GetModelContextLimit(%q) = %d, want %d", tt.model, got, tt.expected)
			}
		})
	}
}

func TestCalculateContextStats(t *testing.T) {
	stats := CalculateContextStats(
		"You are a helpful assistant",
		"User name: Alice",
		"User: Hello\nAssistant: Hi",
		"How are you?",
		"gemma3:12b",
	)

	if stats.TotalTokens == 0 {
		t.Error("TotalTokens should not be 0")
	}

	if stats.ModelLimit != 8192 {
		t.Errorf("ModelLimit = %d, want 8192", stats.ModelLimit)
	}

	if stats.UsagePercent < 0 || stats.UsagePercent > 100 {
		t.Errorf("UsagePercent = %.2f, should be between 0 and 100", stats.UsagePercent)
	}

	expectedTotal := stats.SystemPromptTokens + stats.MemoryTokens +
		stats.ConversationTokens + stats.UserInputTokens
	if stats.TotalTokens != expectedTotal {
		t.Errorf("TotalTokens = %d, want %d", stats.TotalTokens, expectedTotal)
	}
}

func TestFormatContextStats(t *testing.T) {
	stats := ContextStats{
		SystemPromptTokens: 100,
		MemoryTokens:       50,
		ConversationTokens: 200,
		UserInputTokens:    20,
		TotalTokens:        370,
		ModelLimit:         8192,
		UsagePercent:       4.5,
	}

	output := FormatContextStats(stats)

	if output == "" {
		t.Error("FormatContextStats should not return empty string")
	}

	// 检查是否包含关键信息
	if !contains(output, "System Prompt") {
		t.Error("Output should contain 'System Prompt'")
	}
	if !contains(output, "Total") {
		t.Error("Output should contain 'Total'")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
