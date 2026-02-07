package logic

import (
	"context"
	"fmt"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

const maxSwitchCases = 10

// SwitchNode Logic.Switch 节点 — 多条件分支
//
// 将输入值与 cases 数组逐项匹配，命中则将值输出到对应的 case_N 端口，
// 未命中任何项则输出到 default 端口。下游只有被命中的分支会执行。
//
// 可选 data 输入：用于透传与判断无关的数据。命中时同时输出到 case_N_data / default_data 端口。
// 典型场景：LLM_A 产出内容 → data，LLM_B 判断分类 → value，Switch 根据分类路由原始内容。
//
// 参数 cases: ["值A","值B","值C"]  →  匹配后输出到 case_0 / case_1 / case_2
// 参数 mode: "exact"(默认) | "contains" | "prefix" | "suffix"
type SwitchNode struct{}

func (n *SwitchNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	value, _ := inputs["value"].(string)
	data, hasData := inputs["data"]

	// 解析 cases 参数
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

	// 匹配
	outputs := make(map[string]any)
	matched := false
	for i, c := range cases {
		if matchValue(value, c, mode) {
			portName := fmt.Sprintf("case_%d", i)
			outputs[portName] = value
			if hasData {
				outputs[fmt.Sprintf("case_%d_data", i)] = data
			}
			matched = true
			break // 只命中第一个匹配项
		}
	}

	if !matched {
		outputs["default"] = value
		if hasData {
			outputs["default_data"] = data
		}
	}

	return outputs, nil
}

func (n *SwitchNode) Spec() *registry.NodeSpec {
	outputs := make([]types.PortSpec, 0, (maxSwitchCases+1)*2)
	for i := 0; i < maxSwitchCases; i++ {
		outputs = append(outputs, types.PortSpec{
			Name:     fmt.Sprintf("case_%d", i),
			Type:     types.PortTypeText,
			Required: false,
		})
		outputs = append(outputs, types.PortSpec{
			Name:     fmt.Sprintf("case_%d_data", i),
			Type:     types.PortTypeText,
			Required: false,
		})
	}
	outputs = append(outputs, types.PortSpec{
		Name:     "default",
		Type:     types.PortTypeText,
		Required: false,
	})
	outputs = append(outputs, types.PortSpec{
		Name:     "default_data",
		Type:     types.PortTypeText,
		Required: false,
	})

	return &registry.NodeSpec{
		Type:    "Logic.Switch",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "value", Type: types.PortTypeText, Required: true},
			{Name: "data", Type: types.PortTypeText, Required: false},
		},
		Outputs: outputs,
		Runner:  n,
	}
}

// IfNode Logic.If 节点 — 单条件判断
//
// 判断输入值是否满足条件，满足则输出到 true 端口，否则输出到 false 端口。
//
// 可选 data 输入：用于透传与判断无关的数据。命中时同时输出到 true_data / false_data 端口。
// 典型场景：LLM_A 产出内容 → data，LLM_B 判断 yes/no → value，If 根据判断路由原始内容。
//
// 参数 operator: "eq" | "neq" | "contains" | "not_contains" |
//
//	"gt" | "lt" | "gte" | "lte" | "empty" | "not_empty"
//
// 参数 compare: 比较目标值
type IfNode struct{}

func (n *IfNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	value, _ := inputs["value"].(string)
	data, hasData := inputs["data"]

	operator := "eq"
	if op, ok := params["operator"].(string); ok && op != "" {
		operator = op
	}

	compare, _ := params["compare"].(string)

	result := evaluate(value, operator, compare)

	outputs := make(map[string]any)
	if result {
		outputs["true"] = value
		if hasData {
			outputs["true_data"] = data
		}
	} else {
		outputs["false"] = value
		if hasData {
			outputs["false_data"] = data
		}
	}

	return outputs, nil
}

func (n *IfNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Logic.If",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "value", Type: types.PortTypeText, Required: true},
			{Name: "data", Type: types.PortTypeText, Required: false},
		},
		Outputs: []types.PortSpec{
			{Name: "true", Type: types.PortTypeText, Required: false},
			{Name: "false", Type: types.PortTypeText, Required: false},
			{Name: "true_data", Type: types.PortTypeText, Required: false},
			{Name: "false_data", Type: types.PortTypeText, Required: false},
		},
		Runner: n,
	}
}
