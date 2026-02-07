package kb

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"agent-langchain/internal/models"
	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// QueryRewriteNode KB.QueryRewrite 节点
// 输入: text
// 输出: query (text)
type QueryRewriteNode struct{}

func (n *QueryRewriteNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	text, _ := inputs["text"].(string)
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("text input is required")
	}

	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		prompt = "Rewrite the user query into a concise search query."
	}

	model, _ := params["model"].(string)
	provider, _ := params["provider"].(string)
	baseURL, _ := params["base_url"].(string)
	apiKey, _ := params["api_key"].(string)
	temperature, _ := params["temperature"].(float64)
	maxRetries := getIntParam(params, "max_retries", 1)

	content := prompt + "\n\nUser: " + text
	resp, err := callLLM(ctx, rc, provider, baseURL, apiKey, model, temperature, maxRetries, content)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"query": strings.TrimSpace(resp),
	}, nil
}

func (n *QueryRewriteNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "KB.QueryRewrite",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "text", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "query", Type: types.PortTypeText, Required: true},
		},
		Runner: n,
	}
}

// SearchNode KB.Search 节点
// 输入: query (text)
// 输出: kb_docs
type SearchNode struct{}

func (n *SearchNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	if rc.QdrantClient == nil {
		return nil, fmt.Errorf("QdrantClient is required")
	}

	query, _ := inputs["query"].(string)
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query input is required")
	}

	topK := getIntParam(params, "top_k", 3)
	minScore := getFloatParam(params, "min_score", 0.0)
	agentID := uint(getIntParam(params, "agent_id", 1))

	docs, err := rc.QdrantClient.SimilaritySearchKnowledge(ctx, agentID, query, topK)
	if err != nil {
		return nil, err
	}

	filtered := make([]registry.VectorDoc, 0, len(docs))
	for _, d := range docs {
		if d.Score >= minScore {
			filtered = append(filtered, d)
		}
	}

	return map[string]any{
		"kb_docs": filtered,
	}, nil
}

func (n *SearchNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "KB.Search",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "query", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "kb_docs", Type: types.PortTypeKBDocs, Required: true},
		},
		Runner: n,
	}
}

// RerankDedupNode KB.RerankDedup 节点
// 输入: kb_docs
// 输出: kb_docs
type RerankDedupNode struct{}

func (n *RerankDedupNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	docsRaw := inputs["kb_docs"]
	if docsRaw == nil {
		return nil, fmt.Errorf("kb_docs input is required")
	}

	docs, err := normalizeDocs(docsRaw)
	if err != nil {
		return nil, err
	}

	minScore := getFloatParam(params, "min_score", 0.0)
	maxDocs := getIntParam(params, "max_docs", 0)

	seen := make(map[string]bool)
	filtered := make([]registry.VectorDoc, 0, len(docs))
	for _, d := range docs {
		if d.Score < minScore {
			continue
		}
		key := strings.TrimSpace(d.PageContent)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		filtered = append(filtered, d)
		if maxDocs > 0 && len(filtered) >= maxDocs {
			break
		}
	}

	return map[string]any{
		"kb_docs": filtered,
	}, nil
}

func (n *RerankDedupNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "KB.RerankDedup",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "kb_docs", Type: types.PortTypeKBDocs, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "kb_docs", Type: types.PortTypeKBDocs, Required: true},
		},
		Runner: n,
	}
}

// EvidencePackNode KB.EvidencePack 节点
// 输入: kb_docs
// 输出: evidence (text), messages
type EvidencePackNode struct{}

func (n *EvidencePackNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	docsRaw := inputs["kb_docs"]
	if docsRaw == nil {
		return nil, fmt.Errorf("kb_docs input is required")
	}

	docs, err := normalizeDocs(docsRaw)
	if err != nil {
		return nil, err
	}

	maxDocs := getIntParam(params, "max_docs", 5)
	maxChars := getIntParam(params, "max_chars", 1200)

	if maxDocs > 0 && len(docs) > maxDocs {
		docs = docs[:maxDocs]
	}

	var sb strings.Builder
	sb.WriteString("Evidence:\n")
	for i, d := range docs {
		chunk := strings.TrimSpace(d.PageContent)
		if chunk == "" {
			continue
		}
		line := fmt.Sprintf("[%d] %s\n", i+1, chunk)
		if sb.Len()+len(line) > maxChars {
			break
		}
		sb.WriteString(line)
	}

	evidence := sb.String()
	messages := []any{
		map[string]any{"role": "system", "content": evidence},
	}

	return map[string]any{
		"evidence": evidence,
		"messages": messages,
	}, nil
}

func (n *EvidencePackNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "KB.EvidencePack",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "kb_docs", Type: types.PortTypeKBDocs, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "evidence", Type: types.PortTypeText, Required: true},
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
		},
		Runner: n,
	}
}

// InjectEvidenceNode Context.InjectEvidence 节点
// 输入: evidence (text)
// 输出: messages
type InjectEvidenceNode struct{}

func (n *InjectEvidenceNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	evidence, _ := inputs["evidence"].(string)
	if strings.TrimSpace(evidence) == "" {
		return nil, fmt.Errorf("evidence input is required")
	}

	prefix, _ := params["prefix"].(string)
	if prefix == "" {
		prefix = "Evidence:\n"
	}

	msg := prefix + evidence
	messages := []any{
		map[string]any{"role": "system", "content": msg},
	}

	return map[string]any{
		"messages": messages,
	}, nil
}

func (n *InjectEvidenceNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Context.InjectEvidence",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "in", Type: types.PortTypeFlow, Required: false},
			{Name: "evidence", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "messages", Type: types.PortTypeMessages, Required: true},
		},
		Runner: n,
	}
}

func normalizeDocs(raw any) ([]registry.VectorDoc, error) {
	switch v := raw.(type) {
	case []registry.VectorDoc:
		return v, nil
	case []any:
		docs := make([]registry.VectorDoc, 0, len(v))
		for _, item := range v {
			switch d := item.(type) {
			case registry.VectorDoc:
				docs = append(docs, d)
			case map[string]any:
				text, _ := d["page_content"].(string)
				score := getFloatFromAny(d["score"])
				docs = append(docs, registry.VectorDoc{PageContent: text, Score: score})
			default:
				return nil, fmt.Errorf("unsupported kb_docs item type: %T", item)
			}
		}
		return docs, nil
	default:
		return nil, fmt.Errorf("unsupported kb_docs type: %T", raw)
	}
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

func getFloatParam(params map[string]any, key string, defaultValue float64) float64 {
	if val, ok := params[key]; ok {
		return getFloatFromAny(val)
	}
	return defaultValue
}

func getFloatFromAny(val any) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
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
			model = "qwen3:4b"
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
			default:
			}
		}
	}

	return "", fmt.Errorf("LLM call failed after %d retries: %w", maxRetries, err)
}
