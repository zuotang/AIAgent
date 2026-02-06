package dsl

import (
	"encoding/json"
	"fmt"
	"os"
)

// Workflow 工作流定义
type Workflow struct {
	Version string                `json:"version"` // 工作流版本
	Meta    WorkflowMeta          `json:"meta"`    // 元数据
	Nodes   map[string]Node       `json:"nodes"`   // 节点映射
	Edges   []Edge                `json:"edges"`   // 边列表
	UI      map[string]any        `json:"ui"`      // UI 配置（执行器忽略）
}

// WorkflowMeta 工作流元数据
type WorkflowMeta struct {
	ID         string `json:"id"`          // 工作流 ID
	Name       string `json:"name"`        // 工作流名称
	OutputNode string `json:"output_node"` // 输出节点 ID（可选）
}

// Node 节点定义
type Node struct {
	ID      string         `json:"id"`      // 节点 ID
	Type    string         `json:"type"`    // 节点类型（如 "LLM.Generate"）
	Version string         `json:"version"` // 节点版本
	Params  map[string]any `json:"params"`  // 节点参数
}

// Edge 边定义
type Edge struct {
	ID   string   `json:"id"`   // 边 ID
	From PortRef  `json:"from"` // 源端口
	To   PortRef  `json:"to"`   // 目标端口
	Type EdgeType `json:"type"` // 边类型
}

// PortRef 端口引用
type PortRef struct {
	Node string `json:"node"` // 节点 ID
	Port string `json:"port"` // 端口名称
}

// EdgeType 边类型
type EdgeType string

const (
	EdgeTypeData EdgeType = "data" // 数据边
	EdgeTypeFlow EdgeType = "flow" // 控制流边
)

// Load 从文件加载工作流
func Load(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}

	return Unmarshal(data)
}

// Unmarshal 从 JSON 字节解析工作流
func Unmarshal(data []byte) (*Workflow, error) {
	var wf Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow: %w", err)
	}

	// 基本校验
	if wf.Version == "" {
		return nil, fmt.Errorf("workflow version is required")
	}
	if wf.Meta.ID == "" {
		return nil, fmt.Errorf("workflow meta.id is required")
	}
	if len(wf.Nodes) == 0 {
		return nil, fmt.Errorf("workflow must have at least one node")
	}

	return &wf, nil
}

// Marshal 将工作流序列化为 JSON
func (wf *Workflow) Marshal() ([]byte, error) {
	return json.MarshalIndent(wf, "", "  ")
}

// Save 保存工作流到文件
func (wf *Workflow) Save(path string) error {
	data, err := wf.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
