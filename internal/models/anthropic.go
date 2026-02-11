package models

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type UserChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicClient struct {
	BaseURL           string
	APIKey            string
	ChatModel         string
	HTTP              *http.Client
	EmbModel          string
	Debug             bool
	Temperature       float64 // 温度参数
	RepetitionPenalty float64 // 重复惩罚
}

func NewAnthropic(baseURL, chatModel, embModel string) *AnthropicClient {
	return &AnthropicClient{
		BaseURL:   baseURL,
		ChatModel: chatModel,
		EmbModel:  embModel,
		HTTP: &http.Client{
			Timeout: 600 * time.Second,
		},
	}
}

func (c *AnthropicClient) SetDebug(debug bool) {
	c.Debug = debug
}

func (c *AnthropicClient) Chat(ctx context.Context, msgs []ChatMessage, model ...string) (string, error) {
	// 确定使用的模型，如果提供了参数则使用参数中的模型，否则使用默认模型
	useModel := c.ChatModel
	if len(model) > 0 && model[0] != "" {
		useModel = model[0]
	}

	// 构建options参数
	options := make(map[string]any)
	if c.Temperature > 0 {
		options["temperature"] = c.Temperature
	}
	if c.RepetitionPenalty > 0 {
		options["repeat_penalty"] = c.RepetitionPenalty
	}
	prompt := convertMessagesToPrompt(msgs)
	// 直接使用标准messages格式，让Ollama处理chat模板
	reqBody := map[string]any{
		"model":    useModel,
		"messages": prompt, // 使用原始messages，不转换
		"stream":   false,
	}

	// 只有在有options时才添加
	if len(options) > 0 {
		reqBody["options"] = options
	}

	b, _ := json.Marshal(reqBody)

	// 调试输出：发送的请求
	if c.Debug {

		fmt.Printf("\033[35m[DEBUG] 发送到 Ollama API 的请求：\n%s\033[0m\n", string(b))
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

// ChatStream 流式聊天，返回token流
func (c *AnthropicClient) ChatStream(ctx context.Context, msgs []ChatMessage, model ...string) (<-chan string, <-chan error) {
	tokenCh := make(chan string, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(tokenCh)
		defer close(errCh)

		// 确定使用的模型
		useModel := c.ChatModel
		if len(model) > 0 && model[0] != "" {
			useModel = model[0]
		}

		// 构建options参数
		options := make(map[string]any)
		if c.Temperature > 0 {
			options["temperature"] = c.Temperature
		}
		if c.RepetitionPenalty > 0 {
			options["repeat_penalty"] = c.RepetitionPenalty
		}
		prompts := convertMessagesToPrompt(msgs)
		// 直接使用标准messages格式，让Ollama处理chat模板
		reqBody := map[string]any{
			"model":    useModel,
			"messages": prompts, // 使用原始messages，不转换
			"stream":   true,
		}

		// 只有在有options时才添加
		if len(options) > 0 {
			reqBody["options"] = options
		}

		b, _ := json.Marshal(reqBody)

		// 调试输出
		if c.Debug {
			// 使用红色输出调试信息
			fmt.Printf("\033[31m[DEBUG] 发送流式请求到 Ollama API (model: %s, body: %s)\033[0m\n", useModel, string(b))
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/api/chat", bytes.NewReader(b))
		if err != nil {
			errCh <- err
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			errCh <- fmt.Errorf("ollama chat stream http %d", resp.StatusCode)
			return
		}

		// 逐行读取流式响应
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var chunk struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				Done bool `json:"done"`
			}

			if err := json.Unmarshal(line, &chunk); err != nil {
				// 忽略解析错误，继续处理下一行
				continue
			}

			// 发送token到channel
			if chunk.Message.Content != "" {
				select {
				case tokenCh <- chunk.Message.Content:
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				}
			}

			// 如果完成，退出循环
			if chunk.Done {
				break
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			errCh <- err
		}
	}()

	return tokenCh, errCh
}

func convertMessagesToPrompt(msgs []ChatMessage) []map[string]any {
	var prompt strings.Builder

	for _, msg := range msgs {
		switch msg.Role {
		case "system":
			prompt.WriteString(msg.Content)
			prompt.WriteString("\n\n")
		case "user":
			prompt.WriteString("Human: ")
			prompt.WriteString(msg.Content)
			prompt.WriteString("\n\n")
		case "assistant":
			prompt.WriteString("Assistant: ")
			prompt.WriteString(msg.Content)
			prompt.WriteString("\n\n")
		}
	}

	// 添加最后的 Assistant: 提示
	prompt.WriteString("Assistant:")
	userMsg := UserChatMessage{
		Role:    "user",
		Content: prompt.String(),
	}

	userMsgs := []map[string]any{{
		"role":    userMsg.Role,
		"content": userMsg.Content,
	}}

	return userMsgs
}

func (c *AnthropicClient) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]any{
		"model":  c.ChatModel,
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
