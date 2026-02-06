package engine

import (
	"context"
	"fmt"

	"agent-langchain/internal/workflow/dsl"
	"agent-langchain/internal/workflow/registry"
)

// Executor 工作流执行器
type Executor struct {
	registry *registry.Registry
	failFast bool // 是否在节点失败时立即停止
}

// NewExecutor 创建新的执行器
func NewExecutor(reg *registry.Registry) *Executor {
	return &Executor{
		registry: reg,
		failFast: true, // 默认 fail-fast
	}
}

// SetFailFast 设置 fail-fast 模式
func (e *Executor) SetFailFast(failFast bool) {
	e.failFast = failFast
}

// Execute 执行工作流
func (e *Executor) Execute(ctx context.Context, wf *dsl.Workflow, rc *registry.RunContext) (*RunTrace, error) {
	// 1. 校验工作流
	if err := wf.Validate(e.registry); err != nil {
		return nil, fmt.Errorf("workflow validation failed: %w", err)
	}

	// 2. 创建执行追踪
	trace := NewRunTrace(wf.Meta.ID, wf.Meta.Name)

	// 3. 初始化节点追踪
	for nodeID, node := range wf.Nodes {
		trace.AddNode(nodeID, node.Type)
	}

	// 4. 拓扑排序
	order, err := e.topologicalSort(wf)
	if err != nil {
		trace.Fail(err)
		return trace, err
	}

	// 5. 按顺序执行节点
	nodeOutputs := make(map[string]map[string]any) // nodeID -> outputs

	for _, nodeID := range order {
		// 检查 context 是否取消
		select {
		case <-ctx.Done():
			trace.Cancel()
			return trace, ctx.Err()
		default:
		}

		node := wf.Nodes[nodeID]
		nodeTrace := trace.Nodes[nodeID]

		// 收集输入
		inputs, err := e.collectInputs(nodeID, wf.Edges, nodeOutputs)
		if err != nil {
			nodeTrace.FailNode(err)
			if e.failFast {
				trace.Fail(err)
				return trace, err
			}
			continue
		}

		nodeTrace.Inputs = inputs

		// 获取节点规范
		spec, err := e.registry.Get(node.Type, node.Version)
		if err != nil {
			nodeTrace.FailNode(err)
			if e.failFast {
				trace.Fail(err)
				return trace, err
			}
			continue
		}

		// 执行节点
		nodeTrace.StartNode()
		outputs, err := spec.Runner.Run(ctx, rc, inputs, node.Params)
		if err != nil {
			nodeTrace.FailNode(err)
			if e.failFast {
				trace.Fail(err)
				return trace, err
			}
			continue
		}

		// 记录输出
		nodeTrace.CompleteNode(outputs)
		nodeOutputs[nodeID] = outputs
	}

	// 6. 完成执行
	trace.Complete()
	return trace, nil
}

// topologicalSort 拓扑排序
func (e *Executor) topologicalSort(wf *dsl.Workflow) ([]string, error) {
	// 构建邻接表和入度表
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	// 初始化
	for nodeID := range wf.Nodes {
		graph[nodeID] = []string{}
		inDegree[nodeID] = 0
	}

	// 构建图
	for _, edge := range wf.Edges {
		graph[edge.From.Node] = append(graph[edge.From.Node], edge.To.Node)
		inDegree[edge.To.Node]++
	}

	// Kahn 算法
	var queue []string
	for nodeID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, nodeID)
		}
	}

	var order []string
	for len(queue) > 0 {
		// 取出队首
		nodeID := queue[0]
		queue = queue[1:]
		order = append(order, nodeID)

		// 减少邻居的入度
		for _, neighbor := range graph[nodeID] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// 检查是否所有节点都被访问（检测循环）
	if len(order) != len(wf.Nodes) {
		return nil, fmt.Errorf("workflow contains cycle")
	}

	return order, nil
}

// collectInputs 收集节点的输入数据
func (e *Executor) collectInputs(nodeID string, edges []dsl.Edge, nodeOutputs map[string]map[string]any) (map[string]any, error) {
	inputs := make(map[string]any)

	for _, edge := range edges {
		if edge.To.Node == nodeID {
			// 从源节点的输出中获取数据
			sourceOutputs, ok := nodeOutputs[edge.From.Node]
			if !ok {
				return nil, fmt.Errorf("source node %s has no outputs", edge.From.Node)
			}

			data, ok := sourceOutputs[edge.From.Port]
			if !ok {
				return nil, fmt.Errorf("source node %s has no output port %s", edge.From.Node, edge.From.Port)
			}

			inputs[edge.To.Port] = data
		}
	}

	return inputs, nil
}

