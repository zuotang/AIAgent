package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-langchain/internal/models"
	"agent-langchain/internal/tools"
)

// ConversationalAgent 对话型Agent实现
type ConversationalAgent struct {
	llmClient models.LLMClient
	debug     bool
	timeout   int
}

// NewConversationalAgent 创建对话型Agent
func NewConversationalAgent(llmClient models.LLMClient, debug bool, timeout int) *ConversationalAgent {
	return &ConversationalAgent{
		llmClient: llmClient,
		debug:     debug,
		timeout:   timeout,
	}
}

// Run 执行Agent任务
func (a *ConversationalAgent) Run(ctx context.Context, input Input) (Output, error) {
	// 构建prompt
	prompt := a.buildPrompt(input)

	// 构建消息
	msgs := []models.ChatMessage{
		{Role: "system", Content: input.SystemPrompt},
		{Role: "user", Content: prompt},
	}

	// 将消息序列化为字符串，存储到 LLMInput
	llmInput, _ := json.MarshalIndent(msgs, "", "  ")

	// 调用LLM生成响应
	response, err := a.generateResponse(ctx, msgs)
	if err != nil {
		return Output{}, err
	}

	output := Output{
		Response:     response,
		ToolCalls:    []ToolCall{},
		ThoughtTrace: []string{},
		Metadata:     make(map[string]interface{}),
		LLMInput:     string(llmInput),
	}

	// 检测工具调用
	if strings.Contains(response, "TOOL_CALL:") {
		toolCall := extractToolCall(response)
		if toolCall != nil {
			// 执行工具
			result, err := a.executeTool(ctx, toolCall)

			// 记录工具调用
			output.ToolCalls = append(output.ToolCalls, ToolCall{
				ToolName:  toolCall.ToolName,
				Arguments: toolCall.Arguments,
				Result:    result,
				Error:     err,
			})

			if a.debug {
				fmt.Printf("检测到工具调用: %s(%s)\n", toolCall.ToolName, toolCall.Arguments)
				fmt.Printf("工具执行结果: %s\n", result)
			}

			// 将工具结果反馈给LLM
			finalResponse, err := a.generateFinalResponse(ctx, msgs, response, toolCall, result)
			if err != nil {
				return output, err
			}

			output.Response = finalResponse
		}
	}

	return output, nil
}

// RunStream 流式执行Agent任务
func (a *ConversationalAgent) RunStream(ctx context.Context, input Input, callback func(string) error) (Output, error) {
	// 构建prompt
	prompt := a.buildPrompt(input)

	// 构建消息
	msgs := []models.ChatMessage{
		{Role: "system", Content: input.SystemPrompt},
		{Role: "user", Content: prompt},
	}

	// 将消息序列化为字符串，存储到 LLMInput
	llmInput, _ := json.MarshalIndent(msgs, "", "  ")
	//输出msgs
	println(msgs)
	// 调用LLM生成响应（流式）
	response, err := a.generateResponseStream(ctx, msgs, callback)
	if err != nil {
		return Output{}, err
	}

	output := Output{
		Response:     response,
		ToolCalls:    []ToolCall{},
		ThoughtTrace: []string{},
		Metadata:     make(map[string]interface{}),
		LLMInput:     string(llmInput),
	}

	// 检测工具调用
	if strings.Contains(response, "TOOL_CALL:") {
		toolCall := extractToolCall(response)
		if toolCall != nil {
			// 执行工具
			result, err := a.executeTool(ctx, toolCall)

			// 记录工具调用
			output.ToolCalls = append(output.ToolCalls, ToolCall{
				ToolName:  toolCall.ToolName,
				Arguments: toolCall.Arguments,
				Result:    result,
				Error:     err,
			})

			if a.debug {
				fmt.Printf("检测到工具调用: %s(%s)\n", toolCall.ToolName, toolCall.Arguments)
				fmt.Printf("工具执行结果: %s\n", result)
			}

			// 将工具结果反馈给LLM（流式）
			finalResponse, err := a.generateFinalResponseStream(ctx, msgs, response, toolCall, result, callback)
			if err != nil {
				return output, err
			}

			output.Response = finalResponse
		}
	}

	return output, nil
}

// buildPrompt 构建完整的prompt
func (a *ConversationalAgent) buildPrompt(input Input) string {
	var sb strings.Builder

	sb.WriteString("请基于以下信息回应用户：\n\n")

	if input.Conversation != "" {
		sb.WriteString("【短期对话窗口】（用于保持上下文）\n")
		sb.WriteString(input.Conversation)
		sb.WriteString("\n\n")
	}

	if input.Memory != "" {
		sb.WriteString(input.Memory)
		sb.WriteString("\n\n")
	}

	sb.WriteString("【用户输入】\n")
	sb.WriteString(input.Message)
	sb.WriteString("\n\n")

	sb.WriteString("- 给轻量建议/小选择题（不长篇科普),如果需选择以~#开头，格式：~#1、选择(20字以内)\n~#2、选择\n")

	return sb.String()
}

// generateResponse 生成响应（支持流式）
func (a *ConversationalAgent) generateResponse(ctx context.Context, msgs []models.ChatMessage) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(a.timeout)*time.Second)
	defer cancel()

	// 尝试使用流式响应
	if ollamaClient, ok := a.llmClient.(*models.Client); ok {
		fmt.Print("\nAI：")
		tokenCh, errCh := ollamaClient.ChatStream(callCtx, msgs)

		var fullResponse strings.Builder
		for token := range tokenCh {
			fmt.Print(token)
			fullResponse.WriteString(token)
		}

		if err := <-errCh; err != nil {
			return "", err
		}

		fmt.Println()
		return fullResponse.String(), nil
	}

	// 非流式响应
	response, err := a.llmClient.Chat(callCtx, msgs)
	if err != nil {
		return "", err
	}

	fmt.Println("\nAI：", response)
	return response, nil
}

// generateResponseStream 流式生成响应
func (a *ConversationalAgent) generateResponseStream(ctx context.Context, msgs []models.ChatMessage, callback func(string) error) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(a.timeout)*time.Second)
	defer cancel()

	// 尝试使用流式响应
	if ollamaClient, ok := a.llmClient.(*models.Client); ok {
		tokenCh, errCh := ollamaClient.ChatStream(callCtx, msgs)

		var fullResponse strings.Builder
		for token := range tokenCh {
			fullResponse.WriteString(token)
			if err := callback(token); err != nil {
				return "", err
			}
		}

		if err := <-errCh; err != nil {
			return "", err
		}

		return fullResponse.String(), nil
	}

	// 非流式响应（降级）
	response, err := a.llmClient.Chat(callCtx, msgs)
	if err != nil {
		return "", err
	}

	if err := callback(response); err != nil {
		return "", err
	}

	return response, nil
}

// generateFinalResponse 基于工具结果生成最终响应
func (a *ConversationalAgent) generateFinalResponse(
	ctx context.Context,
	msgs []models.ChatMessage,
	initialResponse string,
	toolCall *toolCallInfo,
	toolResult string,
) (string, error) {
	feedbackPrompt := fmt.Sprintf(`工具执行结果：
工具: %s
输入: %s
输出: %s

请基于这个结果，用自然的语言回复用户。不要重复工具调用格式，直接给出友好的回答。`,
		toolCall.ToolName, toolCall.Arguments, toolResult)

	msgs = append(msgs,
		models.ChatMessage{Role: "assistant", Content: initialResponse},
		models.ChatMessage{Role: "user", Content: feedbackPrompt},
	)

	return a.generateResponse(ctx, msgs)
}

// generateFinalResponseStream 基于工具结果流式生成最终响应
func (a *ConversationalAgent) generateFinalResponseStream(
	ctx context.Context,
	msgs []models.ChatMessage,
	initialResponse string,
	toolCall *toolCallInfo,
	toolResult string,
	callback func(string) error,
) (string, error) {
	feedbackPrompt := fmt.Sprintf(`工具执行结果：
工具: %s
输入: %s
输出: %s

请基于这个结果，用自然的语言回复用户。不要重复工具调用格式，直接给出友好的回答。`,
		toolCall.ToolName, toolCall.Arguments, toolResult)

	msgs = append(msgs,
		models.ChatMessage{Role: "assistant", Content: initialResponse},
		models.ChatMessage{Role: "user", Content: feedbackPrompt},
	)

	return a.generateResponseStream(ctx, msgs, callback)
}

// executeTool 执行工具
func (a *ConversationalAgent) executeTool(ctx context.Context, toolCall *toolCallInfo) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	switch toolCall.ToolName {
	case "calculator":
		calc := &tools.CalculatorTool{}
		result, err := calc.Execute(callCtx, toolCall.Arguments)
		if err != nil {
			return fmt.Sprintf("计算错误: %v", err), err
		}
		return result, nil
	case "speak":
		speakTool := &tools.SpeakTool{}
		result, err := speakTool.Execute(callCtx, toolCall.Arguments)
		if err != nil {
			return fmt.Sprintf("语音播放错误: %v", err), err
		}
		return result, nil
	default:
		return fmt.Sprintf("未知工具: %s", toolCall.ToolName), fmt.Errorf("unknown tool: %s", toolCall.ToolName)
	}
}

// toolCallInfo 工具调用信息
type toolCallInfo struct {
	ToolName  string
	Arguments string
}

// extractToolCall 从响应中提取工具调用
func extractToolCall(response string) *toolCallInfo {
	idx := strings.Index(response, "TOOL_CALL:")
	if idx == -1 {
		return nil
	}

	callPart := strings.TrimSpace(response[idx+len("TOOL_CALL:"):])

	openParen := strings.Index(callPart, "(")
	if openParen == -1 {
		return nil
	}

	toolName := strings.TrimSpace(callPart[:openParen])

	closeParen := strings.LastIndex(callPart, ")")
	if closeParen == -1 || closeParen <= openParen {
		return nil
	}

	args := callPart[openParen+1 : closeParen]
	args = strings.Trim(args, `"'`)

	return &toolCallInfo{
		ToolName:  toolName,
		Arguments: args,
	}
}
