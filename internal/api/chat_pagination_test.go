package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agent-langchain/internal/agent"
	"agent-langchain/internal/config"
	"agent-langchain/internal/memory"
	"agent-langchain/internal/rag"

	"github.com/labstack/echo/v4"
)

// MockOrchestrator 用于测试的模拟编排器
type MockOrchestrator struct {
	messages []memory.ChatMessage
	total    int64
}

func (m *MockOrchestrator) GetChatHistoryWithCursor(ctx context.Context, userID string, agentID uint, beforeID uint, limit int) ([]memory.ChatMessage, error) {
	// 模拟返回消息
	result := []memory.ChatMessage{}
	for _, msg := range m.messages {
		if beforeID == 0 || msg.ID < beforeID {
			result = append(result, msg)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockOrchestrator) GetChatHistoryCount(ctx context.Context, userID string, agentID uint) (int64, error) {
	return m.total, nil
}

// 实现其他必需的接口方法（空实现）
func (m *MockOrchestrator) ProcessMessage(ctx context.Context, userID string, agentID uint, userText string, conversationHistory string, systemPrompt string) (agent.Output, error) {
	return agent.Output{}, nil
}
func (m *MockOrchestrator) ProcessMessageStream(ctx context.Context, userID string, agentID uint, userText string, conversationHistory string, systemPrompt string, callback func(string) error) (agent.Output, error) {
	return agent.Output{}, nil
}
func (m *MockOrchestrator) GetConfig() *config.Config { return nil }
func (m *MockOrchestrator) GetStore() *memory.Store   { return nil }
func (m *MockOrchestrator) GetVectorStore() *rag.QdrantStore { return nil }
func (m *MockOrchestrator) GetChatHistory(ctx context.Context, userID string, agentID uint, limit, offset int) ([]memory.ChatMessage, error) {
	return nil, nil
}
func (m *MockOrchestrator) GetChatHistoryAfterID(ctx context.Context, userID string, agentID uint, afterID uint, limit int) ([]memory.ChatMessage, error) {
	return nil, nil
}
func (m *MockOrchestrator) GetChatSessions(ctx context.Context, userID string, agentID uint) ([]memory.ChatSession, error) {
	return nil, nil
}

func TestGetChatHistoryPagination(t *testing.T) {
	// 创建测试数据
	mockMessages := []memory.ChatMessage{
		{ID: 10, UserID: "test_user", AgentID: 1, Role: "user", Content: "Message 10"},
		{ID: 9, UserID: "test_user", AgentID: 1, Role: "assistant", Content: "Message 9"},
		{ID: 8, UserID: "test_user", AgentID: 1, Role: "user", Content: "Message 8"},
		{ID: 7, UserID: "test_user", AgentID: 1, Role: "assistant", Content: "Message 7"},
		{ID: 6, UserID: "test_user", AgentID: 1, Role: "user", Content: "Message 6"},
	}

	mockOrch := &MockOrchestrator{
		messages: mockMessages,
		total:    int64(len(mockMessages)),
	}

	chatService := &ChatService{
		orch: mockOrch,
	}

	// 测试第一页
	t.Run("First Page", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/chat/history?user_id=test_user&agent_id=1&limit=3", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := chatService.GetChatHistory(c); err != nil {
			t.Fatalf("GetChatHistory failed: %v", err)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var response ChatHistoryResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// 验证返回了3条消息
		if len(response.Messages) != 3 {
			t.Errorf("Expected 3 messages, got %d", len(response.Messages))
		}

		// 验证分页信息
		if response.Pagination.Total != 5 {
			t.Errorf("Expected total 5, got %d", response.Pagination.Total)
		}

		if !response.Pagination.HasMore {
			t.Error("Expected has_more to be true")
		}

		if response.Pagination.NextCursor == nil {
			t.Error("Expected next_cursor to be set")
		} else if *response.Pagination.NextCursor != 8 {
			t.Errorf("Expected next_cursor to be 8, got %d", *response.Pagination.NextCursor)
		}
	})

	// 测试第二页
	t.Run("Second Page", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/chat/history?user_id=test_user&agent_id=1&limit=3&before_id=8", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := chatService.GetChatHistory(c); err != nil {
			t.Fatalf("GetChatHistory failed: %v", err)
		}

		var response ChatHistoryResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// 验证返回了2条消息（剩余的消息）
		if len(response.Messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(response.Messages))
		}

		// 验证没有更多数据
		if response.Pagination.HasMore {
			t.Error("Expected has_more to be false")
		}

		if response.Pagination.NextCursor != nil {
			t.Error("Expected next_cursor to be nil")
		}
	})
}
