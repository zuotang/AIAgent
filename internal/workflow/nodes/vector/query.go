package vector

import (
	"context"
	"fmt"
	"strings"

	"agent-langchain/internal/workflow/registry"
	"agent-langchain/internal/workflow/types"
)

// QueryNode Vector.Query 节点 — 向量相似度检索
type QueryNode struct{}

// Run 执行节点
func (n *QueryNode) Run(ctx context.Context, rc *registry.RunContext, inputs map[string]any, params map[string]any) (map[string]any, error) {
	if rc.QdrantClient == nil {
		return nil, fmt.Errorf("QdrantClient not configured")
	}

	query, ok := inputs["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query input is required")
	}

	// 读取参数
	collection := "knowledge"
	if c, ok := params["collection"].(string); ok && c != "" {
		collection = c
	}

	topK := 3
	if k, ok := params["top_k"].(float64); ok {
		topK = int(k)
	}

	minScore := 0.3
	if s, ok := params["min_score"].(float64); ok {
		minScore = s
	}

	userID := "default"
	if u, ok := params["user_id"].(string); ok && u != "" {
		userID = u
	}

	agentID := uint(1)
	if a, ok := params["agent_id"].(float64); ok {
		agentID = uint(a)
	}

	// 根据 collection 选择检索方法
	var docs []registry.VectorDoc
	var err error

	switch collection {
	case "knowledge":
		docs, err = rc.QdrantClient.SimilaritySearchKnowledge(ctx, agentID, query, topK)
	case "memory":
		docs, err = rc.QdrantClient.SimilaritySearchMemory(ctx, userID, agentID, query, topK)
	default:
		return nil, fmt.Errorf("unknown collection: %s (use 'knowledge' or 'memory')", collection)
	}

	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// 按 min_score 过滤
	filtered := make([]registry.VectorDoc, 0, len(docs))
	for _, doc := range docs {
		if doc.Score >= minScore {
			filtered = append(filtered, doc)
		}
	}

	// 拼接文本结果
	var sb strings.Builder
	for i, doc := range filtered {
		if i > 0 {
			sb.WriteString("\n---\n")
		}
		sb.WriteString(doc.PageContent)
	}

	// 构建 docs JSON 输出
	docsJSON := make([]map[string]any, len(filtered))
	for i, doc := range filtered {
		docsJSON[i] = map[string]any{
			"page_content": doc.PageContent,
			"score":        doc.Score,
		}
	}

	return map[string]any{
		"results": sb.String(),
		"docs":    docsJSON,
	}, nil
}

// Spec 返回节点规范
func (n *QueryNode) Spec() *registry.NodeSpec {
	return &registry.NodeSpec{
		Type:    "Vector.Query",
		Version: "1.0",
		Inputs: []types.PortSpec{
			{Name: "query", Type: types.PortTypeText, Required: true},
		},
		Outputs: []types.PortSpec{
			{Name: "results", Type: types.PortTypeText, Required: true},
			{Name: "docs", Type: types.PortTypeJSON, Required: false},
		},
		Runner: n,
	}
}
