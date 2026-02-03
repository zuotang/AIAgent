package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func main() {
	// 测试 Ollama 流式响应
	reqBody := map[string]any{
		"model": "PsychosisU:9B",
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个助手"},
			{"role": "user", "content": "你好"},
		},
		"stream": true,
	}

	b, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "http://127.0.0.1:11434/api/chat", bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("开始接收流式响应...")
	fmt.Println("================================")

	scanner := bufio.NewScanner(resp.Body)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
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
			fmt.Printf("解析错误: %v\n", err)
			continue
		}

		if chunk.Message.Content != "" {
			fmt.Printf("[第%d行] 收到内容: %q (长度: %d)\n", lineCount, chunk.Message.Content, len(chunk.Message.Content))
		}

		if chunk.Done {
			fmt.Println("================================")
			fmt.Println("流式响应完成")
			break
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Scanner 错误: %v\n", err)
	}
}
