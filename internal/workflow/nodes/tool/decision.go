package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"agent-langchain/internal/models"
	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// DecisionNode Tool.Decide 节点 — 使用 LLM 决定是否需要工具
// 输入: text (用户输入)
// 输出: tool_call, need_tool, tool_name, tool_args
type DecisionNode struct{}

type toolDecision struct {
	NeedTool bool   `json:"need_tool"`
	ToolName string `json:"tool_name"`
	ToolArgs string `json:"tool_args"`
}

func (n *DecisionNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	text, _ := inputs["text"].(string)
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("text input is required")
	}

	model, _ := params["model"].(string)
	provider, _ := params["provider"].(string)
	baseURL, _ := params["base_url"].(string)
	apiKey, _ := params["api_key"].(string)
	temperature, _ := params["temperature"].(float64)
	maxRetries := getIntParam(params, "max_retries", 1)
	prompt, _ := params["prompt"].(string)
	toolsList, _ := params["tools"]

	if prompt == "" {
		toolHint := ""
		if toolsList != nil {
			if toolBytes, err := json.Marshal(toolsList); err == nil {
				toolHint = string(toolBytes)
			}
		}
		if toolHint != "" {
			prompt = fmt.Sprintf("Decide if a tool is needed. Available tools: %s. Respond ONLY in JSON: {\"need_tool\":true|false,\"tool_name\":\"\",\"tool_args\":\"\"}.", toolHint)
		} else {
			prompt = "Decide if a tool is needed. Respond ONLY in JSON: {\"need_tool\":true|false,\"tool_name\":\"\",\"tool_args\":\"\"}."
		}
	}

	resp, err := callLLM(ctx, rc, provider, baseURL, apiKey, model, temperature, maxRetries, prompt+"\n\nUser: "+text)
	if err != nil {
		return nil, err
	}

	var decision toolDecision
	if err := json.Unmarshal([]byte(resp), &decision); err != nil {
		return nil, fmt.Errorf("failed to parse decision JSON: %w", err)
	}

	toolCall := map[string]any{
		"need_tool": decision.NeedTool,
		"tool_name": decision.ToolName,
		"tool_args": decision.ToolArgs,
	}

	needText := "no"
	if decision.NeedTool {
		needText = "yes"
	}

	return map[string]any{
		"tool_call": toolCall,
		"need_tool": needText,
		"tool_name": decision.ToolName,
		"tool_args": decision.ToolArgs,
	}, nil
}

func (n *DecisionNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Tool.Decide",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "tool_call", Type: types.PortTypeToolCall, Required: true},
			{Name: "need_tool", Type: types.PortTypeText, Required: true},
			{Name: "tool_name", Type: types.PortTypeText, Required: true},
			{Name: "tool_args", Type: types.PortTypeText, Required: true},
		},
		Runner: n,
	}
}

// ExecuteNode Tool.Execute 节点 — 执行工具
// 输入: tool_call
// 输出: tool_result, text
type ExecuteNode struct{}

func (n *ExecuteNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	raw := inputs["tool_call"]
	if raw == nil {
		return nil, fmt.Errorf("tool_call input is required")
	}

	toolName, toolArgs, err := parseToolCall(raw)
	if err != nil {
		return nil, err
	}

	var result string
	switch strings.ToLower(toolName) {
	case "calculator", "calc":
		val, err := evaluateExpression(toolArgs)
		if err != nil {
			return nil, fmt.Errorf("calculator error: %w", err)
		}
		result = fmt.Sprintf("%.6f", val)
	case "time", "time.now":
		format := "2006-01-02 15:04:05"
		if toolArgs != "" {
			format = toolArgs
		}
		if f, ok := params["format"].(string); ok && f != "" {
			format = f
		}
		result = time.Now().Format(format)
	default:
		return nil, fmt.Errorf("unsupported tool: %s", toolName)
	}

	return map[string]any{
		"tool_result": result,
		"text":        result,
	}, nil
}

func (n *ExecuteNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Tool.Execute",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "tool_call", Type: types.PortTypeToolCall, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "tool_result", Type: types.PortTypeToolResult, Required: true},
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Runner: n,
	}
}

// ValidateNode Tool.Validate 节点
// 输入: tool_result
// 输出: valid(flow), tool_result
type ValidateNode struct{}

func (n *ValidateNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	result, _ := inputs["tool_result"].(string)
	if strings.TrimSpace(result) == "" {
		return map[string]any{"tool_result": result}, nil
	}
	return map[string]any{
		"valid":       true,
		"tool_result": result,
	}, nil
}

func (n *ValidateNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Tool.Validate",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "tool_result", Type: types.PortTypeToolResult, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "valid", Type: types.PortTypeFlow, Required: false},
			{Name: "tool_result", Type: types.PortTypeToolResult, Required: false},
		},
		Runner: n,
	}
}

// SufficientNode Tool.Sufficient 节点 — 判断结果是否足够
// 输入: user_text, tool_result
// 输出: enough (flow), not_enough (flow), decision (text)
type SufficientNode struct{}

func (n *SufficientNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	toolResult, _ := inputs["tool_result"].(string)
	userText, _ := inputs["user_text"].(string)
	if strings.TrimSpace(toolResult) == "" {
		return nil, fmt.Errorf("tool_result input is required")
	}

	maxAttempts := 0
	if v, ok := params["max_attempts"]; ok {
		switch t := v.(type) {
		case float64:
			maxAttempts = int(t)
		case string:
			if parsed, err := strconv.Atoi(t); err == nil {
				maxAttempts = parsed
			}
		}
	}

	key, _ := params["key"].(string)
	if key == "" {
		key = "default"
	}
	cacheKey := "tool.sufficient." + key

	if rc.Cache == nil {
		rc.Cache = make(map[string]any)
	}
	current := 0
	if v, ok := rc.Cache[cacheKey]; ok {
		if iv, ok := v.(int); ok {
			current = iv
		} else if fv, ok := v.(float64); ok {
			current = int(fv)
		}
	}
	current++
	rc.Cache[cacheKey] = current

	if maxAttempts > 0 && current >= maxAttempts {
		return map[string]any{
			"enough":   true,
			"decision": "enough",
		}, nil
	}

	model, _ := params["model"].(string)
	provider, _ := params["provider"].(string)
	baseURL, _ := params["base_url"].(string)
	apiKey, _ := params["api_key"].(string)
	temperature, _ := params["temperature"].(float64)
	maxRetries := getIntParam(params, "max_retries", 1)
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		prompt = "Given the user question and tool result, is the result sufficient to answer? Reply YES or NO only."
	}
	resp, err := callLLM(ctx, rc, provider, baseURL, apiKey, model, temperature, maxRetries, fmt.Sprintf("%s\n\nUser: %s\nToolResult: %s", prompt, userText, toolResult))
	if err != nil {
		return nil, err
	}

	resp = strings.TrimSpace(strings.ToUpper(resp))
	if strings.Contains(resp, "YES") {
		return map[string]any{
			"enough":   true,
			"decision": "enough",
		}, nil
	}
	return map[string]any{
		"not_enough": true,
		"decision":   "not_enough",
	}, nil
}

func (n *SufficientNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Tool.Sufficient",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "user_text", Type: types.PortTypeText, Required: false},
			{Name: "tool_result", Type: types.PortTypeToolResult, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "enough", Type: types.PortTypeFlow, Required: false},
			{Name: "not_enough", Type: types.PortTypeFlow, Required: false},
			{Name: "decision", Type: types.PortTypeText, Required: false},
		},
		Runner: n,
	}
}

func parseToolCall(raw any) (string, string, error) {
	switch v := raw.(type) {
	case map[string]any:
		name, _ := v["tool_name"].(string)
		args, _ := v["tool_args"].(string)
		if name == "" {
			return "", "", fmt.Errorf("tool_name is required in tool_call")
		}
		return name, args, nil
	case string:
		var decoded map[string]any
		if err := json.Unmarshal([]byte(v), &decoded); err != nil {
			return "", "", fmt.Errorf("invalid tool_call string: %w", err)
		}
		name, _ := decoded["tool_name"].(string)
		args, _ := decoded["tool_args"].(string)
		if name == "" {
			return "", "", fmt.Errorf("tool_name is required in tool_call")
		}
		return name, args, nil
	default:
		return "", "", fmt.Errorf("unsupported tool_call type: %T", raw)
	}
}

func callLLM(
	ctx context.Context,
	rc *registry.RunContext,
	provider string,
	baseURL string,
	apiKey string,
	model string,
	temperature float64,
	maxRetries int,
	content string,
) (string, error) {
	if provider == "" {
		if rc.LLMClient == nil {
			return "", fmt.Errorf("LLMClient is required")
		}
		messages := []any{
			map[string]any{"role": "user", "content": content},
		}
		resp, err := rc.LLMClient.Chat(ctx, messages, model)
		if err != nil {
			return "", fmt.Errorf("LLM chat failed: %w", err)
		}
		return resp, nil
	}

	client, err := buildLLMClient(provider, baseURL, apiKey, model, temperature)
	if err != nil {
		return "", err
	}

	msgs := []models.ChatMessage{
		{Role: "user", Content: content},
	}

	resp, err := callLLMWithRetry(ctx, client, msgs, model, maxRetries)
	if err != nil {
		return "", err
	}
	return resp, nil
}

func buildLLMClient(provider, baseURL, apiKey, model string, temperature float64) (models.LLMClient, error) {
	switch strings.ToLower(provider) {
	case "ollama":
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		if model == "" {
			model = "Gemma3UThink:4b"
		}
		client := models.New(baseURL, model, "")
		if temperature > 0 {
			client.Temperature = temperature
		}
		return client, nil
	case "deepseek":
		if baseURL == "" {
			baseURL = "https://api.deepseek.com/v1"
		}
		if model == "" {
			model = "deepseek-chat"
		}
		if apiKey == "" {
			return nil, fmt.Errorf("api_key is required for DeepSeek")
		}
		return models.NewDeepSeek(baseURL, apiKey, model), nil
	case "anthropic":
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		if model == "" {
			model = "claude-3-sonnet-20240229"
		}
		if apiKey == "" {
			return nil, fmt.Errorf("api_key is required for Anthropic")
		}
		return models.NewAnthropic(baseURL, model, ""), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func callLLMWithRetry(ctx context.Context, client models.LLMClient, msgs []models.ChatMessage, model string, maxRetries int) (string, error) {
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var response string
	var err error

	for i := 0; i < maxRetries; i++ {
		response, err = client.Chat(ctx, msgs, model)
		if err == nil {
			return response, nil
		}
		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Second * time.Duration(i+1)):
			}
		}
	}

	return "", fmt.Errorf("LLM call failed after %d retries: %w", maxRetries, err)
}

func getIntParam(params map[string]any, key string, defaultValue int) int {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			var i int
			fmt.Sscanf(v, "%d", &i)
			return i
		}
	}
	return defaultValue
}

func evaluateExpression(expr string) (float64, error) {
	expr = strings.ReplaceAll(expr, " ", "")

	if strings.HasPrefix(expr, "sqrt(") && strings.HasSuffix(expr, ")") {
		inner := expr[5 : len(expr)-1]
		val, err := evaluateExpression(inner)
		if err != nil {
			return 0, err
		}
		if val < 0 {
			return 0, fmt.Errorf("cannot take square root of negative number")
		}
		return math.Sqrt(val), nil
	}

	if strings.HasPrefix(expr, "abs(") && strings.HasSuffix(expr, ")") {
		inner := expr[4 : len(expr)-1]
		val, err := evaluateExpression(inner)
		if err != nil {
			return 0, err
		}
		return math.Abs(val), nil
	}

	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		depth := 0
		for i, ch := range expr {
			if ch == '(' {
				depth++
			} else if ch == ')' {
				depth--
			}
			if depth == 0 && i < len(expr)-1 {
				break
			}
		}
		if depth == 0 {
			return evaluateExpression(expr[1 : len(expr)-1])
		}
	}

	depth := 0
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == ')' {
			depth++
		} else if expr[i] == '(' {
			depth--
		}
		if depth == 0 && expr[i] == '^' {
			left := expr[:i]
			right := expr[i+1:]
			leftVal, err := evaluateExpression(left)
			if err != nil {
				return 0, err
			}
			rightVal, err := evaluateExpression(right)
			if err != nil {
				return 0, err
			}
			return math.Pow(leftVal, rightVal), nil
		}
	}

	depth = 0
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == ')' {
			depth++
		} else if expr[i] == '(' {
			depth--
		}
		if depth == 0 && i > 0 {
			if expr[i] == '+' {
				left := expr[:i]
				right := expr[i+1:]
				leftVal, err := evaluateExpression(left)
				if err != nil {
					return 0, err
				}
				rightVal, err := evaluateExpression(right)
				if err != nil {
					return 0, err
				}
				return leftVal + rightVal, nil
			}
			if expr[i] == '-' {
				left := expr[:i]
				right := expr[i+1:]
				leftVal, err := evaluateExpression(left)
				if err != nil {
					return 0, err
				}
				rightVal, err := evaluateExpression(right)
				if err != nil {
					return 0, err
				}
				return leftVal - rightVal, nil
			}
		}
	}

	depth = 0
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == ')' {
			depth++
		} else if expr[i] == '(' {
			depth--
		}
		if depth == 0 {
			if expr[i] == '*' {
				left := expr[:i]
				right := expr[i+1:]
				leftVal, err := evaluateExpression(left)
				if err != nil {
					return 0, err
				}
				rightVal, err := evaluateExpression(right)
				if err != nil {
					return 0, err
				}
				return leftVal * rightVal, nil
			}
			if expr[i] == '/' {
				left := expr[:i]
				right := expr[i+1:]
				leftVal, err := evaluateExpression(left)
				if err != nil {
					return 0, err
				}
				rightVal, err := evaluateExpression(right)
				if err != nil {
					return 0, err
				}
				if rightVal == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				return leftVal / rightVal, nil
			}
		}
	}

	val, err := strconv.ParseFloat(expr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid expression: %s", expr)
	}

	return val, nil
}
