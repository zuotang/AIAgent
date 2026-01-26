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
	BaseURL    string
	Collection string
	Embedder   func(context.Context, string) ([]float32, error)
	HTTP       *http.Client
}

func NewQdrantStore(qdrantURL, collection string, embedder func(context.Context, string) ([]float32, error)) *QdrantStore {
	return &QdrantStore{
		BaseURL:    qdrantURL,
		Collection: collection,
		Embedder:   embedder,
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// EnsureCollection：你第一次启动时会自动创建 collection（向量维度由第一次 embedding 决定）
func (s *QdrantStore) EnsureCollection(ctx context.Context, dim int) error {
	// GET collection 判断是否存在
	// 检查集合是否存在
	resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, "GET", s.BaseURL+"/collections/"+s.Collection, nil)
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
		req, err := http.NewRequestWithContext(ctx, "PUT", s.BaseURL+"/collections/"+s.Collection, bytes.NewReader(b))
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

func (s *QdrantStore) SimilaritySearch(ctx context.Context, userID, query string, topK int) ([]Doc, error) {
	vec, err := s.Embedder(ctx, query)
	if err != nil {
		return nil, err
	}

	// Qdrant search payload filter by user_id
	body := map[string]any{
		"vector":       vec,
		"limit":        topK,
		"with_payload": true,
		"filter": map[string]any{
			"must": []any{
				map[string]any{
					"key":   "user_id",
					"match": map[string]any{"value": userID},
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
	}, 3) // 最多重试3次
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

func (s *QdrantStore) UpsertTexts(ctx context.Context, userID string, texts []string) error {
	for _, t := range texts {
		if t == "" {
			continue
		}
		vec, err := s.Embedder(ctx, t)
		if err != nil {
			return err
		}

		pointID := randomID()

		body := map[string]any{
			"points": []any{
				map[string]any{
					"id":     pointID,
					"vector": vec,
					"payload": map[string]any{
						"user_id": userID,
						"text":    t,
						"ts":      time.Now().Format(time.RFC3339),
					},
				},
			},
		}

		b, _ := json.Marshal(body)
		url := s.BaseURL + "/collections/" + s.Collection + "/points?wait=true"
		resp, err := s.doWithRetry(ctx, func() (*http.Request, error) {
			req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(b))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/json")
			return req, nil
		}, 3) // 最多重试3次
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("qdrant upsert http %d", resp.StatusCode)
		}
	}
	return nil
}

func randomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// 方便：从 models.Client 直接构造 store
func NewStoreFromOllama(qdrantURL, collection string, ollama *models.Client) *QdrantStore {
	return NewQdrantStore(qdrantURL, collection, ollama.Embed)
}
