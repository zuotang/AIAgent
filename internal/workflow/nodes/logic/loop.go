package logic

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

const maxLoopIterations = 100

// LoopNode Logic.Loop 节点 — 循环执行
//
// 两种模式:
//
// 1. 简单模式（无 LLM）：将输入文本重复收集 N 次，用分隔符拼接输出。
//    适用于：生成重复内容、构造批量数据。
//
// 2. LLM 模式（连接了 RunContext.LLMClient）：循环 N 次调用 LLM。
//    每次迭代将上一轮 LLM 输出注入 prompt 模板中的 {{output}} 占位符，
//    同时 {{index}} 替换为当前迭代索引，{{input}} 替换为原始输入。
//    适用于：迭代优化文本、多轮自我反思、逐步推理。
//
// 参数:
//   - count: 循环次数（默认 1，最大 100）
//   - prompt: LLM 提示词模板（包含 {{input}} {{output}} {{index}} 占位符）
//     留空则为简单模式，不调用 LLM。
//   - model: LLM 模型名称（可选）
//   - separator: all 输出的分隔符（默认 "\n---\n"）
type LoopNode struct{}

func (n *LoopNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	input, _ := inputs["input"].(string)

	count := 1
	if c, ok := params["count"]; ok {
		switch v := c.(type) {
		case float64:
			count = int(v)
		case string:
			if parsed, err := strconv.Atoi(v); err == nil {
				count = parsed
			}
		}
	}
	if count < 1 {
		count = 1
	}
	if count > maxLoopIterations {
		return nil, fmt.Errorf("loop count %d exceeds maximum of %d", count, maxLoopIterations)
	}

	promptTpl, _ := params["prompt"].(string)
	model, _ := params["model"].(string)

	separator := "\n---\n"
	if sep, ok := params["separator"].(string); ok && sep != "" {
		separator = sep
	}

	current := input
	collected := make([]string, 0, count)

	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if promptTpl != "" && rc.LLMClient != nil {
			// LLM 模式：用模板构造 prompt，调用 LLM
			prompt := renderLoopPrompt(promptTpl, input, current, i)
			messages := []any{
				map[string]any{"role": "user", "content": prompt},
			}
			resp, err := rc.LLMClient.Chat(ctx, messages, model)
			if err != nil {
				return nil, fmt.Errorf("loop iteration %d LLM call failed: %w", i, err)
			}
			current = resp
		}
		// 简单模式下 current 保持不变（每轮都是原始 input）

		collected = append(collected, current)
	}

	return map[string]any{
		"output": current,
		"index":  strconv.Itoa(count - 1),
		"all":    strings.Join(collected, separator),
		"count":  strconv.Itoa(count),
	}, nil
}

// renderLoopPrompt 渲染循环 prompt 模板
func renderLoopPrompt(tpl, input, output string, index int) string {
	r := strings.NewReplacer(
		"{{input}}", input,
		"{{output}}", output,
		"{{index}}", strconv.Itoa(index),
	)
	return r.Replace(tpl)
}

func (n *LoopNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Logic.Loop",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "input", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "output", Type: types.PortTypeText, Required: true},
			{Name: "index", Type: types.PortTypeText, Required: false},
			{Name: "all", Type: types.PortTypeText, Required: false},
			{Name: "count", Type: types.PortTypeText, Required: false},
		},
		Runner: n,
	}
}
