package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 主配置结构
type Config struct {
	Base        BaseConfig        `yaml:"base"`
	LLM         LLMConfig         `yaml:"llm"`
	Embedding   EmbeddingConfig   `yaml:"embedding"`
	Extractor   ExtractorConfig   `yaml:"extractor"`
	Classifier  ClassifierConfig  `yaml:"classifier"`
	Storage     StorageConfig     `yaml:"storage"`
	Services    ServicesConfig    `yaml:"services"`
	Memory      MemoryConfig      `yaml:"memory"`
	Performance PerformanceConfig `yaml:"performance"`
}

// BaseConfig 基本配置
type BaseConfig struct {
	Provider string `yaml:"provider"` // ollama 或 deepseek
	Debug    bool   `yaml:"debug"`    // 调试模式
	Timeout  int    `yaml:"timeout"`  // 超时时间（秒）
}

// LLMConfig LLM 配置
type LLMConfig struct {
	Ollama   OllamaConfig   `yaml:"ollama"`
	DeepSeek DeepSeekConfig `yaml:"deepseek"`
	RAG      RAGConfig      `yaml:"rag"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Database DatabaseConfig `yaml:"database"`
	Qdrant   QdrantConfig   `yaml:"qdrant"`
}

// ServicesConfig 服务配置
type ServicesConfig struct {
	API APIConfig `yaml:"api"`
}

// APIConfig API 服务配置
type APIConfig struct {
	Port        int  `yaml:"port"`
	Host        string `yaml:"host"`
	CORSEnabled bool `yaml:"cors_enabled"`
}

// EmbeddingConfig Embedding 配置
type EmbeddingConfig struct {
	Provider   string `yaml:"provider"`    // embedding 提供商
	BaseURL    string `yaml:"base_url"`    // 服务地址
	Model      string `yaml:"model"`       // 模型名称
	APIKey     string `yaml:"api_key"`     // API Key
	BatchSize  int    `yaml:"batch_size"`  // 批量处理大小
	Dimensions int    `yaml:"dimensions"`  // 向量维度
}

// ExtractorConfig 记忆提取器配置
type ExtractorConfig struct {
	Provider    string  `yaml:"provider"`     // 提取器提供商
	BaseURL     string  `yaml:"base_url"`     // 服务地址
	Model       string  `yaml:"model"`        // 模型名称
	APIKey      string  `yaml:"api_key"`      // API Key
	Temperature float64 `yaml:"temperature"`  // 温度
	MaxRetries  int     `yaml:"max_retries"`  // 最大重试次数
}

// ClassifierConfig 分类器配置
type ClassifierConfig struct {
	Provider    string  `yaml:"provider"`     // 分类器提供商
	BaseURL     string  `yaml:"base_url"`     // 服务地址
	Model       string  `yaml:"model"`        // 模型名称
	APIKey      string  `yaml:"api_key"`      // API Key
	Temperature float64 `yaml:"temperature"`  // 温度
	Timeout     int     `yaml:"timeout"`      // 超时时间
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	MaxConcurrentRequests int  `yaml:"max_concurrent_requests"` // 最大并发请求数
	RequestTimeout        int  `yaml:"request_timeout"`         // 请求超时
	EnableCache           bool `yaml:"enable_cache"`            // 启用缓存
	CacheTTL              int  `yaml:"cache_ttl"`               // 缓存过期时间
}

// RAGConfig RAG 配置
type RAGConfig struct {
	ChunkSize        int    `yaml:"chunk_size"`
	ChunkOverlap     int    `yaml:"chunk_overlap"`
	ChunkingStrategy string `yaml:"chunking_strategy"` // 分块策略: tokens, semantic
	Collection       string `yaml:"collection"`
}

// OllamaConfig Ollama 配置
type OllamaConfig struct {
	BaseURL   string `yaml:"base_url"`
	ChatModel string `yaml:"chat_model"`
}

// DeepSeekConfig DeepSeek 配置
type DeepSeekConfig struct {
	BaseURL   string `yaml:"base_url"`
	APIKey    string `yaml:"api_key"`
	ChatModel string `yaml:"chat_model"`
}

// QdrantConfig Qdrant 配置
type QdrantConfig struct {
	BaseURL    string `yaml:"base_url"`
	APIKey     string `yaml:"api_key"`     // Qdrant API Key（可选）
	Collection string `yaml:"collection"`
	TopK       int    `yaml:"top_k"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// MemoryConfig 记忆配置
type MemoryConfig struct {
	WindowSize                int    `yaml:"window_size"`                  // 短期记忆窗口大小
	EnableSmartTrigger        bool   `yaml:"enable_smart_trigger"`         // 启用智能触发
	TriggerMethod             string `yaml:"trigger_method"`               // 触发方法
	MinMessageLength          int    `yaml:"min_message_length"`           // 最小消息长度
	IncludeHistoryContext     bool   `yaml:"include_history_context"`      // 提取时包含历史上下文
	MinConfidence             float64 `yaml:"min_confidence"`              // 最小置信度
	MaxMemoriesPerExtraction  int    `yaml:"max_memories_per_extraction"`  // 每次提取的最大记忆数
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	var cfg Config

	// 如果提供了路径，尝试从文件加载
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// 设置默认值
	cfg.setDefaults()

	return &cfg, nil
}

// setDefaults 设置默认值
func (c *Config) setDefaults() {
	// 基本配置默认值
	if c.Base.Provider == "" {
		c.Base.Provider = "ollama"
	}
	if c.Base.Timeout == 0 {
		c.Base.Timeout = 60
	}

	// LLM 配置默认值
	if c.LLM.Ollama.BaseURL == "" {
		c.LLM.Ollama.BaseURL = "http://127.0.0.1:11434"
	}
	if c.LLM.Ollama.ChatModel == "" {
		c.LLM.Ollama.ChatModel = "gemma3:12b"
	}
	if c.LLM.DeepSeek.BaseURL == "" {
		c.LLM.DeepSeek.BaseURL = "https://api.deepseek.com/v1"
	}
	if c.LLM.DeepSeek.ChatModel == "" {
		c.LLM.DeepSeek.ChatModel = "deepseek-chat"
	}

	// Embedding 配置默认值
	if c.Embedding.Provider == "" {
		c.Embedding.Provider = "ollama"
	}
	if c.Embedding.BaseURL == "" {
		c.Embedding.BaseURL = "http://127.0.0.1:11434"
	}
	if c.Embedding.Model == "" {
		c.Embedding.Model = "nomic-embed-text"
	}
	if c.Embedding.BatchSize == 0 {
		c.Embedding.BatchSize = 10
	}

	// Extractor 配置默认值（如果未配置，使用主 LLM）
	if c.Extractor.Provider == "" {
		c.Extractor.Provider = c.Base.Provider
	}
	if c.Extractor.BaseURL == "" {
		if c.Extractor.Provider == "ollama" {
			c.Extractor.BaseURL = c.LLM.Ollama.BaseURL
		} else if c.Extractor.Provider == "deepseek" {
			c.Extractor.BaseURL = c.LLM.DeepSeek.BaseURL
		}
	}
	if c.Extractor.Model == "" {
		// 留空，运行时使用主 chat_model
	}
	if c.Extractor.Temperature == 0 {
		c.Extractor.Temperature = 0.1
	}
	if c.Extractor.MaxRetries == 0 {
		c.Extractor.MaxRetries = 3
	}

	// Classifier 配置默认值
	if c.Classifier.Provider == "" {
		c.Classifier.Provider = "ollama"
	}
	if c.Classifier.BaseURL == "" {
		c.Classifier.BaseURL = "http://127.0.0.1:11434"
	}
	if c.Classifier.Model == "" {
		c.Classifier.Model = "qwen2.5:0.5b"
	}
	if c.Classifier.Temperature == 0 {
		c.Classifier.Temperature = 0.0
	}
	if c.Classifier.Timeout == 0 {
		c.Classifier.Timeout = 10
	}

	// 存储配置默认值
	if c.Storage.Database.Path == "" {
		c.Storage.Database.Path = "memory.db"
	}
	if c.Storage.Qdrant.BaseURL == "" {
		c.Storage.Qdrant.BaseURL = "http://127.0.0.1:6333"
	}
	if c.Storage.Qdrant.Collection == "" {
		c.Storage.Qdrant.Collection = "memories"
	}
	if c.Storage.Qdrant.TopK == 0 {
		c.Storage.Qdrant.TopK = 6
	}

	// 服务配置默认值
	if c.Services.API.Port == 0 {
		c.Services.API.Port = 8080
	}
	if c.Services.API.Host == "" {
		c.Services.API.Host = "0.0.0.0"
	}
	// CORSEnabled 默认为 true

	// RAG 配置默认值
	if c.LLM.RAG.Collection == "" {
		c.LLM.RAG.Collection = "knowledge"
	}
	if c.LLM.RAG.ChunkSize == 0 {
		c.LLM.RAG.ChunkSize = 1000
	}
	if c.LLM.RAG.ChunkOverlap == 0 {
		c.LLM.RAG.ChunkOverlap = 200
	}
	if c.LLM.RAG.ChunkingStrategy == "" {
		c.LLM.RAG.ChunkingStrategy = "tokens"
	}

	// 记忆配置默认值
	if c.Memory.WindowSize == 0 {
		c.Memory.WindowSize = 8
	}
	if c.Memory.TriggerMethod == "" {
		c.Memory.TriggerMethod = "conservative"
	}
	if c.Memory.MinMessageLength == 0 {
		c.Memory.MinMessageLength = 10
	}
	if c.Memory.MinConfidence == 0 {
		c.Memory.MinConfidence = 0.65
	}
	if c.Memory.MaxMemoriesPerExtraction == 0 {
		c.Memory.MaxMemoriesPerExtraction = 20
	}

	// 性能配置默认值
	if c.Performance.MaxConcurrentRequests == 0 {
		c.Performance.MaxConcurrentRequests = 10
	}
	if c.Performance.RequestTimeout == 0 {
		c.Performance.RequestTimeout = 120
	}
	if c.Performance.CacheTTL == 0 {
		c.Performance.CacheTTL = 3600
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Base.Provider != "ollama" && c.Base.Provider != "deepseek" {
		return fmt.Errorf("invalid provider: %s (must be 'ollama' or 'deepseek')", c.Base.Provider)
	}

	if c.Base.Provider == "deepseek" && c.LLM.DeepSeek.APIKey == "" {
		return fmt.Errorf("deepseek api_key is required when provider is 'deepseek'")
	}

	return nil
}
