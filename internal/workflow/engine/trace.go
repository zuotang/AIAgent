package engine

import (
	"time"

	"agent-langchain/internal/workflow/types"
)

// NodeTrace 节点执行追踪
type NodeTrace struct {
	NodeID    string              `json:"node_id"`    // 节点 ID
	NodeType  string              `json:"node_type"`  // 节点类型
	Status    types.NodeStatus    `json:"status"`     // 执行状态
	StartTime *time.Time          `json:"start_time"` // 开始时间
	EndTime   *time.Time          `json:"end_time"`   // 结束时间
	Duration  time.Duration       `json:"duration"`   // 执行时长
	Inputs    map[string]any      `json:"inputs"`     // 输入数据（摘要）
	Outputs   map[string]any      `json:"outputs"`    // 输出数据（摘要）
	Error     string              `json:"error"`      // 错误信息
}

// RunTrace 工作流执行追踪
type RunTrace struct {
	WorkflowID   string                 `json:"workflow_id"`   // 工作流 ID
	WorkflowName string                 `json:"workflow_name"` // 工作流名称
	StartTime    time.Time              `json:"start_time"`    // 开始时间
	EndTime      *time.Time             `json:"end_time"`      // 结束时间
	Duration     time.Duration          `json:"duration"`      // 总执行时长
	Status       string                 `json:"status"`        // 整体状态（success/error/cancelled）
	Nodes        map[string]*NodeTrace  `json:"nodes"`         // 节点追踪映射
	Error        string                 `json:"error"`         // 整体错误信息
}

// NewRunTrace 创建新的运行追踪
func NewRunTrace(workflowID, workflowName string) *RunTrace {
	return &RunTrace{
		WorkflowID:   workflowID,
		WorkflowName: workflowName,
		StartTime:    time.Now(),
		Nodes:        make(map[string]*NodeTrace),
		Status:       "running",
	}
}

// AddNode 添加节点追踪
func (rt *RunTrace) AddNode(nodeID, nodeType string) *NodeTrace {
	nt := &NodeTrace{
		NodeID:   nodeID,
		NodeType: nodeType,
		Status:   types.NodeStatusPending,
		Inputs:   make(map[string]any),
		Outputs:  make(map[string]any),
	}
	rt.Nodes[nodeID] = nt
	return nt
}

// StartNode 标记节点开始执行
func (nt *NodeTrace) StartNode() {
	now := time.Now()
	nt.StartTime = &now
	nt.Status = types.NodeStatusRunning
}

// CompleteNode 标记节点执行完成
func (nt *NodeTrace) CompleteNode(outputs map[string]any) {
	now := time.Now()
	nt.EndTime = &now
	nt.Status = types.NodeStatusSuccess
	nt.Outputs = outputs
	if nt.StartTime != nil {
		nt.Duration = now.Sub(*nt.StartTime)
	}
}

// FailNode 标记节点执行失败
func (nt *NodeTrace) FailNode(err error) {
	now := time.Now()
	nt.EndTime = &now
	nt.Status = types.NodeStatusError
	nt.Error = err.Error()
	if nt.StartTime != nil {
		nt.Duration = now.Sub(*nt.StartTime)
	}
}

// SkipNode 标记节点跳过
func (nt *NodeTrace) SkipNode() {
	nt.Status = types.NodeStatusSkipped
}

// Complete 标记整个工作流完成
func (rt *RunTrace) Complete() {
	now := time.Now()
	rt.EndTime = &now
	rt.Duration = now.Sub(rt.StartTime)
	rt.Status = "success"
}

// Fail 标记整个工作流失败
func (rt *RunTrace) Fail(err error) {
	now := time.Now()
	rt.EndTime = &now
	rt.Duration = now.Sub(rt.StartTime)
	rt.Status = "error"
	rt.Error = err.Error()
}

// Cancel 标记整个工作流取消
func (rt *RunTrace) Cancel() {
	now := time.Now()
	rt.EndTime = &now
	rt.Duration = now.Sub(rt.StartTime)
	rt.Status = "cancelled"
}

// NodeEventType 节点事件类型
type NodeEventType string

const (
	NodeEventStart    NodeEventType = "node_start"    // 节点开始执行
	NodeEventComplete NodeEventType = "node_complete" // 节点执行完成
	NodeEventError    NodeEventType = "node_error"    // 节点执行失败
	NodeEventSkip     NodeEventType = "node_skip"     // 节点跳过

	WorkflowEventComplete NodeEventType = "workflow_complete" // 工作流完成
	WorkflowEventError    NodeEventType = "workflow_error"    // 工作流失败
)

// NodeEvent 节点执行事件（用于实时推送）
type NodeEvent struct {
	Type     NodeEventType  `json:"type"`               // 事件类型
	NodeID   string         `json:"node_id,omitempty"`   // 节点 ID
	NodeType string         `json:"node_type,omitempty"` // 节点类型（如 "LLM.Chat"）
	Status   string         `json:"status"`              // 状态
	Error    string         `json:"error,omitempty"`     // 错误信息
	Duration float64        `json:"duration,omitempty"`  // 耗时（秒）
	Outputs  map[string]any `json:"outputs,omitempty"`   // 输出数据（仅 complete 时）
	Trace    *RunTrace      `json:"trace,omitempty"`     // 完整 trace（仅 workflow 结束时）
}
