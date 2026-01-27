package models

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LLMClient 定义 LLM 客户端接口
type LLMClient interface {
	Chat(ctx context.Context, msgs []ChatMessage, model ...string) (string, error)
	SetDebug(debug bool)
}

// EmbedClient 定义 Embedding 客户端接口
type EmbedClient interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type Client struct {
	BaseURL   string
	ChatModel string
	EmbModel  string
	HTTP      *http.Client
	Debug     bool
}

// SetDebug 设置调试模式
func (c *Client) SetDebug(debug bool) {
	c.Debug = debug
}

func New(baseURL, chatModel, embModel string) *Client {
	return &Client{
		BaseURL:   baseURL,
		ChatModel: chatModel,
		EmbModel:  embModel,
		HTTP: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type ChatMessage struct {
	Role    string `json:"role"` // "system" | "user" | "assistant"
	Content string `json:"content"`
}

func (c *Client) Chat(ctx context.Context, msgs []ChatMessage, model ...string) (string, error) {
	// 确定使用的模型，如果提供了参数则使用参数中的模型，否则使用默认模型
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
	
	// 调试输出：发送的请求
	if c.Debug {
		fmt.Printf("[DEBUG] 发送到 Ollama API 的请求：\n%s\n", string(b))
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/api/chat", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama chat http %d", resp.StatusCode)
	}

	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	
	// 调试输出：收到的响应
	if c.Debug {
		fmt.Printf("[DEBUG] 从 Ollama API 收到的响应：\n%s\n", out.Message.Content)
	}
	
	return out.Message.Content, nil
}

func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]any{
		"model":  c.EmbModel,
		"prompt": text,
	}
	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/api/embeddings", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama embeddings http %d", resp.StatusCode)
	}

	var out struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Embedding, nil
}
