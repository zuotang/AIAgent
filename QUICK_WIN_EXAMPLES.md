# 快速见效示例 (Quick Wins)

这里提供一些可以**立即**集成到现有项目中的小改进，每个都能在1-2小时内完成，并带来明显的价值提升。

---

## Quick Win 1: 添加计算器工具（30分钟）

### 问题
当前Agent无法进行数学计算，用户询问"2的100次方是多少"时只能猜测或拒绝。

### 解决方案

**创建文件**: `internal/tools/calculator.go`

```go
package tools

import (
    "context"
    "fmt"
    "github.com/Knetic/govaluate"
)

type CalculatorTool struct{}

func (t *CalculatorTool) Name() string {
    return "calculator"
}

func (t *CalculatorTool) Description() string {
    return "Perform mathematical calculations. Input: math expression like '2+3*4' or 'sqrt(16)'"
}

func (t *CalculatorTool) Execute(ctx context.Context, expression string) (string, error) {
    expr, err := govaluate.NewEvaluableExpression(expression)
    if err != nil {
        return "", fmt.Errorf("invalid expression: %w", err)
    }

    result, err := expr.Evaluate(nil)
    if err != nil {
        return "", fmt.Errorf("evaluation error: %w", err)
    }

    return fmt.Sprintf("%.6f", result), nil
}
```

**安装依赖**:
```bash
go get github.com/Knetic/govaluate
```

**集成到对话流程** (在 `cmd/chat/main.go`):
```go
// 在对话前添加工具描述到系统提示
systemPrompt := `You are a helpful AI assistant with access to these tools:

1. **calculator**: Perform math calculations
   - Usage: When user asks a math question, respond with: TOOL_CALL: calculator("expression")
   - Example: "What is 2^100?" → TOOL_CALL: calculator("2^100")

Important: Only use the calculator tool when the user asks a math question.
`

// 在生成响应后检查是否有工具调用
response, err := llmClient.Chat(ctx, messages)

if strings.Contains(response, "TOOL_CALL: calculator(") {
    // 提取表达式
    expr := extractExpression(response)

    // 执行计算
    calc := &tools.CalculatorTool{}
    result, err := calc.Execute(ctx, expr)

    // 将结果反馈给LLM
    messages = append(messages, ChatMessage{
        Role: "tool",
        Content: fmt.Sprintf("Calculator result: %s", result),
    })

    // 再次生成响应
    response, err = llmClient.Chat(ctx, messages)
}
```

**辅助函数**:
```go
func extractExpression(response string) string {
    // 简单提取: TOOL_CALL: calculator("2+3") → "2+3"
    start := strings.Index(response, "calculator(\"")
    if start == -1 {
        return ""
    }
    start += len("calculator(\"")

    end := strings.Index(response[start:], "\")")
    if end == -1 {
        return ""
    }

    return response[start : start+end]
}
```

**测试**:
```
You: 计算 2 的 100 次方
Assistant: [内部思考] 这是一个数学问题，我需要使用计算器工具
          TOOL_CALL: calculator("2^100")
          [工具返回: 1.267651e+30]
          2的100次方等于约 1.27 × 10^30
```

**收益**:
- ✅ Agent 能准确回答数学问题
- ✅ 用户体验显著提升
- ✅ 为后续工具系统打下基础

---

## Quick Win 2: 流式响应（1小时）

### 问题
当前响应是一次性返回，长回复时用户等待体验差。

### 解决方案

**修改 `internal/models/ollama.go`**:

```go
// 添加流式接口
func (c *OllamaClient) ChatStream(ctx context.Context, msgs []ChatMessage, model ...string) (<-chan string, <-chan error) {
    stream := make(chan string, 100)
    errCh := make(chan error, 1)

    selectedModel := c.chatModel
    if len(model) > 0 && model[0] != "" {
        selectedModel = model[0]
    }

    req := &ChatRequest{
        Model:    selectedModel,
        Messages: msgs,
        Stream:   true,  // 关键：启用流式模式
    }

    go func() {
        defer close(stream)
        defer close(errCh)

        reqBody, _ := json.Marshal(req)
        httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(reqBody))
        httpReq.Header.Set("Content-Type", "application/json")

        resp, err := c.client.Do(httpReq)
        if err != nil {
            errCh <- err
            return
        }
        defer resp.Body.Close()

        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            var chunk ChatResponse
            if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
                continue
            }

            if chunk.Message.Content != "" {
                stream <- chunk.Message.Content
            }

            if chunk.Done {
                break
            }
        }

        if err := scanner.Err(); err != nil {
            errCh <- err
        }
    }()

    return stream, errCh
}
```

**修改 `cmd/chat/main.go`**:

```go
import (
    "fmt"
    "os"
)

// 在主循环中使用流式响应
stream, errCh := llmClient.ChatStream(ctx, messages)

fmt.Print("Assistant: ")
fullResponse := ""

// 实时打印token
for token := range stream {
    fmt.Print(token)
    fullResponse += token
    os.Stdout.Sync()  // 强制刷新输出缓冲区
}

// 检查错误
if err := <-errCh; err != nil {
    fmt.Printf("\nError: %v\n", err)
}

fmt.Println()  // 换行
```

**效果对比**:

**之前**:
```
You: 写一首关于Go语言的诗
[等待3秒...]
Assistant: Go语言简洁又高效，
并发编程是其强项，
goroutine轻量又灵活，
开发体验令人称赞。
```

**之后**:
```
You: 写一首关于Go语言的诗
Assistant: Go语言简洁又高效，[逐字显示]
并发编程是其强项，[逐字显示]
goroutine轻量又灵活，[逐字显示]
开发体验令人称赞。[逐字显示]
```

**收益**:
- ✅ 用户感知响应更快
- ✅ 更好的交互体验
- ✅ 可以提前中断长回复（Ctrl+C）

---

## Quick Win 3: 记忆访问统计（30分钟）

### 问题
当前无法知道哪些记忆最常用，无法优化记忆检索。

### 解决方案

**修改 `internal/memory/sqlite.go`**:

```go
// 在 RenderStructuredMemory 中添加访问统计
func (m *SQLiteMemory) RenderStructuredMemory(ctx context.Context, userID string, limit int) (string, error) {
    query := `
        SELECT mtype, mkey, mvalue, confidence, owner
        FROM memories
        WHERE user_id = ?
        ORDER BY
            CASE mtype
                WHEN 'identity' THEN 1
                WHEN 'preference' THEN 2
                WHEN 'goal' THEN 3
                WHEN 'knowledge' THEN 4
                WHEN 'context' THEN 5
            END,
            confidence DESC,
            updated_at DESC
        LIMIT ?
    `

    rows, err := m.db.QueryContext(ctx, query, userID, limit)
    if err != nil {
        return "", err
    }
    defer rows.Close()

    var parts []string
    memoryIDs := []string{}

    for rows.Next() {
        var mtype, mkey, mvalue, owner string
        var confidence float64

        if err := rows.Scan(&mtype, &mkey, &mvalue, &confidence, &owner); err != nil {
            continue
        }

        parts = append(parts, fmt.Sprintf("[%s/%s] %s = %s (%.2f)", owner, mtype, mkey, mvalue, confidence))

        // 记录访问的记忆
        memoryIDs = append(memoryIDs, fmt.Sprintf("%s:%s:%s", userID, mtype, mkey))
    }

    // 异步更新访问统计
    go m.updateAccessStats(context.Background(), memoryIDs)

    return strings.Join(parts, "\n"), nil
}

// 新增：更新访问统计
func (m *SQLiteMemory) updateAccessStats(ctx context.Context, memoryIDs []string) {
    if len(memoryIDs) == 0 {
        return
    }

    tx, err := m.db.BeginTx(ctx, nil)
    if err != nil {
        return
    }
    defer tx.Rollback()

    stmt, err := tx.PrepareContext(ctx, `
        UPDATE memories
        SET access_count = access_count + 1,
            last_accessed_at = CURRENT_TIMESTAMP
        WHERE user_id = ? AND mtype = ? AND mkey = ?
    `)
    if err != nil {
        return
    }
    defer stmt.Close()

    for _, id := range memoryIDs {
        parts := strings.Split(id, ":")
        if len(parts) != 3 {
            continue
        }
        stmt.ExecContext(ctx, parts[0], parts[1], parts[2])
    }

    tx.Commit()
}
```

**添加查询函数**:

```go
// 获取最常访问的记忆
func (m *SQLiteMemory) GetTopAccessedMemories(ctx context.Context, userID string, limit int) ([]ExtractedMemory, error) {
    query := `
        SELECT mtype, mkey, mvalue, confidence, owner, access_count, last_accessed_at
        FROM memories
        WHERE user_id = ?
        ORDER BY access_count DESC, last_accessed_at DESC
        LIMIT ?
    `

    rows, err := m.db.QueryContext(ctx, query, userID, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var memories []ExtractedMemory
    for rows.Next() {
        var mem ExtractedMemory
        var lastAccessed string
        rows.Scan(&mem.Type, &mem.Key, &mem.Value, &mem.Confidence, &mem.Owner, &mem.AccessCount, &lastAccessed)
        memories = append(memories, mem)
    }

    return memories, nil
}
```

**添加 CLI 命令**:

```go
// cmd/chat/main.go

if len(os.Args) > 1 && os.Args[1] == "--stats" {
    showMemoryStats()
    return
}

func showMemoryStats() {
    // ... 初始化数据库 ...

    memories, err := mem.GetTopAccessedMemories(context.Background(), "user123", 10)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Top 10 Most Accessed Memories:")
    fmt.Println(strings.Repeat("=", 60))

    for i, m := range memories {
        fmt.Printf("%d. [%s] %s = %s\n", i+1, m.Type, m.Key, m.Value)
        fmt.Printf("   Accessed: %d times, Last: %s\n\n", m.AccessCount, m.LastAccessed)
    }
}
```

**使用**:
```bash
./chat.exe --stats

Top 10 Most Accessed Memories:
============================================================
1. [identity] name = Alice
   Accessed: 45 times, Last: 2026-01-27 10:30:00

2. [preference] favorite_color = blue
   Accessed: 23 times, Last: 2026-01-27 09:15:00

3. [goal] learn_topic = machine learning
   Accessed: 18 times, Last: 2026-01-26 18:45:00
...
```

**收益**:
- ✅ 了解哪些记忆最重要
- ✅ 为后续记忆优化提供数据支持
- ✅ 可以基于访问频率调整重要性评分

---

## Quick Win 4: 上下文窗口可视化（20分钟）

### 问题
不知道每次对话发送给LLM的上下文有多少token，容易超出限制。

### 解决方案

**添加 token 计数函数**:

```go
// internal/utils/tokens.go
package utils

import (
    "unicode"
)

// EstimateTokens 估算文本的token数量
// 简单规则：英文单词 ≈ 1.3 tokens，中文字符 ≈ 2 tokens
func EstimateTokens(text string) int {
    words := 0
    inWord := false
    chineseChars := 0

    for _, r := range text {
        if unicode.Is(unicode.Han, r) {
            chineseChars++
        } else if unicode.IsSpace(r) || unicode.IsPunct(r) {
            if inWord {
                words++
                inWord = false
            }
        } else {
            inWord = true
        }
    }

    if inWord {
        words++
    }

    // 中文: 每个字符约2 tokens
    // 英文: 每个单词约1.3 tokens
    return int(float64(words)*1.3 + float64(chineseChars)*2.0)
}
```

**修改 `cmd/chat/main.go`**:

```go
import (
    "your-project/internal/utils"
)

// 在发送请求前统计token
systemPromptTokens := utils.EstimateTokens(systemPrompt)
memoryTokens := utils.EstimateTokens(structuredMemory)
conversationTokens := 0
for _, msg := range conversationWindow {
    conversationTokens += utils.EstimateTokens(msg.Content)
}

totalTokens := systemPromptTokens + memoryTokens + conversationTokens

// 如果开启debug模式，打印统计
if cfg.Debug {
    fmt.Printf("\n[Context Window Stats]\n")
    fmt.Printf("  System Prompt: %d tokens\n", systemPromptTokens)
    fmt.Printf("  Memory:        %d tokens\n", memoryTokens)
    fmt.Printf("  Conversation:  %d tokens\n", conversationTokens)
    fmt.Printf("  Total:         %d tokens\n", totalTokens)
    fmt.Printf("  Model Limit:   %d tokens\n", getModelLimit(cfg.Ollama.ChatModel))

    // 警告：接近上下文限制
    modelLimit := getModelLimit(cfg.Ollama.ChatModel)
    if totalTokens > int(float64(modelLimit)*0.8) {
        fmt.Printf("  ⚠️  WARNING: Using %.1f%% of context window!\n", float64(totalTokens)/float64(modelLimit)*100)
    }
    fmt.Println()
}

func getModelLimit(model string) int {
    limits := map[string]int{
        "gemma3:12b":      8192,
        "qwen2.5:7b":      32768,
        "deepseek-chat":   64000,
        "llama3.1:8b":     128000,
    }

    if limit, ok := limits[model]; ok {
        return limit
    }
    return 4096  // 默认值
}
```

**输出示例**:
```
You: 帮我总结一下我们今天的对话

[Context Window Stats]
  System Prompt: 450 tokens
  Memory:        320 tokens
  Conversation:  1850 tokens
  Total:         2620 tokens
  Model Limit:   8192 tokens
