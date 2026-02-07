package nodes

import (
	"agent-langchain/internal/workflow/nodes/context"
	"agent-langchain/internal/workflow/nodes/embedding"
	"agent-langchain/internal/workflow/nodes/io"
	"agent-langchain/internal/workflow/nodes/kb"
	"agent-langchain/internal/workflow/nodes/llm"
	"agent-langchain/internal/workflow/nodes/logic"
	"agent-langchain/internal/workflow/nodes/memory"
	"agent-langchain/internal/workflow/nodes/preprocess"
	"agent-langchain/internal/workflow/nodes/session"
	"agent-langchain/internal/workflow/nodes/tool"
	"agent-langchain/internal/workflow/nodes/transform"
	"agent-langchain/internal/workflow/nodes/vector"
	"agent-langchain/internal/workflow/nodes/workflow"
	"agent-langchain/internal/workflow/registry"
)

// RegisterBuiltinNodes 注册所有内置节点
func RegisterBuiltinNodes(reg *registry.Registry) error {
	// LLM 节点 - 原有节点
	if err := reg.Register((&llm.GenerateNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&llm.JSONNode{}).Spec()); err != nil {
		return err
	}

	// LLM 节点 - 多提供商支持
	if err := reg.Register((&llm.OllamaNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&llm.DeepSeekNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&llm.AnthropicNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&llm.UnifiedLLMNode{}).Spec()); err != nil {
		return err
	}

	// Context 节点
	if err := reg.Register((&context.PackNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&context.CompressNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&context.AssembleNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&context.WindowCheckNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&context.SummaryNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&context.KeepRecentNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&context.KeepCitationsNode{}).Spec()); err != nil {
		return err
	}

	// Tool 节点
	if err := reg.Register((&tool.TimeNowNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&tool.CalcNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&tool.DecisionNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&tool.ExecuteNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&tool.SufficientNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&tool.ValidateNode{}).Spec()); err != nil {
		return err
	}

	// IO 节点
	if err := reg.Register((&io.InputTextNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&io.OutputTextNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&io.InputJSONNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&io.OutputJSONNode{}).Spec()); err != nil {
		return err
	}

	// Transform 节点
	if err := reg.Register((&transform.TextToMessagesNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&transform.MessagesToTextNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&transform.JSONToTextNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&transform.TextToJSONNode{}).Spec()); err != nil {
		return err
	}

	// Embedding 节点
	if err := reg.Register((&embedding.EncodeNode{}).Spec()); err != nil {
		return err
	}

	// Vector 节点
	if err := reg.Register((&vector.QueryNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&vector.UpsertNode{}).Spec()); err != nil {
		return err
	}

	// Memory 节点
	if err := reg.Register((&memory.QueryNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&memory.ChatHistoryNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&memory.ExtractNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&memory.SaveNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&memory.ReadNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&memory.CandidateNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&memory.GateNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&memory.WriteNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&memory.InjectNode{}).Spec()); err != nil {
		return err
	}

	// KB 节点
	if err := reg.Register((&kb.QueryRewriteNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&kb.SearchNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&kb.RerankDedupNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&kb.EvidencePackNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&kb.InjectEvidenceNode{}).Spec()); err != nil {
		return err
	}

	// Logic 节点
	if err := reg.Register((&logic.SwitchNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&logic.IfNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&logic.LoopNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&logic.FlowIfNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&logic.FlowSwitchNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&logic.FlowLoopNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&logic.FlowStartNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&logic.FlowDebugNode{}).Spec()); err != nil {
		return err
	}

	// Session 节点
	if err := reg.Register((&session.EntryNode{}).Spec()); err != nil {
		return err
	}

	// Preprocess 节点
	if err := reg.Register((&preprocess.BasicNode{}).Spec()); err != nil {
		return err
	}

	// Workflow 节点
	if err := reg.Register((&workflow.CallNode{}).Spec()); err != nil {
		return err
	}

	return nil
}
