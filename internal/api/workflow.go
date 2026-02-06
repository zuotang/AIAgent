package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"agent-langchain/internal/models"
	"agent-langchain/internal/workflow/dsl"
	"agent-langchain/internal/workflow/engine"
	"agent-langchain/internal/workflow/nodes"
	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/store"
)

// WorkflowService 工作流服务
type WorkflowService struct {
	registry  *registry.Registry
	executor  *engine.Executor
	llmClient models.LLMClient
	store     *store.WorkflowStore
}

// NewWorkflowService 创建工作流服务
func NewWorkflowService(llmClient models.LLMClient, dbPath string) (*WorkflowService, error) {
	// 创建注册中心
	reg := registry.NewRegistry()

	// 注册内置节点
	if err := nodes.RegisterBuiltinNodes(reg); err != nil {
		return nil, fmt.Errorf("failed to register nodes: %w", err)
	}

	// 创建执行器
	executor := engine.NewExecutor(reg)

	// 创建存储
	workflowStore, err := store.NewWorkflowStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow store: %w", err)
	}

	return &WorkflowService{
		registry:  reg,
		executor:  executor,
		llmClient: llmClient,
		store:     workflowStore,
	}, nil
}

// NodeInfo 节点信息（用于API响应）
type NodeInfo struct {
	Type        string      `json:"type"`
	Version     string      `json:"version"`
	Category    string      `json:"category"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Inputs      []PortInfo  `json:"inputs"`
	Outputs     []PortInfo  `json:"outputs"`
	Params      []ParamInfo `json:"params"`
}

// PortInfo 端口信息
type PortInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// ParamInfo 参数信息
type ParamInfo struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Description string      `json:"description"`
	Options     []string    `json:"options,omitempty"`
	Min         *float64    `json:"min,omitempty"`
	Max         *float64    `json:"max,omitempty"`
}

// HandleGetNodes 获取所有可用节点
func (s *WorkflowService) HandleGetNodes(c echo.Context) error {
	specs := s.registry.List()

	nodes := make([]NodeInfo, 0, len(specs))
	for _, spec := range specs {
		nodeInfo := NodeInfo{
			Type:        spec.Type,
			Version:     spec.Version,
			Category:    getCategory(spec.Type),
			Name:        getName(spec.Type),
			Description: getDescription(spec.Type),
			Inputs:      make([]PortInfo, len(spec.Inputs)),
			Outputs:     make([]PortInfo, len(spec.Outputs)),
			Params:      getParamInfo(spec.Type),
		}

		// 转换输入端口
		for i, input := range spec.Inputs {
			nodeInfo.Inputs[i] = PortInfo{
				Name:        input.Name,
				Type:        string(input.Type),
				Required:    input.Required,
				Description: getPortDescription(spec.Type, input.Name, true),
			}
		}

		// 转换输出端口
		for i, output := range spec.Outputs {
			nodeInfo.Outputs[i] = PortInfo{
				Name:        output.Name,
				Type:        string(output.Type),
				Required:    output.Required,
				Description: getPortDescription(spec.Type, output.Name, false),
			}
		}

		nodes = append(nodes, nodeInfo)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"nodes": nodes,
	})
}

// HandleValidateWorkflow 校验工作流
func (s *WorkflowService) HandleValidateWorkflow(c echo.Context) error {
	var wf dsl.Workflow
	if err := c.Bind(&wf); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"valid":  false,
			"errors": []string{fmt.Sprintf("Invalid workflow JSON: %v", err)},
		})
	}

	// 校验工作流
	if err := wf.Validate(s.registry); err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"valid":  false,
			"errors": []string{err.Error()},
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid":  true,
		"errors": []string{},
	})
}

// ExecuteRequest 执行请求
type ExecuteRequest struct {
	Workflow dsl.Workflow `json:"workflow"`
	Async    bool         `json:"async"`
}

// HandleExecuteWorkflow 执行工作流
func (s *WorkflowService) HandleExecuteWorkflow(c echo.Context) error {
	var req ExecuteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid request: %v", err),
		})
	}

	// 创建运行时上下文
	rc := &registry.RunContext{
		LLMClient: registry.NewLLMClientAdapter(s.llmClient),
	}

	// 执行工作流
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	trace, err := s.executor.Execute(ctx, &req.Workflow, rc)

	// 保存执行记录（无论成功失败）
	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		userID = "default"
	}
	if trace != nil {
		_ = s.store.SaveExecution(req.Workflow.Meta.ID, userID, trace)
	}

	if err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"trace":   trace,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"trace":   trace,
	})
}

// Helper functions for node metadata

func getCategory(nodeType string) string {
	switch {
	case contains(nodeType, "LLM"):
		return "LLM"
	case contains(nodeType, "Context"):
		return "Context"
	case contains(nodeType, "Tool"):
		return "Tool"
	case contains(nodeType, "Input") || contains(nodeType, "Output"):
		return "IO"
	case contains(nodeType, "Transform"):
		return "Transform"
	default:
		return "Other"
	}
}

func getName(nodeType string) string {
	// Extract name from type (e.g., "LLM.Ollama" -> "Ollama")
	for i := len(nodeType) - 1; i >= 0; i-- {
		if nodeType[i] == '.' {
			return nodeType[i+1:]
		}
	}
	return nodeType
}

func getDescription(nodeType string) string {
	descriptions := map[string]string{
		"LLM.Ollama":                "使用本地Ollama运行的开源模型",
		"LLM.DeepSeek":              "使用DeepSeek云端API",
		"LLM.Anthropic":             "使用Anthropic Claude API",
		"LLM.Chat":                  "统一的LLM接口，支持多种提供商",
		"LLM.Generate":              "使用LLM生成文本响应",
		"LLM.JSON":                  "使用LLM生成JSON输出",
		"Context.Pack":              "将多个输入打包成context_pack",
		"Context.Compress":          "使用LLM压缩上下文",
		"Tool.Time.Now":             "获取当前时间",
		"Tool.Calc":                 "执行数学计算",
		"Input.Text":                "文本输入节点",
		"Output.Text":               "文本输出节点",
		"Input.JSON":                "JSON输入节点",
		"Output.JSON":               "JSON输出节点",
		"Transform.TextToMessages":  "将文本转换为消息列表",
		"Transform.MessagesToText":  "将消息列表转换为文本",
		"Transform.JSONToText":      "将JSON转换为文本",
		"Transform.TextToJSON":      "将文本转换为JSON",
	}
	if desc, ok := descriptions[nodeType]; ok {
		return desc
	}
	return ""
}

func getPortDescription(nodeType, portName string, isInput bool) string {
	// Add port descriptions as needed
	return ""
}

func getParamInfo(nodeType string) []ParamInfo {
	params := make([]ParamInfo, 0)

	switch nodeType {
	case "LLM.Ollama":
		params = append(params,
			ParamInfo{
				Name:        "model",
				Type:        "string",
				Required:    false,
				Default:     "qwen2.5:7b",
				Description: "模型名称",
				Options:     []string{"qwen2.5:7b", "llama2", "mistral", "codellama"},
			},
			ParamInfo{
				Name:        "base_url",
				Type:        "string",
				Required:    false,
				Default:     "http://localhost:11434",
				Description: "Ollama API地址",
			},
			ParamInfo{
				Name:        "temperature",
				Type:        "number",
				Required:    false,
				Default:     0.0,
				Description: "温度参数(0-2)",
				Min:         float64Ptr(0),
				Max:         float64Ptr(2),
			},
			ParamInfo{
				Name:        "max_retries",
				Type:        "number",
				Required:    false,
				Default:     1,
				Description: "最大重试次数",
				Min:         float64Ptr(1),
				Max:         float64Ptr(10),
			},
		)
	case "LLM.DeepSeek":
		params = append(params,
			ParamInfo{
				Name:        "model",
				Type:        "string",
				Required:    false,
				Default:     "deepseek-chat",
				Description: "模型名称",
				Options:     []string{"deepseek-chat", "deepseek-coder"},
			},
			ParamInfo{
				Name:        "api_key",
				Type:        "string",
				Required:    true,
				Description: "DeepSeek API密钥",
			},
			ParamInfo{
				Name:        "base_url",
				Type:        "string",
				Required:    false,
				Default:     "https://api.deepseek.com/v1",
				Description: "API地址",
			},
			ParamInfo{
				Name:        "temperature",
				Type:        "number",
				Required:    false,
				Default:     0.0,
				Description: "温度参数(0-2)",
				Min:         float64Ptr(0),
				Max:         float64Ptr(2),
			},
		)
	case "LLM.Anthropic":
		params = append(params,
			ParamInfo{
				Name:        "model",
				Type:        "string",
				Required:    false,
				Default:     "claude-3-sonnet-20240229",
				Description: "模型名称",
				Options:     []string{"claude-3-opus-20240229", "claude-3-sonnet-20240229", "claude-3-haiku-20240307"},
			},
			ParamInfo{
				Name:        "api_key",
				Type:        "string",
				Required:    true,
				Description: "Anthropic API密钥",
			},
			ParamInfo{
				Name:        "base_url",
				Type:        "string",
				Required:    false,
				Default:     "https://api.anthropic.com/v1",
				Description: "API地址",
			},
			ParamInfo{
				Name:        "temperature",
				Type:        "number",
				Required:    false,
				Default:     0.0,
				Description: "温度参数(0-1)",
				Min:         float64Ptr(0),
				Max:         float64Ptr(1),
			},
		)
	case "LLM.Chat":
		params = append(params,
			ParamInfo{
				Name:        "provider",
				Type:        "string",
				Required:    false,
				Default:     "ollama",
				Description: "提供商",
				Options:     []string{"ollama", "deepseek", "anthropic"},
			},
			ParamInfo{
				Name:        "model",
				Type:        "string",
				Required:    false,
				Description: "模型名称",
			},
			ParamInfo{
				Name:        "api_key",
				Type:        "string",
				Required:    false,
				Description: "API密钥(DeepSeek/Anthropic需要)",
			},
			ParamInfo{
				Name:        "temperature",
				Type:        "number",
				Required:    false,
				Default:     0.0,
				Description: "温度参数",
			},
		)
	case "Tool.Time.Now":
		params = append(params,
			ParamInfo{
				Name:        "format",
				Type:        "string",
				Required:    false,
				Default:     "2006-01-02 15:04:05",
				Description: "时间格式",
			},
		)
	case "Input.Text":
		params = append(params,
			ParamInfo{
				Name:        "text",
				Type:        "string",
				Required:    true,
				Description: "输入文本内容",
			},
		)
	case "Input.JSON":
		params = append(params,
			ParamInfo{
				Name:        "json",
				Type:        "object",
				Required:    true,
				Description: "JSON数据",
			},
		)
	case "Transform.TextToMessages":
		params = append(params,
			ParamInfo{
				Name:        "role",
				Type:        "string",
				Required:    false,
				Default:     "user",
				Description: "消息角色",
				Options:     []string{"user", "assistant", "system"},
			},
		)
	}

	return params
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		   len(s) > len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func float64Ptr(f float64) *float64 {
	return &f
}

// HandleSaveWorkflow 保存工作流
func (s *WorkflowService) HandleSaveWorkflow(c echo.Context) error {
	var req struct {
		ID          string       `json:"id"`
		Name        string       `json:"name"`
		Description string       `json:"description"`
		Workflow    dsl.Workflow `json:"workflow"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid request: %v", err),
		})
	}

	// 设置工作流ID
	if req.ID != "" {
		req.Workflow.Meta.ID = req.ID
	}
	if req.Name != "" {
		req.Workflow.Meta.Name = req.Name
	}

	// 获取用户ID（从请求头或上下文）
	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		userID = "default"
	}

	// 保存到数据库
	if err := s.store.SaveWorkflow(userID, &req.Workflow, req.Name, req.Description); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to save workflow: %v", err),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      req.Workflow.Meta.ID,
	})
}

// HandleListWorkflows 列出工作流
func (s *WorkflowService) HandleListWorkflows(c echo.Context) error {
	// 获取分页参数
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// 获取用户ID
	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		userID = "default"
	}

	// 查询数据库
	records, total, err := s.store.ListWorkflows(userID, page, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": fmt.Sprintf("Failed to list workflows: %v", err),
		})
	}

	// 转换为响应格式
	workflows := make([]map[string]interface{}, len(records))
	for i, record := range records {
		workflows[i] = map[string]interface{}{
			"id":          record.ID,
			"name":        record.Name,
			"description": record.Description,
			"created_at":  record.CreatedAt,
			"updated_at":  record.UpdatedAt,
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"workflows": workflows,
		"total":     total,
		"page":      page,
		"limit":     limit,
	})
}

// HandleGetWorkflow 获取单个工作流
func (s *WorkflowService) HandleGetWorkflow(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Workflow ID is required",
		})
	}

	workflow, record, err := s.store.GetWorkflow(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": fmt.Sprintf("Workflow not found: %v", err),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":          record.ID,
		"name":        record.Name,
		"description": record.Description,
		"workflow":    workflow,
		"created_at":  record.CreatedAt,
		"updated_at":  record.UpdatedAt,
	})
}

// HandleDeleteWorkflow 删除工作流
func (s *WorkflowService) HandleDeleteWorkflow(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Workflow ID is required",
		})
	}

	if err := s.store.DeleteWorkflow(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to delete workflow: %v", err),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// HandleGetTrace 获取执行追踪
func (s *WorkflowService) HandleGetTrace(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Trace ID is required",
		})
	}

	trace, err := s.store.GetExecution(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": fmt.Sprintf("Trace not found: %v", err),
		})
	}

	return c.JSON(http.StatusOK, trace)
}

