package agent

import (
	"context"
)

// Agent 定义AI Agent的核心接口
type Agent interface {
	// Run 执行Agent任务
	Run(ctx context.Context, input Input) (Output, error)
}

// Input Agent的输入
type Input struct {
	UserID         string            // 用户ID
	SessionID      string            // 会话ID
	Message        string            // 用户消息
	SystemPrompt   string            // 系统提示
	Memory         string            // 记忆上下文
	Conversation   string            // 对话历史
	Context        map[string]string // 额外上下文
}

// Output Agent的输出
type Output struct {
	Response     string              // 响应内容
	ToolCalls    []ToolCall          // 工具调用记录
	ThoughtTrace []string            // 思考过程
	Metadata     map[string]interface{} // 元数据
}

// ToolCall 工具调用记录
type ToolCall struct {
	ToolName  string // 工具名称
	Arguments string // 参数
	Result    string // 结果
	Error     error  // 错误
}
