package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"agent-langchain/internal/utils"
)

// ChunkingStrategy 分块策略
type ChunkingStrategy string

const (
	// ChunkingStrategyTokens 基于Token的分块策略
	ChunkingStrategyTokens ChunkingStrategy = "tokens"
	// ChunkingStrategySemantic 基于语义的分块策略
	ChunkingStrategySemantic ChunkingStrategy = "semantic"
)

// Ingestor 表示知识库录入器
type Ingestor struct {
	store            *QdrantStore
	ChunkSize        int              // 分块大小（Token）
	ChunkOverlap     int              // 分块重叠（Token）
	ChunkingStrategy ChunkingStrategy // 分块策略
	UserID           string
	AgentID          uint   // Agent ID，用于隔离不同 agent 的知识
	MaxRetries       int
	Concurrency      int
	cache            *utils.Cache
}

// NewIngestor 创建一个新的知识库录入器
func NewIngestor(store *QdrantStore, chunkSize, chunkOverlap int, strategy ChunkingStrategy, userID string) *Ingestor {
	// 创建缓存目录
	cacheDir := filepath.Join(os.TempDir(), "agent-langchain", "cache")
	cache, err := utils.NewCache(cacheDir)
	if err != nil {
		fmt.Printf("警告: 创建缓存失败，将不使用缓存: %v\n", err)
	}

	// 默认策略
	if strategy == "" {
		strategy = ChunkingStrategyTokens
	}

	return &Ingestor{
		store:            store,
		ChunkSize:        chunkSize,
		ChunkOverlap:     chunkOverlap,
		ChunkingStrategy: strategy,
		UserID:           userID,
		AgentID:          1, // 默认 Agent ID
		MaxRetries:       3,
		Concurrency:      4,
		cache:            cache,
	}
}

// SetAgentID 设置 Agent ID
func (i *Ingestor) SetAgentID(agentID uint) {
	i.AgentID = agentID
}

// IngestFile 录入单个文件到知识库
func (i *Ingestor) IngestFile(ctx context.Context, filePath string) error {
	// 读取文件
	doc, err := utils.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// 分割文档
	chunks := i.splitDocument(doc)
	if len(chunks) == 0 {
		return fmt.Errorf("no chunks created from file: %s", filePath)
	}

	// 录入分块
	return i.ingestChunks(ctx, chunks)
}

// IngestDirectory 录入目录中的所有文件到知识库
func (i *Ingestor) IngestDirectory(ctx context.Context, dirPath string) error {
	// 读取目录中的文件
	docs, err := utils.ReadDirectory(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %v", err)
	}

	if len(docs) == 0 {
		return fmt.Errorf("no files found in directory: %s", dirPath)
	}

	// 处理每个文档
	for _, doc := range docs {
		// 分割文档
		chunks := i.splitDocument(doc)
		if len(chunks) == 0 {
			fmt.Printf("No chunks created from file: %s\n", doc.Metadata["source"])
			continue
		}

		// 录入分块
		if err := i.ingestChunks(ctx, chunks); err != nil {
			fmt.Printf("Failed to ingest file %s: %v\n", doc.Metadata["source"], err)
			// 继续处理其他文件
			continue
		}

		fmt.Printf("Successfully ingested file: %s (chunks: %d)\n", doc.Metadata["source"], len(chunks))
	}

	return nil
}

// splitDocument 根据分块策略分割文档
func (i *Ingestor) splitDocument(doc *utils.Document) []*utils.Chunk {
	switch i.ChunkingStrategy {
	case ChunkingStrategySemantic:
		return utils.SplitDocumentBySemantics(doc, i.ChunkSize, i.ChunkOverlap)
	default:
		return utils.SplitDocumentByTokens(doc, i.ChunkSize, i.ChunkOverlap)
	}
}

// ingestChunks 录入文档分块到知识库
func (i *Ingestor) ingestChunks(ctx context.Context, chunks []*utils.Chunk) error {
	// 创建工作池
	var wg sync.WaitGroup
	errCh := make(chan error, len(chunks))
	chunkCh := make(chan *utils.Chunk, len(chunks))

	// 启动工作协程
	for j := 0; j < i.Concurrency; j++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for chunk := range chunkCh {
				if err := i.ingestChunk(ctx, chunk); err != nil {
					errCh <- fmt.Errorf("failed to ingest chunk: %v", err)
				}
			}
		}()
	}

	// 发送分块到工作池
	for _, chunk := range chunks {
		chunkCh <- chunk
	}
	close(chunkCh)

	// 等待所有工作协程完成
	wg.Wait()
	close(errCh)

	// 检查错误
	for err := range errCh {
		return err
	}

	return nil
}

// ingestChunk 录入单个文档分块到知识库
func (i *Ingestor) ingestChunk(ctx context.Context, chunk *utils.Chunk) error {
	// 生成缓存键
	cacheKey := fmt.Sprintf("chunk:%s", chunk.Content)

	// 检查缓存
	if i.cache != nil {
		var cachedResult bool
		found, err := i.cache.Get(cacheKey, &cachedResult)
		if err == nil && found && cachedResult {
			fmt.Printf("Chunk already processed, skipping (cache hit)\n")
			return nil
		}
	}

	// 重试机制
	var lastErr error
	for j := 0; j <= i.MaxRetries; j++ {
		// 嵌入文本
		vec, err := i.store.Embedder(ctx, chunk.Content)
		if err != nil {
			lastErr = fmt.Errorf("failed to embed text: %v", err)
			if j < i.MaxRetries {
				backoff := time.Duration(j+1) * 500 * time.Millisecond
				fmt.Printf("Error embedding text, retrying in %v (%d/%d): %v\n", backoff, j+1, i.MaxRetries, err)
				time.Sleep(backoff)
				continue
			}
			return fmt.Errorf("failed to embed text after %d retries: %v", i.MaxRetries, lastErr)
		}

		// 确保集合存在
		if err := i.store.EnsureCollection(ctx, len(vec)); err != nil {
			lastErr = fmt.Errorf("failed to ensure collection: %v", err)
			if j < i.MaxRetries {
				backoff := time.Duration(j+1) * 500 * time.Millisecond
				fmt.Printf("Error ensuring collection, retrying in %v (%d/%d): %v\n", backoff, j+1, i.MaxRetries, err)
				time.Sleep(backoff)
				continue
			}
			return fmt.Errorf("failed to ensure collection after %d retries: %v", i.MaxRetries, lastErr)
		}

		// 创建元数据
		payload := map[string]any{
			"agent_id": i.AgentID, // 知识库只需要 agent_id，不需要 user_id
			"text":     chunk.Content,
			"type":     "knowledge", // 标记为知识类型
			"ts":       time.Now().Format(time.RFC3339),
		}

		// 添加分块元数据
		for k, v := range chunk.Metadata {
			payload[k] = v
		}

		// 存储向量
		pointID := randomID()
		body := map[string]any{
			"points": []any{
				map[string]any{
					"id":     pointID,
					"vector": vec,
					"payload": payload,
				},
			},
		}

		// 调用 Qdrant API 存储向量
		if err = i.store.UpsertPointToCollection(ctx, body, i.store.Collection); err != nil {
			lastErr = fmt.Errorf("failed to upsert point: %v", err)
			if j < i.MaxRetries {
				backoff := time.Duration(j+1) * 500 * time.Millisecond
				fmt.Printf("Error upserting point, retrying in %v (%d/%d): %v\n", backoff, j+1, i.MaxRetries, err)
				time.Sleep(backoff)
				continue
			}
			return fmt.Errorf("failed to upsert point after %d retries: %v", i.MaxRetries, lastErr)
		}

		// 缓存结果
		if i.cache != nil {
			if err := i.cache.Set(cacheKey, true); err != nil {
				fmt.Printf("Warning: failed to cache result: %v\n", err)
			}
		}

		return nil
	}

	return lastErr
}