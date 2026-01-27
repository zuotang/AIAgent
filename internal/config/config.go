package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 主配置结构
type Config struct {
	Provider string          `yaml:"provider"` // ollama 或 deepseek
	Ollama   OllamaConfig    `yaml:"ollama"`
	DeepSeek DeepSeekConfig  `yaml:"deepseek"`
	Qdrant   QdrantConfig    `yaml:"qdrant"`
	Database DatabaseConfig  `yaml:"database"`
	Memory   MemoryConfig    `yaml:"memory"`
	Debug    bool            `yaml:"debug"`
	Timeout  int             `yaml:"timeout"` // 超时时间（秒）
}

// OllamaConfig Ollama 配置
type OllamaConfig struct {
	BaseURL   string `yaml:"base_url"`
	ChatModel string `yaml:"chat_model"`
	EmbedModel string `yaml:"embed_model"`
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
	WindowSize      int    `yaml:"window_size"`       // 短期记忆窗口大小
	ExtractorModel  string `yaml:"extractor_model"`   // 记忆提取模型
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 设置默认值
	cfg.setDefaults()

	return &cfg, nil
}

// setDefaults 设置默认值
func (c *Config) setDefaults() {
	if c.Provider == "" {
		c.Provider = "ollama"
	}
	if c.Ollama.BaseURL == "" {
		c.Ollama.BaseURL = "http://127.0.0.1:11434"
	}
	if c.Ollama.ChatModel == "" {
		c.Ollama.ChatModel = "gemma3:12b"
	}
	if c.Ollama.EmbedModel == "" {
		c.Ollama.EmbedModel = "nomic-embed-text"
	}
	if c.DeepSeek.BaseURL == "" {
		c.DeepSeek.BaseURL = "https://api.deepseek.com/v1"
	}
	if c.DeepSeek.ChatModel == "" {
		c.DeepSeek.ChatModel = "deepseek-chat"
	}
	if c.Qdrant.BaseURL == "" {
		c.Qdrant.BaseURL = "http://127.0.0.1:6333"
	}
	if c.Qdrant.Collection == "" {
		c.Qdrant.Collection = "memories"
	}
	if c.Qdrant.TopK == 0 {
		c.Qdrant.TopK = 6
	}
	if c.Database.Path == "" {
		c.Database.Path = "memory.db"
	}
	if c.Memory.WindowSize == 0 {
		c.Memory.WindowSize = 8
	}
	if c.Timeout == 0 {
		c.Timeout = 60
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Provider != "ollama" && c.Provider != "deepseek" {
		return fmt.Errorf("invalid provider: %s (must be 'ollama' or 'deepseek')", c.Provider)
	}

	if c.Provider == "deepseek" && c.DeepSeek.APIKey == "" {
		return fmt.Errorf("deepseek api_key is required when provider is 'deepseek'")
	}

	return nil
}
