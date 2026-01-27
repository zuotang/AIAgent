package models

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DeepSeekClient 实现 DeepSeek API 调用
type DeepSeekClient struct {
	BaseURL   string
	APIKey    string
	ChatModel string
	HTTP      *http.Client
	Debug     bool
}

// NewDeepSeek 创建新的 DeepSeek 客户端
func NewDeepSeek(baseURL, apiKey, chatModel string) *DeepSeekClient {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	if chatModel == "" {
		chatModel = "deepseek-chat" // 默认模型
	}
	return &DeepSeekClient{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		ChatModel: chatModel,
		HTTP: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// SetDebug 设置调试模式
func (c *DeepSeekClient) SetDebug(debug bool) {
	c.Debug = debug
}

// Chat 发送聊天请求到 DeepSeek API
func (c *DeepSeekClient) Chat(ctx context.Context, msgs []ChatMessage, model ...string) (string, error) {
	useModel := c.ChatModel
	if len(model) > 0 && model[0] != "" {
		useModel = model[0]
	}

	reqBody := map[string]any{
		"model":    useModel,
		"messages": msgs,
		"stream":   false,
	}
	b, _ := json.Marshal(reqBody)

	if c.Debug {
		fmt.Printf("[DEBUG] 发送到 DeepSeek API 的请求：\n%s\n", string(b))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("deepseek chat http %d", resp.StatusCode)
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}

	if c.Debug {
		fmt.Printf("[DEBUG] 从 DeepSeek API 收到的响应：\n%s\n", out.Choices[0].Message.Content)
	}

	if len(out.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return out.Choices[0].Message.Content, nil
}

// Embed DeepSeek 暂不支持 embedding，返回错误
// 如果需要 embedding，建议继续使用 Ollama 的 embedding 模型
func (c *DeepSeekClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, fmt.Errorf("DeepSeek API does not support embeddings, please use Ollama for embeddings")
}
