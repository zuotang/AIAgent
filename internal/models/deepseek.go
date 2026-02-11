package models

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
			Timeout: 600 * time.Second,
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

// ChatStream 发送流式聊天请求到 DeepSeek API
func (c *DeepSeekClient) ChatStream(ctx context.Context, msgs []ChatMessage, model ...string) (<-chan string, <-chan error) {
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

		reqBody := map[string]any{
			"model":    useModel,
			"messages": msgs,
			"stream":   true, // 启用流式模式
		}
		b, _ := json.Marshal(reqBody)

		// 调试输出
		if c.Debug {
			fmt.Printf("\033[31m[DEBUG] 发送流式请求到 DeepSeek API (model: %s, body: %s)\033[0m\n", useModel, string(b))
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(b))
		if err != nil {
			errCh <- err
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.APIKey)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			errCh <- fmt.Errorf("deepseek chat stream http %d", resp.StatusCode)
			return
		}

		// 逐行读取 SSE 流式响应
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// 跳过空行
			if len(line) == 0 {
				continue
			}

			// SSE 格式：data: {...}
			if len(line) > 6 && line[:6] == "data: " {
				data := line[6:]

				// 检查是否是结束标记
				if data == "[DONE]" {
					break
				}

				// 解析 JSON
				var chunk struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
					} `json:"choices"`
				}

				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					// 忽略解析错误，继续处理下一行
					if c.Debug {
						fmt.Printf("\033[33m[DEBUG] 解析流式响应失败: %v, 数据: %s\033[0m\n", err, data)
					}
					continue
				}

				// 发送 token 到 channel
				if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
					select {
					case tokenCh <- chunk.Choices[0].Delta.Content:
					case <-ctx.Done():
						errCh <- ctx.Err()
						return
					}
				}
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			errCh <- err
		}
	}()

	return tokenCh, errCh
}
