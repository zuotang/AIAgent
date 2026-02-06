package engine

import (
	"context"
	"strings"
	"testing"

	"agent-langchain/internal/workflow/dsl"
	"agent-langchain/internal/workflow/nodes"
	"agent-langchain/internal/workflow/registry"
)

// MockLLMClient 模拟 LLM 客户端
type MockLLMClient struct {
	Response string
	Error    error
}

func (m *MockLLMClient) Chat(ctx context.Context, msgs []any, model ...string) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	return m.Response, nil
}

// TestExecutor_ValidWorkflow 测试合法工作流能跑通
func TestExecutor_ValidWorkflow(t *testing.T) {
	// 创建注册中心并注册节点
	reg := registry.NewRegistry()
	if err := nodes.RegisterBuiltinNodes(reg); err != nil {
		t.Fatalf("Failed to register nodes: %v", err)
	}

	// 创建工作流
	wf := &dsl.Workflow{
		Version: "1.0",
		Meta: dsl.WorkflowMeta{
			ID:   "test-wf-1",
			Name: "Test Workflow",
		},
		Nodes: map[string]dsl.Node{
			"node-1": {
				ID:      "node-1",
				Type:    "Tool.Time.Now",
				Version: "1.0",
				Params:  map[string]any{},
			},
		},
		Edges: []dsl.Edge{},
	}

	// 创建执行器
	executor := NewExecutor(reg)

	// 创建运行时上下文
	rc := &registry.RunContext{
		LLMClient: &MockLLMClient{Response: "Hello"},
	}

	// 执行工作流
	trace, err := executor.Execute(context.Background(), wf, rc)
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// 验证结果
	if trace.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", trace.Status)
	}

	if len(trace.Nodes) != 1 {
		t.Errorf("Expected 1 node in trace, got %d", len(trace.Nodes))
	}
}

// TestExecutor_PortTypeMismatch 测试端口类型不匹配的 edge 校验失败
func TestExecutor_PortTypeMismatch(t *testing.T) {
	// 创建注册中心并注册节点
	reg := registry.NewRegistry()
	if err := nodes.RegisterBuiltinNodes(reg); err != nil {
		t.Fatalf("Failed to register nodes: %v", err)
	}

	// 创建工作流（故意连接类型不匹配的端口）
	wf := &dsl.Workflow{
		Version: "1.0",
		Meta: dsl.WorkflowMeta{
			ID:   "test-wf-2",
			Name: "Invalid Workflow",
		},
		Nodes: map[string]dsl.Node{
			"node-1": {
				ID:      "node-1",
				Type:    "Tool.Time.Now",
				Version: "1.0",
				Params:  map[string]any{},
			},
			"node-2": {
				ID:      "node-2",
				Type:    "LLM.JSON",
				Version: "1.0",
				Params:  map[string]any{},
			},
		},
		Edges: []dsl.Edge{
			{
				ID:   "e1",
				// text -> messages (类型不匹配)
				From: dsl.PortRef{Node: "node-1", Port: "text"},
				To:   dsl.PortRef{Node: "node-2", Port: "messages"},
				Type: dsl.EdgeTypeData,
			},
		},
	}

	// 创建执行器
	executor := NewExecutor(reg)

	// 创建运行时上下文
	rc := &registry.RunContext{
		LLMClient: &MockLLMClient{Response: "{}"},
	}

	// 执行工作流（应该失败）
	_, err := executor.Execute(context.Background(), wf, rc)
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	// 验证错误信息包含类型不匹配
	if !strings.Contains(err.Error(), "type mismatch") {
		t.Errorf("Expected 'type mismatch' error, got: %v", err)
	}
}

// TestExecutor_NodeError 测试节点运行报错会在 trace 中体现并停止
func TestExecutor_NodeError(t *testing.T) {
	// 创建注册中心并注册节点
	reg := registry.NewRegistry()
	if err := nodes.RegisterBuiltinNodes(reg); err != nil {
		t.Fatalf("Failed to register nodes: %v", err)
	}

	// 创建工作流
	wf := &dsl.Workflow{
		Version: "1.0",
		Meta: dsl.WorkflowMeta{
			ID:   "test-wf-3",
			Name: "Error Workflow",
		},
		Nodes: map[string]dsl.Node{
			"node-1": {
				ID:      "node-1",
				Type:    "LLM.Generate",
				Version: "1.0",
				Params:  map[string]any{},
			},
		},
		Edges: []dsl.Edge{},
	}

	// 创建执行器
	executor := NewExecutor(reg)

	// 创建运行时上下文（LLM 返回错误）
	rc := &registry.RunContext{
		LLMClient: &MockLLMClient{
			Error: context.DeadlineExceeded,
		},
	}

	// 执行工作流（应该失败）
	trace, err := executor.Execute(context.Background(), wf, rc)
	if err == nil {
		t.Fatal("Expected execution error, got nil")
	}

	// 验证 trace 中记录了错误
	if trace.Status != "error" {
		t.Errorf("Expected trace status 'error', got '%s'", trace.Status)
	}

	nodeTrace := trace.Nodes["node-1"]
	if nodeTrace.Status != "error" {
		t.Errorf("Expected node status 'error', got '%s'", nodeTrace.Status)
	}

	if nodeTrace.Error == "" {
		t.Error("Expected node error message, got empty string")
	}
}
