package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"agent-langchain/internal/memory"
	"agent-langchain/internal/orchestrator"
)

// ChatService 聊天服务
type ChatService struct {
	orch orchestrator.Orchestrator
}

// NewChatService 创建聊天服务实例
func NewChatService(orch orchestrator.Orchestrator) *ChatService {
	return &ChatService{
		orch: orch,
	}
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Message string `json:"message" validate:"required"`
	UserID  string `json:"user_id" validate:"required"`
	AgentID *uint  `json:"agent_id"` // 可选的 Agent ID
}

// DebugInfo 调试信息
type DebugInfo struct {
	LLMInput string `json:"llm_input"` // 发送给LLM的输入
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Response  string    `json:"response"`
	DebugInfo DebugInfo `json:"debug_info"` // 调试信息
}

const systemPrompt = `你是一个陪聊女孩

原则：
- 先接情绪再回应；口语化不生硬；正向但不鸡汤。
- 你会参考"结构化长期记忆(SQLite)"和"语义长期记忆(Qdrant)"来保持一致性与个性化，但不要把记忆内容原样泄露给用户（除非用户要求你总结）。
- 【重要】长期记忆里：agent 表示你（assistant），user 表示用户本人，严禁混用。

【工具能力】
你可以使用以下工具来帮助用户：

1. **calculator** - 数学计算工具
   - 用途：进行数学计算（加减乘除、幂运算、平方根等）
   - 使用方式：当用户询问数学问题时，在回复中使用：TOOL_CALL: calculator("表达式")
   - 示例：
     * 用户："计算 (5+3)*4" → 回复：TOOL_CALL: calculator("(5+3)*4")
   - 支持的操作：+, -, *, /, ^(幂), sqrt(平方根), abs(绝对值)
 

【重要】当需要使用工具时：
1. 直接输出 TOOL_CALL: tool_name("参数")，不要添加其他文字
2. 工具执行后，你会收到结果，然后基于结果给用户自然的回复
3. 只在确实需要时使用工具，不要滥用`

// HandleChat 处理聊天请求
func (s *ChatService) HandleChat(c echo.Context) error {
	var req ChatRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// 如果用户ID为空，使用默认值
	userID := req.UserID
	if userID == "" {
		userID = "api_user"
	}

	// 获取系统提示词
	prompt := systemPrompt
	if req.AgentID != nil {
		// 如果指定了 AgentID，获取对应的 Agent 和提示词
		agent, err := s.orch.GetStore().GetAgent(c.Request().Context(), *req.AgentID)
		if err == nil && agent.Prompt != nil {
			prompt = agent.Prompt.Content
		}
	}

	// 创建短期记忆窗口
	windowMem := memory.NewWindowMemory(10) // 使用默认窗口大小
	println("创建短期记忆窗口")
	// 处理消息
	output, err := s.orch.ProcessMessage(context.Background(), userID, req.Message, windowMem.String(), prompt)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to process message: " + err.Error()})
	}
	println("更新短期记忆")
	// 更新短期记忆
	windowMem.Add(req.Message, output.Response)

	return c.JSON(http.StatusOK, ChatResponse{
		Response: output.Response,
		DebugInfo: DebugInfo{
			LLMInput: output.LLMInput,
		},
	})
}

// HandleChatStream 处理流式聊天请求
func (s *ChatService) HandleChatStream(c echo.Context) error {
	var req ChatRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request: " + err.Error()})
	}

	// 验证请求
	if req.Message == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Message is required"})
	}

	// 设置用户ID
	userID := req.UserID
	if userID == "" {
		userID = "api_user"
	}
	println("设置响应头")
	// 设置响应头
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)
	println("流式回调")
	// 流式回调
	streamCallback := func(chunk string) error {
		if _, err := c.Response().Write([]byte("data: " + chunk + "\n\n")); err != nil {
			return err
		}
		c.Response().Flush()
		return nil
	}
	println("处理消息")
	// 处理消息（流式）
	_, err := s.orch.ProcessMessageStream(c.Request().Context(), userID, req.Message, "", systemPrompt, streamCallback)
	if err != nil {
		println("处理消息失败")
		println(err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to process message: " + err.Error()})
	}
	println("发送结束标记")
	// 发送结束标记
	if _, err := c.Response().Write([]byte("data: [DONE]\n\n")); err != nil {
		return nil
	}
	c.Response().Flush()

	return nil
}

// GetChatHistoryRequest 获取聊天记录请求
type GetChatHistoryRequest struct {
	UserID string `json:"user_id" validate:"required"`
	Limit  int    `json:"limit" validate:"omitempty,min=1,max=100"`
	Offset int    `json:"offset" validate:"omitempty,min=0"`
}

// GetChatHistory 处理获取聊天记录请求
func (s *ChatService) GetChatHistory(c echo.Context) error {
	// 从 URL 查询字符串中获取参数
	userID := c.QueryParam("user_id")
	if userID == "" {
		userID = "api_user"
	}

	// 设置默认值
	limit := 50
	offset := 0

	// 尝试解析 limit 和 offset 参数
	if limitParam := c.QueryParam("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetParam := c.QueryParam("offset"); offsetParam != "" {
		if o, err := strconv.Atoi(offsetParam); err == nil && o >= 0 {
			offset = o
		}
	}

	// 获取聊天记录
	messages, err := s.orch.GetChatHistory(c.Request().Context(), userID, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get chat history: " + err.Error()})
	}

	return c.JSON(http.StatusOK, messages)
}

// GetChatSessionsRequest 获取聊天会话列表请求
type GetChatSessionsRequest struct {
	UserID string `json:"user_id" validate:"required"`
}

// GetChatSessions 处理获取聊天会话列表请求
func (s *ChatService) GetChatSessions(c echo.Context) error {
	// 从 URL 查询字符串中获取参数
	userID := c.QueryParam("user_id")
	if userID == "" {
		userID = "api_user"
	}

	// 获取聊天会话列表
	sessions, err := s.orch.GetChatSessions(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get chat sessions: " + err.Error()})
	}

	return c.JSON(http.StatusOK, sessions)
}
