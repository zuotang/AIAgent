package api

import (
	"context"
	"fmt"
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
	AgentID uint   `json:"agent_id"` // 可选的 Agent ID，默认使用 ID=1 的 Agent
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

// 以下是之前硬编码的提示词，现已移至数据库
// 可以通过 Prompt 表和 Agent 表进行管理
/*
const systemPrompt = `你是一个陪聊女孩

原则：
- 先接情绪再回应；口语化不生硬；正向但不鸡汤。
- 你会参考"结构化长期记忆(SQLite)"和"语义长期记忆(Qdrant)"来保持一致性与个性化，但不要把记忆内容原样泄露给用户（除非用户要求你总结）。
- 【重要】长期记忆里：agent 表示你（assistant），user 表示用户本人，严禁混用。

`
*/

/*
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
3. 只在确实需要时使用工具，不要滥用
*/

// HandleChat 处理聊天请求
func (s *ChatService) HandleChat(c echo.Context) error {
	var req ChatRequest
	if err := c.Bind(&req); err != nil {
		// 添加详细的错误信息
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Invalid request",
			"message": err.Error(),
			"details": "Failed to parse JSON request body",
		})
	}

	// 打印接收到的请求信息用于调试
	println("=== Chat Request Debug ===")
	println("Message:", req.Message)
	println("UserID:", req.UserID)
	println("AgentID:", req.AgentID)
	println("========================")

	// 验证必填字段
	if req.Message == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Validation failed",
			"message": "message field is required",
		})
	}

	// 如果用户ID为空，使用默认值
	userID := req.UserID
	if userID == "" {
		userID = "api_user"
	}

	// 如果未指定 AgentID，使用默认 Agent (ID=1)
	agentID := req.AgentID
	if agentID == 0 {
		agentID = 1
		println("No agent_id provided, using default agent ID: 1")
	}

	// 从数据库获取 Agent 和提示词
	println("Loading agent with ID:", agentID)
	agent, err := s.orch.GetStore().GetAgent(c.Request().Context(), agentID)
	if err != nil {
		println("Failed to get agent:", err.Error())
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Agent not found",
			"message": fmt.Sprintf("Failed to load agent with ID: %d", agentID),
			"details": err.Error(),
		})
	}

	// 获取提示词
	var prompt string
	if agent.Prompt != nil {
		println("Using agent prompt, length:", len(agent.Prompt.Content))
		prompt = agent.Prompt.Content
	} else {
		println("Agent has no prompt")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Agent configuration error",
			"message": "Agent does not have a prompt configured",
		})
	}

	// 创建短期记忆窗口并加载历史消息
	windowMem := memory.NewWindowMemory(10) // 最多保留10轮对话
	println("加载历史聊天记录作为上下文")

	// 检查是否有压缩上下文
	compressedCtx, err := s.orch.GetStore().GetCompressedContext(c.Request().Context(), userID, agentID)
	var historyMessages []memory.ChatMessage

	if err == nil && compressedCtx.LastMessageID > 0 {
		// 有压缩上下文，只加载新消息
		println("发现压缩上下文，LastMessageID:", compressedCtx.LastMessageID)
		historyMessages, err = s.orch.GetChatHistoryAfterID(c.Request().Context(), userID, agentID, compressedCtx.LastMessageID, 20)
		if err != nil {
			println("Failed to load chat history after ID:", err.Error())
		}
	} else {
		// 没有压缩上下文，加载所有历史消息
		println("没有压缩上下文，加载所有历史消息")
		historyMessages, err = s.orch.GetChatHistory(c.Request().Context(), userID, agentID, 20, 0)
		if err != nil {
			println("Failed to load chat history:", err.Error())
		}
	}

	// 将历史消息按时间正序排列（数据库返回的是倒序）
	if len(historyMessages) > 0 {
		for i := len(historyMessages) - 1; i >= 0; i-- {
			msg := historyMessages[i]
			// 找到成对的 user 和 assistant 消息
			if i > 0 && msg.Role == "user" && historyMessages[i-1].Role == "assistant" {
				// 过滤掉assistant消息中的thinking标签
				cleanedAssistant := orchestrator.RemoveThinkingTags(historyMessages[i-1].Content)
				windowMem.Add(msg.Content, cleanedAssistant)
				i-- // 跳过已处理的 assistant 消息
			}
		}
		println("已加载", windowMem.Size(), "轮历史对话")
	}

	// 仅使用历史对话作为上下文，当前用户消息单独走 Message 字段
	conversationContext := windowMem.String()

	// 处理消息
	output, err := s.orch.ProcessMessage(context.Background(), userID, agentID, req.Message, conversationContext, prompt)
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
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Invalid request",
			"message": err.Error(),
			"details": "Failed to parse JSON request body",
		})
	}

	// 打印接收到的请求信息用于调试
	println("=== Chat Stream Request Debug ===")
	println("Message:", req.Message)
	println("UserID:", req.UserID)
	println("AgentID:", req.AgentID)
	println("================================")

	// 验证请求
	if req.Message == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Validation failed",
			"message": "message field is required",
		})
	}

	// 设置用户ID
	userID := req.UserID
	if userID == "" {
		userID = "api_user"
	}

	// 如果未指定 AgentID，使用默认 Agent (ID=1)
	agentID := req.AgentID
	if agentID == 0 {
		agentID = 1
		println("No agent_id provided, using default agent ID: 1")
	}

	// 从数据库获取 Agent 和提示词
	println("Loading agent with ID:", agentID)
	agent, err := s.orch.GetStore().GetAgent(c.Request().Context(), agentID)
	if err != nil {
		println("Failed to get agent:", err.Error())
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Agent not found",
			"message": fmt.Sprintf("Failed to load agent with ID: %d", agentID),
			"details": err.Error(),
		})
	}

	// 获取提示词
	var prompt string
	if agent.Prompt != nil {
		println("Using agent prompt, length:", len(agent.Prompt.Content))
		prompt = agent.Prompt.Content
	} else {
		println("Agent has no prompt")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Agent configuration error",
			"message": "Agent does not have a prompt configured",
		})
	}

	println("设置响应头")
	// 设置响应头
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	// 加载历史消息作为上下文
	println("加载历史聊天记录作为上下文")
	windowMem := memory.NewWindowMemory(10)

	// 检查是否有压缩上下文
	compressedCtx, err := s.orch.GetStore().GetCompressedContext(c.Request().Context(), userID, agentID)
	var historyMessages []memory.ChatMessage

	if err == nil && compressedCtx.LastMessageID > 0 {
		// 有压缩上下文，只加载新消息
		println("发现压缩上下文，LastMessageID:", compressedCtx.LastMessageID)
		historyMessages, err = s.orch.GetChatHistoryAfterID(c.Request().Context(), userID, agentID, compressedCtx.LastMessageID, 20)
		if err != nil {
			println("Failed to load chat history after ID:", err.Error())
		}
	} else {
		// 没有压缩上下文，加载所有历史消息
		println("没有压缩上下文，加载所有历史消息")
		historyMessages, err = s.orch.GetChatHistory(c.Request().Context(), userID, agentID, 20, 0)
		if err != nil {
			println("Failed to load chat history:", err.Error())
		}
	}

	// 将历史消息按时间正序排列并填充到窗口记忆
	if len(historyMessages) > 0 {
		for i := len(historyMessages) - 1; i >= 0; i-- {
			msg := historyMessages[i]
			if i > 0 && msg.Role == "user" && historyMessages[i-1].Role == "assistant" {
				// 过滤掉assistant消息中的thinking标签
				cleanedAssistant := orchestrator.RemoveThinkingTags(historyMessages[i-1].Content)
				windowMem.Add(msg.Content, cleanedAssistant)
				i--
			}
		}
		println("已加载", windowMem.Size(), "轮历史对话")
	}

	// 仅使用历史对话作为上下文，当前用户消息单独走 Message 字段
	conversationContext := windowMem.String()
	println("上下文:", conversationContext)
	println("流式回调")
	// 流式回调
	streamCallback := func(chunk string) error {
		if _, err := c.Response().Write([]byte("data: " + chunk + "\n\n")); err != nil {
			return err
		}
		// 强制刷新缓冲区，确保立即发送
		if flusher, ok := c.Response().Writer.(http.Flusher); ok {
			flusher.Flush()
		}
		return nil
	}
	println("处理消息")
	// 处理消息（流式）
	println(c.Request().Context())
	_, err = s.orch.ProcessMessageStream(c.Request().Context(), userID, agentID, req.Message, conversationContext, prompt, streamCallback)
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
	if flusher, ok := c.Response().Writer.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}

// GetChatHistoryRequest 获取聊天记录请求
type GetChatHistoryRequest struct {
	UserID string `json:"user_id" validate:"required"`
	Limit  int    `json:"limit" validate:"omitempty,min=1,max=100"`
	Offset int    `json:"offset" validate:"omitempty,min=0"`
}

// PaginationMeta 分页元数据
type PaginationMeta struct {
	Total      int64 `json:"total"`       // 总记录数
	Limit      int   `json:"limit"`       // 每页数量
	HasMore    bool  `json:"has_more"`    // 是否有更多数据
	NextCursor *uint `json:"next_cursor"` // 下一页的游标（最后一条消息的ID），null表示没有更多数据
}

// ChatHistoryResponse 聊天记录响应（带分页）
type ChatHistoryResponse struct {
	Messages   []memory.ChatMessage `json:"messages"`
	Pagination PaginationMeta       `json:"pagination"`
}

// GetChatHistory 处理获取聊天记录请求（支持游标分页）
func (s *ChatService) GetChatHistory(c echo.Context) error {
	// 从 URL 查询字符串中获取参数
	userID := c.QueryParam("user_id")
	if userID == "" {
		userID = "api_user"
	}

	// 设置默认值
	agentID := uint(1)
	limit := 20         // 默认每页20条
	beforeID := uint(0) // 0表示从最新消息开始

	// 尝试解析 agent_id 参数
	if agentIDParam := c.QueryParam("agent_id"); agentIDParam != "" {
		if aid, err := strconv.ParseUint(agentIDParam, 10, 32); err == nil && aid > 0 {
			agentID = uint(aid)
		}
	}

	// 尝试解析 limit 参数
	if limitParam := c.QueryParam("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// 尝试解析 before_id 参数（游标分页）
	if beforeIDParam := c.QueryParam("before_id"); beforeIDParam != "" {
		if bid, err := strconv.ParseUint(beforeIDParam, 10, 32); err == nil {
			beforeID = uint(bid)
		}
	}

	ctx := c.Request().Context()

	// 获取总数
	total, err := s.orch.GetChatHistoryCount(ctx, userID, agentID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get chat history count: " + err.Error()})
	}

	// 获取聊天记录（使用游标分页）
	messages, err := s.orch.GetChatHistoryWithCursor(ctx, userID, agentID, beforeID, limit+1) // 多取一条用于判断是否有更多
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get chat history: " + err.Error()})
	}

	// 判断是否有更多数据
	hasMore := len(messages) > limit
	var nextCursor *uint
	if hasMore {
		// 移除多取的那一条
		messages = messages[:limit]
		// 设置下一页的游标为最后一条消息的ID
		lastID := messages[len(messages)-1].ID
		nextCursor = &lastID
	}

	// 构造响应
	response := ChatHistoryResponse{
		Messages: messages,
		Pagination: PaginationMeta{
			Total:      total,
			Limit:      limit,
			HasMore:    hasMore,
			NextCursor: nextCursor,
		},
	}

	return c.JSON(http.StatusOK, response)
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
	sessions, err := s.orch.GetChatSessions(c.Request().Context(), userID, 1)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get chat sessions: " + err.Error()})
	}

	return c.JSON(http.StatusOK, sessions)
}

// ClearDataRequest 清空数据请求
type ClearDataRequest struct {
	UserID  string `json:"user_id" validate:"required"`
	AgentID uint   `json:"agent_id" validate:"required"`
}

// ClearDataResponse 清空数据响应
type ClearDataResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// HandleClearData 处理清空历史记录和记忆请求
func (s *ChatService) HandleClearData(c echo.Context) error {
	var req ClearDataRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Invalid request",
			"message": err.Error(),
			"details": "Failed to parse JSON request body",
		})
	}

	// 验证必填字段
	if req.UserID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Validation failed",
			"message": "user_id field is required",
		})
	}

	if req.AgentID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Validation failed",
			"message": "agent_id field is required and must be greater than 0",
		})
	}

	ctx := c.Request().Context()

	// 清空 SQLite 中的数据（聊天记录、记忆、压缩上下文）
	if err := s.orch.GetStore().ClearAllData(ctx, req.UserID, req.AgentID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to clear data",
			"message": err.Error(),
		})
	}

	// 清空 Qdrant 中的记忆向量数据
	if err := s.orch.GetVectorStore().DeleteMemoryByFilter(ctx, req.UserID, req.AgentID); err != nil {
		// 向量删除失败不应该阻止整个操作，记录错误但继续
		println("Warning: Failed to delete memory vectors from Qdrant:", err.Error())
	}

	return c.JSON(http.StatusOK, ClearDataResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully cleared all data for user_id=%s and agent_id=%d", req.UserID, req.AgentID),
	})
}
