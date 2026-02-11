package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"agent-langchain/internal/memory"
	"agent-langchain/internal/models"
	"agent-langchain/internal/rag"
	"agent-langchain/internal/workflow/dsl"
	"agent-langchain/internal/workflow/engine"
	"agent-langchain/internal/workflow/nodes"
	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/store"
)

// WorkflowService 工作流服务
type WorkflowService struct {
	registry    *registry.Registry
	executor    *engine.Executor
	llmClient   models.LLMClient
	embedClient models.EmbedClient
	memStore    *memory.Store
	vectorStore *rag.QdrantStore
	store       *store.WorkflowStore
}

// NewWorkflowService 创建工作流服务
func NewWorkflowService(llmClient models.LLMClient, embedClient models.EmbedClient, memStore *memory.Store, vectorStore *rag.QdrantStore, dbPath string) (*WorkflowService, error) {
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
		registry:    reg,
		executor:    executor,
		llmClient:   llmClient,
		embedClient: embedClient,
		memStore:    memStore,
		vectorStore: vectorStore,
		store:       workflowStore,
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
		LLMClient:    registry.NewLLMClientAdapter(s.llmClient),
		EmbedClient:  s.embedClient,
		MemoryStore:  newMemoryStoreAdapter(s.memStore),
		QdrantClient: newQdrantAdapter(s.vectorStore),
		Cache:        make(map[string]any),
	}
	rc.Cache["executor"] = s.executor

	// 执行工作流
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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

// HandleExecuteWorkflowStream 以 SSE 流式执行工作流，实时推送节点执行进度
func (s *WorkflowService) HandleExecuteWorkflowStream(c echo.Context) error {
	var req ExecuteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid request: %v", err),
		})
	}

	// 设置 SSE 响应头
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	// SSE 写入辅助函数
	writeSSE := func(eventType string, data any) {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return
		}
		fmt.Fprintf(c.Response(), "event: %s\ndata: %s\n\n", eventType, jsonData)
		if flusher, ok := c.Response().Writer.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	// 创建运行时上下文
	rc := &registry.RunContext{
		LLMClient:    registry.NewLLMClientAdapter(s.llmClient),
		EmbedClient:  s.embedClient,
		MemoryStore:  newMemoryStoreAdapter(s.memStore),
		QdrantClient: newQdrantAdapter(s.vectorStore),
		Cache:        make(map[string]any),
	}
	rc.Cache["executor"] = s.executor

	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Minute)
	defer cancel()

	// 使用带事件回调的执行器
	trace, err := s.executor.ExecuteWithEvents(ctx, &req.Workflow, rc, func(event engine.NodeEvent) {
		writeSSE(string(event.Type), event)
	})

	// 保存执行记录
	userID := c.Request().Header.Get("X-User-ID")
	if userID == "" {
		userID = "default"
	}
	if trace != nil {
		_ = s.store.SaveExecution(req.Workflow.Meta.ID, userID, trace)
	}

	// 如果执行器没有发送 workflow 结束事件（比如 validation 失败），补发
	if err != nil && trace == nil {
		writeSSE(string(engine.WorkflowEventError), map[string]any{
			"type":   engine.WorkflowEventError,
			"status": "error",
			"error":  err.Error(),
		})
	}

	// 发送 SSE 结束标记
	fmt.Fprintf(c.Response(), "event: done\ndata: {}\n\n")
	if flusher, ok := c.Response().Writer.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}

func getCategory(nodeType string) string {
	switch {
	case contains(nodeType, "Agent"):
		return "Agent"
	case contains(nodeType, "LLM"):
		return "LLM"
	case contains(nodeType, "Context"):
		return "Context"
	case contains(nodeType, "Tool"):
		return "Tool"
	case contains(nodeType, "Session"):
		return "Session"
	case contains(nodeType, "Preprocess"):
		return "Preprocess"
	case contains(nodeType, "Input") || contains(nodeType, "Output"):
		return "IO"
	case contains(nodeType, "Transform"):
		return "Transform"
	case contains(nodeType, "Embedding"):
		return "Embedding"
	case contains(nodeType, "Vector"):
		return "Vector"
	case contains(nodeType, "Memory"):
		return "Memory"
	case contains(nodeType, "KB"):
		return "KB"
	case contains(nodeType, "Logic"):
		return "Logic"
	case contains(nodeType, "Workflow"):
		return "Workflow"
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
		"Agent.Create":             "动态创建 LLM Agent 实例",
		"Agent.Chat":               "使用 Agent 进行对话交互",
		"LLM.Ollama":               "使用本地Ollama运行的开源模型",
		"LLM.DeepSeek":             "使用DeepSeek云端API",
		"LLM.Anthropic":            "使用Anthropic Claude API",
		"LLM.Chat":                 "统一的LLM接口，支持多种提供商",
		"LLM.Generate":             "使用LLM生成文本响应",
		"LLM.JSON":                 "使用LLM生成JSON输出",
		"Context.Pack":             "将多个消息输入(系统提示词/用户提示词/通用消息)打包成context_pack",
		"Context.Compress":         "使用LLM压缩上下文",
		"Context.Assemble":         "组装系统/用户/证据/记忆上下文为 context_pack",
		"Context.WindowCheck":      "检测上下文是否超窗",
		"Context.Summary":          "对话摘要（占位节点）",
		"Context.KeepRecent":       "保留最近高价值片段（占位节点）",
		"Context.KeepCitations":    "保留引用片段（占位节点）",
		"Tool.Time.Now":            "获取当前时间",
		"Tool.Calc":                "执行数学计算",
		"Tool.Decide":              "使用LLM判断是否需要工具并生成工具调用参数",
		"Tool.Execute":             "执行工具调用（calculator/time）",
		"Tool.Sufficient":          "使用LLM判断工具结果是否足够",
		"Tool.Validate":            "工具结果校验（占位节点）",
		"Input.Text":               "文本输入节点",
		"Output.Text":              "文本输出节点",
		"Output.SaveFile":          "将文本保存到文件",
		"Input.JSON":               "JSON输入节点",
		"Output.JSON":              "JSON输出节点",
		"Transform.TextToMessages": "将文本转换为消息列表",
		"Transform.MessagesToText": "将消息列表转换为文本",
		"Transform.JSONToText":     "将JSON转换为文本",
		"Transform.TextToJSON":     "将文本转换为JSON",
		"Embedding.Encode":         "将文本转换为向量嵌入",
		"Vector.Query":             "向量相似度检索，支持知识库和记忆库",
		"Vector.Upsert":            "将文本写入向量存储",
		"KB.QueryRewrite":          "生成检索用的 Query Rewrite",
		"KB.Search":                "知识库检索（向量检索）",
		"KB.RerankDedup":           "检索结果重排与去重",
		"KB.EvidencePack":          "证据打包为上下文片段",
		"Context.InjectEvidence":   "将证据注入为系统消息",
		"Memory.Query":             "查询结构化记忆（SQLite）",
		"Memory.ChatHistory":       "获取聊天历史消息",
		"Memory.Extract":           "使用LLM从对话文本中提取结构化记忆",
		"Memory.Save":              "将提取的记忆写入SQLite和向量存储",
		"Memory.Read":              "读取长期记忆（结构化渲染）",
		"Memory.Candidate":         "候选记忆生成（占位节点）",
		"Memory.Gate":              "记忆门控（占位节点）",
		"Memory.Write":             "写入长期记忆存储",
		"Context.InjectMemory":     "将记忆注入为系统消息",
		"Session.Entry":            "会话入口：记录元数据",
		"Preprocess.Basic":         "预处理：分词/语言检测/意图识别（占位）",
		"Workflow.Call":            "调用子工作流（组合节点）",
		"Logic.Switch":             "多条件分支：将输入值与多个选项匹配，路由到对应出口。支持 data 透传端口",
		"Logic.If":                 "单条件判断：判断输入值是否满足条件，分流到 true/false 出口。支持 data 透传端口",
		"Logic.Loop":               "循环执行：重复处理 N 次，支持内置 LLM 迭代调用（模板占位符 {{input}} {{output}} {{index}}）",
		"Flow.If":                  "控制流条件判断：根据输入值输出 true/false 的 flow 信号",
		"Flow.Switch":              "控制流多分支：根据输入值输出对应 case 的 flow 信号",
		"Flow.Loop":                "控制流循环：按计数器输出 continue/done 的 flow 信号",
		"Flow.Start":               "流程起点：输出一次 flow 信号",
		"Flow.Debug":               "调试 flow 信号：触发时输出调试文本",
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
	case "Agent.Create":
		params = append(params,
			ParamInfo{
				Name:        "prompt",
				Type:        "string",
				Required:    true,
				Description: "系统提示词，定义 Agent 的角色和行为",
			},
			ParamInfo{
				Name:        "provider",
				Type:        "string",
				Required:    false,
				Default:     "ollama",
				Description: "LLM 提供商",
				Options:     []string{"ollama", "deepseek", "anthropic"},
			},
			ParamInfo{
				Name:        "model",
				Type:        "string",
				Required:    false,
				Default:     "qwen3:4b",
				Description: "模型名称",
			},
			ParamInfo{
				Name:        "base_url",
				Type:        "string",
				Required:    false,
				Description: "API 基础 URL（可选）",
			},
			ParamInfo{
				Name:        "api_key",
				Type:        "string",
				Required:    false,
				Description: "API 密钥（DeepSeek/Anthropic 需要）",
			},
			ParamInfo{
				Name:        "temperature",
				Type:        "number",
				Required:    false,
				Default:     0.7,
				Description: "温度参数，控制输出的随机性 (0-2)",
				Min:         float64Ptr(0),
				Max:         float64Ptr(2),
			},
		)
	case "LLM.Ollama":
		params = append(params,
			ParamInfo{
				Name:        "model",
				Type:        "string",
				Required:    false,
				Default:     "qwen3:4b",
				Description: "模型名称",
				//Options:     []string{"qwen3:4b", "llama2", "mistral", "codellama"},
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
	case "LLM.Generate":
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
				Name:        "base_url",
				Type:        "string",
				Required:    false,
				Default:     "http://localhost:11434",
				Description: "API地址（provider=ollama 时为本地地址）",
			},
			ParamInfo{
				Name:        "api_key",
				Type:        "string",
				Required:    false,
				Description: "API密钥(DeepSeek/Anthropic需要)",
			},
			ParamInfo{
				Name:        "model",
				Type:        "string",
				Required:    false,
				Default:     "qwen3:4b",
				Description: "模型名称（默认使用 Ollama qwen3:4b）",
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
	case "Tool.Decide":
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
				Name:        "base_url",
				Type:        "string",
				Required:    false,
				Default:     "http://localhost:11434",
				Description: "API地址（provider=ollama 时为本地地址）",
			},
			ParamInfo{
				Name:        "api_key",
				Type:        "string",
				Required:    false,
				Description: "API密钥(DeepSeek/Anthropic需要)",
			},
			ParamInfo{
				Name:        "model",
				Type:        "string",
				Required:    false,
				Default:     "Gemma3UThink:4b",
				Description: "模型名称",
			},
			ParamInfo{
				Name:        "temperature",
				Type:        "number",
				Required:    false,
				Default:     0.0,
				Description: "温度参数",
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
			ParamInfo{
				Name:        "prompt",
				Type:        "string",
				Required:    false,
				Description: "工具判断提示词",
			},
			ParamInfo{
				Name:        "tools",
				Type:        "array",
				Required:    false,
				Description: "可用工具列表",
			},
		)
	case "Tool.Sufficient":
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
				Name:        "base_url",
				Type:        "string",
				Required:    false,
				Default:     "http://localhost:11434",
				Description: "API地址（provider=ollama 时为本地地址）",
			},
			ParamInfo{
				Name:        "api_key",
				Type:        "string",
				Required:    false,
				Description: "API密钥(DeepSeek/Anthropic需要)",
			},
			ParamInfo{
				Name:        "model",
				Type:        "string",
				Required:    false,
				Default:     "Gemma3UThink:4b",
				Description: "模型名称",
			},
			ParamInfo{
				Name:        "temperature",
				Type:        "number",
				Required:    false,
				Default:     0.0,
				Description: "温度参数",
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
			ParamInfo{
				Name:        "prompt",
				Type:        "string",
				Required:    false,
				Description: "结果是否足够的判断提示词",
			},
			ParamInfo{
				Name:        "max_attempts",
				Type:        "number",
				Required:    false,
				Default:     3,
				Description: "最大循环次数（达到后强制 enough）",
				Min:         float64Ptr(1),
				Max:         float64Ptr(20),
			},
			ParamInfo{
				Name:        "key",
				Type:        "string",
				Required:    false,
				Default:     "default",
				Description: "循环计数器 key",
			},
		)
	case "Tool.Execute":
		params = append(params,
			ParamInfo{
				Name:        "format",
				Type:        "string",
				Required:    false,
				Default:     "2006-01-02 15:04:05",
				Description: "time 工具格式（可选）",
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
	case "Vector.Query":
		params = append(params,
			ParamInfo{
				Name:        "collection",
				Type:        "string",
				Required:    false,
				Default:     "knowledge",
				Description: "检索集合",
				Options:     []string{"knowledge", "memory"},
			},
			ParamInfo{
				Name:        "top_k",
				Type:        "number",
				Required:    false,
				Default:     3,
				Description: "返回条数",
				Min:         float64Ptr(1),
				Max:         float64Ptr(20),
			},
			ParamInfo{
				Name:        "min_score",
				Type:        "number",
				Required:    false,
				Default:     0.3,
				Description: "最低相似度阈值(0-1)",
				Min:         float64Ptr(0),
				Max:         float64Ptr(1),
			},
			ParamInfo{
				Name:        "user_id",
				Type:        "string",
				Required:    false,
				Default:     "default",
				Description: "用户ID（memory集合需要）",
			},
			ParamInfo{
				Name:        "agent_id",
				Type:        "number",
				Required:    false,
				Default:     1,
				Description: "Agent ID",
				Min:         float64Ptr(1),
			},
		)
	case "Vector.Upsert":
		params = append(params,
			ParamInfo{
				Name:        "collection",
				Type:        "string",
				Required:    false,
				Default:     "knowledge",
				Description: "目标集合",
				Options:     []string{"knowledge", "memory"},
			},
			ParamInfo{
				Name:        "user_id",
				Type:        "string",
				Required:    false,
				Default:     "default",
				Description: "用户ID（memory集合需要）",
			},
			ParamInfo{
				Name:        "agent_id",
				Type:        "number",
				Required:    false,
				Default:     1,
				Description: "Agent ID",
				Min:         float64Ptr(1),
			},
			ParamInfo{
				Name:        "file_name",
				Type:        "string",
				Required:    false,
				Default:     "",
				Description: "文件名标记（knowledge集合用）",
			},
		)
	case "Memory.Query":
		params = append(params,
			ParamInfo{
				Name:        "user_id",
				Type:        "string",
				Required:    false,
				Default:     "default",
				Description: "用户ID",
			},
			ParamInfo{
				Name:        "agent_id",
				Type:        "number",
				Required:    false,
				Default:     1,
				Description: "Agent ID",
				Min:         float64Ptr(1),
			},
			ParamInfo{
				Name:        "limit",
				Type:        "number",
				Required:    false,
				Default:     50,
				Description: "最大记忆条数",
				Min:         float64Ptr(1),
				Max:         float64Ptr(200),
			},
		)
	case "Memory.ChatHistory":
		params = append(params,
			ParamInfo{
				Name:        "user_id",
				Type:        "string",
				Required:    false,
				Default:     "default",
				Description: "用户ID",
			},
			ParamInfo{
				Name:        "agent_id",
				Type:        "number",
				Required:    false,
				Default:     1,
				Description: "Agent ID",
				Min:         float64Ptr(1),
			},
			ParamInfo{
				Name:        "limit",
				Type:        "number",
				Required:    false,
				Default:     20,
				Description: "最大消息条数",
				Min:         float64Ptr(1),
				Max:         float64Ptr(100),
			},
		)
	case "Memory.Extract":
		params = append(params,
			ParamInfo{
				Name:        "model",
				Type:        "string",
				Required:    false,
				Description: "提取器使用的LLM模型（留空使用默认模型）",
			},
			ParamInfo{
				Name:        "include_history",
				Type:        "boolean",
				Required:    false,
				Default:     true,
				Description: "是否将历史上下文传给LLM（有助于理解但可能导致重复提取）",
			},
		)
	case "Memory.Save":
		params = append(params,
			ParamInfo{
				Name:        "user_id",
				Type:        "string",
				Required:    false,
				Default:     "default",
				Description: "用户ID",
			},
			ParamInfo{
				Name:        "agent_id",
				Type:        "number",
				Required:    false,
				Default:     1,
				Description: "Agent ID",
				Min:         float64Ptr(1),
			},
			ParamInfo{
				Name:        "also_vector",
				Type:        "boolean",
				Required:    false,
				Default:     true,
				Description: "是否同时写入Qdrant向量存储",
			},
		)
	case "Logic.Switch":
		params = append(params,
			ParamInfo{
				Name:        "cases",
				Type:        "array",
				Required:    true,
				Description: "匹配值数组，如 [\"选项A\",\"选项B\",\"选项C\"]，依次对应输出端口 case_0, case_1, case_2",
			},
			ParamInfo{
				Name:        "mode",
				Type:        "string",
				Required:    false,
				Default:     "exact",
				Description: "匹配模式",
				Options:     []string{"exact", "contains", "prefix", "suffix", "iexact", "icontains"},
			},
		)
	case "Logic.If":
		params = append(params,
			ParamInfo{
				Name:        "operator",
				Type:        "string",
				Required:    false,
				Default:     "eq",
				Description: "比较运算符",
				Options:     []string{"eq", "neq", "contains", "not_contains", "prefix", "suffix", "gt", "lt", "gte", "lte", "empty", "not_empty"},
			},
			ParamInfo{
				Name:        "compare",
				Type:        "string",
				Required:    false,
				Default:     "",
				Description: "比较目标值（empty/not_empty 运算符不需要）",
			},
		)
	case "Logic.Loop":
		params = append(params,
			ParamInfo{
				Name:        "count",
				Type:        "number",
				Required:    false,
				Default:     1,
				Description: "循环次数",
				Min:         float64Ptr(1),
				Max:         float64Ptr(100),
			},
			ParamInfo{
				Name:        "prompt",
				Type:        "string",
				Required:    false,
				Default:     "",
				Description: "LLM 提示词模板，支持占位符 {{input}}(原始输入) {{output}}(上轮输出) {{index}}(当前轮次)。留空则为简单模式不调用 LLM",
			},
			ParamInfo{
				Name:        "model",
				Type:        "string",
				Required:    false,
				Description: "LLM 模型名称（留空使用默认模型，仅 prompt 非空时生效）",
			},
			ParamInfo{
				Name:        "separator",
				Type:        "string",
				Required:    false,
				Default:     "\n---\n",
				Description: "all 输出端口的分隔符",
			},
		)
	case "Flow.If":
		params = append(params,
			ParamInfo{
				Name:        "operator",
				Type:        "string",
				Required:    false,
				Default:     "eq",
				Description: "比较运算符",
				Options:     []string{"eq", "neq", "contains", "not_contains", "prefix", "suffix", "gt", "lt", "gte", "lte", "empty", "not_empty"},
			},
			ParamInfo{
				Name:        "compare",
				Type:        "string",
				Required:    false,
				Default:     "",
				Description: "比较目标值（empty/not_empty 运算符不需要）",
			},
		)
	case "Session.Entry":
		params = append(params,
			ParamInfo{
				Name:        "channel",
				Type:        "string",
				Required:    false,
				Default:     "web",
				Description: "渠道标识",
			},
		)
	case "Preprocess.Basic":
		params = append(params,
			ParamInfo{
				Name:        "lang",
				Type:        "string",
				Required:    false,
				Default:     "auto",
				Description: "语言设置（占位）",
			},
		)
	case "KB.QueryRewrite":
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
				Name:        "base_url",
				Type:        "string",
				Required:    false,
				Default:     "http://localhost:11434",
				Description: "API地址（provider=ollama 时为本地地址）",
			},
			ParamInfo{
				Name:        "api_key",
				Type:        "string",
				Required:    false,
				Description: "API密钥(DeepSeek/Anthropic需要)",
			},
			ParamInfo{
				Name:        "model",
				Type:        "string",
				Required:    false,
				Default:     "qwen3:4b",
				Description: "模型名称",
			},
			ParamInfo{
				Name:        "temperature",
				Type:        "number",
				Required:    false,
				Default:     0.0,
				Description: "温度参数",
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
			ParamInfo{
				Name:        "prompt",
				Type:        "string",
				Required:    false,
				Description: "Query Rewrite 提示词",
			},
		)
	case "KB.Search":
		params = append(params,
			ParamInfo{
				Name:        "top_k",
				Type:        "number",
				Required:    false,
				Default:     3,
				Description: "返回条数",
				Min:         float64Ptr(1),
				Max:         float64Ptr(20),
			},
			ParamInfo{
				Name:        "min_score",
				Type:        "number",
				Required:    false,
				Default:     0.0,
				Description: "最低相似度阈值(0-1)",
				Min:         float64Ptr(0),
				Max:         float64Ptr(1),
			},
			ParamInfo{
				Name:        "agent_id",
				Type:        "number",
				Required:    false,
				Default:     1,
				Description: "Agent ID",
				Min:         float64Ptr(1),
			},
		)
	case "KB.RerankDedup":
		params = append(params,
			ParamInfo{
				Name:        "min_score",
				Type:        "number",
				Required:    false,
				Default:     0.0,
				Description: "最低相似度阈值(0-1)",
				Min:         float64Ptr(0),
				Max:         float64Ptr(1),
			},
			ParamInfo{
				Name:        "max_docs",
				Type:        "number",
				Required:    false,
				Default:     5,
				Description: "最多保留文档数",
				Min:         float64Ptr(1),
				Max:         float64Ptr(50),
			},
		)
	case "KB.EvidencePack":
		params = append(params,
			ParamInfo{
				Name:        "max_docs",
				Type:        "number",
				Required:    false,
				Default:     5,
				Description: "最多证据条数",
				Min:         float64Ptr(1),
				Max:         float64Ptr(20),
			},
			ParamInfo{
				Name:        "max_chars",
				Type:        "number",
				Required:    false,
				Default:     1200,
				Description: "证据最大字符数",
				Min:         float64Ptr(200),
				Max:         float64Ptr(5000),
			},
		)
	case "Context.InjectEvidence":
		params = append(params,
			ParamInfo{
				Name:        "prefix",
				Type:        "string",
				Required:    false,
				Default:     "Evidence:\\n",
				Description: "注入前缀",
			},
		)
	case "Context.Assemble":
		params = append(params,
			ParamInfo{
				Name:        "note",
				Type:        "string",
				Required:    false,
				Default:     "",
				Description: "占位参数",
			},
		)
	case "Context.WindowCheck":
		params = append(params,
			ParamInfo{
				Name:        "max_chars",
				Type:        "number",
				Required:    false,
				Default:     4000,
				Description: "上下文最大字符数",
				Min:         float64Ptr(500),
				Max:         float64Ptr(20000),
			},
		)
	case "Context.Summary":
		params = append(params, ParamInfo{Name: "note", Type: "string", Required: false, Default: "", Description: "占位参数"})
	case "Context.KeepRecent":
		params = append(params, ParamInfo{Name: "note", Type: "string", Required: false, Default: "", Description: "占位参数"})
	case "Context.KeepCitations":
		params = append(params, ParamInfo{Name: "note", Type: "string", Required: false, Default: "", Description: "占位参数"})
	case "Memory.Read":
		params = append(params,
			ParamInfo{
				Name:        "agent_id",
				Type:        "number",
				Required:    false,
				Default:     1,
				Description: "Agent ID",
				Min:         float64Ptr(1),
			},
			ParamInfo{
				Name:        "limit",
				Type:        "number",
				Required:    false,
				Default:     20,
				Description: "最大记忆条数",
				Min:         float64Ptr(1),
				Max:         float64Ptr(200),
			},
		)
	case "Memory.Candidate":
		params = append(params, ParamInfo{Name: "note", Type: "string", Required: false, Default: "", Description: "占位参数"})
	case "Memory.Gate":
		params = append(params, ParamInfo{Name: "note", Type: "string", Required: false, Default: "", Description: "占位参数"})
	case "Memory.Write":
		params = append(params,
			ParamInfo{
				Name:        "user_id",
				Type:        "string",
				Required:    false,
				Default:     "default",
				Description: "用户ID",
			},
			ParamInfo{
				Name:        "agent_id",
				Type:        "number",
				Required:    false,
				Default:     1,
				Description: "Agent ID",
				Min:         float64Ptr(1),
			},
		)
	case "Context.InjectMemory":
		params = append(params, ParamInfo{Name: "note", Type: "string", Required: false, Default: "", Description: "占位参数"})
	case "Tool.Validate":
		params = append(params, ParamInfo{Name: "note", Type: "string", Required: false, Default: "", Description: "占位参数"})
	case "Workflow.Call":
		params = append(params,
			ParamInfo{
				Name:        "workflow_json",
				Type:        "object",
				Required:    true,
				Description: "子工作流 JSON（对象或字符串）",
			},
			ParamInfo{
				Name:        "input_text_node",
				Type:        "string",
				Required:    false,
				Default:     "",
				Description: "子工作流中 Input.Text 节点的 ID",
			},
		)
	case "Flow.Switch":
		params = append(params,
			ParamInfo{
				Name:        "cases",
				Type:        "array",
				Required:    true,
				Description: "匹配值数组，如 [\"选项A\",\"选项B\",\"选项C\"]，依次对应输出端口 case_0, case_1, case_2",
			},
			ParamInfo{
				Name:        "mode",
				Type:        "string",
				Required:    false,
				Default:     "exact",
				Description: "匹配模式",
				Options:     []string{"exact", "contains", "prefix", "suffix", "iexact", "icontains"},
			},
		)
	case "Flow.Loop":
		params = append(params,
			ParamInfo{
				Name:        "max",
				Type:        "number",
				Required:    false,
				Default:     1,
				Description: "最大循环次数",
				Min:         float64Ptr(1),
				Max:         float64Ptr(100),
			},
			ParamInfo{
				Name:        "key",
				Type:        "string",
				Required:    false,
				Default:     "default",
				Description: "循环计数器 key",
			},
		)
	case "Flow.Debug":
		params = append(params,
			ParamInfo{
				Name:        "label",
				Type:        "string",
				Required:    false,
				Default:     "flow",
				Description: "调试标签",
			},
		)
	case "Output.SaveFile":
		params = append(params,
			ParamInfo{
				Name:        "file_path",
				Type:        "string",
				Required:    true,
				Description: "保存文件的路径（相对或绝对路径）",
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

// --- Adapters: 将实际存储类型转换为 registry 接口 ---

// memoryStoreAdapter 将 memory.Store 适配为 registry.RunContext.MemoryStore 接口
type memoryStoreAdapter struct {
	store *memory.Store
}

func newMemoryStoreAdapter(s *memory.Store) *memoryStoreAdapter {
	if s == nil {
		return nil
	}
	return &memoryStoreAdapter{store: s}
}

func (a *memoryStoreAdapter) RenderStructuredMemory(ctx context.Context, userID string, agentID uint, limit int) (string, error) {
	return a.store.RenderStructuredMemory(ctx, userID, agentID, limit)
}

func (a *memoryStoreAdapter) GetChatHistory(ctx context.Context, userID string, agentID uint, limit, offset int) ([]registry.ChatMessageItem, error) {
	msgs, err := a.store.GetChatHistory(ctx, userID, agentID, limit, offset)
	if err != nil {
		return nil, err
	}
	items := make([]registry.ChatMessageItem, len(msgs))
	for i, m := range msgs {
		items[i] = registry.ChatMessageItem{
			ID:        m.ID,
			UserID:    m.UserID,
			AgentID:   m.AgentID,
			Role:      m.Role,
			Content:   m.Content,
			SessionID: m.SessionID,
		}
	}
	return items, nil
}

func (a *memoryStoreAdapter) UpsertExtractedMemories(ctx context.Context, userID string, agentID uint, memories []registry.ExtractedMemoryItem) error {
	// 转换 registry.ExtractedMemoryItem → memory.ExtractedMemory
	mems := make([]memory.ExtractedMemory, len(memories))
	for i, m := range memories {
		mems[i] = memory.ExtractedMemory{
			Type:       m.Type,
			Key:        m.Key,
			Value:      m.Value,
			Confidence: m.Confidence,
			AlsoVector: m.AlsoVector,
			Text:       m.Text,
			Owner:      m.Owner,
			Layer:      m.Layer,
			Importance: m.Importance,
		}
	}
	return a.store.UpsertExtractedMemories(ctx, userID, agentID, mems)
}

// qdrantAdapter 将 rag.QdrantStore 适配为 registry.RunContext.QdrantClient 接口
type qdrantAdapter struct {
	store *rag.QdrantStore
}

func newQdrantAdapter(s *rag.QdrantStore) *qdrantAdapter {
	if s == nil {
		return nil
	}
	return &qdrantAdapter{store: s}
}

func (a *qdrantAdapter) SimilaritySearchKnowledge(ctx context.Context, agentID uint, query string, topK int) ([]registry.VectorDoc, error) {
	docs, err := a.store.SimilaritySearchKnowledge(ctx, agentID, query, topK)
	if err != nil {
		return nil, err
	}
	return convertDocs(docs), nil
}

func (a *qdrantAdapter) SimilaritySearchMemory(ctx context.Context, userID string, agentID uint, query string, topK int) ([]registry.VectorDoc, error) {
	docs, err := a.store.SimilaritySearchMemory(ctx, userID, agentID, query, topK)
	if err != nil {
		return nil, err
	}
	return convertDocs(docs), nil
}

func (a *qdrantAdapter) UpsertKnowledgeTexts(ctx context.Context, agentID uint, texts []string, fileName string) error {
	return a.store.UpsertKnowledgeTexts(ctx, agentID, texts, fileName)
}

func (a *qdrantAdapter) UpsertMemoryTexts(ctx context.Context, userID string, agentID uint, texts []string) error {
	return a.store.UpsertMemoryTexts(ctx, userID, agentID, texts)
}

func convertDocs(docs []rag.Doc) []registry.VectorDoc {
	result := make([]registry.VectorDoc, len(docs))
	for i, d := range docs {
		result[i] = registry.VectorDoc{
			PageContent: d.PageContent,
			Score:       d.Score,
		}
	}
	return result
}
