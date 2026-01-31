package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"agent-langchain/internal/memory"
)

// AgentService Agent 服务
type AgentService struct {
	store *memory.Store
}

// NewAgentService 创建 Agent 服务
func NewAgentService(store *memory.Store) *AgentService {
	return &AgentService{
		store: store,
	}
}

// CreateAgentRequest 创建 Agent 请求
type CreateAgentRequest struct {
	Name        string `json:"name" validate:"required"`
	Gender      string `json:"gender"`      // 性别
	Age         int    `json:"age"`         // 年龄
	Description string `json:"description"`
	PromptID    uint   `json:"prompt_id" validate:"required"` // 必须是数字类型
	Avatar      string `json:"avatar"`
	Config      string `json:"config"` // JSON 字符串
	IsActive    bool   `json:"is_active"`
}

// UpdateAgentRequest 更新 Agent 请求
type UpdateAgentRequest struct {
	Name        string `json:"name"`
	Gender      string `json:"gender"`      // 性别
	Age         *int   `json:"age"`         // 年龄（指针类型，允许更新为0）
	Description string `json:"description"`
	PromptID    *uint  `json:"prompt_id"` // 可选，必须是数字类型
	Avatar      string `json:"avatar"`
	Config      string `json:"config"`
	IsActive    *bool  `json:"is_active"`
}

// HandleCreateAgent 创建 Agent
func (s *AgentService) HandleCreateAgent(c echo.Context) error {
	var req CreateAgentRequest
	if err := c.Bind(&req); err != nil {
		println("=== Create Agent Error ===")
		println("Bind error:", err.Error())
		println("========================")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Invalid request body",
			"message": err.Error(),
			"details": "Failed to parse JSON request body",
		})
	}

	// 打印接收到的请求信息用于调试
	println("=== Create Agent Request Debug ===")
	println("Name:", req.Name)
	println("Gender:", req.Gender)
	println("Age:", req.Age)
	println("Description:", req.Description)
	println("PromptID:", req.PromptID)
	println("Avatar:", req.Avatar)
	println("Config:", req.Config)
	println("IsActive:", req.IsActive)
	println("==================================")

	// 验证必填字段
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Validation failed",
			"message": "name field is required",
		})
	}

	if req.PromptID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Validation failed",
			"message": "prompt_id field is required and must be greater than 0",
		})
	}

	agent := &memory.Agent{
		Name:        req.Name,
		Gender:      req.Gender,
		Age:         req.Age,
		Description: req.Description,
		PromptID:    req.PromptID,
		Avatar:      req.Avatar,
		Config:      req.Config,
		IsActive:    req.IsActive,
	}

	println("Creating agent in database...")
	if err := s.store.CreateAgent(c.Request().Context(), agent); err != nil {
		println("Failed to create agent:", err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to create agent",
			"message": err.Error(),
		})
	}

	println("Agent created successfully with ID:", agent.ID)
	// 重新获取以包含关联的 Prompt
	agent, _ = s.store.GetAgent(c.Request().Context(), agent.ID)

	return c.JSON(http.StatusCreated, agent)
}

// HandleGetAgent 获取单个 Agent
func (s *AgentService) HandleGetAgent(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid agent ID",
		})
	}

	agent, err := s.store.GetAgent(c.Request().Context(), uint(id))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Agent not found",
		})
	}

	return c.JSON(http.StatusOK, agent)
}

// HandleListAgents 列出 Agent
func (s *AgentService) HandleListAgents(c echo.Context) error {
	var isActive *bool
	if activeStr := c.QueryParam("is_active"); activeStr != "" {
		active := activeStr == "true"
		isActive = &active
	}

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	if limit <= 0 {
		limit = 20
	}

	agents, err := s.store.ListAgents(c.Request().Context(), isActive, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to list agents: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"agents": agents,
		"total":  len(agents),
	})
}

// HandleUpdateAgent 更新 Agent
func (s *AgentService) HandleUpdateAgent(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Invalid agent ID",
			"message": "Agent ID must be a valid number",
		})
	}

	var req UpdateAgentRequest
	if err := c.Bind(&req); err != nil {
		println("=== Update Agent Error ===")
		println("Bind error:", err.Error())
		println("=========================")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Invalid request body",
			"message": err.Error(),
			"details": "Failed to parse JSON request body",
		})
	}

	// 打印接收到的请求信息用于调试
	println("=== Update Agent Request Debug ===")
	println("Agent ID:", id)
	println("Name:", req.Name)
	println("Gender:", req.Gender)
	if req.Age != nil {
		println("Age:", *req.Age)
	}
	println("Description:", req.Description)
	if req.PromptID != nil {
		println("PromptID:", *req.PromptID)
	}
	println("Avatar:", req.Avatar)
	println("Config:", req.Config)
	if req.IsActive != nil {
		println("IsActive:", *req.IsActive)
	}
	println("==================================")

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Gender != "" {
		updates["gender"] = req.Gender
	}
	if req.Age != nil {
		updates["age"] = *req.Age
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.PromptID != nil {
		// 验证 PromptID 是否存在
		_, err = s.store.GetPrompt(c.Request().Context(), *req.PromptID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error":   "Invalid prompt_id",
				"message": fmt.Sprintf("prompt not found with id: %d", *req.PromptID),
				"details": err.Error(),
			})
		}
		updates["prompt_id"] = *req.PromptID
	}
	if req.Avatar != "" {
		updates["avatar"] = req.Avatar
	}
	if req.Config != "" {
		updates["config"] = req.Config
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if err := s.store.UpdateAgent(c.Request().Context(), uint(id), updates); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to update agent: " + err.Error(),
		})
	}

	// 返回更新后的 Agent
	agent, _ := s.store.GetAgent(c.Request().Context(), uint(id))
	return c.JSON(http.StatusOK, agent)
}

// HandleDeleteAgent 删除 Agent
func (s *AgentService) HandleDeleteAgent(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid agent ID",
		})
	}

	if err := s.store.DeleteAgent(c.Request().Context(), uint(id)); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to delete agent: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Agent deleted successfully",
	})
}

// HandleGetActiveAgents 获取所有激活的 Agent
func (s *AgentService) HandleGetActiveAgents(c echo.Context) error {
	agents, err := s.store.GetActiveAgents(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to get active agents: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"agents": agents,
		"total":  len(agents),
	})
}
