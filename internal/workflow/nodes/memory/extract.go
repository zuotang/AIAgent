package memory

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// ExtractNode Memory.Extract 节点 — 从对话文本中提取结构化记忆
type ExtractNode struct{}

// extractorOutput LLM 提取器输出
type extractorOutput struct {
	Memories []registry.ExtractedMemoryItem `json:"memories"`
}

// Run 执行节点
func (n *ExtractNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	if rc.LLMClient == nil {
		return nil, fmt.Errorf("LLMClient not configured")
	}

	text, ok := inputs["text"].(string)
	if !ok || text == "" {
		return nil, fmt.Errorf("text input is required")
	}

	// 可选的历史上下文输入
	history, _ := inputs["history"].(string)

	// 读取参数
	model := ""
	if m, ok := params["model"].(string); ok {
		model = m
	}

	includeHistory := true
	if h, ok := params["include_history"].(bool); ok {
		includeHistory = h
	}

	// 构建提取 prompt
	sys := buildExtractorSystemPrompt()
	prompt := buildExtractorUserPrompt(text, history, includeHistory)

	msgs := []any{
		map[string]any{"role": "system", "content": sys},
		map[string]any{"role": "user", "content": prompt},
	}

	// 调用 LLM
	var raw string
	var err error
	if model != "" {
		raw, err = rc.LLMClient.Chat(ctx, msgs, model)
	} else {
		raw, err = rc.LLMClient.Chat(ctx, msgs)
	}
	if err != nil {
		return nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	// 解析 JSON
	memories, err := parseExtractorOutput(raw)
	if err != nil {
		return nil, fmt.Errorf("parse extraction result failed: %w", err)
	}

	// 清洗和过滤
	memories = sanitizeAndFilter(memories)

	// 转为 JSON 输出
	memoriesJSON, _ := json.Marshal(memories)

	return map[string]any{
		"memories": string(memoriesJSON),
		"count":    len(memories),
	}, nil
}

// Spec 返回节点规范
func (n *ExtractNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Memory.Extract",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
			{Name: "history", Type: types.PortTypeText, Required: false},
		},
		Outputs: []types.PortSpec{
			{Name: "memories", Type: types.PortTypeJSON, Required: true},
			{Name: "count", Type: types.PortTypeJSON, Required: false},
		},
		Runner: n,
	}
}
