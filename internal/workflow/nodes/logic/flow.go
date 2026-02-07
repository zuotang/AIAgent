package logic

import (
	"context"
	"fmt"
	"strconv"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

const defaultLoopKey = "flow_loop_default"

// FlowIfNode Flow.If 节点 — 控制流条件判断
type FlowIfNode struct{}

func (n *FlowIfNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	value, _ := inputs["value"].(string)

	operator := "eq"
	if op, ok := params["operator"].(string); ok && op != "" {
		operator = op
	}
	compare, _ := params["compare"].(string)

	result := evaluate(value, operator, compare)

	if result {
		return map[string]any{"true": true}, nil
	}
	return map[string]any{"false": true}, nil
}

func (n *FlowIfNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Flow.If",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "value", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "true", Type: types.PortTypeFlow, Required: false},
			{Name: "false", Type: types.PortTypeFlow, Required: false},
		},
		Runner: n,
	}
}

// FlowSwitchNode Flow.Switch 节点 — 控制流多分支
type FlowSwitchNode struct{}

func (n *FlowSwitchNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	value, _ := inputs["value"].(string)

	cases, err := parseCases(params["cases"])
	if err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("cases parameter is required and must be a non-empty array")
	}
	if len(cases) > maxSwitchCases {
		return nil, fmt.Errorf("cases exceeds maximum of %d", maxSwitchCases)
	}

	mode := "exact"
	if m, ok := params["mode"].(string); ok && m != "" {
		mode = m
	}

	for i, c := range cases {
		if matchValue(value, c, mode) {
			return map[string]any{fmt.Sprintf("case_%d", i): true}, nil
		}
	}

	return map[string]any{"default": true}, nil
}

func (n *FlowSwitchNode) Spec() *registry.NodeSpec {
	outputs := make([]types.PortSpec, 0, maxSwitchCases+1)
	for i := 0; i < maxSwitchCases; i++ {
		outputs = append(outputs, types.PortSpec{
			Name:     fmt.Sprintf("case_%d", i),
			Type:     types.PortTypeFlow,
			Required: false,
		})
	}
	outputs = append(outputs, types.PortSpec{
		Name:     "default",
		Type:     types.PortTypeFlow,
		Required: false,
	})

	return &registry.NodeSpec{
		Type:    "Flow.Switch",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "value", Type: types.PortTypeText, Required: true},
		},
		Outputs: outputs,
		Runner:  n,
	}
}

// FlowLoopNode Flow.Loop 节点 — 控制流循环次数
// params:
//   - max: 最大循环次数（默认 1）
//   - key: 计数器 key（用于区分多个循环节点）
// outputs:
//   - continue: flow
//   - done: flow
//   - count: text (当前计数)
type FlowLoopNode struct{}

func (n *FlowLoopNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	max := 1
	if v, ok := params["max"]; ok {
		switch t := v.(type) {
		case float64:
			max = int(t)
		case string:
			if parsed, err := strconv.Atoi(t); err == nil {
				max = parsed
			}
		}
	}
	if max < 1 {
		max = 1
	}

	key, _ := params["key"].(string)
	if key == "" {
		key = defaultLoopKey
	}
	cacheKey := "flow.loop." + key

	if rc.Cache == nil {
		rc.Cache = make(map[string]any)
	}
	current := 0
	if v, ok := rc.Cache[cacheKey]; ok {
		if iv, ok := v.(int); ok {
			current = iv
		} else if fv, ok := v.(float64); ok {
			current = int(fv)
		}
	}

	current++
	rc.Cache[cacheKey] = current

	if current < max {
		return map[string]any{
			"continue": true,
			"count":    strconv.Itoa(current),
		}, nil
	}

	return map[string]any{
		"done":  true,
		"count": strconv.Itoa(current),
	}, nil
}

func (n *FlowLoopNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Flow.Loop",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "continue", Type: types.PortTypeFlow, Required: false},
			{Name: "done", Type: types.PortTypeFlow, Required: false},
			{Name: "count", Type: types.PortTypeText, Required: false},
		},
		Runner: n,
	}
}

// FlowStartNode Flow.Start 节点 — 流程起点
// 无输入，输出 flow 信号
type FlowStartNode struct{}

func (n *FlowStartNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	return map[string]any{"out": true}, nil
}

func (n *FlowStartNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Flow.Start",
		Version: "1.0",
		Inputs:  []types.PortSpec{},
		Outputs: []types.PortSpec{
			{Name: "out", Type: types.PortTypeFlow, Required: true},
		},
		Runner: n,
	}
}
