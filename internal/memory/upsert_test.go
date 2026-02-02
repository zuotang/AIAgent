package memory

import (
	"context"
	"testing"
)

func TestUpsertExtractedMemories_NoDuplicateError(t *testing.T) {
	// 创建临时数据库
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	userID := "test_user"
	agentID := uint(1)

	// 第一次插入记忆
	memories1 := []ExtractedMemory{
		{
			Type:       "preference",
			Key:        "favorite_color",
			Value:      "blue",
			Confidence: 0.9,
			Owner:      "user",
		},
		{
			Type:       "identity",
			Key:        "name",
			Value:      "Alice",
			Confidence: 0.95,
			Owner:      "user",
		},
	}

	err = store.UpsertExtractedMemories(ctx, userID, agentID, memories1)
	if err != nil {
		t.Fatalf("First upsert failed: %v", err)
	}

	// 验证记忆已保存
	count1, err := store.GetMemoryCount(ctx, userID, agentID)
	if err != nil {
		t.Fatalf("Failed to get memory count: %v", err)
	}
	if count1 != 2 {
		t.Errorf("Expected 2 memories, got %d", count1)
	}

	// 第二次插入相同的记忆（应该更新而不是报错）
	memories2 := []ExtractedMemory{
		{
			Type:       "preference",
			Key:        "favorite_color",
			Value:      "red", // 更新值
			Confidence: 0.85,
			Owner:      "user",
		},
		{
			Type:       "identity",
			Key:        "name",
			Value:      "Alice Smith", // 更新值
			Confidence: 0.98,
			Owner:      "user",
		},
	}

	// 这里不应该报 UNIQUE 约束错误
	err = store.UpsertExtractedMemories(ctx, userID, agentID, memories2)
	if err != nil {
		t.Fatalf("Second upsert failed (should update, not error): %v", err)
	}

	// 验证记忆数量没有增加（应该是更新而不是插入）
	count2, err := store.GetMemoryCount(ctx, userID, agentID)
	if err != nil {
		t.Fatalf("Failed to get memory count: %v", err)
	}
	if count2 != 2 {
		t.Errorf("Expected 2 memories (updated), got %d", count2)
	}

	// 验证值已更新
	memories, err := store.GetMemoriesByType(ctx, userID, agentID, "preference")
	if err != nil {
		t.Fatalf("Failed to get memories: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("Expected 1 preference memory, got %d", len(memories))
	}
	if memories[0].Value != "red" {
		t.Errorf("Expected value 'red', got '%s'", memories[0].Value)
	}
	if memories[0].Confidence != 0.85 {
		t.Errorf("Expected confidence 0.85, got %f", memories[0].Confidence)
	}

	// 第三次插入：混合新记忆和已存在的记忆
	memories3 := []ExtractedMemory{
		{
			Type:       "preference",
			Key:        "favorite_color",
			Value:      "green", // 再次更新
			Confidence: 0.92,
			Owner:      "user",
		},
		{
			Type:       "preference",
			Key:        "favorite_food",
			Value:      "pizza", // 新记忆
			Confidence: 0.88,
			Owner:      "user",
		},
	}

	err = store.UpsertExtractedMemories(ctx, userID, agentID, memories3)
	if err != nil {
		t.Fatalf("Third upsert failed: %v", err)
	}

	// 验证记忆数量增加了1（1个更新 + 1个新增）
	count3, err := store.GetMemoryCount(ctx, userID, agentID)
	if err != nil {
		t.Fatalf("Failed to get memory count: %v", err)
	}
	if count3 != 3 {
		t.Errorf("Expected 3 memories (2 original + 1 new), got %d", count3)
	}
}

func TestUpsertExtractedMemories_DifferentOwners(t *testing.T) {
	// 创建临时数据库
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	userID := "test_user"
	agentID := uint(1)

	// 插入用户记忆
	userMemories := []ExtractedMemory{
		{
			Type:       "preference",
			Key:        "favorite_color",
			Value:      "blue",
			Confidence: 0.9,
			Owner:      "user",
		},
	}

	err = store.UpsertExtractedMemories(ctx, userID, agentID, userMemories)
	if err != nil {
		t.Fatalf("User memory upsert failed: %v", err)
	}

	// 插入助手记忆（相同的 type 和 key，但 owner 不同）
	agentMemories := []ExtractedMemory{
		{
			Type:       "preference",
			Key:        "favorite_color",
			Value:      "red",
			Confidence: 0.85,
			Owner:      "agent",
		},
	}

	err = store.UpsertExtractedMemories(ctx, userID, agentID, agentMemories)
	if err != nil {
		t.Fatalf("Agent memory upsert failed: %v", err)
	}

	// 验证两条记忆都存在（因为 owner 不同）
	count, err := store.GetMemoryCount(ctx, userID, agentID)
	if err != nil {
		t.Fatalf("Failed to get memory count: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 memories (different owners), got %d", count)
	}
}
