package dsl

import (
	"fmt"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// Validate 校验工作流
func (wf *Workflow) Validate(reg *registry.Registry) error {
	// 1. 校验节点类型存在性
	for nodeID, node := range wf.Nodes {
		spec, err := reg.Get(node.Type, node.Version)
		if err != nil {
			return fmt.Errorf("node %s: %w", nodeID, err)
		}

		// 2. 校验必需输入端口
		if err := validateRequiredInputs(nodeID, node, spec, wf.Edges); err != nil {
			return err
		}
	}

	// 3. 校验边的端口类型匹配
	if err := validateEdgeTypes(wf, reg); err != nil {
		return err
	}

	// 4. 校验拓扑结构（检测循环）
	if err := validateTopology(wf); err != nil {
		return err
	}

	return nil
}

// validateRequiredInputs 校验必需输入端口
func validateRequiredInputs(nodeID string, node Node, spec *registry.NodeSpec, edges []Edge) error {
	// 收集该节点的输入连接
	inputPorts := make(map[string]bool)
	for _, edge := range edges {
		if edge.To.Node == nodeID {
			inputPorts[edge.To.Port] = true
		}
	}

	// 检查必需端口
	for _, input := range spec.Inputs {
		if input.Required && !inputPorts[input.Name] {
			return fmt.Errorf("node %s: required input port '%s' is not connected", nodeID, input.Name)
		}
	}

	return nil
}

// validateEdgeTypes 校验边的端口类型匹配
func validateEdgeTypes(wf *Workflow, reg *registry.Registry) error {
	for _, edge := range wf.Edges {
		// 获取源节点和目标节点的规范
		fromNode, ok := wf.Nodes[edge.From.Node]
		if !ok {
			return fmt.Errorf("edge %s: source node '%s' not found", edge.ID, edge.From.Node)
		}
		toNode, ok := wf.Nodes[edge.To.Node]
		if !ok {
			return fmt.Errorf("edge %s: target node '%s' not found", edge.ID, edge.To.Node)
		}

		fromSpec, err := reg.Get(fromNode.Type, fromNode.Version)
		if err != nil {
			return fmt.Errorf("edge %s: %w", edge.ID, err)
		}
		toSpec, err := reg.Get(toNode.Type, toNode.Version)
		if err != nil {
			return fmt.Errorf("edge %s: %w", edge.ID, err)
		}

		// 查找源端口和目标端口
		var fromPort *types.PortSpec
		for i := range fromSpec.Outputs {
			if fromSpec.Outputs[i].Name == edge.From.Port {
				fromPort = &fromSpec.Outputs[i]
				break
			}
		}
		if fromPort == nil {
			return fmt.Errorf("edge %s: output port '%s' not found in node '%s' (type: %s)",
				edge.ID, edge.From.Port, edge.From.Node, fromNode.Type)
		}

		var toPort *types.PortSpec
		for i := range toSpec.Inputs {
			if toSpec.Inputs[i].Name == edge.To.Port {
				toPort = &toSpec.Inputs[i]
				break
			}
		}
		if toPort == nil {
			return fmt.Errorf("edge %s: input port '%s' not found in node '%s' (type: %s)",
				edge.ID, edge.To.Port, edge.To.Node, toNode.Type)
		}

		// 校验类型匹配
		if fromPort.Type != toPort.Type {
			return fmt.Errorf("edge %s: port type mismatch: %s.%s (%s) -> %s.%s (%s)",
				edge.ID, edge.From.Node, edge.From.Port, fromPort.Type,
				edge.To.Node, edge.To.Port, toPort.Type)
		}
	}

	return nil
}

// validateTopology 校验拓扑结构（检测循环）
func validateTopology(wf *Workflow) error {
	// 构建邻接表
	graph := make(map[string][]string)
	for nodeID := range wf.Nodes {
		graph[nodeID] = []string{}
	}
	for _, edge := range wf.Edges {
		if edge.Type != EdgeTypeFlow {
			graph[edge.From.Node] = append(graph[edge.From.Node], edge.To.Node)
		}
	}

	// DFS 检测循环
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycle func(string) bool
	hasCycle = func(nodeID string) bool {
		visited[nodeID] = true
		recStack[nodeID] = true

		for _, neighbor := range graph[nodeID] {
			if !visited[neighbor] {
				if hasCycle(neighbor) {
					return true
				}
			} else if recStack[neighbor] {
				return true
			}
		}

		recStack[nodeID] = false
		return false
	}

	for nodeID := range wf.Nodes {
		if !visited[nodeID] {
			if hasCycle(nodeID) {
				return fmt.Errorf("workflow contains cycle")
			}
		}
	}

	return nil
}
