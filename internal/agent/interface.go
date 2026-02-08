package agent

import (
	"context"

	"agent-langchain/internal/models"
)

// Agent 定义AI Agent的核心接口
type Agent interface {
	// Run 执行Agent任务
	Run(ctx context.Context, input Input) (Output, error)
	// RunStream 流式执行Agent任务
	RunStream(ctx context.Context, input Input, callback func(string) error) (Output, error)
}

// Input Agent的输入
type Input struct {
	UserID               string               // 用户ID
	SessionID            string               // 会话ID
	Message              string               // 用户消息
	SystemPrompt         string               // 系统提示
	Memory               string               // 记忆上下文
	Conversation         string               // 对话历史（用于压缩/记忆提取）
	ConversationMessages []models.ChatMessage // 对话历史（messages格式）
	ConversationSummary  string               // 对话摘要（system消息注入）
	Context              map[string]string    // 额外上下文
}

// Output Agent的输出
type Output struct {
	Response     string              // 响应内容
	ToolCalls    []ToolCall          // 工具调用记录
	ThoughtTrace []string            // 思考过程
	Metadata     map[string]interface{} // 元数据
	LLMInput     string              // 发送给LLM的输入
}

// ToolCall 工具调用记录
type ToolCall struct {
	ToolName  string // 工具名称
	Arguments string // 参数
	Result    string // 结果
	Error     error  // 错误
}
