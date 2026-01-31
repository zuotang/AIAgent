package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"agent-langchain/internal/rag"
	"agent-langchain/internal/utils"
)

// IngestRequest 录入请求结构
type IngestRequest struct {
	UserID       string `json:"user_id" validate:"required"`
	Path         string `json:"path" validate:"required"`
	ChunkSize    int    `json:"chunk_size"`
	ChunkOverlap int    `json:"chunk_overlap"`
}

// IngestTextRequest 文本录入请求结构
type IngestTextRequest struct {
	UserID       string `json:"user_id" validate:"required"`
	Text         string `json:"text" validate:"required"`
	FileName     string `json:"file_name"`
	ChunkSize    int    `json:"chunk_size"`
	ChunkOverlap int    `json:"chunk_overlap"`
}

// QueryRequest 查询请求结构
type QueryRequest struct {
	UserID      string  `json:"user_id" validate:"required"`
	Query       string  `json:"query" validate:"required"`
	Limit       int     `json:"limit"`
	ScoreThreshold float64 `json:"score_threshold"`
}

// KnowledgeService 知识库服务接口
type KnowledgeService interface {
	IngestFile(ctx context.Context, req IngestRequest) error
	IngestDirectory(ctx context.Context, req IngestRequest) error
	IngestText(ctx context.Context, req IngestTextRequest) error
	Query(ctx context.Context, req QueryRequest) ([]rag.Doc, error)
	List(ctx context.Context, userID string) ([]string, error)
	HandleIngestFile(c echo.Context) error
	HandleIngestDirectory(c echo.Context) error
	HandleIngestText(c echo.Context) error
	HandleKnowledgeQuery(c echo.Context) error
	HandleKnowledgeList(c echo.Context) error
}

// KnowledgeServiceImpl 知识库服务实现
type KnowledgeServiceImpl struct {
	ingestor *rag.Ingestor
	store    *rag.QdrantStore
}

// NewKnowledgeService 创建知识库服务实例
func NewKnowledgeService(ingestor *rag.Ingestor, store *rag.QdrantStore) KnowledgeService {
	return &KnowledgeServiceImpl{
		ingestor: ingestor,
		store:    store,
	}
}

// IngestFile 实现文件录入功能
func (s *KnowledgeServiceImpl) IngestFile(ctx context.Context, req IngestRequest) error {
	// 创建临时录入器，使用请求中的 UserID
	tempIngestor := rag.NewIngestor(s.store, req.ChunkSize, req.ChunkOverlap, rag.ChunkingStrategyTokens, req.UserID)
	return tempIngestor.IngestFile(ctx, req.Path)
}

// IngestDirectory 实现目录录入功能
func (s *KnowledgeServiceImpl) IngestDirectory(ctx context.Context, req IngestRequest) error {
	// 创建临时录入器，使用请求中的 UserID
	tempIngestor := rag.NewIngestor(s.store, req.ChunkSize, req.ChunkOverlap, rag.ChunkingStrategyTokens, req.UserID)
	return tempIngestor.IngestDirectory(ctx, req.Path)
}

// IngestText 实现文本录入功能
func (s *KnowledgeServiceImpl) IngestText(ctx context.Context, req IngestTextRequest) error {
	// 设置默认值
	if req.ChunkSize == 0 {
		req.ChunkSize = 1000
	}
	if req.ChunkOverlap == 0 {
		req.ChunkOverlap = 200
	}

	// 分割文本
	chunks := utils.SplitText(req.Text, req.ChunkSize, req.ChunkOverlap)
	if len(chunks) == 0 {
		return fmt.Errorf("no chunks created from text")
	}

	// 录入文本分块
	return s.store.UpsertTexts(ctx, req.UserID, chunks, req.FileName)
}

// Query 实现知识库查询功能
func (s *KnowledgeServiceImpl) Query(ctx context.Context, req QueryRequest) ([]rag.Doc, error) {
	// 设置默认值
	if req.Limit == 0 {
		req.Limit = 5
	}

	return s.store.SimilaritySearch(ctx, req.UserID, req.Query, req.Limit)
}

// List 实现知识库列表功能
func (s *KnowledgeServiceImpl) List(ctx context.Context, userID string) ([]string, error) {
	// 调用存储的 ListFiles 方法获取文件列表
	return s.store.ListFiles(ctx, userID)
}

// HandleIngestFile 处理文件录入请求
func (s *KnowledgeServiceImpl) HandleIngestFile(c echo.Context) error {
	// 解析请求
	var req IngestRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// 设置默认值
	if req.ChunkSize == 0 {
		req.ChunkSize = 1000
	}
	if req.ChunkOverlap == 0 {
		req.ChunkOverlap = 200
	}

	// 处理请求
	if err := s.IngestFile(c.Request().Context(), req); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "success", "message": "File ingested successfully"})
}

// HandleIngestDirectory 处理目录录入请求
func (s *KnowledgeServiceImpl) HandleIngestDirectory(c echo.Context) error {
	// 解析请求
	var req IngestRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// 设置默认值
	if req.ChunkSize == 0 {
		req.ChunkSize = 1000
	}
	if req.ChunkOverlap == 0 {
		req.ChunkOverlap = 200
	}

	// 处理请求
	if err := s.IngestDirectory(c.Request().Context(), req); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "success", "message": "Directory ingested successfully"})
}

// HandleIngestText 处理文本录入请求
func (s *KnowledgeServiceImpl) HandleIngestText(c echo.Context) error {
	// 解析请求
	var req IngestTextRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// 设置默认值
	if req.ChunkSize == 0 {
		req.ChunkSize = 1000
	}
	if req.ChunkOverlap == 0 {
		req.ChunkOverlap = 200
	}

	// 处理请求
	if err := s.IngestText(c.Request().Context(), req); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "success", "message": "Text ingested successfully"})
}

// HandleKnowledgeQuery 处理知识库查询请求
func (s *KnowledgeServiceImpl) HandleKnowledgeQuery(c echo.Context) error {
	// 解析请求
	var req QueryRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// 处理请求
	results, err := s.Query(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// HandleKnowledgeList 处理知识库列表请求
func (s *KnowledgeServiceImpl) HandleKnowledgeList(c echo.Context) error {
	// 获取用户ID
	userID := c.QueryParam("user_id")
	if userID == "" {
		userID = "default"
	}

	// 处理请求
	files, err := s.List(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"status": "success", "files": files})
}