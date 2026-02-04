package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"agent-langchain/internal/agent"
	"agent-langchain/internal/api"
	"agent-langchain/internal/config"
	"agent-langchain/internal/memory"
	"agent-langchain/internal/models"
	"agent-langchain/internal/orchestrator"
	"agent-langchain/internal/rag"
)

func main() {
	// 解析命令行参数
	configFile := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	log.Println("Starting API server...")

	// 初始化配置
	log.Println("Loading configuration...")
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Configuration loaded successfully. API Port: %d", cfg.Services.API.Port)

	// 创建上下文
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 初始化 Echo 实例
	log.Println("Initializing Echo instance...")
	e := echo.New()

	// 中间件
	log.Println("Configuring middleware...")
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// 初始化 LLM 客户端
	log.Println("Initializing LLM client...")
	llmClient, chatModel := initLLMClient(cfg)

	// 初始化分类器客户端
	log.Println("Initializing classifier client...")
	classifierClient := initClassifierClient(cfg)

	// 初始化 Ollama 客户端（用于 Embedding）
	log.Println("Initializing Ollama client for embedding...")
	ollamaClient := initOllamaClient(cfg)

	// 初始化记忆存储
	log.Println("Initializing memory store...")
	memStore, err := memory.New(cfg.Storage.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize memory store: %v", err)
	}
	defer memStore.Close()
	log.Println("Memory store initialized successfully")

	// 初始化向量存储
	log.Println("Initializing vector store...")
	vectorStore := initVectorStore(cfg, ollamaClient, ctx)
	log.Printf("Vector store initialized: %s", cfg.Storage.Qdrant.BaseURL)

	// 初始化知识库录入器
	log.Println("Initializing knowledge ingestor...")
	chunkingStrategy := rag.ChunkingStrategyTokens
	if cfg.LLM.RAG.ChunkingStrategy == "semantic" {
		chunkingStrategy = rag.ChunkingStrategySemantic
	}

	ingestor := rag.NewIngestor(vectorStore, cfg.LLM.RAG.ChunkSize, cfg.LLM.RAG.ChunkOverlap, chunkingStrategy, "default")
	log.Println("Knowledge ingestor initialized successfully")

	// 创建 Agent
	log.Println("Creating conversational agent...")
	ag := agent.NewConversationalAgent(llmClient, cfg.Base.Debug, cfg.Base.Timeout)
	log.Println("Agent created successfully")

	// 创建编排器
	log.Println("Creating orchestrator...")
	orch := orchestrator.New(cfg, llmClient, classifierClient, ollamaClient, memStore, vectorStore, ag, chatModel)
	log.Println("Orchestrator created successfully")

	// 初始化服务
	log.Println("Initializing services...")
	chatService := api.NewChatService(orch)
	log.Println("Chat service initialized successfully")

	knowledgeService := api.NewKnowledgeService(ingestor, vectorStore)
	log.Println("Knowledge service initialized successfully")

	debugService := api.NewDebugService(cfg)
	log.Println("Debug service initialized successfully")

	promptService := api.NewPromptService(memStore)
	log.Println("Prompt service initialized successfully")

	agentService := api.NewAgentService(memStore)
	log.Println("Agent service initialized successfully")

	// 注册路由
	log.Println("Registering routes...")
	// 健康检查
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	log.Println("Health check route registered")

	// 聊天相关路由
	e.POST("/api/chat", chatService.HandleChat)
	e.POST("/api/chat/stream", chatService.HandleChatStream)
	e.GET("/api/chat/history", chatService.GetChatHistory)
	e.GET("/api/chat/sessions", chatService.GetChatSessions)
	e.POST("/api/chat/clear", chatService.HandleClearData)
	log.Println("Chat routes registered")

	// 知识库相关路由
	e.POST("/api/knowledge/ingest/file", knowledgeService.HandleIngestFile)
	e.POST("/api/knowledge/ingest/directory", knowledgeService.HandleIngestDirectory)
	e.POST("/api/knowledge/ingest/text", knowledgeService.HandleIngestText)
	e.POST("/api/knowledge/query", knowledgeService.HandleKnowledgeQuery)
	e.GET("/api/knowledge/list", knowledgeService.HandleKnowledgeList)
	log.Println("Knowledge routes registered")

	// 提示词相关路由
	e.POST("/api/prompts", promptService.HandleCreatePrompt)
	e.GET("/api/prompts", promptService.HandleListPrompts)
	e.GET("/api/prompts/default", promptService.HandleGetDefaultPrompt)
	e.GET("/api/prompts/:id", promptService.HandleGetPrompt)
	e.PUT("/api/prompts/:id", promptService.HandleUpdatePrompt)
	e.DELETE("/api/prompts/:id", promptService.HandleDeletePrompt)
	log.Println("Prompt routes registered")

	// Agent 相关路由
	e.POST("/api/agents", agentService.HandleCreateAgent)
	e.GET("/api/agents", agentService.HandleListAgents)
	e.GET("/api/agents/active", agentService.HandleGetActiveAgents)
	e.GET("/api/agents/:id", agentService.HandleGetAgent)
	e.PUT("/api/agents/:id", agentService.HandleUpdateAgent)
	e.DELETE("/api/agents/:id", agentService.HandleDeleteAgent)
	log.Println("Agent routes registered")

	// 调试相关路由
	e.GET("/api/debug/config", debugService.HandleGetConfig)
	e.GET("/api/debug/logs", debugService.HandleGetLogs)
	e.GET("/api/debug/test", debugService.HandleTestConnections)
	e.POST("/api/debug/request", debugService.HandleRequestDebug)
	log.Println("Debug routes registered")

	// 启动服务器
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Services.API.Port),
		Handler: e,
	}

	// 优雅关闭
	go func() {
		// 监听中断信号
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down server...")

		// 创建关闭上下文
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// 关闭服务器
		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}
	}()

	// 启动服务器
	addr := fmt.Sprintf(":%d", cfg.Services.API.Port)
	log.Printf("Server starting on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Println("Server exited")
}

// initLLMClient 初始化LLM客户端
func initLLMClient(cfg *config.Config) (models.LLMClient, string) {
	var llmClient models.LLMClient
	var chatModel string

	switch cfg.Base.Provider {
	case "deepseek":
		deepseek := models.NewDeepSeek(cfg.LLM.DeepSeek.BaseURL, cfg.LLM.DeepSeek.APIKey, cfg.LLM.DeepSeek.ChatModel)
		deepseek.SetDebug(cfg.Base.Debug)
		llmClient = deepseek
		chatModel = cfg.LLM.DeepSeek.ChatModel
		log.Printf("使用 DeepSeek API (base_url: %s, model: %s)", cfg.LLM.DeepSeek.BaseURL, chatModel)
	case "anthropic":
		anthropic := models.NewAnthropic(cfg.LLM.Anthropic.BaseURL, cfg.LLM.Anthropic.ChatModel, cfg.Embedding.Model)
		anthropic.SetDebug(cfg.Base.Debug)
		anthropic.Temperature = cfg.LLM.Anthropic.Temperature
		anthropic.RepetitionPenalty = cfg.LLM.Anthropic.RepetitionPenalty
		llmClient = anthropic
		chatModel = cfg.LLM.Anthropic.ChatModel
		log.Printf("使用 Anthropic API (base_url: %s, model: %s)", cfg.LLM.Anthropic.BaseURL, chatModel)
	case "ollama":
		ollama := models.New(cfg.LLM.Ollama.BaseURL, cfg.LLM.Ollama.ChatModel, cfg.Embedding.Model)
		ollama.SetDebug(cfg.Base.Debug)
		ollama.Temperature = cfg.LLM.Ollama.Temperature
		ollama.RepetitionPenalty = cfg.LLM.Ollama.RepetitionPenalty
		llmClient = ollama
		chatModel = cfg.LLM.Ollama.ChatModel
		log.Printf("使用 Ollama (model: %s)", chatModel)
	default:
		log.Fatalf("Unknown provider: %s. Use 'ollama', 'deepseek', or 'anthropic'", cfg.Base.Provider)
	}

	return llmClient, chatModel
}

// initClassifierClient 初始化分类器客户端
func initClassifierClient(cfg *config.Config) models.LLMClient {
	var classifierClient models.LLMClient

	switch cfg.Classifier.Provider {
	case "deepseek":
		deepseek := models.NewDeepSeek(cfg.Classifier.BaseURL, cfg.Classifier.APIKey, cfg.Classifier.Model)
		deepseek.SetDebug(cfg.Base.Debug)
		classifierClient = deepseek
		log.Printf("分类器使用 DeepSeek (base_url: %s, model: %s)", cfg.Classifier.BaseURL, cfg.Classifier.Model)
	case "anthropic":
		anthropic := models.NewAnthropic(cfg.Classifier.BaseURL, cfg.Classifier.Model, cfg.Embedding.Model)
		anthropic.SetDebug(cfg.Base.Debug)
		classifierClient = anthropic
		log.Printf("分类器使用 Anthropic (base_url: %s, model: %s)", cfg.Classifier.BaseURL, cfg.Classifier.Model)
	case "ollama":
		ollama := models.New(cfg.Classifier.BaseURL, cfg.Classifier.Model, cfg.Embedding.Model)
		ollama.SetDebug(cfg.Base.Debug)
		classifierClient = ollama
		log.Printf("分类器使用 Ollama (base_url: %s, model: %s)", cfg.Classifier.BaseURL, cfg.Classifier.Model)
	default:
		log.Fatalf("Unknown classifier provider: %s. Use 'ollama', 'deepseek', or 'anthropic'", cfg.Classifier.Provider)
	}

	return classifierClient
}

// initOllamaClient 初始化Ollama客户端（用于Embedding）
func initOllamaClient(cfg *config.Config) *models.Client {
	// 使用独立的 Embedding 配置
	ollamaClient := models.New(cfg.Embedding.BaseURL, cfg.LLM.Ollama.ChatModel, cfg.Embedding.Model)
	ollamaClient.SetDebug(cfg.Base.Debug)
	log.Printf("使用 Ollama 进行 Embedding (base_url: %s, model: %s)", cfg.Embedding.BaseURL, cfg.Embedding.Model)
	return ollamaClient
}

// initVectorStore 初始化向量存储
func initVectorStore(cfg *config.Config, ollamaClient *models.Client, ctx context.Context) *rag.QdrantStore {
	store := rag.NewStoreFromOllama(cfg.Storage.Qdrant.BaseURL, cfg.Storage.Qdrant.APIKey, cfg.Storage.Qdrant.Collection, ollamaClient)
	if cfg.Storage.Qdrant.APIKey != "" {
		log.Printf("使用 Qdrant (base_url: %s, 已启用 API Key 认证)", cfg.Storage.Qdrant.BaseURL)
	} else {
		log.Printf("使用 Qdrant (base_url: %s)", cfg.Storage.Qdrant.BaseURL)
	}

	// 确保collection存在
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Base.Timeout)*time.Second)
	defer cancel()

	testVec, err := ollamaClient.Embed(callCtx, "init")
	if err != nil {
		log.Fatalf("ollama embedding failed: %v", err)
	}
	if err := store.EnsureCollection(callCtx, len(testVec)); err != nil {
		log.Fatalf("ensure qdrant collection failed: %v", err)
	}

	return store
}
