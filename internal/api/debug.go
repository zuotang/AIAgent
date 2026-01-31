package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"agent-langchain/internal/config"
)

// DebugService 调试服务接口
type DebugService interface {
	GetConfig(ctx context.Context) (config.Config, error)
	GetLogs(ctx context.Context, limit int) ([]string, error)
	TestConnections(ctx context.Context) (map[string]bool, error)
	HandleGetConfig(c echo.Context) error
	HandleGetLogs(c echo.Context) error
	HandleTestConnections(c echo.Context) error
	HandleRequestDebug(c echo.Context) error
}

// DebugServiceImpl 调试服务实现
type DebugServiceImpl struct {
	config *config.Config
}

// NewDebugService 创建调试服务实例
func NewDebugService(config *config.Config) DebugService {
	return &DebugServiceImpl{
		config: config,
	}
}

// GetConfig 实现获取配置功能
func (s *DebugServiceImpl) GetConfig(ctx context.Context) (config.Config, error) {
	return *s.config, nil
}

// GetLogs 实现获取日志功能
func (s *DebugServiceImpl) GetLogs(ctx context.Context, limit int) ([]string, error) {
	// 这里实现获取日志逻辑
	// 临时实现
	return []string{
		"[INFO] Server started",
		"[INFO] Database connected",
		"[INFO] Qdrant connected",
		"[INFO] Cache initialized",
		"[INFO] API routes registered",
	}, nil
}

// TestConnections 实现连接测试功能
func (s *DebugServiceImpl) TestConnections(ctx context.Context) (map[string]bool, error) {
	// 这里实现连接测试逻辑
	// 临时实现
	return map[string]bool{
		"database": true,
		"qdrant":   true,
		"ollama":   true,
		"cache":    true,
	}, nil
}

// HandleGetConfig 处理获取配置请求
func (s *DebugServiceImpl) HandleGetConfig(c echo.Context) error {
	// 处理请求
	cfg, err := s.GetConfig(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, cfg)
}

// HandleGetLogs 处理获取日志请求
func (s *DebugServiceImpl) HandleGetLogs(c echo.Context) error {
	// 获取limit参数
	limit := 10 // 默认值

	// 处理请求
	logs, err := s.GetLogs(c.Request().Context(), limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"status": "success", "logs": logs})
}

// HandleTestConnections 处理连接测试请求
func (s *DebugServiceImpl) HandleTestConnections(c echo.Context) error {
	// 处理请求
	connections, err := s.TestConnections(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"status": "success", "connections": connections})
}

// HandleRequestDebug 处理请求调试功能
func (s *DebugServiceImpl) HandleRequestDebug(c echo.Context) error {
	// 获取请求信息
	req := c.Request()
	requestInfo := map[string]interface{}{
		"method":        req.Method,
		"url":           req.URL.String(),
		"headers":       req.Header,
		"host":          req.Host,
		"remote_addr":   req.RemoteAddr,
		"content_length": req.ContentLength,
		"content_type":  req.Header.Get("Content-Type"),
	}

	// 获取查询参数
	queryParams := make(map[string][]string)
	for key, values := range req.URL.Query() {
		queryParams[key] = values
	}
	requestInfo["query_params"] = queryParams

	// 获取路径参数
	pathParams := make(map[string]string)
	for key, value := range c.ParamValues() {
		pathParams[fmt.Sprintf("param_%d", key)] = value
	}
	requestInfo["path_params"] = pathParams

	return c.JSON(http.StatusOK, map[string]interface{}{"status": "success", "request": requestInfo})
}