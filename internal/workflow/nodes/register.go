package nodes

import (
	"agent-langchain/internal/workflow/nodes/context"
	"agent-langchain/internal/workflow/nodes/io"
	"agent-langchain/internal/workflow/nodes/llm"
	"agent-langchain/internal/workflow/nodes/tool"
	"agent-langchain/internal/workflow/nodes/transform"
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

	// Tool 节点
	if err := reg.Register((&tool.TimeNowNode{}).Spec()); err != nil {
		return err
	}
	if err := reg.Register((&tool.CalcNode{}).Spec()); err != nil {
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

	// TODO: 注册其他节点
	// - Memory.Extract
	// - Memory.Longterm.Get/Put
	// - Embedding.Encode
	// - Vector.Query.Build
	// - Memory.Vector.Query/Upsert
	// - KB.Query
	// - Tool.Dispatch
	// - Tool.Execute

	return nil
}
