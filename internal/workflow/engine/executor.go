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

const maxExecutionSteps = 10000

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

// Execute 执行工作流（无事件回调）
func (e *Executor) Execute(ctx context.Context, wf *dsl.Workflow, rc *registry.RunContext) (*RunTrace, error) {
	return e.executeInternal(ctx, wf, rc, nil)
}

// ExecuteWithEvents 执行工作流（带事件回调，用于 SSE 实时推送）
func (e *Executor) ExecuteWithEvents(ctx context.Context, wf *dsl.Workflow, rc *registry.RunContext, onEvent func(NodeEvent)) (*RunTrace, error) {
	return e.executeInternal(ctx, wf, rc, onEvent)
}

// executeInternal 内部执行逻辑
func (e *Executor) executeInternal(ctx context.Context, wf *dsl.Workflow, rc *registry.RunContext, onEvent func(NodeEvent)) (*RunTrace, error) {
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

	// 4. 构建数据边和控制流边索引
	dataIn := make(map[string][]dsl.Edge)
	dataOut := make(map[string][]dsl.Edge)
	flowIn := make(map[string][]dsl.Edge)
	flowOut := make(map[string][]dsl.Edge)

	for nodeID := range wf.Nodes {
		dataIn[nodeID] = []dsl.Edge{}
		dataOut[nodeID] = []dsl.Edge{}
		flowIn[nodeID] = []dsl.Edge{}
		flowOut[nodeID] = []dsl.Edge{}
	}
	for _, edge := range wf.Edges {
		if edge.Type == dsl.EdgeTypeFlow {
			flowOut[edge.From.Node] = append(flowOut[edge.From.Node], edge)
			flowIn[edge.To.Node] = append(flowIn[edge.To.Node], edge)
			continue
		}
		dataOut[edge.From.Node] = append(dataOut[edge.From.Node], edge)
		dataIn[edge.To.Node] = append(dataIn[edge.To.Node], edge)
	}

	dataReadyCount := make(map[string]int)
	dataNeededCount := make(map[string]int)
	dataInputs := make(map[string]map[string]any)
	flowTokens := make(map[string]map[string]int)
	executedNoFlow := make(map[string]bool)

	for nodeID := range wf.Nodes {
		dataReadyCount[nodeID] = 0
		dataNeededCount[nodeID] = len(dataIn[nodeID])
	}

	queue := make([]string, 0)
	inQueue := make(map[string]bool)

	hasFlowInputs := func(nodeID string) bool {
		return len(flowIn[nodeID]) > 0
	}

	flowReady := func(nodeID string) bool {
		if !hasFlowInputs(nodeID) {
			return true
		}
		if flowTokens[nodeID] == nil {
			return false
		}
		for _, edge := range flowIn[nodeID] {
			if flowTokens[nodeID][edge.To.Port] > 0 {
				return true
			}
		}
		return false
	}

	consumeFlow := func(nodeID string) {
		if !hasFlowInputs(nodeID) || flowTokens[nodeID] == nil {
			return
		}
		for _, edge := range flowIn[nodeID] {
			if flowTokens[nodeID][edge.To.Port] > 0 {
				flowTokens[nodeID][edge.To.Port]--
				return
			}
		}
	}

	enqueue := func(nodeID string) {
		if inQueue[nodeID] {
			return
		}
		if dataReadyCount[nodeID] != dataNeededCount[nodeID] {
			return
		}
		if !flowReady(nodeID) {
			return
		}
		if !hasFlowInputs(nodeID) && executedNoFlow[nodeID] {
			return
		}
		queue = append(queue, nodeID)
		inQueue[nodeID] = true
	}

	// 5. 初始化队列：无 flow 输入且无 data 依赖的节点
	for nodeID := range wf.Nodes {
		if !hasFlowInputs(nodeID) && dataNeededCount[nodeID] == 0 {
			enqueue(nodeID)
		}
	}

	steps := 0
	for len(queue) > 0 {
		if steps >= maxExecutionSteps {
			err := fmt.Errorf("workflow exceeded max execution steps (%d)", maxExecutionSteps)
			trace.Fail(err)
			emit(onEvent, NodeEvent{Type: WorkflowEventError, Status: "error", Error: err.Error(), Trace: trace})
			return trace, err
		}
		steps++

		nodeID := queue[0]
		queue = queue[1:]
		inQueue[nodeID] = false

		// 检查 context 是否取消
		select {
		case <-ctx.Done():
			trace.Cancel()
			return trace, ctx.Err()
		default:
		}

		node := wf.Nodes[nodeID]
		nodeTrace := trace.Nodes[nodeID]

		if hasFlowInputs(nodeID) {
			consumeFlow(nodeID)
		} else {
			executedNoFlow[nodeID] = true
		}

		inputs := make(map[string]any)
		if dataInputs[nodeID] != nil {
			for k, v := range dataInputs[nodeID] {
				inputs[k] = v
			}
		}
		nodeTrace.Inputs = inputs

		// 获取节点规范
		spec, err := e.registry.Get(node.Type, node.Version)
		if err != nil {
			nodeTrace.FailNode(err)
			emit(onEvent, NodeEvent{Type: NodeEventError, NodeID: nodeID, NodeType: node.Type, Status: "error", Error: err.Error()})
			if e.failFast {
				trace.Fail(err)
				emit(onEvent, NodeEvent{Type: WorkflowEventError, Status: "error", Error: err.Error(), Trace: trace})
				return trace, err
			}
			continue
		}

		// 发送节点开始事件
		emit(onEvent, NodeEvent{Type: NodeEventStart, NodeID: nodeID, NodeType: node.Type, Status: "running"})

		// 执行节点
		nodeTrace.StartNode()
		outputs, err := spec.Runner.Run(ctx, rc, inputs, node.Params)
		if err != nil {
			nodeTrace.FailNode(err)
			emit(onEvent, NodeEvent{Type: NodeEventError, NodeID: nodeID, NodeType: node.Type, Status: "error", Error: err.Error(), Duration: nodeTrace.Duration.Seconds()})
			if e.failFast {
				trace.Fail(err)
				emit(onEvent, NodeEvent{Type: WorkflowEventError, Status: "error", Error: err.Error(), Trace: trace})
				return trace, err
			}
			continue
		}

		// 记录输出
		nodeTrace.CompleteNode(outputs)
		emit(onEvent, NodeEvent{Type: NodeEventComplete, NodeID: nodeID, NodeType: node.Type, Status: "success", Duration: nodeTrace.Duration.Seconds(), Outputs: outputs})

		// 传播数据边
		for _, edge := range dataOut[nodeID] {
			if val, ok := outputs[edge.From.Port]; ok {
				if dataInputs[edge.To.Node] == nil {
					dataInputs[edge.To.Node] = make(map[string]any)
				}
				if _, exists := dataInputs[edge.To.Node][edge.To.Port]; !exists {
					dataReadyCount[edge.To.Node]++
				}
				dataInputs[edge.To.Node][edge.To.Port] = val
				enqueue(edge.To.Node)
			}
		}

		// 传播控制流边
		for _, edge := range flowOut[nodeID] {
			if _, ok := outputs[edge.From.Port]; ok {
				if flowTokens[edge.To.Node] == nil {
					flowTokens[edge.To.Node] = make(map[string]int)
				}
				flowTokens[edge.To.Node][edge.To.Port]++
				enqueue(edge.To.Node)
			}
		}
	}

	// 6. 完成执行
	trace.Complete()
	emit(onEvent, NodeEvent{Type: WorkflowEventComplete, Status: "success", Trace: trace})
	return trace, nil
}

// emit 发送节点事件（如果设置了回调）
func emit(onEvent func(NodeEvent), event NodeEvent) {
	if onEvent != nil {
		onEvent(event)
	}
}

