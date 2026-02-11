package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-langchain/internal/workflow/dsl"
	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// CallNode Workflow.Call 节点
// 输入: text (可选)
// 输出: text/json/context_pack
// 参数:
//   - workflow_json: 子工作流 JSON (string 或 object)
//   - input_text_node: 子工作流中 Input.Text 节点的 ID（可选）
type CallNode struct{}

func (n *CallNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	raw, ok := params["workflow_json"]
	if !ok {
		return nil, fmt.Errorf("workflow_json param is required")
	}

	wf, err := parseWorkflow(raw)
	if err != nil {
		return nil, err
	}

	if inputNodeID, ok := params["input_text_node"].(string); ok && inputNodeID != "" {
		if text, ok := inputs["text"].(string); ok {
			if node, exists := wf.Nodes[inputNodeID]; exists {
				if node.Params == nil {
					node.Params = make(map[string]any)
				}
				node.Params["text"] = text
				wf.Nodes[inputNodeID] = node
			}
		}
	}

	if rc.WorkflowExecutor == nil {
		return nil, fmt.Errorf("workflow executor not found in run context")
	}

	subRc := &registry.RunContext{
		LLMClient:        rc.LLMClient,
		EmbedClient:      rc.EmbedClient,
		MemoryStore:      rc.MemoryStore,
		QdrantClient:     rc.QdrantClient,
		ToolRegistry:     rc.ToolRegistry,
		WorkflowExecutor: rc.WorkflowExecutor,
		Cache:            make(map[string]any),
	}

	trace, err := rc.WorkflowExecutor.Execute(ctx, wf, subRc)
	if err != nil {
		return nil, err
	}
	_ = trace

	output := subRc.Cache["output"]
	result := map[string]any{}
	switch v := output.(type) {
	case string:
		result["text"] = v
	case map[string]any:
		result["json"] = v
	default:
		if output != nil {
			result["json"] = output
		}
	}

	return result, nil
}

func (n *CallNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Workflow.Call",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: false},
		},
		Outputs: []types.PortSpec{
			{Name: "text", Type: types.PortTypeText, Required: false},
			{Name: "json", Type: types.PortTypeJSON, Required: false},
			{Name: "context_pack", Type: types.PortTypeContextPack, Required: false},
		},
		Runner: n,
	}
}

func parseWorkflow(raw any) (*dsl.Workflow, error) {
	switch v := raw.(type) {
	case string:
		return dsl.Unmarshal([]byte(v))
	case map[string]any:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal workflow_json: %w", err)
		}
		return dsl.Unmarshal(data)
	default:
		return nil, fmt.Errorf("unsupported workflow_json type: %T", raw)
	}
}
