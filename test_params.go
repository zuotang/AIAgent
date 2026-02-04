package main

import (
	"context"
	"fmt"
	"log"

	"agent-langchain/internal/config"
	"agent-langchain/internal/models"
)

func main() {
	// 加载配置
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("=== 配置信息 ===\n")
	fmt.Printf("Provider: %s\n", cfg.Base.Provider)
	fmt.Printf("Debug: %v\n", cfg.Base.Debug)
	fmt.Printf("\n")

	// 根据provider创建客户端
	var client models.LLMClient
	var clientName string

	switch cfg.Base.Provider {
	case "anthropic":
		fmt.Printf("=== Anthropic 配置 ===\n")
		fmt.Printf("Model: %s\n", cfg.LLM.Anthropic.ChatModel)
		fmt.Printf("Temperature: %.2f\n", cfg.LLM.Anthropic.Temperature)
		fmt.Printf("RepetitionPenalty: %.2f\n", cfg.LLM.Anthropic.RepetitionPenalty)
		fmt.Printf("\n")

		anthropic := models.NewAnthropic(cfg.LLM.Anthropic.BaseURL, cfg.LLM.Anthropic.ChatModel, cfg.Embedding.Model)
		anthropic.SetDebug(true) // 强制启用调试
		anthropic.Temperature = cfg.LLM.Anthropic.Temperature
		anthropic.RepetitionPenalty = cfg.LLM.Anthropic.RepetitionPenalty
		client = anthropic
		clientName = "Anthropic"

	case "ollama":
		fmt.Printf("=== Ollama 配置 ===\n")
		fmt.Printf("Model: %s\n", cfg.LLM.Ollama.ChatModel)
		fmt.Printf("Temperature: %.2f\n", cfg.LLM.Ollama.Temperature)
		fmt.Printf("RepetitionPenalty: %.2f\n", cfg.LLM.Ollama.RepetitionPenalty)
		fmt.Printf("\n")

		ollama := models.New(cfg.LLM.Ollama.BaseURL, cfg.LLM.Ollama.ChatModel, cfg.Embedding.Model)
		ollama.SetDebug(true) // 强制启用调试
		ollama.Temperature = cfg.LLM.Ollama.Temperature
		ollama.RepetitionPenalty = cfg.LLM.Ollama.RepetitionPenalty
		client = ollama
		clientName = "Ollama"

	default:
		log.Fatalf("Unknown provider: %s", cfg.Base.Provider)
	}

	// 发送测试消息
	fmt.Printf("=== 发送测试请求 (%s) ===\n", clientName)
	msgs := []models.ChatMessage{
		{Role: "user", Content: "Say hello in one sentence."},
	}

	ctx := context.Background()
	response, err := client.Chat(ctx, msgs)
	if err != nil {
		log.Fatalf("Chat failed: %v", err)
	}

	fmt.Printf("\n=== 响应 ===\n")
	fmt.Printf("%s\n", response)
}
