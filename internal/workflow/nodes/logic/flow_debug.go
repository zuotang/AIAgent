package logic

import (
	"context"
	"fmt"
	"time"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// FlowDebugNode Flow.Debug 节点 — 调试 flow 信号
// 输入: in (flow)
// 输出: text (调试信息)
// 参数: label (可选)
type FlowDebugNode struct{}

func (n *FlowDebugNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	label, _ := params["label"].(string)
	if label == "" {
		label = "flow"
	}

	msg := fmt.Sprintf("[%s] %s triggered", time.Now().Format("15:04:05"), label)

	if rc.Cache != nil {
		rc.Cache["flow_debug"] = msg
	}

	return map[string]any{
		"text": msg,
	}, nil
}

func (n *FlowDebugNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Flow.Debug",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Runner: n,
	}
}
