package rag

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"agent-langchain/internal/models"
)

// 重试函数，用于处理网络请求错误
func (s *QdrantStore) doWithRetry(ctx context.Context, reqFn func() (*http.Request, error), maxRetries int) (*http.Response, error) {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		// 每次重试时创建新的请求
		req, err := reqFn()
		if err != nil {
			lastErr = err
			if i < maxRetries {
				time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			}
			continue
		}
		// 如果配置了 API Key，添加认证头
		if s.APIKey != "" {
			req.Header.Set("api-key", s.APIKey)
		}
		resp, err := s.HTTP.Do(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		// 只有在还有重试次数时才等待
		if i < maxRetries {
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
		}
	}
	return nil, lastErr
}

type Doc struct {
	PageContent string
	Score       float64
}

type QdrantStore struct {
	BaseURL            string
	APIKey             string // Qdrant API Key（可选）
	Collection         string // 默认为 memories，用于存储所有数据
	Embedder           func(context.Context, string) ([]float32, error)
	HTTP               *http.Client
}

func NewQdrantStore(qdrantURL, apiKey, collection string, embedder func(context.Context, string) ([]float32, error)) *QdrantStore {
	return &QdrantStore{
		BaseURL:            qdrantURL,
		APIKey:             apiKey,
		Collection:         collection,
		Embedder:           embedder,
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// EnsureCollection：你第一次启动时会自动创建 collection（向量维度由第一次 embedding 决定）
func (s *QdrantStore) EnsureCollection(ctx context.Context, dim int) error {
	// 确保集合存在
	return s.ensureCollection(ctx, s.Collection, dim)
}

func (s *QdrantStore) ensureCollection(ctx context.Context, collection string, dim int) error {
	// 检查集合是否存在
	resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, "GET", s.BaseURL+"/collections/"+collection, nil)
	}, 3) // 最多重试3次
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil // 集合已存在
	}

	// 创建集合
	body := map[string]any{
		"vectors": map[string]any{
			"size":     dim,
			"distance": "Cosine",
		},
	}
	b, _ := json.Marshal(body)
	resp2, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "PUT", s.BaseURL+"/collections/"+collection, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, 3) // 最多重试3次
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode >= 300 {
		return fmt.Errorf("create collection http %d", resp2.StatusCode)
	}
	return nil
}

// SimilaritySearch 搜索记忆（需要 user_id 和 agent_id 过滤）
func (s *QdrantStore) SimilaritySearch(ctx context.Context, userID string, agentID uint, query string, topK int) ([]Doc, error) {
	return s.SimilaritySearchMemory(ctx, userID, agentID, query, topK)
}

// SimilaritySearchMemory 搜索记忆（需要 user_id 和 agent_id 过滤）
func (s *QdrantStore) SimilaritySearchMemory(ctx context.Context, userID string, agentID uint, query string, topK int) ([]Doc, error) {
	vec, err := s.Embedder(ctx, query)
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"vector":       vec,
		"limit":        topK,
		"with_payload": true,
		"with_vectors": false,
		"filter": map[string]any{
			"must": []any{
				map[string]any{
					"key":   "user_id",
					"match": map[string]any{"value": userID},
				},
				map[string]any{
					"key":   "agent_id",
					"match": map[string]any{"value": agentID},
				},
				map[string]any{
					"key":   "type",
					"match": map[string]any{"value": "memory"},
				},
			},
		},
	}
	b, _ := json.Marshal(body)
	url := s.BaseURL + "/collections/" + s.Collection + "/points/search"
	resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant search http %d", resp.StatusCode)
	}

	var out struct {
		Result []struct {
			Score   float64        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	docs := make([]Doc, 0, len(out.Result))
	for _, r := range out.Result {
		text, _ := r.Payload["text"].(string)
		if text == "" {
			continue
		}
		docs = append(docs, Doc{PageContent: text, Score: r.Score})
	}
	return docs, nil
}

// SimilaritySearchKnowledge 搜索知识库（只需要 agent_id 过滤，所有用户可访问）
func (s *QdrantStore) SimilaritySearchKnowledge(ctx context.Context, agentID uint, query string, topK int) ([]Doc, error) {
	vec, err := s.Embedder(ctx, query)
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"vector":       vec,
		"limit":        topK,
		"with_payload": true,
		"with_vectors": false,
		"filter": map[string]any{
			"must": []any{
				map[string]any{
					"key":   "agent_id",
					"match": map[string]any{"value": agentID},
				},
				map[string]any{
					"key":   "type",
					"match": map[string]any{"value": "knowledge"},
				},
			},
		},
	}
	b, _ := json.Marshal(body)
	url := s.BaseURL + "/collections/" + s.Collection + "/points/search"
	resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant search http %d", resp.StatusCode)
	}

	var out struct {
		Result []struct {
			Score   float64        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	docs := make([]Doc, 0, len(out.Result))
	for _, r := range out.Result {
		text, _ := r.Payload["text"].(string)
		if text == "" {
			// 尝试使用 content 字段
			text, _ = r.Payload["content"].(string)
		}
		if text == "" {
			continue
		}
		docs = append(docs, Doc{PageContent: text, Score: r.Score})
	}
	return docs, nil
}

// UpsertMemoryTexts 存储记忆文本（需要 user_id 和 agent_id）
func (s *QdrantStore) UpsertMemoryTexts(ctx context.Context, userID string, agentID uint, texts []string) error {
	for _, t := range texts {
		if t == "" {
			continue
		}
		vec, err := s.Embedder(ctx, t)
		if err != nil {
			return err
		}

		pointID := randomID()

		payload := map[string]any{
			"user_id":   userID,
			"agent_id":  agentID,
			"text":      t,
			"timestamp": time.Now().Unix(),
			"type":      "memory",
		}

		body := map[string]any{
			"points": []any{
				map[string]any{
					"id":      pointID,
					"vector":  vec,
					"payload": payload,
				},
			},
		}

		if err := s.UpsertPointToCollection(ctx, body, s.Collection); err != nil {
			return err
		}
	}
	return nil
}

// UpsertKnowledgeTexts 存储知识库文本（只需要 agent_id，不需要 user_id）
func (s *QdrantStore) UpsertKnowledgeTexts(ctx context.Context, agentID uint, texts []string, fileName string) error {
	for _, t := range texts {
		if t == "" {
			continue
		}
		vec, err := s.Embedder(ctx, t)
		if err != nil {
			return err
		}

		pointID := randomID()

		payload := map[string]any{
			"agent_id":  agentID,
			"content":   t,
			"file_name": fileName,
			"timestamp": time.Now().Unix(),
			"type":      "knowledge",
		}

		body := map[string]any{
			"points": []any{
				map[string]any{
					"id":      pointID,
					"vector":  vec,
					"payload": payload,
				},
			},
		}

		if err := s.UpsertPointToCollection(ctx, body, s.Collection); err != nil {
			return err
		}
	}
	return nil
}

// UpsertTexts 存储知识库文本（兼容旧接口，但不再使用 user_id）
// 已废弃：请使用 UpsertKnowledgeTexts 或 UpsertMemoryTexts
func (s *QdrantStore) UpsertTexts(ctx context.Context, userID string, agentID uint, texts []string, fileName string) error {
	// 为了向后兼容，默认作为知识库处理（不使用 user_id）
	return s.UpsertKnowledgeTexts(ctx, agentID, texts, fileName)
}

// UpsertPoint 向 Qdrant 存储单个点，支持自定义元数据
func (s *QdrantStore) UpsertPoint(ctx context.Context, body map[string]any) error {
	return s.UpsertPointToCollection(ctx, body, s.Collection)
}

func (s *QdrantStore) UpsertPointToCollection(ctx context.Context, body map[string]any, collection string) error {
	resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		url := fmt.Sprintf("%s/collections/%s/points", s.BaseURL, collection)
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		if s.APIKey != "" {
			req.Header.Set("api-key", s.APIKey)
		}

		return req, nil
	}, 3)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upsert point failed with status code: %d", resp.StatusCode)
	}

	return nil
}

func randomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// ListFiles 获取知识库文件列表（只需要 agent_id）
func (s *QdrantStore) ListFiles(ctx context.Context, agentID uint) ([]string, error) {
	// 使用 Qdrant 的 scroll API 来获取所有匹配的点
	body := map[string]any{
		"filter": map[string]any{
			"must": []any{
				map[string]any{
					"key":   "agent_id",
					"match": map[string]any{"value": agentID},
				},
				map[string]any{
					"key":   "type",
					"match": map[string]any{"value": "knowledge"},
				},
			},
		},
		"limit":        100,
		"with_payload": true,
	}
	b, _ := json.Marshal(body)
	url := s.BaseURL + "/collections/" + s.Collection + "/points/scroll"
	resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("scroll points http %d", resp.StatusCode)
	}

	var out struct {
		Result struct {
			Points []struct {
				Payload map[string]any `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	// 提取唯一的文件路径
	fileMap := make(map[string]bool)
	for _, r := range out.Result.Points {
		if source, ok := r.Payload["source"].(string); ok && source != "" {
			fileMap[source] = true
		}
	}

	// 将 map 转换为 slice
	files := make([]string, 0, len(fileMap))
	for file := range fileMap {
		files = append(files, file)
	}

	return files, nil
}

// 方便：从 models.Client 直接构造 store
func NewStoreFromOllama(qdrantURL, apiKey, collection string, ollama *models.Client) *QdrantStore {
	return NewQdrantStore(qdrantURL, apiKey, collection, ollama.Embed)
}

// DeleteKnowledgeByFilter 根据过滤条件删除知识库向量点（只需要 agent_id）
func (s *QdrantStore) DeleteKnowledgeByFilter(ctx context.Context, agentID uint) error {
	body := map[string]any{
		"filter": map[string]any{
			"must": []any{
				map[string]any{
					"key":   "agent_id",
					"match": map[string]any{"value": agentID},
				},
				map[string]any{
					"key":   "type",
					"match": map[string]any{"value": "knowledge"},
				},
			},
		},
	}

	b, _ := json.Marshal(body)
	url := s.BaseURL + "/collections/" + s.Collection + "/points/delete"
	resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, 3)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("delete points http %d", resp.StatusCode)
	}
	return nil
}

// DeleteMemoryByFilter 根据过滤条件删除记忆向量点（需要 user_id 和 agent_id）
func (s *QdrantStore) DeleteMemoryByFilter(ctx context.Context, userID string, agentID uint) error {
	body := map[string]any{
		"filter": map[string]any{
			"must": []any{
				map[string]any{
					"key":   "user_id",
					"match": map[string]any{"value": userID},
				},
				map[string]any{
					"key":   "agent_id",
					"match": map[string]any{"value": agentID},
				},
				map[string]any{
					"key":   "type",
					"match": map[string]any{"value": "memory"},
				},
			},
		},
	}

	b, _ := json.Marshal(body)
	url := s.BaseURL + "/collections/" + s.Collection + "/points/delete"
	resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, 3)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("delete points http %d", resp.StatusCode)
	}
	return nil
}

// DeletePointsByFilter 根据过滤条件删除向量点（兼容旧接口）
// 已废弃：请使用 DeleteKnowledgeByFilter 或 DeleteMemoryByFilter
func (s *QdrantStore) DeletePointsByFilter(ctx context.Context, userID string, agentID uint) error {
	// 为了向后兼容，删除该用户的所有数据（包括记忆和知识）
	// 但实际上知识库不应该按用户删除
	return s.DeleteMemoryByFilter(ctx, userID, agentID)
}
