package io

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// InputTextNode Input.Text 节点
// 无输入，输出用户提供的文本
type InputTextNode struct{}

func (n *InputTextNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	// 从参数获取文本
	text, _ := params["text"].(string)

	return map[string]any{
		"text": text,
	}, nil
}

func (n *InputTextNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Input.Text",
		Version: "1.0",
		Inputs:  []types.PortSpec{},
		Outputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Runner: n,
	}
}

// OutputTextNode Output.Text 节点
// 输入文本，作为工作流输出
type OutputTextNode struct{}

func (n *OutputTextNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	text, _ := inputs["text"].(string)

	// 输出到 cache 中，供外部获取
	if rc.Cache != nil {
		rc.Cache["output"] = text
	}

	return map[string]any{
		"text": text,
	}, nil
}

func (n *OutputTextNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Output.Text",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Runner: n,
	}
}

// InputJSONNode Input.JSON 节点
type InputJSONNode struct{}

func (n *InputJSONNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	// 从参数获取 JSON
	jsonData, _ := params["json"]

	return map[string]any{
		"json": jsonData,
	}, nil
}

func (n *InputJSONNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Input.JSON",
		Version: "1.0",
		Inputs:  []types.PortSpec{},
		Outputs: []types.PortSpec{
			{Name: "json", Type: types.PortTypeJSON, Required: true},
		},
		Runner: n,
	}
}

// OutputJSONNode Output.JSON 节点
type OutputJSONNode struct{}

func (n *OutputJSONNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	jsonData, _ := inputs["json"]

	// 输出到 cache 中
	if rc.Cache != nil {
		rc.Cache["output"] = jsonData
	}

	return map[string]any{
		"json": jsonData,
	}, nil
}

func (n *OutputJSONNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Output.JSON",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "json", Type: types.PortTypeJSON, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "json", Type: types.PortTypeJSON, Required: true},
		},
		Runner: n,
	}
}

// SaveFileNode Output.SaveFile 节点
// 将文本保存到文件
type SaveFileNode struct{}

func (n *SaveFileNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	text, ok := inputs["text"].(string)
	if !ok {
		return nil, fmt.Errorf("text input is required")
	}

	filePath, ok := params["file_path"].(string)
	if !ok || filePath == "" {
		return nil, fmt.Errorf("file_path parameter is required")
	}

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, []byte(text), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return map[string]any{
		"text":      text,
		"file_path": filePath,
	}, nil
}

func (n *SaveFileNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Output.SaveFile",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: true},
			{Name: "file_path", Type: types.PortTypeText, Required: true},
		},
		Runner: n,
	}
}

