package api

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"agent-langchain/internal/memory"
)

// PromptService 提示词服务
type PromptService struct {
	store *memory.Store
}

// NewPromptService 创建提示词服务
func NewPromptService(store *memory.Store) *PromptService {
	return &PromptService{
		store: store,
	}
}

// CreatePromptRequest 创建提示词请求
type CreatePromptRequest struct {
	Name        string `json:"name" validate:"required"`
	Content     string `json:"content" validate:"required"`
	Description string `json:"description"`
	Category    string `json:"category"`
	IsDefault   bool   `json:"is_default"`
}

// UpdatePromptRequest 更新提示词请求
type UpdatePromptRequest struct {
	Name        string `json:"name"`
	Content     string `json:"content"`
	Description string `json:"description"`
	Category    string `json:"category"`
	IsDefault   *bool  `json:"is_default"`
}

// HandleCreatePrompt 创建提示词
func (s *PromptService) HandleCreatePrompt(c echo.Context) error {
	var req CreatePromptRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	prompt := &memory.Prompt{
		Name:        req.Name,
		Content:     req.Content,
		Description: req.Description,
		Category:    req.Category,
		IsDefault:   req.IsDefault,
	}

	if prompt.Category == "" {
		prompt.Category = "assistant"
	}

	if err := s.store.CreatePrompt(c.Request().Context(), prompt); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to create prompt: " + err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, prompt)
}

// HandleGetPrompt 获取单个提示词
func (s *PromptService) HandleGetPrompt(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid prompt ID",
		})
	}

	prompt, err := s.store.GetPrompt(c.Request().Context(), uint(id))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Prompt not found",
		})
	}

	return c.JSON(http.StatusOK, prompt)
}

// HandleListPrompts 列出提示词
func (s *PromptService) HandleListPrompts(c echo.Context) error {
	category := c.QueryParam("category")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	if limit <= 0 {
		limit = 20
	}

	prompts, err := s.store.ListPrompts(c.Request().Context(), category, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to list prompts: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"prompts": prompts,
		"total":   len(prompts),
	})
}

// HandleUpdatePrompt 更新提示词
func (s *PromptService) HandleUpdatePrompt(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid prompt ID",
		})
	}

	var req UpdatePromptRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.IsDefault != nil {
		updates["is_default"] = *req.IsDefault
	}

	if err := s.store.UpdatePrompt(c.Request().Context(), uint(id), updates); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to update prompt: " + err.Error(),
		})
	}

	// 返回更新后的提示词
	prompt, _ := s.store.GetPrompt(c.Request().Context(), uint(id))
	return c.JSON(http.StatusOK, prompt)
}

// HandleDeletePrompt 删除提示词
func (s *PromptService) HandleDeletePrompt(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid prompt ID",
		})
	}

	if err := s.store.DeletePrompt(c.Request().Context(), uint(id)); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to delete prompt: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Prompt deleted successfully",
	})
}

// HandleGetDefaultPrompt 获取默认提示词
func (s *PromptService) HandleGetDefaultPrompt(c echo.Context) error {
	prompt, err := s.store.GetDefaultPrompt(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Default prompt not found",
		})
	}

	return c.JSON(http.StatusOK, prompt)
}
