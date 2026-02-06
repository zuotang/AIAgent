package tool

import (
	"context"
	"fmt"
	"time"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// TimeNowNode Tool.Time.Now 节点
// 无输入
// 输出: text (当前时间)
type TimeNowNode struct{}

// Run 执行节点
func (n *TimeNowNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	// 获取当前时间
	now := time.Now()

	// 格式化时间
	format := "2006-01-02 15:04:05"
	if f, ok := params["format"].(string); ok && f != "" {
		format = f
	}

	timeStr := now.Format(format)

	// 返回输出
	return map[string]any{
		"text": timeStr,
		"json": map[string]any{
			"timestamp": now.Unix(),
			"formatted": timeStr,
		},
	}, nil
}

// Spec 返回节点规范
func (n *TimeNowNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Tool.Time.Now",
		Version: "1.0",
		Inputs:  []types.PortSpec{}, // 无输入
		Outputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
			{Name: "json", Type: types.PortTypeJSON, Required: true},
		},
		Runner: n,
	}
}

// CalcNode Tool.Calc 节点
// 输入: text (expression)
// 输出: text/json (result)
type CalcNode struct{}

// Run 执行节点
func (n *CalcNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	// 获取表达式
	expr, ok := inputs["text"].(string)
	if !ok || expr == "" {
		return nil, fmt.Errorf("text input (expression) is required")
	}

	// TODO: 实现计算器逻辑
	// 这里应该调用现有的 calculator tool
	// 目前先返回占位符

	result := fmt.Sprintf("Result of %s: TODO", expr)

	// 返回输出
	return map[string]any{
		"text": result,
		"json": map[string]any{
			"expression": expr,
			"result":     result,
		},
	}, nil
}

// Spec 返回节点规范
func (n *CalcNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Tool.Calc",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
			{Name: "json", Type: types.PortTypeJSON, Required: true},
		},
		Runner: n,
	}
}
